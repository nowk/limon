package dimensions

import (
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

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

func New(name, value string) *cloudwatch.Dimension {
	return (&cloudwatch.Dimension{}).
		SetName(name).
		SetValue(value)
}
