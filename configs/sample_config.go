// Package configs embeds the sample configuration referenced by
// `kwd --print-sample-config` (and the configs/kwd.sample.yml artifact).
package configs

import _ "embed"

//go:embed kwd.sample.yml
var sampleYAML string

// SampleYAML returns the annotated sample config as a string.
func SampleYAML() string {
	return sampleYAML
}
