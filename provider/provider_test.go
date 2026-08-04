package provider

import (
	"path/filepath"
	"testing"

	"github.com/metafates/mangal/filesystem"
	"github.com/metafates/mangal/provider/manganelo"
	"github.com/metafates/mangal/where"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGet(t *testing.T) {
	Convey("When trying to get a valid provider", t, func() {
		_, ok := Get(manganelo.Config.Name)
		Convey("Then ok should be true", func() {
			So(ok, ShouldBeTrue)
		})
	})

	Convey("When trying to get an invalid provider", t, func() {
		_, ok := Get("kek")
		Convey("Then ok should be false", func() {
			So(ok, ShouldBeFalse)
		})
	})
}

func TestCustomsDiscoversTomlSources(t *testing.T) {
	Convey("Given a sources directory with a .toml and a .lua file", t, func() {
		filesystem.SetMemMapFs()
		defer filesystem.SetOsFs()

		dir := where.Sources()
		So(filesystem.Api().MkdirAll(dir, 0755), ShouldBeNil)
		So(filesystem.Api().WriteFile(filepath.Join(dir, "theclimber.toml"), []byte("base_url = \"x\""), 0644), ShouldBeNil)
		So(filesystem.Api().WriteFile(filepath.Join(dir, "somelua.lua"), []byte("-- lua"), 0644), ShouldBeNil)
		So(filesystem.Api().WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0644), ShouldBeNil)

		customs := Customs()

		Convey("Then both sources are discovered and the .txt is ignored", func() {
			byName := make(map[string]*Provider, len(customs))
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
