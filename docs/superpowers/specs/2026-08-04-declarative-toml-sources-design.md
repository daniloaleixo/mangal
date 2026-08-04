# Declarative TOML Sources

**Date:** 2026-08-04
**Status:** Approved, ready for implementation planning

## Problem

Adding a manga source to mangal currently costs more than it should.

There are two extension paths today, and each has a gap:

- **Compile-time Go configs** (`provider/generic/`). Purely declarative — a search URL builder plus three CSS-selector extractors. `manganelo`, `manganato`, and `mangapill` are each about 50 lines of configuration with no logic. But adding one means editing `provider/init.go` and rebuilding.
- **Runtime Lua sources** (`provider/custom/`). Loaded from `where.Sources()` with no rebuild, but they are real code: three functions, each performing its own HTTP request and HTML parsing.

The gap is a *runtime* path that is *declarative*. The existing Go configs prove most manga sites need no logic at all — only selectors. This design exposes that same capability as a TOML file loaded at runtime.

The motivating site is `theclimber.club`, which also exercises an edge the current model cannot express: it is a single-series site with no catalog to search.

## Goals

- Add a source by writing a TOML file into `mangal where --sources`, with no rebuild.
- Support both catalog sites (searchable) and single-series sites (nothing to search).
- Keep new code small and testable by reusing the existing scraper.
- Ship a working `theclimber.club` config as the first example.

## Non-Goals

- **JavaScript-rendered sites.** Colly does not execute JS. Sites whose readers build the page client-side remain the domain of Lua sources, which have headless Chrome available. The failure is made legible rather than silent (see Error Handling).
- **Extracting many pages from one `<script>` blob.** `generic` maps one matched element to exactly one page. Expanding a script blob containing many image URLs needs both a new flag and a bypass of that assumption. Deferred; it is a separate piece of work.
- **Migrating the existing built-in Go sources to TOML.** They work and are proven. Leave them.
- **A general expression language.** Selector plus attribute plus regex is the ceiling. Anything beyond that is what Lua is for.

## Approach

Compile the TOML into a `generic.Configuration` and hand it to the existing `generic.New()`.

The declarative layer is a **translator**, not a scraper. Each `Name`/`URL`/`Cover` function in `generic.Extractor` becomes a closure synthesized from its declarative field spec. Everything risky — colly setup, disk caching, rate limiting, header spoofing, chapter reversal — is existing code that already works. The new code is pure string manipulation.

Two rejected alternatives:

- **A standalone declarative scraper package.** Re-implements what `generic` already does correctly, and bugs fixed in one would not be fixed in the other.
- **Transpiling TOML to Lua.** Avoids new Go scraping code, but generated Lua is miserable to debug and errors surface as stack traces referencing code the user never wrote.

Two aspects of `generic` do not fit as-is:

1. **`Search` always performs an HTTP request.** For static configs, the declarative source is a decorator around the generic scraper. It implements `source.Source`, delegates `ChaptersOf`/`PagesOf`/`Name`/`ID` through, and overrides only `Search` to return the fuzzy-filtered static list. `source.Source` has five methods, so this is small.
2. **Discovery needs a source name before parsing.** The name comes from the filename stem, exactly as Lua sources do, and all parsing defers to `CreateSource`. A malformed config therefore appears in the source list and reports a real error when selected, rather than vanishing silently or crashing at startup.

## Config Format

The source name comes from the filename: `theclimber.toml` yields a source named `theclimber`.

### Single-series site (the motivating example)

```toml
base_url         = "https://theclimber.club"
reverse_chapters = true          # site lists newest first
delay_ms         = 50            # optional, default 50
parallelism      = 16            # optional, default 16

[[manga]]
name  = "The Climber"
url   = "https://theclimber.club/"
cover = "https://theclimber.club/wp-content/uploads/2024/07/The-Climber.jpg"

[chapters]
selector = "a[href*='-chapter-']"
  [chapters.name]              # no attr → element's text
  [chapters.url]
  attr = "href"

[pages]
selector = "img[src*='/manga/']"
  [pages.url]
  attr = "src"
```

### Catalog site

