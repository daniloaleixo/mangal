# Running mangal

A quick start for building and running this fork from source.

## 1. Prerequisites

- **Go 1.18 or newer** — check with `go version`
- **make** — optional, but the commands below assume it (`sudo apt install make` on Debian/Ubuntu)

Dependencies are vendored in `vendor/`, so there is nothing to download.

## 2. Build

```sh
make build
```

This produces a `mangal` binary in the repo root (it's gitignored).

**No make?** Run the equivalent directly:

```sh
go build -ldflags="\
-X 'github.com/metafates/mangal/constant.BuiltAt=$(date -u)' \
-X 'github.com/metafates/mangal/constant.BuiltBy=$(whoami)' \
-X 'github.com/metafates/mangal/constant.Revision=$(git rev-parse --short HEAD)' \
-s -w"
```

Plain `go build` also works — it just leaves the version metadata empty.

Verify it:

```sh
./mangal version
```

## 3. Run

```sh
./mangal
```

Launches the interactive TUI: pick a source, search, select chapters, download.

Other modes:

```sh
./mangal mini                   # minimal interactive mode
./mangal inline --help          # scripting mode, JSON output
./mangal sources list           # show available sources
./mangal where --downloads      # where files get saved
```

## 4. Test

```sh
make test          # or: go test ./...
```

Some built-in source tests hit live websites and fail when those sites change
or are unreachable. Failures in `manganelo`, `manganato`, or `mangapill` are
usually this, not your changes. Everything else should pass offline.

Run one package's tests:

```sh
go test ./provider/declarative -v
```

## 5. Add a site without recompiling

Sites can be described in a TOML file — no Go code, no rebuild. An example
for a single-series site ships with the repo:

```sh
cp assets/examples/theclimber.toml "$(./mangal where --sources)/"
./mangal sources list      # "theclimber" now appears under Custom
```

Then `./mangal`, pick `theclimber`, and press enter on an empty search.

To remove it again:

```sh
./mangal sources remove --name theclimber
```

See the **Declarative scrapers** section of [README.md](README.md) for the full
config format. For sites that build their reader in JavaScript, you need a Lua
source instead — `./mangal sources gen`.

## 6. Download from the command line

Inline mode downloads without the TUI. This grabs chapters 48–58 as PDFs:

```sh
./mangal inline \
  --source theclimber \
  --query climber \
  --manga first \
  --chapters "47-57" \
  --format pdf \
  --download
```

**Chapter selectors are 0-based, but chapters are numbered from 1** — so the
selector is always one less than the chapter number you want. `47-57` gives you
Chapter 48 through 58. Ranges include both ends.

| selector | meaning |
|---|---|
| `47-57` | a range by index — chapters 48–58 |
| `0` | a single chapter by index — chapter 1 |
| `first`, `last`, `all` | what they say |
| `@Chapter 5@` | name substring — **also matches 50–59**, use with care |

`pdf` is already the default (`formats.use`); the other options are `zip`,
`cbz`, and `plain`. Swap `--download` for `--json` to preview the selection
without fetching anything — the two flags are mutually exclusive.

Downloads land in `downloader.path`, which defaults to the **current
directory**. Check with `./mangal where --downloads`, and override it if you
don't want a `The_Climber/` folder appearing wherever you happened to run from:

```sh
MANGAL_DOWNLOADER_PATH=~/manga ./mangal inline --source theclimber ... --download
```

## Config

Config lives in `$(./mangal where --config)` as TOML. Every key can also be set
by environment variable: `MANGAL_` + the key with dots replaced by underscores,
uppercased. For example `downloader.path` becomes `MANGAL_DOWNLOADER_PATH`.

To experiment without touching your real setup, point `MANGAL_CONFIG_PATH` at a
throwaway **directory** (not a file):

```sh
MANGAL_CONFIG_PATH=/tmp/mangal-test ./mangal sources list
```
