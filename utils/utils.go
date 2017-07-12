package utils

import (
	"errors"
)

// ParseMemoryUnit returns capitalized version of acceptable unites, or returns
// error
func ParseMemoryUnit(unit string) (string, error) {
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