```toml
base_url   = "https://example.com"
search_url = "https://example.com/search?q={query}"   # {query} is URL-escaped

[search]
selector = "div.result"
  [search.name]
  selector = "h3 a"
  [search.url]
  selector = "h3 a"
  attr = "href"
  [search.cover]
  selector = "img"
  attr = "data-src"

[chapters]
selector = "div.chapter-list a"
  [chapters.name]
  [chapters.url]
  attr = "href"

[pages]
selector = "div.reader img"
  [pages.url]
  attr = "data-src"
```

`[search]` and `[[manga]]` deliberately use different key names so a config can never mix an extractor table with static entries.

### Field vocabulary

Four optional keys:

| key | meaning |
|---|---|
| `selector` | sub-selector within the matched item; omitted means the item itself |
| `attr` | attribute to read; omitted means text content |
| `regex` | applied to the raw value |
| `replace` | replacement template; requires `regex` |

Evaluation rule, in order:

1. Resolve `selector` if present, otherwise use the matched element.
2. Read `attr` if present, otherwise the element's text.
3. If `regex` and `replace` are both set, apply `ReplaceAllString`.
4. If only `regex` is set, take capture group 1, or the whole match when the pattern has no groups. No match yields an empty string.
5. Trim surrounding whitespace.

This covers pulling chapter numbers out of link text, stripping prefixes, and repairing protocol-relative URLs.

### Which fields each extractor needs

| table | required | optional |
|---|---|---|
| `[search]` | `name`, `url` | `cover` |
| `[chapters]` | `name`, `url` | `volume` |
| `[pages]` | `url` | — |

### Top-level keys

| key | required | default |
|---|---|---|
| `base_url` | yes | — |
| `search_url` | exactly one of `search_url` / `[[manga]]` | — |
| `[[manga]]` | exactly one of `search_url` / `[[manga]]` | — |
| `reverse_chapters` | no | `false` |
| `delay_ms` | no | `50` |
| `parallelism` | no | `16` |

`base_url` mirrors the existing `generic.Configuration.BaseURL` field, which feeds the `Host` request header. Chapter and page URLs are resolved against the request URL by colly's `AbsoluteURL`, so relative hrefs work regardless.

### Validation rules

- `base_url` must be present and non-empty.
- Exactly one of `search_url` or `[[manga]]` must be set. Neither is an error; both is an error.
- `[search]` is required when `search_url` is set, and rejected when it is not.
- `[chapters]` and `[pages]` are always required, each with its required sub-fields.
- Every `regex` must compile.
- `replace` without `regex` is an error.
- Unknown keys are rejected.

## Components

New package `provider/declarative/`:

| file | responsibility |
|---|---|
| `spec.go` | TOML schema structs: `Spec`, `ExtractorSpec`, `FieldSpec`, `StaticManga` |
| `parse.go` | read via `filesystem.Api()`, unmarshal, validate; returns `*Spec` or a descriptive error |
| `field.go` | `FieldSpec` → `func(*goquery.Selection) string`; the evaluation rule above |
| `compile.go` | `Spec` → `*generic.Configuration` |
| `source.go` | `LoadSource(path)` and the static-search decorator |

Dependencies run one direction: `parse` → `field` → `compile` → `source`. `field.go` holds the interesting logic and depends only on goquery, which makes it cheap to test exhaustively.

### Changes to existing code

Two files:

- `provider/init.go` — add `DeclarativeProviderExtension = ".toml"` beside the existing `.lua` constant.
- `provider/provider.go` — `Customs()` currently keeps only `.lua` files. It becomes a switch on extension: `.lua` builds the existing Lua provider, `.toml` builds one whose `CreateSource` calls `declarative.LoadSource(path)`. `UsesHeadless` is always false for TOML, so the headless byte-grep is skipped.

Nothing else changes. `mangal sources`, the `--source` flag, the TUI, inline mode, and the downloader all operate on `provider.Provider` and `source.Source`.

### Dependencies

`pelletier/go-toml/v2` and `lithammer/fuzzysearch` are already vendored as indirect dependencies. Promoting them to direct requires `go mod tidy && go mod vendor`, because `vendor/modules.txt` gates packages not marked explicit. No new downloads.

## Data Flow

