# Declarative TOML Sources Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a user add a manga source to mangal by dropping a TOML file into `mangal where --sources`, with no rebuild.

**Architecture:** A new `provider/declarative` package parses a TOML spec and compiles it into a `*generic.Configuration`, then hands that to the existing `generic.New()` scraper. The declarative layer is a translator, not a scraper — all networking, caching, and rate limiting is existing proven code. The resulting source is wrapped in a thin decorator that owns the source ID, static-manga search, and empty-result errors.

**Tech Stack:** Go 1.18, `pelletier/go-toml/v2` (parsing), `PuerkitoBio/goquery` (selectors), `lithammer/fuzzysearch` (static search), `smartystreets/goconvey` (tests). All four are already vendored.

## Global Constraints

- Target Go version is **1.18** (`go.mod`). Do not use generics-heavy stdlib added later, `any` is fine (1.18), but avoid 1.19+ stdlib additions.
- `vendor/` is committed. After touching `go.mod`, always run `go mod tidy && go mod vendor`.
- All file I/O goes through `filesystem.Api()` (afero), never `os` directly.
- Tests use goconvey with the dot-import `. "github.com/smartystreets/goconvey/convey"`, matching the existing 23 test files.
- **No test may hit the live network.** Existing provider tests already scrape real sites and break when those sites change; do not add to that.
- Source ID suffix for declarative sources is exactly `" toml"` — distinct from `" built-in"` (generic) and `" custom"` (Lua).
- The `{query}` placeholder in `search_url` is replaced with `url.QueryEscape(query)`.
- Default `delay_ms` is `50`. Default `parallelism` is `16`.

## Prerequisite: Install the Go toolchain

**There is no Go toolchain on this machine.** `go` is not on `PATH`, and there is no install under `/usr/local/go` or `/usr/lib/go*`. Every task below runs tests, so this must be resolved first. It needs sudo, so the user must run it.

Ask the user to run one of these in the session with the `!` prefix:

```
! sudo snap install go --classic
```

or

```
! sudo apt install -y golang-go
```

Either provides Go 1.26, comfortably above the 1.18 floor in `go.mod`.

Verify before starting Task 1:

```bash
go version && go env GOFLAGS
```

Expected: a version string of 1.18 or higher.

---

### Task 1: Field extraction

The evaluation rule that turns a declarative field description into a value pulled from an HTML element. Pure string manipulation — no network, no config, no scraper. This is the highest-value test file in the project.

**Files:**
- Create: `provider/declarative/spec.go`
- Create: `provider/declarative/field.go`
- Test: `provider/declarative/field_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `type FieldSpec struct { Selector, Attr, Regex string; Replace *string; regex *regexp.Regexp }` with TOML tags `selector`, `attr`, `regex`, `replace`
  - `func (f *FieldSpec) Extract(s *goquery.Selection) string`
  - `func (f *FieldSpec) compile() error`

- [ ] **Step 1: Write the failing test**

Create `provider/declarative/field_test.go`:

```go
package declarative

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	. "github.com/smartystreets/goconvey/convey"
)

// sel parses html and returns the first element matching selector.
func sel(t *testing.T, html, selector string) *goquery.Selection {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	return doc.Find(selector).First()
}

func strptr(s string) *string { return &s }

func TestFieldSpecExtract(t *testing.T) {
	const html = `<div class="item">
		<a href="/manga/the-climber-chapter-6/">  Chapter 6  </a>
		<img src="//cdn.example.com/1.png" data-src="https://cdn.example.com/1.png">
	</div>`

	Convey("Given an element", t, func() {
		item := sel(t, html, "div.item")

		Convey("When no attr is set, the text is used and trimmed", func() {
			f := &FieldSpec{Selector: "a"}
			So(f.compile(), ShouldBeNil)
			So(f.Extract(item), ShouldEqual, "Chapter 6")
		})

		Convey("When attr is set, the attribute is used", func() {
			f := &FieldSpec{Selector: "a", Attr: "href"}
			So(f.compile(), ShouldBeNil)
			So(f.Extract(item), ShouldEqual, "/manga/the-climber-chapter-6/")
		})

		Convey("When selector is empty, the element itself is used", func() {
			f := &FieldSpec{Attr: "class"}
			So(f.compile(), ShouldBeNil)
			So(f.Extract(item), ShouldEqual, "item")
		})

		Convey("When a missing attr is requested, empty is returned", func() {
			f := &FieldSpec{Selector: "a", Attr: "title"}
			So(f.compile(), ShouldBeNil)
			So(f.Extract(item), ShouldEqual, "")
		})

		Convey("When regex has a capture group, group 1 is returned", func() {
			f := &FieldSpec{Selector: "a", Attr: "href", Regex: `chapter-(\d+)`}
			So(f.compile(), ShouldBeNil)
			So(f.Extract(item), ShouldEqual, "6")
		})

		Convey("When regex has no capture group, the whole match is returned", func() {
			f := &FieldSpec{Selector: "a", Attr: "href", Regex: `chapter-\d+`}
			So(f.compile(), ShouldBeNil)
			So(f.Extract(item), ShouldEqual, "chapter-6")
		})

		Convey("When regex does not match, empty is returned", func() {
			f := &FieldSpec{Selector: "a", Attr: "href", Regex: `volume-(\d+)`}
			So(f.compile(), ShouldBeNil)
			So(f.Extract(item), ShouldEqual, "")
		})

		Convey("When replace is set, substitution is applied", func() {
			f := &FieldSpec{Selector: "img", Attr: "src", Regex: `^//`, Replace: strptr("https://")}
			So(f.compile(), ShouldBeNil)
			So(f.Extract(item), ShouldEqual, "https://cdn.example.com/1.png")
		})

		Convey("When the spec is nil, empty is returned", func() {
			var f *FieldSpec
			So(f.Extract(item), ShouldEqual, "")
		})
	})
}

