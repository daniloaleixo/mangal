package declarative

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/metafates/mangal/filesystem"
	"github.com/pelletier/go-toml/v2"
)

// Parse reads and validates a declarative source config from disk.
func Parse(path string) (*Spec, error) {
	data, err := filesystem.Api().ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseBytes(data)
}

// ParseBytes decodes and validates a declarative source config.
//
// Decoding is strict: an unknown key is an error rather than a silent no-op,
// because a typo'd selector key is indistinguishable from a site changing its
// markup once downloads start coming back empty.
func ParseBytes(data []byte) (*Spec, error) {
	var spec Spec

	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&spec); err != nil {
		return nil, err
	}

	if err := spec.validate(); err != nil {
		return nil, err
	}

	return &spec, nil
}

func (s *Spec) validate() error {
	if s.BaseURL == "" {
		return errors.New("base_url is required")
	}

	hasSearchURL := s.SearchURL != ""
	hasStatic := len(s.Manga) > 0

	switch {
	case hasSearchURL && hasStatic:
		return errors.New("exactly one of search_url or [[manga]] must be set, got both")
	case !hasSearchURL && !hasStatic:
		return errors.New("exactly one of search_url or [[manga]] must be set, got neither")
	}

	if hasSearchURL && s.Search == nil {
		return errors.New("[search] is required when search_url is set")
	}

	if !hasSearchURL && s.Search != nil {
		return errors.New("[search] is not allowed without search_url")
	}

	for i, manga := range s.Manga {
		if manga.Name == "" || manga.URL == "" {
			return fmt.Errorf("[[manga]] entry %d: both name and url are required", i)
		}
	}

	if s.Chapters == nil {
		return errors.New("[chapters] is required")
	}

	if s.Pages == nil {
		return errors.New("[pages] is required")
	}

	extractors := []struct {
		label    string
		spec     *ExtractorSpec
		required []string
	}{
		{"chapters", s.Chapters, []string{"name", "url"}},
		{"pages", s.Pages, []string{"url"}},
	}

	if s.Search != nil {
		extractors = append(extractors, struct {
			label    string
			spec     *ExtractorSpec
			required []string
		}{"search", s.Search, []string{"name", "url"}})
	}

	for _, e := range extractors {
		if err := e.spec.validate(e.label, e.required); err != nil {
			return err
		}
	}

	return nil
}

func (e *ExtractorSpec) validate(label string, required []string) error {
	if e.Selector == "" {
		return fmt.Errorf("[%s]: selector is required", label)
	}

	fields := map[string]*FieldSpec{
		"name":   e.Name,
		"url":    e.URL,
		"cover":  e.Cover,
		"volume": e.Volume,
	}

	for _, name := range required {
		if fields[name] == nil {
			return fmt.Errorf("[%s.%s] is required", label, name)
		}
	}

	for name, field := range fields {
		if err := field.compile(); err != nil {
			return fmt.Errorf("[%s.%s]: %w", label, name, err)
		}
	}

	return nil
}