1. **Discovery.** `provider.Customs()` scans `where.Sources()`, finds `theclimber.toml`, and builds `Provider{Name: "theclimber", ID: "theclimber toml", IsCustom: true, UsesHeadless: false}`. No parsing yet.
2. **Selection.** `CreateSource()` calls `declarative.LoadSource(path)`, which parses, validates, compiles a `*generic.Configuration`, and calls `generic.New`. Static configs are wrapped in the decorator.
3. **Search.** The decorator matches the query against each `[[manga]]` name using `fuzzy.MatchNormalizedFold` — case-insensitive and normalized — and returns the matches. An empty query returns all entries. Catalog configs instead let `generic` fetch `search_url` and run `[search]`.
4. **Chapters.** Delegated to `generic`: GET the manga URL, apply `[chapters]`, produce `*source.Chapter` values. `reverse_chapters` is applied by existing code.
5. **Pages.** Delegated to `generic`: GET the chapter URL, apply `[pages]`, produce `*source.Page` values.
6. **Download.** Unchanged mangal machinery.

The ID suffix `toml` is distinct from `built-in` and `custom` so cache and history keys cannot collide with a same-named Lua source.

Static `*source.Manga` values are constructed once at load, with their `Source` field pointing at the decorator rather than the inner scraper, so downstream calls route correctly.

## Error Handling

| condition | behavior |
|---|---|
| Malformed TOML or failed validation | `CreateSource` returns an error naming the file and the specific problem, e.g. `theclimber.toml: exactly one of search_url or [[manga]] must be set`. The source stays listed; selecting it reports the error. Nothing crashes at startup. |
| Unknown key | Rejected by strict decoding, naming the offending key. A typo such as `selectors` must not silently match nothing — that failure looks identical to the site changing its markup. |
| Invalid regex | Compiled during validation, so it fails at load rather than mid-download. |
| Zero search results | Returned as an empty list; existing UI handles it. |
| Zero chapters | Error naming the manga URL and the `[chapters]` selector. |
| Zero pages | Error reporting the count and pointing at Lua sources for JavaScript-driven readers, rather than writing an empty file. |
| Network failure | Propagated from colly unchanged. |

## Testing

Nearly all new code is pure string manipulation and tests without a network. All tests use goconvey, matching the existing 23 test files.

- `field_test.go` — table-driven over small HTML fixtures: text vs attribute, sub-selector, regex capture, regex with replace, no-match, trimming. The highest-value file.
- `parse_test.go` — validation: missing `base_url`, both search modes, neither, unknown key, uncompilable regex, `replace` without `regex`, missing `[pages]`.
- `compile_test.go` — correct selectors reach the correct extractors; defaults applied.
- `source_test.go` — decorator behavior: empty query returns all static manga, a partial-title query matches, an unrelated query does not, and delegation passes through.
- One end-to-end test against `httptest` serving saved HTML fixtures from `assets/testdata/`, exercising search → chapters → pages.

No live test against `theclimber.club`. The existing provider tests already scrape real sites and break when those sites change; this must not add to that.

## Deliverables

1. The `provider/declarative` package.
2. The two edits to `provider/init.go` and `provider/provider.go`.
3. `assets/examples/theclimber.toml`, with selectors written against the real markup.
4. Tests as above.
5. A README section documenting the format alongside the existing "Custom scrapers" section.

## Implementation Notes

**Selectors are unverified.** The example config's selectors must be written against raw HTML fetched during implementation. The design was informed by a summarized rendering of those pages, not the markup itself, so the first implementation task is fetching the homepage and one chapter page and confirming the actual element structure.

**Chapter numbering gaps.** `theclimber.club` lists chapters with gaps (no 17, 20, 24, and others). mangal's default filename template is `[{padded-index}] {chapter}`, so downloaded files are numbered contiguously while chapter names retain the real numbers. This is existing mangal behavior, not something this design changes. Users who find it confusing can set `downloader.chapter_name_template = "{chapter}"`.

## Open Question

**No Go toolchain is installed on the development machine.** `go` is not on `PATH`, and there is no install under `/usr/local/go` or `/usr/lib/go*`. The code can be written, but nothing can be compiled or tested until Go is installed. Resolve before implementation begins: either install Go, or accept that verification happens elsewhere.