func TestFieldSpecCompile(t *testing.T) {
	Convey("Given a field spec", t, func() {
		Convey("When the regex is invalid, compile fails", func() {
			f := &FieldSpec{Regex: `([`}
			So(f.compile(), ShouldNotBeNil)
		})

		Convey("When replace is set without regex, compile fails", func() {
			f := &FieldSpec{Replace: strptr("x")}
			So(f.compile(), ShouldNotBeNil)
		})

		Convey("When nothing is set, compile succeeds", func() {
			f := &FieldSpec{}
			So(f.compile(), ShouldBeNil)
		})
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/declarative/ -run TestFieldSpec -v`
Expected: FAIL — build error, `undefined: FieldSpec`.

- [ ] **Step 3: Write the spec struct**

Create `provider/declarative/spec.go`:

```go
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
```

- [ ] **Step 4: Write the extraction logic**

Create `provider/declarative/field.go`:

```go
package declarative

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Extract returns the value described by the spec, read from the given element.
//
// The rule, in order: resolve Selector if set (otherwise use the element
// itself), read Attr if set (otherwise the text), apply Regex if set, then
// trim surrounding whitespace.
func (f *FieldSpec) Extract(s *goquery.Selection) string {
	if f == nil {
		return ""
	}

	target := s
	if f.Selector != "" {
		target = s.Find(f.Selector).First()
	}

	var raw string
	if f.Attr != "" {
		raw = target.AttrOr(f.Attr, "")
	} else {
		raw = target.Text()
	}

	if f.regex != nil {
		if f.Replace != nil {
			raw = f.regex.ReplaceAllString(raw, *f.Replace)
		} else {
			match := f.regex.FindStringSubmatch(raw)
			switch {
			case match == nil:
				raw = ""
			case len(match) > 1:
				raw = match[1]
			default:
				raw = match[0]
			}
		}
	}

	return strings.TrimSpace(raw)
}

// compile prepares the regex and validates the field. Called during parsing so
// that a bad pattern fails at load time rather than mid-download.
func (f *FieldSpec) compile() error {
	if f == nil {
		return nil
	}

	if f.Regex == "" {
		if f.Replace != nil {
			return errors.New("replace requires regex")
		}
		return nil
	}

	re, err := regexp.Compile(f.Regex)
	if err != nil {
		return fmt.Errorf("invalid regex %q: %w", f.Regex, err)
	}

	f.regex = re
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./provider/declarative/ -v`
Expected: PASS, all cases.

- [ ] **Step 6: Commit**

```bash
git add provider/declarative/spec.go provider/declarative/field.go provider/declarative/field_test.go
git commit -m "feat(declarative): add field extraction rule"
```

---

### Task 2: Spec parsing and validation

Reads a TOML file into a `Spec` and rejects anything malformed, with messages that name the offending key. Strict decoding is the point: a typo'd key that silently matches nothing looks identical to the site changing its markup.

**Files:**
- Modify: `provider/declarative/spec.go` (add `ExtractorSpec`, `StaticManga`, `Spec`)
- Create: `provider/declarative/parse.go`
- Test: `provider/declarative/parse_test.go`
- Modify: `go.mod` (promote `pelletier/go-toml/v2` to direct)

**Interfaces:**
- Consumes: `FieldSpec`, `(*FieldSpec).compile()` from Task 1.
- Produces:
  - `type ExtractorSpec struct { Selector string; Name, URL, Cover, Volume *FieldSpec }`
  - `type StaticManga struct { Name, URL, Cover string }`
  - `type Spec struct { BaseURL, SearchURL string; ReverseChapters bool; DelayMs *int; Parallelism *uint8; Manga []StaticManga; Search, Chapters, Pages *ExtractorSpec }`
  - `func Parse(path string) (*Spec, error)`
  - `func ParseBytes(data []byte) (*Spec, error)`

- [ ] **Step 1: Write the failing test**

Create `provider/declarative/parse_test.go`:

```go
package declarative

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

const validStatic = `
base_url = "https://theclimber.club"

[[manga]]
name = "The Climber"
url  = "https://theclimber.club/"

[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
  attr = "href"

[pages]
selector = "img"
  [pages.url]
  attr = "src"
`

const validCatalog = `
base_url   = "https://example.com"
search_url = "https://example.com/search?q={query}"

[search]
selector = "div.result"
  [search.name]
  [search.url]
  attr = "href"

[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
  attr = "href"

[pages]
selector = "img"
  [pages.url]
  attr = "src"
`

func TestParseBytesValid(t *testing.T) {
	Convey("Given a valid static config", t, func() {
		spec, err := ParseBytes([]byte(validStatic))
		Convey("Then it parses", func() {
			So(err, ShouldBeNil)
			So(spec.BaseURL, ShouldEqual, "https://theclimber.club")
			So(spec.Manga, ShouldHaveLength, 1)
			So(spec.Manga[0].Name, ShouldEqual, "The Climber")
			So(spec.Search, ShouldBeNil)
			So(spec.Chapters.URL.Attr, ShouldEqual, "href")
		})
	})

	Convey("Given a valid catalog config", t, func() {
		spec, err := ParseBytes([]byte(validCatalog))
		Convey("Then it parses", func() {
			So(err, ShouldBeNil)
			So(spec.SearchURL, ShouldEqual, "https://example.com/search?q={query}")
			So(spec.Search, ShouldNotBeNil)
			So(spec.Manga, ShouldHaveLength, 0)
		})
	})

	Convey("Given a config with an empty field table", t, func() {
		spec, err := ParseBytes([]byte(validStatic))
		Convey("Then the empty table yields a non-nil zero FieldSpec", func() {
			So(err, ShouldBeNil)
			So(spec.Chapters.Name, ShouldNotBeNil)
			So(spec.Chapters.Name.Attr, ShouldEqual, "")
		})
	})
}

func TestParseBytesInvalid(t *testing.T) {
	cases := []struct {
		desc  string
		input string
	}{
		{"missing base_url", `
[[manga]]
name = "X"
url  = "https://x.test/"
[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
		{"neither search_url nor manga", `
base_url = "https://x.test"
[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
		{"both search_url and manga", `
base_url   = "https://x.test"
search_url = "https://x.test/?q={query}"
[[manga]]
name = "X"
url  = "https://x.test/"
[search]
selector = "div"
  [search.name]
  [search.url]
[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
		{"search_url without [search]", `
base_url   = "https://x.test"
search_url = "https://x.test/?q={query}"
[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
		{"[search] without search_url", `
base_url = "https://x.test"
[[manga]]
name = "X"
url  = "https://x.test/"
[search]
selector = "div"
  [search.name]
  [search.url]
[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
		{"missing [pages]", `
base_url = "https://x.test"
[[manga]]
name = "X"
url  = "https://x.test/"
[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
`},
		{"missing [chapters.url]", `
base_url = "https://x.test"
[[manga]]
name = "X"
url  = "https://x.test/"
[chapters]
selector = "a"
  [chapters.name]
[pages]
selector = "img"
  [pages.url]
`},
		{"extractor without selector", `
base_url = "https://x.test"
[[manga]]
name = "X"
url  = "https://x.test/"
[chapters]
  [chapters.name]
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
		{"unknown key", `
base_url  = "https://x.test"
selectors = "oops"
[[manga]]
name = "X"
url  = "https://x.test/"
[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
		{"uncompilable regex", `
base_url = "https://x.test"
[[manga]]
name = "X"
url  = "https://x.test/"
[chapters]
selector = "a"
  [chapters.name]
  regex = "(["
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
		{"replace without regex", `
base_url = "https://x.test"
[[manga]]
name = "X"
url  = "https://x.test/"
[chapters]
selector = "a"
  [chapters.name]
  replace = "x"
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
		{"static manga missing url", `
base_url = "https://x.test"
[[manga]]
name = "X"
[chapters]
selector = "a"
  [chapters.name]
  [chapters.url]
[pages]
selector = "img"
  [pages.url]
`},
	}

	for _, c := range cases {
		c := c
		Convey("Given a config with "+c.desc, t, func() {
			_, err := ParseBytes([]byte(c.input))
			Convey("Then parsing fails", func() {
				So(err, ShouldNotBeNil)
			})
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/declarative/ -run TestParse -v`
Expected: FAIL — build error, `undefined: ParseBytes`.

- [ ] **Step 3: Add the remaining spec structs**

Append to `provider/declarative/spec.go`:

```go
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
```

- [ ] **Step 4: Write the parser and validator**

Create `provider/declarative/parse.go`:

```go
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
```

- [ ] **Step 5: Promote the TOML dependency and re-vendor**

`pelletier/go-toml/v2` is currently an indirect dependency, and `vendor/modules.txt` gates packages not marked explicit. Run:

```bash
go mod tidy && go mod vendor
```

Confirm `go.mod` now lists `github.com/pelletier/go-toml/v2` in the direct `require` block (no `// indirect` comment).

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./provider/declarative/ -v`
Expected: PASS, including all twelve invalid cases.

- [ ] **Step 7: Commit**

```bash
git add provider/declarative/spec.go provider/declarative/parse.go provider/declarative/parse_test.go go.mod go.sum vendor/
git commit -m "feat(declarative): add strict TOML parsing and validation"
```

---

### Task 3: Compile a spec into a generic.Configuration

Turns the validated spec into the struct `generic.New()` already knows how to consume. Every extractor function becomes a closure over its `FieldSpec`.

**Files:**
- Create: `provider/declarative/compile.go`
- Test: `provider/declarative/compile_test.go`

**Interfaces:**
- Consumes: `Spec`, `ExtractorSpec`, `FieldSpec`, `(*FieldSpec).Extract` from Tasks 1–2.
- Produces:
  - `func (s *Spec) Configuration(name string) *generic.Configuration`

Note on `generic.Extractor`: every function field must be non-nil. `generic.New` calls `Name`, `URL`, `Volume`, and `Cover` on the manga extractor and `Name`, `URL`, `Volume` on the chapter extractor without nil checks, so unset fields must compile to functions returning `""`.

- [ ] **Step 1: Write the failing test**

Create `provider/declarative/compile_test.go`:

```go
package declarative

import (
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSpecConfiguration(t *testing.T) {
	Convey("Given a parsed catalog spec", t, func() {
		spec, err := ParseBytes([]byte(validCatalog))
		So(err, ShouldBeNil)

		conf := spec.Configuration("example")

		Convey("Then the scalar settings are carried over", func() {
			So(conf.Name, ShouldEqual, "example")
			So(conf.BaseURL, ShouldEqual, "https://example.com")
			So(conf.ReverseChapters, ShouldBeFalse)
		})

		Convey("Then defaults are applied", func() {
			So(conf.Delay, ShouldEqual, 50*time.Millisecond)
			So(conf.Parallelism, ShouldEqual, uint8(16))
		})

		Convey("Then the selectors reach the right extractors", func() {
			So(conf.MangaExtractor.Selector, ShouldEqual, "div.result")
			So(conf.ChapterExtractor.Selector, ShouldEqual, "a")
			So(conf.PageExtractor.Selector, ShouldEqual, "img")
		})

		Convey("Then the search URL escapes the query", func() {
			So(conf.GenerateSearchURL("one piece"), ShouldEqual,
				"https://example.com/search?q=one+piece")
		})

		Convey("Then unset extractor functions return empty strings", func() {
			item := sel(t, `<div class="result"></div>`, "div.result")
			So(conf.MangaExtractor.Cover(item), ShouldEqual, "")
			So(conf.ChapterExtractor.Volume(item), ShouldEqual, "")
		})
	})

	Convey("Given a parsed static spec", t, func() {
		spec, err := ParseBytes([]byte(validStatic))
		So(err, ShouldBeNil)

		conf := spec.Configuration("theclimber")

		Convey("Then a placeholder manga extractor is present", func() {
			So(conf.MangaExtractor, ShouldNotBeNil)
			So(conf.MangaExtractor.Name, ShouldNotBeNil)
			So(conf.MangaExtractor.URL, ShouldNotBeNil)
			So(conf.MangaExtractor.Cover, ShouldNotBeNil)
			So(conf.MangaExtractor.Volume, ShouldNotBeNil)
		})

		Convey("Then the chapter extractor pulls href and text", func() {
			const html = `<div><a href="/c/6">Chapter 6</a></div>`
			item := sel(t, html, "a")
			So(conf.ChapterExtractor.Name(item), ShouldEqual, "Chapter 6")
			So(conf.ChapterExtractor.URL(item), ShouldEqual, "/c/6")
		})
	})

	Convey("Given a spec with explicit delay and parallelism", t, func() {
		input := strings.Replace(
			validStatic,
			`base_url = "https://theclimber.club"`,
			"base_url = \"https://theclimber.club\"\ndelay_ms = 250\nparallelism = 4",
			1,
		)
		spec, err := ParseBytes([]byte(input))
		So(err, ShouldBeNil)

		conf := spec.Configuration("theclimber")

		Convey("Then the explicit values win", func() {
			So(conf.Delay, ShouldEqual, 250*time.Millisecond)
			So(conf.Parallelism, ShouldEqual, uint8(4))
		})
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/declarative/ -run TestSpecConfiguration -v`
Expected: FAIL — build error, `spec.Configuration undefined`.

- [ ] **Step 3: Write the compiler**

Create `provider/declarative/compile.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./provider/declarative/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add provider/declarative/compile.go provider/declarative/compile_test.go
git commit -m "feat(declarative): compile specs into generic configurations"
```

---

### Task 4: Source decorator and loader

Wraps the generic scraper so the declarative source owns its ID, handles static-manga search, and turns empty results into errors that name the failing selector.

**Design refinement:** the spec described the decorator as applying only to static configs. It applies to **all** declarative sources, because `generic.Configuration.ID()` hardcodes the `" built-in"` suffix and cannot be overridden any other way. Catalog configs simply delegate `Search` through.

**Files:**
- Create: `provider/declarative/source.go`
- Test: `provider/declarative/source_test.go`
- Modify: `go.mod` (promote `lithammer/fuzzysearch` to direct)

**Interfaces:**
- Consumes: `Parse`, `(*Spec).Configuration` from Tasks 2–3.
- Produces:
  - `func IDfromName(name string) string` — returns `name + " toml"`
  - `func LoadSource(path string) (source.Source, error)`
  - `type Source struct` implementing `source.Source`

- [ ] **Step 1: Write the failing test**

Create `provider/declarative/source_test.go`:

```go
package declarative

import (
	"testing"

	"github.com/metafates/mangal/filesystem"
	"github.com/metafates/mangal/source"
	. "github.com/smartystreets/goconvey/convey"
)

func TestIDfromName(t *testing.T) {
	Convey("The declarative ID suffix is distinct from built-in and custom", t, func() {
		So(IDfromName("theclimber"), ShouldEqual, "theclimber toml")
	})
}

func TestLoadSource(t *testing.T) {
	Convey("Given a static config on an in-memory filesystem", t, func() {
		filesystem.SetMemMapFs()
		defer filesystem.SetOsFs()

		So(filesystem.Api().WriteFile("/sources/theclimber.toml", []byte(validStatic), 0644), ShouldBeNil)

		src, err := LoadSource("/sources/theclimber.toml")

		Convey("Then the source loads", func() {
			So(err, ShouldBeNil)
			So(src.Name(), ShouldEqual, "theclimber")
			So(src.ID(), ShouldEqual, "theclimber toml")
		})
	})

	Convey("Given an invalid config", t, func() {
		filesystem.SetMemMapFs()
		defer filesystem.SetOsFs()

		So(filesystem.Api().WriteFile("/sources/broken.toml", []byte("base_url = 1"), 0644), ShouldBeNil)

		_, err := LoadSource("/sources/broken.toml")

		Convey("Then the error names the file", func() {
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "broken.toml")
		})
	})
}

func TestStaticSearch(t *testing.T) {
	Convey("Given a static source with two manga", t, func() {
		spec, err := ParseBytes([]byte(validStatic))
		So(err, ShouldBeNil)
		spec.Manga = append(spec.Manga, StaticManga{
			Name: "Another Title",
			URL:  "https://theclimber.club/another/",
		})

		src := newSource("theclimber", spec)

		Convey("When the query is empty, all entries are returned", func() {
			found, err := src.Search("")
			So(err, ShouldBeNil)
			So(found, ShouldHaveLength, 2)
		})

		Convey("When the query matches one title, only it is returned", func() {
			found, err := src.Search("climber")
			So(err, ShouldBeNil)
			So(found, ShouldHaveLength, 1)
			So(found[0].Name, ShouldEqual, "The Climber")
		})

		Convey("When the query matches nothing, the result is empty", func() {
			found, err := src.Search("one piece")
			So(err, ShouldBeNil)
			So(found, ShouldHaveLength, 0)
		})

		Convey("Then static manga point back at the decorator", func() {
			found, err := src.Search("")
			So(err, ShouldBeNil)
			So(found[0].Source, ShouldEqual, source.Source(src))
		})
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/declarative/ -run "TestIDfromName|TestLoadSource|TestStaticSearch" -v`
Expected: FAIL — build error, `undefined: IDfromName`.

- [ ] **Step 3: Write the decorator and loader**

Create `provider/declarative/source.go`:

```go
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
		return s.inner.Search(query)
	}

	if strings.TrimSpace(query) == "" {
		return s.static, nil
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
		return nil, fmt.Errorf("no chapters found at %s, check the [chapters] selector", manga.URL)
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
```

- [ ] **Step 4: Promote the fuzzysearch dependency and re-vendor**

```bash
go mod tidy && go mod vendor
```

Confirm `github.com/lithammer/fuzzysearch` appears in the direct `require` block of `go.mod`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./provider/declarative/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add provider/declarative/source.go provider/declarative/source_test.go go.mod go.sum vendor/
git commit -m "feat(declarative): add source decorator and loader"
```

---

### Task 5: Register .toml files as providers

Makes `Customs()` discover TOML configs alongside Lua ones. Discovery stays cheap — no parsing — so a malformed config still appears in the source list and reports its error when selected.

**Files:**
- Modify: `provider/init.go:12` (add the extension constant)
- Modify: `provider/provider.go:30-71` (rewrite `Customs`)
- Test: `provider/provider_test.go`

**Interfaces:**
- Consumes: `declarative.LoadSource`, `declarative.IDfromName` from Task 4.
- Produces: no new exported API; `provider.Customs()` now returns TOML-backed providers.

- [ ] **Step 1: Write the failing test**

Read `provider/provider_test.go` first and append to it, preserving what is already there:

```go
func TestCustomsDiscoversTomlSources(t *testing.T) {
	Convey("Given a sources directory with a .toml and a .lua file", t, func() {
		filesystem.SetMemMapFs()
		defer filesystem.SetOsFs()

		dir := where.Sources()
		So(filesystem.Api().MkdirAll(dir, 0755), ShouldBeNil)
		So(filesystem.Api().WriteFile(filepath.Join(dir, "theclimber.toml"), []byte("base_url = \"x\""), 0644), ShouldBeNil)
		So(filesystem.Api().WriteFile(filepath.Join(dir, "somelua.lua"), []byte("-- lua"), 0644), ShouldBeNil)
		So(filesystem.Api().WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0644), ShouldBeNil)

		customs := provider.Customs()

		Convey("Then both sources are discovered and the .txt is ignored", func() {
			byName := make(map[string]*provider.Provider, len(customs))
			for _, p := range customs {
				byName[p.Name] = p
			}

			So(byName, ShouldContainKey, "theclimber")
			So(byName, ShouldContainKey, "somelua")
			So(byName, ShouldNotContainKey, "notes")
		})

		Convey("Then the toml provider carries the toml ID and is not headless", func() {
			for _, p := range customs {
				if p.Name == "theclimber" {
					So(p.ID, ShouldEqual, "theclimber toml")
					So(p.IsCustom, ShouldBeTrue)
					So(p.UsesHeadless, ShouldBeFalse)
				}
			}
		})

		Convey("Then a malformed toml source still lists but errors on creation", func() {
			for _, p := range customs {
				if p.Name == "theclimber" {
					_, err := p.CreateSource()
					So(err, ShouldNotBeNil)
					So(err.Error(), ShouldContainSubstring, "theclimber.toml")
				}
			}
		})
	})
}
```

Add these imports to the test file's import block: `path/filepath`, `github.com/metafates/mangal/filesystem`, `github.com/metafates/mangal/provider`, `github.com/metafates/mangal/where`.

Note: if `provider/provider_test.go` is in package `provider` rather than `provider_test`, drop the `provider.` qualifiers and the provider import.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/ -run TestCustoms -v`
Expected: FAIL — only `somelua` is discovered; `theclimber` is missing.

- [ ] **Step 3: Add the extension constant**

In `provider/init.go`, alongside the existing `CustomProviderExtension`:

```go
const (
	CustomProviderExtension      = ".lua"
	DeclarativeProviderExtension = ".toml"
)
```

Remove the now-duplicated standalone `const CustomProviderExtension = ".lua"` line.

- [ ] **Step 4: Rewrite Customs**

Replace the body of `Customs()` in `provider/provider.go`:

```go
func Customs() []*Provider {
	files, err := filesystem.Api().ReadDir(where.Sources())

	if err != nil {
		return make([]*Provider, 0)
	}

	providers := make([]*Provider, 0, len(files))

	for _, file := range files {
		path := filepath.Join(where.Sources(), file.Name())
		name := util.FileStem(path)

		switch filepath.Ext(file.Name()) {
		case CustomProviderExtension:
			// Check if source contains line `require("headless")`
			// if so, set UsesHeadless to true.
			// This approach is not ideal, but it's the only way to do it without
			// actually loading the source.
			usesHeadless, _ := filesystem.Api().FileContainsAnyBytes(path, [][]byte{
				[]byte("require(\"headless\")"),
				[]byte("require('headless')"),
				[]byte("require(headless)"),
				[]byte("require'headless'"),
			})

			path := path
			providers = append(providers, &Provider{
				ID:           custom.IDfromName(name),
				UsesHeadless: usesHeadless,
				IsCustom:     true,
				Name:         name,
				CreateSource: func() (source.Source, error) {
					return custom.LoadSource(path, true)
				},
			})

		case DeclarativeProviderExtension:
			path := path
			providers = append(providers, &Provider{
				ID:       declarative.IDfromName(name),
				IsCustom: true,
				Name:     name,
				CreateSource: func() (source.Source, error) {
					return declarative.LoadSource(path)
				},
			})
		}
	}

	return providers
}
```

Update the import block of `provider/provider.go`: add `github.com/metafates/mangal/provider/declarative`, and remove `github.com/samber/lo` and `os` if the rewrite leaves them unused.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./provider/ ./provider/declarative/ -v`
Expected: PASS.

- [ ] **Step 6: Verify the whole tree still builds**

Run: `go build ./...`
Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add provider/init.go provider/provider.go provider/provider_test.go
git commit -m "feat(provider): discover declarative .toml sources"
```

---

### Task 6: End-to-end test against a fixture server

Proves search → chapters → pages works through the real generic scraper, with no live network.

**Files:**
- Create: `provider/declarative/e2e_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1–4.
- Produces: no production code.

Note: `generic.New` writes a colly cache under `where.Cache()`. `httptest` servers get a random port each run, so cached responses cannot leak between runs.

- [ ] **Step 1: Write the test**

Create `provider/declarative/e2e_test.go`:

```go
package declarative

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	fixtureSearchHTML = `<html><body>
		<div class="result"><a href="/manga/alpha">Alpha</a><img data-src="/cover.jpg"></div>
	</body></html>`

	fixtureChaptersHTML = `<html><body>
		<div class="chapters">
			<a href="/manga/alpha/2">Chapter 2</a>
			<a href="/manga/alpha/1">Chapter 1</a>
		</div>
	</body></html>`

	fixturePagesHTML = `<html><body>
		<div class="reader">
			<img data-src="//cdn.test/1.png">
			<img data-src="//cdn.test/2.png">
		</div>
	</body></html>`
)

func fixtureServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixtureSearchHTML)
	})
	mux.HandleFunc("/manga/alpha", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixtureChaptersHTML)
	})
	mux.HandleFunc("/manga/alpha/1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixturePagesHTML)
	})
	mux.HandleFunc("/manga/alpha/2", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixturePagesHTML)
	})
	return httptest.NewServer(mux)
}

func TestEndToEndCatalog(t *testing.T) {
	Convey("Given a fixture site and a catalog config", t, func() {
		server := fixtureServer()
		defer server.Close()

		config := fmt.Sprintf(`
base_url         = %q
search_url       = "%s/search?q={query}"
reverse_chapters = true

[search]
selector = "div.result"
  [search.name]
  selector = "a"
  [search.url]
  selector = "a"
  attr = "href"
  [search.cover]
  selector = "img"
  attr = "data-src"

[chapters]
selector = "div.chapters a"
  [chapters.name]
  [chapters.url]
  attr = "href"

[pages]
selector = "div.reader img"
  [pages.url]
  attr = "data-src"
  regex = "^//"
  replace = "https://"
`, server.URL, server.URL)

		spec, err := ParseBytes([]byte(config))
		So(err, ShouldBeNil)

		src := newSource("fixture", spec)

		Convey("When searching, the manga is found", func() {
			mangas, err := src.Search("alpha")
			So(err, ShouldBeNil)
			So(mangas, ShouldHaveLength, 1)
			So(mangas[0].Name, ShouldEqual, "Alpha")

			Convey("And its chapters are listed in reverse", func() {
				chapters, err := src.ChaptersOf(mangas[0])
				So(err, ShouldBeNil)
				So(chapters, ShouldHaveLength, 2)
				So(chapters[0].Name, ShouldEqual, "Chapter 1")

				Convey("And its pages resolve, with the regex applied", func() {
					pages, err := src.PagesOf(chapters[0])
					So(err, ShouldBeNil)
					So(pages, ShouldHaveLength, 2)
					So(pages[0].URL, ShouldEqual, "https://cdn.test/1.png")
					So(pages[0].Extension, ShouldEqual, ".png")
				})
			})
		})
	})
}

func TestEndToEndStatic(t *testing.T) {
	Convey("Given a fixture site and a static config", t, func() {
		server := fixtureServer()
		defer server.Close()

		config := fmt.Sprintf(`
base_url = %q

[[manga]]
name = "Alpha"
url  = "%s/manga/alpha"

[chapters]
selector = "div.chapters a"
  [chapters.name]
  [chapters.url]
  attr = "href"

[pages]
selector = "div.reader img"
  [pages.url]
  attr = "data-src"
  regex = "^//"
  replace = "https://"
`, server.URL, server.URL)

		spec, err := ParseBytes([]byte(config))
		So(err, ShouldBeNil)

		src := newSource("fixture", spec)

		Convey("When searching with no query, the static manga is returned", func() {
			mangas, err := src.Search("")
			So(err, ShouldBeNil)
			So(mangas, ShouldHaveLength, 1)

			Convey("And its chapters and pages resolve without a search request", func() {
				chapters, err := src.ChaptersOf(mangas[0])
				So(err, ShouldBeNil)
				So(chapters, ShouldHaveLength, 2)

				pages, err := src.PagesOf(chapters[0])
				So(err, ShouldBeNil)
				So(pages, ShouldHaveLength, 2)
			})
		})
	})
}

func TestEmptyResultsAreErrors(t *testing.T) {
	Convey("Given a static config whose selectors match nothing", t, func() {
		server := fixtureServer()
		defer server.Close()

		config := fmt.Sprintf(`
base_url = %q

[[manga]]
name = "Alpha"
url  = "%s/manga/alpha"

[chapters]
selector = "div.nonexistent a"
  [chapters.name]
  [chapters.url]
  attr = "href"

[pages]
selector = "div.reader img"
  [pages.url]
  attr = "data-src"
`, server.URL, server.URL)

		spec, err := ParseBytes([]byte(config))
		So(err, ShouldBeNil)

		src := newSource("fixture", spec)

		Convey("Then ChaptersOf reports the failing selector", func() {
			mangas, err := src.Search("")
			So(err, ShouldBeNil)

			_, err = src.ChaptersOf(mangas[0])
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "[chapters] selector")
		})
	})
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./provider/declarative/ -run TestEndToEnd -v`
Expected: PASS.

If `TestEndToEndCatalog` fails on chapter ordering, confirm `reverse_chapters` reached the configuration — `generic.ChaptersOf` reverses only when `ReverseChapters` is true.

- [ ] **Step 3: Run the empty-result test**

Run: `go test ./provider/declarative/ -run TestEmptyResultsAreErrors -v`
Expected: PASS.

- [ ] **Step 4: Run the full package with the race detector**

Run: `go test -race ./provider/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add provider/declarative/e2e_test.go
git commit -m "test(declarative): add offline end-to-end coverage"
```

---

### Task 7: The theclimber.club config and documentation

The worked example, plus the README section that makes the format discoverable.

**Files:**
- Create: `assets/examples/theclimber.toml`
- Modify: `README.md` (new section after "Custom scrapers", around line 282)

**Interfaces:**
- Consumes: the config format from Tasks 2–4.
- Produces: no code.

- [ ] **Step 1: Fetch the real markup**

The selectors below are a starting point derived from a summarized rendering of the site, **not from the markup**. Confirm them before writing the file:

```bash
curl -sL -A "Mozilla/5.0" https://theclimber.club/ -o /tmp/climber-home.html
curl -sL -A "Mozilla/5.0" https://theclimber.club/manga/the-climber-chapter-6/ -o /tmp/climber-ch6.html
```

From `climber-home.html`, find the element wrapping the chapter links and note its class or id, plus the anchor shape. From `climber-ch6.html`, find the page images and confirm whether the URL sits in `src` or a lazy-load attribute such as `data-src` or `data-lazy-src`.

Record what you actually find; do not assume the values below are correct.

- [ ] **Step 2: Write the config**

Create `assets/examples/theclimber.toml`, substituting the selectors confirmed in Step 1:

```toml
# theclimber.club — a single-series site with no catalog to search.
#
# Install with:
#   cp theclimber.toml "$(mangal where --sources)/"
#
# The source will appear as "theclimber".

base_url         = "https://theclimber.club"
reverse_chapters = true          # the site lists newest chapters first

[[manga]]
name  = "The Climber"
url   = "https://theclimber.club/"
cover = "https://theclimber.club/wp-content/uploads/2024/07/The-Climber.jpg"

[chapters]
selector = "a[href*='-chapter-']"
  [chapters.name]
  [chapters.url]
  attr = "href"

[pages]
selector = "img[src*='/manga/']"
  [pages.url]
  attr = "src"
```

- [ ] **Step 3: Verify the config against the live site**

This is the one place a live request is appropriate, because it validates the deliverable rather than gating the test suite.

```bash
go run . sources install --help >/dev/null 2>&1 || true
mkdir -p "$(go run . where --sources)"
cp assets/examples/theclimber.toml "$(go run . where --sources)/"
go run . inline --source theclimber --query "climber" --json
```

Expected: JSON naming "The Climber". Then confirm chapters and pages:

```bash
go run . inline --source theclimber --query "climber" --manga first --chapters all --json
```

Expected: a chapter list with roughly 150 entries. If it returns zero, revisit the `[chapters]` selector from Step 1.

- [ ] **Step 4: Document the format in the README**

Insert a section after the "Custom scrapers" section (which ends around line 282, before "## Anilist"):

````markdown
## Declarative scrapers

For sites that serve plain HTML, a scraper can be a TOML file instead of Lua —
no code and no rebuild. Drop it into `mangal where --sources` and the source
appears under the file's name.

    cp theclimber.toml "$(mangal where --sources)/"

A single-series site, with no catalog to search:

```toml
base_url         = "https://theclimber.club"
reverse_chapters = true

[[manga]]
name = "The Climber"
url  = "https://theclimber.club/"

[chapters]
selector = "a[href*='-chapter-']"
  [chapters.name]              # no attr means the element's text
  [chapters.url]
  attr = "href"

[pages]
selector = "img[src*='/manga/']"
  [pages.url]
  attr = "src"
```

A site with a catalog uses `search_url` and `[search]` instead of `[[manga]]`,
where `{query}` is replaced with the URL-escaped search term.

Each field accepts four optional keys: `selector` (a sub-selector within the
matched item), `attr` (the attribute to read, defaulting to the element's
text), `regex`, and `replace`. With `regex` alone, capture group 1 is taken;
with `replace`, a substitution is applied. Values are always trimmed.

Unknown keys are rejected, so a typo fails loudly rather than silently matching
nothing.

Sites that build their reader in JavaScript cannot be scraped this way — colly
does not execute JS. Those still need a Lua source, which has headless Chrome
available.

See `assets/examples/theclimber.toml` for a complete example.
````

- [ ] **Step 5: Confirm the tree still builds and tests pass**

Run: `go build ./... && go test ./provider/...`
Expected: no build output, tests PASS.

- [ ] **Step 6: Commit**

```bash
git add assets/examples/theclimber.toml README.md
git commit -m "docs: add declarative scraper format and theclimber example"
```

---

## Self-Review Notes

Checked against `docs/superpowers/specs/2026-08-04-declarative-toml-sources-design.md`:

- Every spec section maps to a task: config format → Tasks 2–3, components → Tasks 1–4, provider changes → Task 5, error handling → Tasks 2/4, testing → Tasks 1–4 and 6, deliverables → Task 7.
- One deliberate refinement of the spec, called out in Task 4: the decorator wraps **all** declarative sources, not only static ones, because `generic.Configuration.ID()` hardcodes `" built-in"` and is otherwise unoverridable.
- The spec's open question (no Go toolchain) is resolved as a prerequisite block requiring user action before Task 1.
- Naming is consistent across tasks: `FieldSpec.Extract`, `FieldSpec.compile`, `ExtractorSpec.compileExtractor`, `Spec.Configuration`, `Spec.validate`, `ParseBytes`, `Parse`, `LoadSource`, `newSource`, `IDfromName`.
- Test helpers `sel` and `strptr` are defined once in `field_test.go` (Task 1) and reused by later test files in the same package. The fixtures `validStatic` and `validCatalog` are defined once in `parse_test.go` (Task 2) and reused in Tasks 3–4.
