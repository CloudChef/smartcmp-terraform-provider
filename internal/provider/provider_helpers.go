package provider

import (
	"regexp"
)

var durationRegexp = regexp.MustCompile(`^\d+(ns|us|µs|ms|s|m|h)$`)
