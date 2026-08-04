package declarative

import "regexp"

// FieldSpec describes how to extract a single value from a matched element.
type FieldSpec struct {
	// Selector is an optional sub-selector within the matched item.
	// When empty the matched item itself is used.
	Selector string `toml:"selector"`
	// Attr is the attribute to read. When empty the element's text is used.
	Attr string `toml:"attr"`
	// Regex is applied to the raw value.
	Regex string `toml:"regex"`
	// Replace is a replacement template. Requires Regex.
	// A pointer so that an explicit empty replacement is distinguishable
	// from the key being absent.
	Replace *string `toml:"replace"`

	regex *regexp.Regexp
}
