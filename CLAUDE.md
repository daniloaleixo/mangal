# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`mangal` is a CLI manga downloader (Go, module `github.com/metafates/mangal`). Upstream is unmaintained as of April 2025; this checkout is a personal fork.

## Commands

```sh
make build      # go build with ldflags injecting constant.BuiltAt/BuiltBy/Revision
make install    # go install, same ldflags
make test       # go test ./...
```

Build without `make` only if you don't need version metadata — `constant.Revision` etc. stay empty.

CI runs `go test -race -v ./...` (no race detector on Windows) with `GOFLAGS=-mod=readonly`, on Go 1.18. go.mod declares 1.18; release builds use 1.19.

Single test: `go test ./anilist -run TestSearch -v`. Tests use GoConvey with a dot-import, so `-run` only matches the top-level `func Test*` — nested `Convey` blocks can't be filtered.

There is no linter or formatter config in this repo. `gofmt` and `go vet` are the baseline.

## Architecture

- `source.Source` (`source/source.go`) is the scraping interface: `Name`, `ID`, `Search`, `ChaptersOf`, `PagesOf`. Everything downstream (`downloader/`, `tui/`, `mini/`, `inline/`) works against it.
- `provider/` wraps sources in a `Provider` descriptor with a lazy `CreateSource` func. Built-ins are registered in `provider/init.go`; custom Lua sources are discovered at runtime by scanning `where.Sources()` for `*.lua`.
- Most built-in sources are *not* new code — `manganelo`, `manganato`, `mangapill` are just `*generic.Configuration` values (CSS selectors + extractor funcs) appended to the loop in `provider/init.go`. Only `mangadex` has bespoke logic. Add a scraper the generic way unless the site needs an API client.
- Custom sources are Lua 5.1 files (gopher-lua) that must define three globals: `SearchManga`, `MangaChapters`, `ChapterPages`. Lua↔Go conversion is a declarative field table in `provider/custom/translator.go`. `mangal sources gen` renders `constant.SourceTemplate`, not a file on disk.
- `converter/` maps a format name to a `Converter` (`plain`, `cbz`, `pdf`, `zip`) via the map in `converter/converter.go`.

## Config system

- Every config key is a string const in `key/keys.go`; its default and description live in the `defaults` array in `config/default.go`.
- **Adding a key requires bumping `key.DefinedFieldsCount`.** `config/default.go`'s `init()` panics at startup if the array length doesn't match it, or on a duplicate key.
- Env vars are derived automatically: `MANGAL_` prefix, `.` → `_`, uppercased. `downloader.path` → `MANGAL_DOWNLOADER_PATH`.
- Config is TOML via viper, read from `where.Config()`; `MANGAL_CONFIG_PATH` overrides the location.

## Conventions

- All file I/O goes through `filesystem.Api()` (afero), never `os` directly — `filesystem.SetMemMapFs()` swaps in an in-memory FS, and viper is pointed at the same wrapper.
- `where.*()` helpers create their directories as a side effect of being called.
- Commit messages follow Conventional Commits. `.goreleaser.yaml` groups release notes by prefix: `feat:`, `fix:`, `docs:`, `feat(deps):`; `test`/`chore`/`refactor`/`build` are excluded from release notes.
- Releasing bumps `constant.Version` and prepends a section to `CHANGELOG.md` — the top `## X.Y.Z` section becomes the GitHub release notes verbatim.
