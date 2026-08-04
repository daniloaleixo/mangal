package declarative

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/metafates/mangal/provider/generic"
	"github.com/metafates/mangal/source"
	"github.com/metafates/mangal/util"
)

// IDfromName builds the source ID for a declarative source. The suffix is
// distinct from generic's "built-in" and Lua's "custom" so that cache and
// history keys cannot collide with a same-named Lua source.
func IDfromName(name string) string {
	return name + " toml"
}

// Source wraps the generic scraper. It exists so a declarative source can own
// its ID, serve statically declared manga without an HTTP request, and report
// empty results as errors naming the selector at fault.
type Source struct {
	inner  source.Source
	name   string
	id     string
	static []*source.Manga
}

// LoadSource reads a declarative config and returns the source it describes.
// The source name is the file's stem, matching how Lua sources are named.
func LoadSource(path string) (source.Source, error) {
	spec, err := Parse(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	return newSource(util.FileStem(path), spec), nil
}

func newSource(name string, spec *Spec) *Source {
	s := &Source{
		inner: generic.New(spec.Configuration(name)),
		name:  name,
		id:    IDfromName(name),
	}

	if len(spec.Manga) == 0 {
		return s
	}

	s.static = make([]*source.Manga, len(spec.Manga))
	for i, declared := range spec.Manga {
		manga := &source.Manga{
			Name:     declared.Name,
			URL:      declared.URL,
			Index:    uint16(i),
			ID:       filepath.Base(declared.URL),
			Chapters: make([]*source.Chapter, 0),
			Source:   s,
		}
		manga.Metadata.Cover.ExtraLarge = declared.Cover
		s.static[i] = manga
	}

	return s
}

func (s *Source) Name() string { return s.name }

func (s *Source) ID() string { return s.id }

// Search returns statically declared manga when the config has them, filtered
// by a fuzzy match against the query. An empty query returns everything.
func (s *Source) Search(query string) ([]*source.Manga, error) {
	if s.static == nil {
		mangas, err := s.inner.Search(query)
		if err != nil {
			return nil, err
		}

		// generic.New stamps Source with the raw *generic.Scraper it builds
		// (see provider/generic/new.go), so every manga returned here still
		// points at s.inner instead of this decorator. Re-point it so IDs,
		// history, and error messages all see the " toml" suffix rather than
		// generic's hardcoded " built-in". The inner scraper caches these
		// *source.Manga values, so this is idempotent and safe to repeat.
		for _, manga := range mangas {
			manga.Source = s
		}

		return mangas, nil
	}

	if strings.TrimSpace(query) == "" {
		found := make([]*source.Manga, len(s.static))
		copy(found, s.static)
		return found, nil
	}

	found := make([]*source.Manga, 0, len(s.static))
	for _, manga := range s.static {
		if fuzzy.MatchNormalizedFold(query, manga.Name) {
			found = append(found, manga)
		}
	}

	return found, nil
}

func (s *Source) ChaptersOf(manga *source.Manga) ([]*source.Chapter, error) {
	chapters, err := s.inner.ChaptersOf(manga)
	if err != nil {
		return nil, err
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf(
			"no chapters found at %s, check the [chapters] selector (or the URL may have redirected)",
			manga.URL,
		)
	}

	return chapters, nil
}

func (s *Source) PagesOf(chapter *source.Chapter) ([]*source.Page, error) {
	pages, err := s.inner.PagesOf(chapter)
	if err != nil {
		return nil, err
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf(
			"no pages found at %s, check the [pages] selector. "+
				"If this site builds its reader in JavaScript, it needs a Lua source instead (see `mangal sources gen`)",
			chapter.URL,
		)
	}

	return pages, nil
}
