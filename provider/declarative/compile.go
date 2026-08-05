package declarative

import (
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/metafates/mangal/provider/generic"
)

const (
	defaultDelayMs     = 50
	defaultParallelism = uint8(16)

	// queryPlaceholder is substituted with the URL-escaped search query.
	queryPlaceholder = "{query}"
)

// Configuration compiles the spec into a configuration the generic scraper
// understands. name becomes the source name, and comes from the config's
// filename.
func (s *Spec) Configuration(name string) *generic.Configuration {
	delay := defaultDelayMs
	if s.DelayMs != nil {
		delay = *s.DelayMs
	}

	parallelism := defaultParallelism
	if s.Parallelism != nil {
		parallelism = *s.Parallelism
	}

	searchURL := s.SearchURL

	conf := &generic.Configuration{
		Name:            name,
		Delay:           time.Duration(delay) * time.Millisecond,
		Parallelism:     parallelism,
		ReverseChapters: s.ReverseChapters,
		BaseURL:         s.BaseURL,
		GenerateSearchURL: func(query string) string {
			return strings.ReplaceAll(searchURL, queryPlaceholder, url.QueryEscape(query))
		},
		ChapterExtractor: s.Chapters.compileExtractor(),
		PageExtractor:    s.Pages.compileExtractor(),
	}

	// A static source never visits a search URL, but generic.Configuration
	// requires a non-nil manga extractor regardless.
	if s.Search != nil {
		conf.MangaExtractor = s.Search.compileExtractor()
	} else {
		conf.MangaExtractor = (&ExtractorSpec{}).compileExtractor()
	}

	return conf
}

// compileExtractor turns declarative field descriptions into the closures
// generic.Extractor expects. Every function must be non-nil, because the
// generic scraper calls them unconditionally.
func (e *ExtractorSpec) compileExtractor() *generic.Extractor {
	return &generic.Extractor{
		Selector: e.Selector,
		Name:     extractorFunc(e.Name),
		URL:      extractorFunc(e.URL),
		Cover:    extractorFunc(e.Cover),
		Volume:   extractorFunc(e.Volume),
	}
}

func extractorFunc(field *FieldSpec) func(*goquery.Selection) string {
	return func(selection *goquery.Selection) string {
		return field.Extract(selection)
	}
}
