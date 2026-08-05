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
