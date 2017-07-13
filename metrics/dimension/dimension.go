package dimension

import (
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

// FromMap returns an array of Dimensons from a map and skips any key:value
// pair that has a blank key or value, or if the value is not a string type
func FromMap(m map[string]interface{}) []*cloudwatch.Dimension {
	var dims []*cloudwatch.Dimension

	for k, v := range m {
		val, ok := v.(string)
		if !ok {
			continue
		}
		if k == "" || val == "" {
			continue
		}

		dims = append(dims, New(k, val))
	}

	return dims
}

// New returns a new Dimension
func New(name, value string) *cloudwatch.Dimension {
	return (&cloudwatch.Dimension{}).
		SetName(name).
		SetValue(value)
}
