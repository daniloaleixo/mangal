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
