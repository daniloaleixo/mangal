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
