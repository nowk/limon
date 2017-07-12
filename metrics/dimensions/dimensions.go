package dimensions

import (
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

// FromSet returns an array of Dimensons form a set of name:value pairs. This
// skips any pair that has a blank name or value
func FromSet(set [][]string) []*cloudwatch.Dimension {
	var dims []*cloudwatch.Dimension

	for _, v := range set {
		var (
			name  = v[0]
			value = v[1]
		)
		if name == "" || value == "" {
			continue
		}

		dims = append(dims, New(name, value))
	}

	return dims
}

// New returns a new Dimension
func New(name, value string) *cloudwatch.Dimension {
	return (&cloudwatch.Dimension{}).
		SetName(name).
		SetValue(value)
}
