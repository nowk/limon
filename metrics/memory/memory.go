package memory

import (
	"time"

	"github.com/apex/log"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
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

// Metric provides a cloudwatch metric object bound to a namespace, unit and
// dimensions
type Metric struct {
	cw *cloudwatch.CloudWatch

	namespace  string
	memoryUnit string

	dims []*cloudwatch.Dimension
}

func New(
	cw *cloudwatch.CloudWatch, namespace, unit string, dims ...*cloudwatch.Dimension) *Metric {

	return &Metric{
		cw:         cw,
		namespace:  namespace,
		memoryUnit: unit,
		dims:       dims,
	}
}

// Put sends the metric data to cloudwatch in the context of the given Metric
func (m *Metric) Put(util, used, free uint64) (err error) {
	defer log.Trace("put").Stop(&err)

	now := time.Now()
	_, err = m.cw.PutMetricData(newInput(m.namespace,
		newMetric("MemoryUtilization", now, "Percent", util, m.dims...),
		newMetric("MemoryUsed", now, m.memoryUnit, used, m.dims...),
		newMetric("MemoryAvailable", now, m.memoryUnit, free, m.dims...),
	))

	return
}

// convertValueByUnit converts the value by supported unit. Bytes and Percent
// unit types will be returned unconverted
func convertValueByUnit(unit string, value uint64) float64 {
	var v uint64
	switch unit {
	case "Bytes", "Percent":
		v = value

	case "Kilobytes":
		v = kb(value)

	case "Megabytes":
		v = mb(value)

	case "Gigabytes":
		v = gb(value)

	}

	return float64(v)
}

// newMetric returns a new cloudwatch.MetricDatum
func newMetric(
	name string, t time.Time, unit string, value uint64, dims ...*cloudwatch.Dimension) *cloudwatch.MetricDatum {

	unitValue := convertValueByUnit(unit, value)

	log.WithFields(log.Fields{
		"name":      name,
		"timestamp": t,
		"unit":      unit,
		"value":     unitValue,
	}).Info("metric")

	return (&cloudwatch.MetricDatum{}).
		SetMetricName(name).
		SetTimestamp(t).
		SetUnit(unit).
		SetValue(unitValue).
		SetDimensions(dims)
}

// newInput returns a new cloudwatch.PutMetricDataInput
func newInput(
	ns string, datum ...*cloudwatch.MetricDatum) *cloudwatch.PutMetricDataInput {

	return (&cloudwatch.PutMetricDataInput{}).
		SetNamespace(ns).
		SetMetricData(datum)

}
