# Configuration comparison methodology

This guide describes the common-map workloads, library options and reproduction
commands for the [Go parser comparison report](../multiline-allocation-2026-10-24/README.md).

This isolated Go module compares the current dotReg Go sample with JSON, TOML,
YAML and INI libraries. Every operation starts with preloaded UTF-8 bytes and
ends with the same application representation:

```go
map[string]map[string]string // section -> key -> decoded string
```

These are measurements of particular library versions, APIs and inputs. A
format's full expressive power, other implementations, invalid-input handling,
application schema validation and file system performance are outside this
comparison. The [benchmark guide](../README.md) explains how this workload
differs from native file-input measurements.

## Reproduction

Run from this directory with Go 1.22 or later. Dependencies are pinned by
`go.mod` and `go.sum`; the production dotReg module remains dependency-free.

```powershell
go test ./...
go vet ./...
go run ./cmd/configcompare -generate ./fixtures
go test -c -o ./.artifacts/comparison.test.exe
```

The generator validates each generated input against an independently generated
expected map before writing that file. Generated inputs are ignored
by Git to avoid committing large repetitive files. The tracked
[fixtures-manifest.json](fixtures-manifest.json) records exact byte counts,
SHA-256 hashes and a semantic fingerprint; regenerating with this source must
reproduce it. The fingerprint is SHA-256 over Go `encoding/json.Marshal` of the
expected map, whose map keys are sorted by the standard library.

For an isolated measurement campaign, build once and run serially. Every one
of the 36 scenario/operation pairs receives its own fresh process in each of
ten balanced rounds, giving **360 measured processes** for this configuration.
The external driver uses the recorded seed to shuffle all pairs in each round, then selects exactly
one operation in each child. Keep other CPU-intensive work idle.

Run the following command from the repository root after building above:

```powershell
$comparisonOutputDirectory = Join-Path $env:TEMP 'dotreg-comparison-2026-10-24'
New-Item -ItemType Directory -Path $comparisonOutputDirectory -Force | Out-Null
python implementations/benchmarks/run-go.py --binary implementations/benchmarks/comparison/.artifacts/comparison.test.exe --module implementations/benchmarks/comparison --bench '^BenchmarkConfig$' --scope common-map --order seeded --isolate --samples 10 --benchtime 200ms --seed 20261024 --manifest implementations/benchmarks/comparison/fixtures-manifest.json --output (Join-Path $comparisonOutputDirectory 'results.json') --raw (Join-Path $comparisonOutputDirectory 'results.txt')
```

The runner records the toolchain, environment, commands, seeds and source
hashes with the raw output. It fixes `GOMAXPROCS=1`, `GOGC=100` and
`GOMEMLIMIT=off`. An untimed one-iteration discovery process identifies all
operation names; each measured child must report exactly the selected
operation. Adaptive iteration counts are chosen by Go's `testing.B`; the
200 ms target is per operation per observation. These are short, warmed parses
with Go's adaptive calibration, not a guarantee of universal steady-state
behaviour. Compiler and process startup time are excluded. Record raw output
and report metadata outside the benchmark timer.

For a before/after comparison, follow the balanced paired protocol and
validation requirements in the
[Go multiline and allocation report](../multiline-allocation-2026-10-24/README.md).

For an external process-memory observer, build and run the single-operation
CLI. The command reads its input once before timing and checks exact semantics
before warmup and the final fingerprint after timing.

```powershell
go build -o ./.artifacts/configcompare.exe ./cmd/configcompare
./.artifacts/configcompare.exe -format dotreg-direct -fixture ./fixtures/large -iterations 100 -warmup 5
```

Choose `dotreg-direct`, `dotreg-collected`, `json`, `toml`, `yaml` or `ini`.
The CLI explicitly sets one Go execution thread, `GOGC=100` and an unlimited Go
memory limit before validation. Its allocated bytes and allocation counts cover
the timed parses, and its JSON identifies these settings. A process peak
working set measured externally includes runtime, preloaded encoded input,
expected map, validation, warmup and transient outputs from successive loads;
it is not parser-retained memory. This CLI does not itself report peak RSS.

## Equivalent workloads

| Scenario | Sections | Strings per section | Total strings | Content |
|---|---:|---:|---:|---|
| `small` | 4 | 8 | 32 | ASCII values |
| `medium` | 32 | 16 | 512 | ASCII values |
| `large` | 128 | 64 | 8,192 | ASCII values |
| `unicode` | 32 | 16 | 512 | Japanese names/values, accented Latin, emoji, distinct composed/decomposed text |
| `escaped` | 32 | 16 | 512 | Quotes, backslashes, `#`, `;`, `=`, tabs and significant surrounding spaces |
| `multiline` | 32 | 16 | 512 | Three LF-separated lines, preserved internal indentation and Japanese text |

Every format has the same exact names and decoded values for a scenario.
`reflect.DeepEqual` checks every map entry; count agreement alone is
insufficient. Each operation has additional string-contract tests for `001`,
`true`, `null`, an empty string, case-sensitive keys, interpolation-looking
`%(name)s`, quotes, backslashes, tabs, comment markers, Unicode and multiline
text. No string is normalised or converted to another type.

Fixtures use UTF-8, LF line endings, a final LF and a stable lexical ordering.
No comments, BOM, alignment padding or duplicate keys are included. dotReg
includes the mandatory `@dotReg 1` declaration and blank separator. There are
no unset values, type annotations, repeated occurrences, arrays, aliases,
application profiles, prefix selection or schema validation. Source-order
retention is not required by the common map model. The valid inputs do not
establish equivalence of error handling or Unicode validation between libraries.

