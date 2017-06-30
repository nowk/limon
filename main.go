package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/cloudfoundry/gosigar"
	"github.com/nowk/nflag"
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
func check(err error, exit bool) {
	if err == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "[error] %s", err)

	if exit {
		os.Exit(1)
	}
}

var (
	aws_access_key_id     string
	aws_secret_access_key string
	aws_region            string
	// TODO aws_iam_role string

	instance_id            string
	autoscaling_group_name string
	instance_type          string
	image_id               string

	namespace string

	period uint64

	// TODO verbose bool
	// TODO memory_units string
)

func init() {
	nflag.StringVar(&aws_access_key_id, "AWS_ACCESS_KEY_ID", "aws-access-key-id", "", "Specifies the AWS access key ID to use to identify the caller.")
	nflag.StringVar(&aws_secret_access_key, "AWS_SECRET_ACCESS_KEY", "aws-secret-access-key", "", "Specifies the AWS secret key to use to sign the request.")
	nflag.StringVar(&aws_region, "AWS_REGION", "aws-region", "us-east-1", "Specifies the AWS region.")

	nflag.StringVar(&instance_id, "INSTANCE_ID", "instance-id", "", "Specifies the InstanceId")
	nflag.StringVar(&autoscaling_group_name, "AUTOSCALING_GROUP_NAME", "autoscaling-group-name", "", "Specifies the AutoScalingGroupName")
	nflag.StringVar(&instance_type, "INSTANCE_TYPE", "instance-type", "", "Specifies the InstanceType")
	nflag.StringVar(&image_id, "IMAGE_ID", "image-id", "", "Specifies the ImageId")

	nflag.StringVar(&namespace, "NAMESPACE", "namespace", "System/Linux", "The namespace of the metric")

	nflag.Uint64Var(&period, "PERIOD", "period", 5, "The period in seconds which specifies when a metric measurement is take.")

	nflag.Parse()
}

func main() {
	var (
		creds  = credentials.NewStaticCredentials(aws_access_key_id, aws_secret_access_key, "")
		_, err = creds.Get()
	)
	check(err, true)

	var (
		cfg = aws.NewConfig().WithRegion(aws_region).WithCredentials(creds)
		cw  = cloudwatch.New(session.New(), cfg)
	)

	// dimensions for our metrics
	dims := dimsOnly(
		newDim("InstanceId", instance_id),
		newDim("AutoScalingGroupName", autoscaling_group_name),
		newDim("InstanceType", instance_type),
		newDim("ImageId", image_id),
	)

	var (
		mem  = sigar.Mem{}
		poll = time.NewTicker(time.Duration(period) * time.Second)

		memUtil uint64
	)

	fmt.Fprintf(os.Stdout, "%-20s %-19s   %-20s %s\n", "MetricName", "Timestamp", "Unit", "Value")

	for _ = range poll.C {
		check(mem.Get(), true)

		// calculate memory utilization
		if mem.Total > 0 {
			memUtil = 100 * mem.Used / mem.Total
		}

		input := newInput(namespace,
			newMetric("MemoryUtilization", time.Now(), "Percent", float64(memUtil), dims...),
			newMetric("MemoryUsed", time.Now(), "Bytes", float64(mem.Used), dims...),
			newMetric("MemoryAvailable", time.Now(), "Bytes", float64(mem.Free), dims...),
		)

		_, err := cw.PutMetricData(input)
		check(err, true) // TODO how to handle error putting metric data, should we just kill the process?
	}
}

// newMetric returns a new cloudwatch.MetricDatum
func newMetric(
	name string, t time.Time, unit string, value float64, dims ...*cloudwatch.Dimension) *cloudwatch.MetricDatum {

	fmt.Fprintf(os.Stdout, "%-20s %d   %-20s %0.3f\n", name, t.UnixNano(), unit, value)

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

// newDim returns a new cloudwatch.Dimension. empty values will return nil
func newDim(name, value string) *cloudwatch.Dimension {
	if value == "" {
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
