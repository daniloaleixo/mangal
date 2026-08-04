package provider

import (
	"github.com/metafates/mangal/filesystem"
	"github.com/metafates/mangal/log"
	"github.com/metafates/mangal/provider/custom"
	"github.com/metafates/mangal/provider/declarative"
	"github.com/metafates/mangal/source"
	"github.com/metafates/mangal/util"
	"github.com/metafates/mangal/where"
	"path/filepath"
)

type Provider struct {
	ID           string
	Name         string
	UsesHeadless bool
	IsCustom     bool
	CreateSource func() (source.Source, error)
}

func (p Provider) String() string {
	return p.Name
}

func Builtins() []*Provider {
	return builtinProviders
}

func Customs() []*Provider {
	files, err := filesystem.Api().ReadDir(where.Sources())

	if err != nil {
		return make([]*Provider, 0)
	}

	providers := make([]*Provider, 0, len(files))
	// stems tracks the first file seen for each name so a same-stem .lua and
	// .toml pair (e.g. "foo.lua" and "foo.toml") can be flagged: both are
	// kept (neither is silently dropped), but Get and the TUI can only
	// disambiguate them by ID, so the collision is worth surfacing.
	stems := make(map[string]string)

	for _, file := range files {
		path := filepath.Join(where.Sources(), file.Name())
		name := util.FileStem(path)

		if seenAs, ok := stems[name]; ok {
			log.Warnf(
				"custom source name %q is used by both %s and %s; only the first one found will be resolved by name",
				name, seenAs, file.Name(),
			)
		} else {
			stems[name] = file.Name()
		}

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

func Get(name string) (*Provider, bool) {
	for _, provider := range Builtins() {
		if provider.Name == name {
			return provider, true
		}
	}

	for _, provider := range Customs() {
		if provider.Name == name {
			return provider, true
		}
	}

	return nil, false
}