The hand-written fixture encoders use ordinary readable syntax with minimal
necessary quoting. dotReg and INI use plain single-line strings where that
preserves the value. TOML strings require quotes. YAML plain strings are limited
to a safe letter-leading subset and exclude resolver words; ambiguous values
are quoted. JSON strings always use its required quotes. TOML Unicode names
are quoted; YAML and dotReg generated names are safe plain names. Each library
receives its natural input size: padding files to equal bytes would change the
parsing work while preserving no useful application contract.

Multiline strings use native forms: dotReg pipe lines, TOML multiline literal
strings, YAML literal blocks with `|-`, and go-ini triple quotes. JSON uses
escaped LF characters. The fixture subset has no trailing multiline LF, triple
apostrophe or triple double-quote sequence. These are limits of the benchmark's
small encoders, not claims about the formats' full capabilities.

The logical payload size sums UTF-8 byte lengths of section name, key name and
decoded value **for each setting**. A shared section name is counted once per
setting, consistently across formats. It is a workload measure rather than a
compressed document size.

## Timed operations and options

| Operation | Timed work |
|---|---|
| `dotreg-direct` | `ParseEach` -> `DecodeString` -> fresh common map |
| `dotreg-collected` | `Parse` into syntax entries -> `DecodeString` -> fresh common map |
| `json` | standard-library `encoding/json.Unmarshal` directly into the common map |
| `toml` | `toml.Unmarshal` directly into the common map |
| `yaml` | `yaml.Unmarshal` directly into the common map |
| `ini` | `ini.LoadSources` into its native file model -> copy names and `Key.Value()` into a fresh common map |

dotReg's direct operation avoids retaining all syntax entries; it still
materialises every decoded setting. Its collected operation also materialises
every setting and includes the intermediate entry collection. Neither is a
syntax-only or discard-only parse. The dotReg adapter checks the required
two-element path and absence of unset/type-annotated values while constructing
the map. Other library decoders enforce their string map target during decode.
No parser receives oracle-derived capacity hints, reused destination maps or
cached parse results. Each iteration builds a fresh result retained in a global
sink; the previous sink stays reachable while the next result is being built.

JSON, TOML and YAML use the pinned library's default `Unmarshal` options. No
unsafe scanner, custom tokenizer, alternate JSON implementation, schema
inference, post-parse type conversion or custom library fork is substituted.

INI uses `LoadOptions{IgnoreContinuation: true, IgnoreInlineComment: true,
KeyValueDelimiters: "="}`; all other options are their zero values. Sections and
keys stay case-sensitive, surrounding quote removal remains enabled, shadow
keys and Python multiline mode are disabled. Native triple quotes preserve
literal special characters and LF text. The adapter calls `Key.Value()`, which
avoids the interpolation performed by `Key.String()`. The empty implicit
default section is omitted. The native file model and common-map copy are both
timed; the library still builds its own ordering/comment metadata. INI has no
single universal specification, so this result identifies this precise dialect.

Input generation, serialisation, input-file I/O, expected-map creation, semantic
equality checks and SHA-256 work are all outside the benchmark timer. The map
construction and necessary adaptations above are inside it. Garbage collection
remains enabled at the same setting for all operations; the harness does not
force GC inside the timed loop.

## Metrics and interpretation

Use `ns/op` (or ms per configuration load) and `values/s` as the primary latency
and application-throughput metrics. `logical-B/s` normalises throughput to
identical decoded content. `values/op` and `logical-B/op` identify the workload.
`SetBytes` additionally reports Go's `MB/s` using **encoded input bytes**; that
diagnostic is not a fair standalone ranking when serialisation sizes differ.

`B/op` is total Go heap allocation traffic per load, and `allocs/op` is the
number of allocations. Neither is retained heap size or process peak memory.
Allocation traffic is useful evidence about GC pressure and memory efficiency.

Compare repetitions and spread, rather than assigning significance to a small
median difference. Results on one CPU and one runtime do not establish a
universal ranking between formats or libraries. This fixed, valid two-level
string workload also does not measure deeper nesting, rich types, invalid
documents or all application access patterns.

## Pinned libraries and primary sources

Versions were resolved for the selected stable major lines on 24 October 2026.
The Go toolchain version determines `encoding/json`; record it with results.

- JSON: [Go `encoding/json.Unmarshal`](https://pkg.go.dev/encoding/json#Unmarshal).
- TOML: `github.com/pelletier/go-toml/v2 v2.4.3`,
  [official release](https://github.com/pelletier/go-toml/releases/tag/v2.4.3)
  and [decoder source](https://github.com/pelletier/go-toml/blob/v2.4.3/unmarshaler.go).
- YAML: `go.yaml.in/yaml/v3 v3.0.5`,
  [versioned package/API and maintenance notes](https://pkg.go.dev/go.yaml.in/yaml/v3@v3.0.5)
  and [YAML organisation repository](https://github.com/yaml/go-yaml).
- INI: `gopkg.in/ini.v1 v1.67.3`,
  [official release](https://github.com/go-ini/ini/releases/tag/v1.67.3),
  [load options](https://github.com/go-ini/ini/blob/v1.67.3/ini.go),
  [literal and multiline parsing](https://github.com/go-ini/ini/blob/v1.67.3/parser.go)
  and [raw versus interpolated value access](https://github.com/go-ini/ini/blob/v1.67.3/key.go).
