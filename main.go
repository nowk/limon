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
	"github.com/nowk/limon/metrics/dimensions"
	"github.com/nowk/limon/metrics/memory"
	"github.com/nowk/limon/utils"
)

// check outputs an error to stderr and exits the process
func check(err error, label string) {
	if err == nil {
		return
	}

	log.WithError(err).Fatal(label)
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

	// parse memory unit early
	unit, err := utils.ParseMemoryUnit(memory_unit)
	check(err, "memory unit")

	// aws session and cloudwatch
	creds := credentials.NewStaticCredentials(aws_access_key_id, aws_secret_access_key, "")
	_, err = creds.Get()
	check(err, "credentials")

	cfg := aws.NewConfig().WithRegion(aws_region).WithCredentials(creds)
	cw := cloudwatch.New(session.New(), cfg)

	// start!
	log.WithField("namespace", namespace).Info("start")

	// dimensions for our metrics
	dims := dimensions.FromSet(
		[][]string{
			{"InstanceId", instance_id},
			{"AutoScalingGroupName", autoscaling_group_name},
			{"InstanceType", instance_type},
			{"ImageId", image_id},
		},
	)
	var (
		memoryMetric = memory.New(cw, namespace, unit, dims...)
		mem          = mem.New()
		tic          = time.NewTicker(time.Duration(period) * time.Second)

		i uint64 // grace count
	)
	for _ = range tic.C {
		check(mem.Get(), "memGet")

		err := memoryMetric.Put(
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
			check(errors.New("exceeded grace count"), "put")
		}
	}
}
