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
