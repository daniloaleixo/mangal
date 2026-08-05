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

// ExtractorSpec describes how to find items on a page and pull fields from each.
type ExtractorSpec struct {
	// Selector matches the repeated item elements.
	Selector string `toml:"selector"`

	Name   *FieldSpec `toml:"name"`
	URL    *FieldSpec `toml:"url"`
	Cover  *FieldSpec `toml:"cover"`
	Volume *FieldSpec `toml:"volume"`
}

// StaticManga is a manga declared directly in the config, for sites with no
// catalog to search.
type StaticManga struct {
	Name  string `toml:"name"`
	URL   string `toml:"url"`
	Cover string `toml:"cover"`
}

// Spec is a full declarative source definition.
type Spec struct {
	BaseURL         string `toml:"base_url"`
	SearchURL       string `toml:"search_url"`
	ReverseChapters bool   `toml:"reverse_chapters"`

	// Pointers so that "absent" is distinguishable from an explicit zero.
	DelayMs     *int   `toml:"delay_ms"`
	Parallelism *uint8 `toml:"parallelism"`

	Manga []StaticManga `toml:"manga"`

	Search   *ExtractorSpec `toml:"search"`
	Chapters *ExtractorSpec `toml:"chapters"`
	Pages    *ExtractorSpec `toml:"pages"`
}
