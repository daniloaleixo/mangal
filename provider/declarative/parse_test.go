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
