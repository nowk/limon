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
		cfg = aws.NewConfig().WithRegion("us-east-1").WithCredentials(creds)
		cw  = cloudwatch.New(session.New(), cfg)
	)

	// dimensions for our metrics
	dim := []*cloudwatch.Dimension{
		newDim("InstanceId", instance_id),
		newDim("AutoScalingGroupName", autoscaling_group_name),
		newDim("InstanceType", instance_type),
		newDim("ImageId", image_id),
	}

	var (
		mem  = sigar.Mem{}
		poll = time.NewTicker(time.Duration(period) * time.Second)

		memUtil uint64
	)

	for {
		select {
		case _ = <-poll.C:
			check(mem.Get(), true)

			// calculate memory utilization
			if mem.Total > 0 {
				memUtil = 100 * mem.Used / mem.Total
			}

			input := newInput(namespace,
				newMetric("MemoryUtilization", time.Now(), "Percent", float64(memUtil), dim...),
				newMetric("MemoryUsed", time.Now(), "Bytes", float64(mem.Used), dim...),
				newMetric("MemoryAvailable", time.Now(), "Bytes", float64(mem.Free), dim...),
			)

			_, err := cw.PutMetricData(input)
			check(err, true) // TODO how to handle error putting metric data, should we just kill the process?

			// TODO output should mirror metrics
			fmt.Fprintf(os.Stdout, "mem:  %d%% %10d %10d %10d\n", memUtil, mb(mem.Total), mb(mem.Used), mb(mem.Free))
		}
	}
}

// newMetric returns a new cloudwatch.MetricDatum
func newMetric(
	name string, t time.Time, unit string, value float64, dim ...*cloudwatch.Dimension) *cloudwatch.MetricDatum {
	return (&cloudwatch.MetricDatum{}).
		SetMetricName(name).
		SetTimestamp(t).
		SetUnit(unit).
		SetValue(value).
		SetDimensions(dim)
}

// newDim returns a new cloudwatch.Dimension
func newDim(name, value string) *cloudwatch.Dimension {
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
