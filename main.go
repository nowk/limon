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
	"github.com/cloudfoundry/gosigar"
)

const (
	KILO = 1024
	MEGA = 1048576
	GIGA = 1073741824
)

// kb returns the value in kilobytes
func kb(val uint64) uint64 {
	return val / KILO
}

// mb returns the value in megabytes
func mb(val uint64) uint64 {
	return val / MEGA
}

// gb returns the value in gigabytes
func gb(val uint64) uint64 {
	return val / GIGA
}

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

	namespace string

	period uint64
	grace  uint64

	log_level string

	// TODO memory_units string
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

	creds := credentials.NewStaticCredentials(aws_access_key_id, aws_secret_access_key, "")
	_, err := creds.Get()
	check(err, "credentials", true)

	cfg := aws.NewConfig().WithRegion(aws_region).WithCredentials(creds)
	cw := cloudwatch.New(session.New(), cfg)

	// dimensions for our metrics
	dims := dimsOnly(
		newDim("InstanceId", instance_id),
		newDim("AutoScalingGroupName", autoscaling_group_name),
		newDim("InstanceType", instance_type),
		newDim("ImageId", image_id),
	)

	log.WithField("namespace", namespace).Info("start")

	mem := sigar.Mem{}
	tic := time.NewTicker(time.Duration(period) * time.Second)

	mm := &MemoryMetric{
		cw:        cw,
		namespace: namespace,
		dims:      dims,
	}

	var (
		memUtil uint64

		i uint64 // retry count
	)
	for _ = range tic.C {
		check(mem.Get(), "memGet", true)

		// calculate memory utilization
		if mem.Total > 0 {
			memUtil = 100 * mem.Used / mem.Total
		}

		err := mm.Put(
			float64(memUtil),
			float64(mem.Used),
			float64(mem.Free),
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

type MemoryMetric struct {
	cw *cloudwatch.CloudWatch

	namespace string

	dims []*cloudwatch.Dimension
}

func (m *MemoryMetric) Put(util, used, free float64) (err error) {
	defer log.Trace("put").Stop(&err)

	now := time.Now()
	_, err = m.cw.PutMetricData(newInput(m.namespace,
		newMetric("MemoryUtilization", now, "Percent", util, m.dims...),
		newMetric("MemoryUsed", now, "Bytes", used, m.dims...),
		newMetric("MemoryAvailable", now, "Bytes", free, m.dims...),
	))

	return
}

// newMetric returns a new cloudwatch.MetricDatum
func newMetric(
	name string, t time.Time, unit string, value float64, dims ...*cloudwatch.Dimension) *cloudwatch.MetricDatum {

	log.WithFields(log.Fields{
		"name":      name,
		"timestamp": t,
		"unit":      unit,
		"value":     value,
	}).Info("metric")

	return (&cloudwatch.MetricDatum{}).
		SetMetricName(name).
		SetTimestamp(t).
		SetUnit(unit).
		SetValue(value).
		SetDimensions(dims)
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

// newInput returns a new cloudwatch.PutMetricDataInput
func newInput(
	ns string, datum ...*cloudwatch.MetricDatum) *cloudwatch.PutMetricDataInput {

	return (&cloudwatch.PutMetricDataInput{}).
		SetNamespace(ns).
		SetMetricData(datum)

}
