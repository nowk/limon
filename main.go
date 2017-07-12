package main

import (
	"errors"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/apex/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/nowk/limon/mem"
	metricsmemory "github.com/nowk/limon/metrics/memory"
)

// check outputs an error to stderr and exits the process if true
func check(err error, label string, exit bool) {
	if err == nil {
		return
	}

	l := log.WithError(err)
	if exit {
		l.Fatal(label)
	} else {
		l.Error(label)
	}
}

var (
	aws_access_key_id     string
	aws_secret_access_key string
	aws_region            string

	instance_id            string
	autoscaling_group_name string
	instance_type          string
	image_id               string

	namespace   string
	memory_unit string

	period uint64
	grace  uint64

	log_level string
)

func init() {
	kingpin.Flag("aws-access-key-id", "AWS Access Key ID").
		OverrideDefaultFromEnvar("AWS_ACCESS_KEY_ID").
		Required().
		StringVar(&aws_access_key_id)

	kingpin.Flag("aws-secret-access-key", "AWS Secret Key").
		OverrideDefaultFromEnvar("AWS_SECRET_ACCESS_KEY").
		Required().
		StringVar(&aws_secret_access_key)

	kingpin.Flag("aws-region", "AWS Region").
		OverrideDefaultFromEnvar("AWS_REGION").
		Default("us-east-1").
		StringVar(&aws_region)

	kingpin.Flag("instance-id", "EC2 Instance ID").
		OverrideDefaultFromEnvar("INSTANCE_ID").
		StringVar(&instance_id)

	kingpin.Flag("autoscaling-group-name", "AutoScaling Group Name").
		OverrideDefaultFromEnvar("AUTOSCALING_GROUP_NAME").
		StringVar(&autoscaling_group_name)

	kingpin.Flag("instance-type", "EC2 Instance Type").
		OverrideDefaultFromEnvar("INSTANCE_TYPE").
		StringVar(&instance_type)

	kingpin.Flag("image-id", "EC2 Image ID").
		OverrideDefaultFromEnvar("IMAGE_ID").
		StringVar(&image_id)

	kingpin.Flag("namespace", "Metric Namespace").
		Default("System/Linux").
		StringVar(&namespace)

	kingpin.Flag("unit", "Unit of measurement for raw memory metrics").
		Default("bytes").
		HintOptions("bytes", "kilobytes", "megabytes", "gigabytes").
		StringVar(&memory_unit)

	kingpin.Flag("period", "Period (in seconds) to take metric measurement").
		Short('p').
		Default("5").
		Uint64Var(&period)

	kingpin.Flag("grace", "Number of consecutive put errors allowed before a forced exit").
		Short('g').
		Default("3").
		Uint64Var(&grace)

	kingpin.Flag("level", "Log Level").
		Short('l').
		Default("info").
		HintOptions("debug", "info", "warn", "error", "fatal").
		StringVar(&log_level)
}

func main() {
	kingpin.Parse()
	log.SetLevelFromString(log_level)

	unit, err := parseMemoryUnit(memory_unit)
	check(err, "memory unit", true)

	creds := credentials.NewStaticCredentials(aws_access_key_id, aws_secret_access_key, "")
	_, err = creds.Get()
	check(err, "credentials", true)

	cfg := aws.NewConfig().WithRegion(aws_region).WithCredentials(creds)
	cw := cloudwatch.New(session.New(), cfg)

	log.WithField("namespace", namespace).Info("start")

	// dimensions for our metrics
	dims := dimsOnly(
		newDim("InstanceId", instance_id),
		newDim("AutoScalingGroupName", autoscaling_group_name),
		newDim("InstanceType", instance_type),
		newDim("ImageId", image_id),
	)

	metric := metricsmemory.New(cw, namespace, unit, dims...)

	mem := mem.New()
	tic := time.NewTicker(time.Duration(period) * time.Second)

	var i uint64 // grace count

	for _ = range tic.C {
		check(mem.Get(), "memGet", true)

		err := metric.Put(
			mem.Util,
			mem.Used,
			mem.Free,
		)
		if err != nil {
			i++
		} else {
			i = 0 // it's ok, reset the count
		}
		if i > grace {
			check(errors.New("exceeded grace count"), "put", true)
		}
	}
}

// parseMemoryUnit returns capitalized version of acceptable unites, or returns
// error
func parseMemoryUnit(unit string) (string, error) {
	var out string
	switch unit {
	case "bytes":
		out = "Bytes"

	case "megabytes":
		out = "Megabytes"

	case "gigabytes":
		out = "Gigabytes"

	default:
		return "", errors.New("invalid memory unit")
	}

	return out, nil
}

// dimsOnly pops out nil and returns only valid dimensions
func dimsOnly(in ...*cloudwatch.Dimension) []*cloudwatch.Dimension {
	var out []*cloudwatch.Dimension

	for _, v := range in {
		if v == nil {
			continue
		}

		out = append(out, v)
	}

	return out
}

// newDim returns a new cloudwatch.Dimension. empty name or values will return
// nil
func newDim(name, value string) *cloudwatch.Dimension {
	if name == "" || value == "" {
		return nil
	}

	return (&cloudwatch.Dimension{}).
		SetName(name).
		SetValue(value)
}
