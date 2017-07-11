package main

import (
	"os"
	"time"

	"github.com/apex/log"
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
func check(err error, label string, exit bool) {
	if err == nil {
		return
	}

	log.WithError(err).Error(label)

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
	check(err, "credentials", true)

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

	log.WithField("namespace", namespace).Info("start")

	mem := sigar.Mem{}
	tic := time.NewTicker(time.Duration(period) * time.Second)

	mm := &MemoryMetric{
		cw:        cw,
		namespace: namespace,
		dims:      dims,
	}

	var memUtil uint64

	for _ = range tic.C {
		check(mem.Get(), "memGet", true)

		// calculate memory utilization
		if mem.Total > 0 {
			memUtil = 100 * mem.Used / mem.Total
		}

		_ = mm.Put(
			float64(memUtil),
			float64(mem.Used),
			float64(mem.Free),
		)
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
