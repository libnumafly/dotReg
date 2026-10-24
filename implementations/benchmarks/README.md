# Current-specification benchmarks

This directory measures the current [dotReg specification](../../docs/config-format.md)
and its [sample implementations](../README.md). The campaign dated 24 October
2026 has three measurement tiers. They answer different questions and use
different output contracts, so their timings are presented separately.

**The latest [Go parser tuning follow-up](tuning-2026-10-24/README.md) remeasures
the original and optimised implementations together against the same inputs and
libraries.** The tables on this overview page retain the initial campaign before
that tuning, including the seven-engine native comparison. The follow-up reports
the current Go parser results and controlled before/after changes separately.

| Tier | Question | Timed result |
|---|---|---|
| Native parser APIs | How do the seven engines handle the same dotReg documents in streaming and collecting modes? | Syntax plus common string decoding, with buffered file reads |
| Other configuration formats | How much work does each selected Go library need to load equivalent string settings? | A fresh, identical two-level decoded string map |
| Current Go consumer API | What does the ordered consumer and prefix selection cost? | A built `Config`, or selected occurrences from an already built `Config`, according to the benchmark name |

TypeScript shares the JavaScript engine, giving eight language interfaces and
seven engines in the native tier. The Go consumer tier covers the current
[section 9.1 prefix-selection contract](../../docs/config-format.md#91-prefix-selection).
That consumer API is implemented in Go; the other samples currently cover
syntax and common string decoding. Prefix selection is outside both the native
parser timings and the other-format comparison.

## Contents

- [Recorded data and measured source](#recorded-data-and-measured-source)
- [Native parser APIs](#native-parser-apis)
- [Equivalent configuration formats](#equivalent-configuration-formats)
- [Current Go consumer API](#current-go-consumer-api)
- [Reproduction from the repository root](#reproduction-from-the-repository-root)
- [Interpreting time and memory](#interpreting-time-and-memory)
- [Validation and CI](#validation-and-ci)
- [Historical single-fixture baseline](#historical-single-fixture-baseline)

## Recorded data and measured source

The following files retain the observations, runtime details, source hashes,
fixture hashes and reproduction settings:

- Native suite: [method and workload guide](native-2026-10-24/README.md),
  [all process samples and summaries](native-2026-10-24/report.json).
- Format comparison: [method and pinned libraries](comparison/README.md),
  [fixture manifest](comparison/fixtures-manifest.json),
  [all observations and summaries](comparison/results-2026-10-24.json),
  [raw Go benchmark output](comparison/results-2026-10-24.txt).
- Go API stages: [all observations and summaries](go-api-2026-10-24.json),
  [raw Go benchmark output](go-api-2026-10-24.txt),
  [ordered-load and prefix-selection source](../go/prefix_benchmark_test.go),
  [individual parser, decoder and build stages](../go/benchmark_test.go).

The initial campaign started from base commit
`e45c1ca433aeee61e45e278f6dcffccdb403f40c` and measured the working-tree changes
subsequently published in commit `d57788b518ae28240e2d2e65b3fd1527c21b43da`.
**The recorded SHA-256 manifest is
the authoritative identification of the measured source snapshot.** The base
commit alone does not identify that snapshot. Compiled binary hashes, exact
toolchains and flags are also retained where the runner records them.

All three tiers ran serially on the same Windows 11 Enterprise 10.0.26200
AMD64 host: Intel Core i7-1370P (14 cores, 20 logical processors),
16,819,617,792 bytes of physical RAM. The existing custom power scheme was
retained. The [native environment record](native-2026-10-24/README.md#conditions-and-validation)
provides the host observations and runtime versions. Desktop/OS activity and
thermal state were not continuously controlled.

The report keeps every observation. Native summaries include medians, ranges
and median absolute deviation; the Go reports include the median and range of
each metric. Small differences should be assessed against that spread.

Earlier Go batch runs showed a substantial slowdown associated with later
operation positions in a process, across different parsers. Its cause remains
unidentified. Every Go operation was therefore rerun using a separate fresh
process per observation. The accepted reports use that revised protocol for
the complete operation set; they do not mix batch observations with isolated
observations or select individual outliers to remove.

The earlier complete batches remain available as method diagnostics only:
[format-comparison JSON](comparison/diagnostic-batch-2026-10-24.json),
[format-comparison raw output](comparison/diagnostic-batch-2026-10-24.txt),
[Go API JSON](diagnostic-go-api-batch-2026-10-24.json),
[Go API raw output](diagnostic-go-api-batch-2026-10-24.txt), and the
[original batch driver](diagnostic-batch-driver.py). They support inspection
of the protocol change and are excluded from the reported performance ranking.

## Native parser APIs

The native suite runs Go, Python, JavaScript, Java, C#, Rust and C++ against the
same valid UTF-8/LF documents, including the required `@dotReg 1` declaration
and blank separator. Each parser's complete syntax and decoded output is checked
against independently constructed expected entries before measurement.

| Workload | Content | Timed passes per process |
|---|---|---:|
| `mixed-32` | 32 values: repeated keys, annotations, Unicode, unset values and both multiline forms | 1,000 |
| `mixed-512` | The same mix at 512 values | 50 |
| `mixed-20000` | The same mix at 20,000 values | 3 |
| `line-1mib` | One value with a 1,048,576-byte literal payload on one physical line | 3 |
| `blocks-32kib` | One pipe block and one triple-quoted block, each with 32,768 payload bytes and 31 LF separators | 20 |
| `depth-256` | 256 heading levels, with a value at every level | 10 |

Each engine runs both `stream` and `collect`. Five fresh process samples per
condition give **420 process jobs**: seven engines × six documents × two modes
× five samples. Each sample round contains every condition once; a recorded
seed shuffles its order. The suite executes jobs serially.

Every process performs two untimed warmup passes and then the fixed number of
timed passes for that document. The number of passes is identical across
engines and modes within a document. The file is opened once, then rewound and
read through the implementation's normal file buffers on every pass. Inputs
are normally served from the operating system's file cache after warmup;
uncached disk performance is outside this measurement.

Streaming decodes and consumes each complete occurrence without retaining past
entries. Collecting retains syntax entries and then decodes their values. Entry
JSON output is excluded from both timed paths. Both modes still need the active
heading state, current expanded path and complete current multiline body; a
single large line or block can dominate their working memory.

For the 20,000-value document (1,183,212 bytes), the median observations are:

| Engine | Stream, ms/pass | Collect, ms/pass | Stream peak, MiB | Collect peak, MiB |
|---|---:|---:|---:|---:|
| Go | 6.33 | 8.77 | 12.50 | 21.75 |
| Python | 176.50 | 166.86 | 15.59 | 22.79 |
| JavaScript / TypeScript | 30.02 | 34.40 | 55.04 | 125.12 |
| Java | 23.80 | 21.30 | 185.88 | 196.36 |
| C# | 21.94 | 31.66 | 45.71 | 55.12 |
| Rust | 9.25 | 13.48 | 5.08 | 12.28 |
| C++ | 10.01 | 13.69 | 4.57 | 10.62 |

The [full native tables](native-2026-10-24/README.md#time-and-process-memory)
retain minimum/maximum ranges and all six documents. On this large mixed input,
streaming used less observed process memory in every engine; Python and Java
collecting were faster. This does not extend to every workload: with one large
value, a complete value body is needed in either mode and their memory can be
similar or reversed. These figures compare sample implementations and runtimes,
not language-independent parser algorithms.

Native measurements use each runtime's default GC and runtime settings. Two
warmups and a finite number of passes do not guarantee steady state in a JIT
runtime. Heap growth, compilation, scheduling and background desktop activity
can affect a particular sample. These observations describe this finite-load
policy, with runtime versions and flags recorded in the native report.

## Equivalent configuration formats

The separate [comparison module](comparison/README.md) runs all libraries under
the same Go toolchain. It compares dotReg with standard-library JSON,
`go-toml/v2` TOML, YAML-organisation `yaml/v3`, and `go-ini` INI. Exact dependency
versions and source links are recorded in that module, leaving the sample Go
parser free of external dependencies.

Every operation loads the same semantic configuration into
`map[string]map[string]string`: section name → key name → decoded string. The
six cases contain 32, 512 and 8,192 ASCII settings, plus 512-setting Unicode,
escaped and multiline configurations. All names and decoded values are checked
for exact equality against an independent oracle, including significant spaces,
LF characters, quotes, backslashes and Unicode.

There are six measured operations across five formats: dotReg's direct visitor
and collected-entry paths, JSON, TOML, YAML and INI. Both dotReg paths construct
the complete application map. INI includes its native file model and the
necessary common-map adaptation. This comparison does not place a syntax-only
dotReg parse beside a full deserialisation in another library.

All input bytes are preloaded; fixture construction, encoding, file I/O and
semantic verification are outside the timer. Each iteration creates a fresh
result. Each of ten balanced rounds runs the 36 case/operation pairs once,
with a separate fresh Go test-binary process for each pair: **360 measured
processes** in total. The external driver shuffles all pairs in each round
using a recorded seed and selects exactly one operation in each child process.
Go's adaptive calibration uses a 200 ms timing target for that operation.
`GOMAXPROCS=1`, `GOGC=100` and `GOMEMLIMIT=off` are fixed across these Go
measurements; GC remains enabled and is not forced inside the timed loop.
These short, warmed processes do not establish universal steady-state behaviour.

Use time per complete configuration load and settings per second for the main
comparison. `logical-B/s` uses the same decoded UTF-8 content for all formats.
The manifest also records each format's encoded byte size. Go's additional
`MB/s` metric uses those encoded bytes, so it is a diagnostic rather than a
standalone ranking between different serialisations. `B/op` and `allocs/op`
measure heap allocation traffic and counts per load.

The shared model excludes duplicate keys, unset values, annotations, rich
types, arrays, aliases, application profiles and ordered occurrence retrieval.
The encoders use ordinary native forms, including each format's supported
multiline strings. The [detailed method](comparison/README.md#equivalent-workloads)
states the quoting policy and INI dialect options. This valid-input subset
does not establish equal functionality, error handling or validation behaviour
between formats.

The accepted isolated run gives the following median milliseconds per complete
configuration load. Each cell summarises ten independent process observations;
the [detailed tables](comparison/README.md#recorded-results) retain every
minimum/maximum range and the allocation counters.

| Case (settings) | dotReg direct | dotReg collected | JSON | TOML | YAML | INI |
|---|---:|---:|---:|---:|---:|---:|
| small (32) | 0.0104 | 0.0119 | 0.0063 | 0.0079 | 0.0421 | 0.0085 |
| medium (512) | 0.1676 | 0.1668 | 0.1092 | 0.1585 | 0.6806 | 0.1850 |
| large (8,192) | 3.0040 | 4.0774 | 1.9315 | 3.6028 | 12.2554 | 4.2438 |
| unicode (512) | 0.2206 | 0.2192 | 0.1625 | 0.1947 | 0.7741 | 0.2027 |
| escaped (512) | 0.2547 | 0.2545 | 0.1897 | 0.1868 | 0.8058 | 0.1933 |
| multiline (512) | 0.2848 | 0.2860 | 0.1944 | 0.1564 | 0.7004 | 0.2797 |

For the 512-setting ASCII case, median heap allocation traffic per load is:

| Operation | KiB/op | allocs/op |
|---|---:|---:|
| dotReg direct | 149.55 | 2,446 |
| dotReg collected | 277.31 | 2,456 |
| JSON | 91.67 | 1,356 |
| TOML | 94.14 | 1,258 |
| YAML | 370.51 | 7,374 |
| INI | 353.20 | 2,990 |

With these library versions, APIs and encodings, JSON has the lowest median
time in the ASCII and Unicode cases; TOML has the lowest in the escaped and
multiline cases. dotReg direct allocates fewer bytes than dotReg collected in
every case. Their latency ranges overlap in the 512-setting cases, so their
small median differences do not establish a consistent timing advantage.
At 8,192 settings, the direct path also has a materially lower observed time
range than the collected path. These are results for the current Go APIs and
serialisations, rather than intrinsic or universal properties of the notations.

## Current Go consumer API

The current consumer maintains repeated occurrences in global source order.
Its dedicated fixtures interleave unrelated branches, prefix values and
descendants, typed and untyped values, explicit empty strings, unset values,
quoted element boundaries and empty-name elements. The expected values and
selection matches are constructed independently of `Config` internals.

`BenchmarkParseBuildOrdered` measures parsing and building a fresh ordered
`Config` for 512 and 8,192 occurrences. `BenchmarkSelectPrefix` measures selection
from an already built `Config`; parsing and building are outside that timer.
Selection conditions cover all values via a nil prefix, all values via an empty
prefix, the prefix's own values and descendants, a selective descendant, no
matches, a quoted element boundary, an empty-name element and an empty `Config`.
Results include source order, full and relative paths, values and occurrence
metadata. A prefix's own value has an empty relative path.

This tier also records the separate parser, string-decoder and build stages
identified by their benchmark names. Their output contracts differ; a decoder
alone is not a complete configuration load. Prefix selection reports stored
and selected occurrence counts, without input-byte throughput, because it
does not read or parse text. A no-match operation should be assessed in time
and allocations per selection rather than selected values per second.

The Go API report uses the same runtime/GC controls and 200 ms timing target
as the format comparison. Its 28 operations each run in a separate fresh
process in every one of ten balanced rounds: **280 measured processes** in
total. The external driver shuffles the operation order with a recorded seed
and selects one exact benchmark per process. This control does not require
internal shuffling in the API benchmark functions. Parsing, decoding, building
and selection keep their individual scopes, identified by their benchmark names.

The accepted API run used Go 1.27.0 on Windows amd64 from 11:35:29 to
11:37:08 +09:00 on 24 October 2026. The following tables show median
microseconds per operation and the minimum–maximum across ten observations.
KiB/op is median heap allocation traffic (1 KiB = 1,024 bytes), rather than
retained heap or process peak. The raw reports retain the unrounded values.

For the repeated-occurrence fixtures, parsing and building the complete ordered
consumer produced:

| Occurrences | Median µs/load | Min–max µs/load | KiB/op | allocs/op |
|---|---:|---:|---:|---:|
| 512 | 243.920 | 224.771–255.415 | 358.20 | 3,694 |
| 8,192 | 5,591.377 | 5,390.888–6,428.015 | 8,021.33 | 57,511 |

Selection below starts with that consumer already built. `All` supplies a nil
prefix; `EmptyPrefix` supplies a zero-element slice. The empty-name condition
instead supplies a prefix containing an actual empty key element.

| Stored occurrences | Selection | Returned | Median µs/op | Min–max µs/op | KiB/op | allocs/op |
|---|---|---:|---:|---:|---:|---:|
| 512 | `All` | 512 | 63.966 | 61.292–82.282 | 200.78 | 2,218 |
| 512 | `EmptyPrefix` | 512 | 63.721 | 60.256–76.757 | 200.78 | 2,218 |
| 512 | `BaseAndDescendants` | 320 | 33.118 | 32.611–35.064 | 101.30 | 1,290 |
| 512 | `SelectiveDescendant` | 32 | 4.633 | 4.566–4.783 | 11.30 | 135 |
| 512 | `NoMatch` | 0 | 0.885 | 0.858–0.961 | 0.02 | 1 |
| 512 | `QuotedElementBoundary` | 32 | 4.512 | 4.457–5.242 | 11.80 | 167 |
| 512 | `EmptyName` | 64 | 7.977 | 7.733–8.905 | 21.79 | 264 |
| 8,192 | `All` | 8,192 | 2,169.294 | 2,097.976–2,444.237 | 4,319.28 | 35,346 |
| 8,192 | `EmptyPrefix` | 8,192 | 2,242.771 | 2,109.972–2,459.939 | 4,319.28 | 35,346 |
| 8,192 | `BaseAndDescendants` | 5,120 | 1,165.771 | 1,112.215–1,290.600 | 2,327.30 | 20,497 |
| 8,192 | `SelectiveDescendant` | 512 | 113.532 | 110.381–126.652 | 191.30 | 2,059 |
| 8,192 | `NoMatch` | 0 | 15.009 | 14.710–16.339 | 0.02 | 1 |
| 8,192 | `QuotedElementBoundary` | 512 | 113.609 | 111.248–118.145 | 199.30 | 2,571 |
| 8,192 | `EmptyName` | 1,024 | 193.679 | 185.427–198.842 | 335.29 | 4,108 |
| 0 | `EmptyConfig` | 0 | 0.004 | 0.004–0.004 | 0.00 | 0 |

The current implementation scans the global occurrence index even when nothing
matches. `NoMatch` therefore grows from about 0.885 µs at 512 stored occurrences
to 15.01 µs at 8,192. Matching also allocates the result records and independent
full/relative path slices: selecting all 8,192 occurrences takes a median
2.169 ms and allocates about 4.22 MiB per call. These are useful current costs
for future implementation work; this campaign makes no before/after claim.

The remaining stage measurements use their separately named fixtures. `Build`
starts with pre-parsed entries, `ParseDecode` parses and decodes common strings,
and `ParseBuild` includes the consumer build. `ParseEach` measures syntax with a
visitor, while `DecodeTriple` starts with an already parsed block body. Their
scope differs from both the common-map comparison and the ordered-load fixture.

| Benchmark stage | Median µs/op | Min–max µs/op | KiB/op | allocs/op |
|---|---:|---:|---:|---:|
| `BuildUntyped512` | 106.450 | 102.284–110.499 | 226.09 | 1,301 |
| `DecodeTriple32KiB/Escaped` | 51.004 | 49.243–52.597 | 110.74 | 19 |
| `DecodeTriple32KiB/Plain` | 7.149 | 7.058–7.368 | 0.00 | 0 |
| `ParseBuild512` | 233.055 | 213.657–246.238 | 435.02 | 3,536 |
| `ParseDecode512` | 163.633 | 154.207–188.601 | 215.94 | 2,875 |
| `ParseEach/BareMixed512` | 98.427 | 97.505–111.005 | 77.11 | 2,089 |
| `ParseEach/LongRaw1MiB` | 222.129 | 213.103–231.597 | 2,075.55 | 273 |
| `ParseEach/PipeLiteral32KiB` | 56.827 | 54.809–62.136 | 190.20 | 807 |
| `ParseEach/QuotedMixed512` | 114.397 | 113.505–119.401 | 85.17 | 2,609 |
| `ParseEach/TripleEscaped32KiB` | 73.559 | 70.519–82.621 | 258.77 | 818 |
| `ParseEach/TriplePlain32KiB` | 58.718 | 56.742–61.595 | 260.07 | 623 |

## Reproduction from the repository root

Use the exact installed toolchains and record their versions. The Windows
native preparation script expects Go on `PATH`, Python, Node.js, a JDK, .NET 8,
GNU-target Rust and LLVM-MinGW C++. Its inputs are explicit local paths; the
published report omits those executable paths. Resolve the actual compiler
executables, especially an underlying GNU Rust toolchain rather than a shim.

```powershell
$benchDirectory = Join-Path $env:TEMP 'dotreg-benchmark-2026-10-24'
$pythonExe = (Get-Command python).Source
$nodeExe = (Get-Command node).Source
$jdkDirectory = Split-Path (Split-Path (Get-Command javac).Source)
$dotnetExe = (Get-Command dotnet).Source
$rustcExe = (Get-Command rustc).Source
$cxxExe = (Get-Command clang++).Source

& $pythonExe implementations/benchmarks/native-2026-10-24/prepare_windows.py --directory $benchDirectory --python $pythonExe --node $nodeExe --jdk $jdkDirectory --dotnet $dotnetExe --rustc $rustcExe --cxx $cxxExe
& $pythonExe implementations/benchmarks/native-2026-10-24/matrix.py --config (Join-Path $benchDirectory 'toolchains.json') --fixtures (Join-Path $benchDirectory 'fixtures') --samples 5 --seed 20261024 --output (Join-Path $benchDirectory 'native-report.json')
Copy-Item -LiteralPath (Join-Path $benchDirectory 'native-report.json') -Destination implementations/benchmarks/native-2026-10-24/report.json
```

Inspect the generated local configuration's runtime descriptions and host
metadata before running on a different machine. The Windows preparation script
queries the supplied toolchains and current host; power and battery data are
timestamped observations rather than continuously controlled settings. On other
platforms, build via the language guides and create a configuration using
[config.example.json](native-2026-10-24/config.example.json). The
[native guide](native-2026-10-24/README.md) describes the configuration contract.

The Go format and API runners use the reusable test binaries below. Build all
binaries first, finish correctness checks, and then run each suite serially
without concurrent builds, tests or other benchmark suites.

```powershell
$goReportsDirectory = Join-Path $env:TEMP 'dotreg-go-reports-2026-10-24'
New-Item -ItemType Directory -Path $goReportsDirectory -Force | Out-Null
go -C implementations/benchmarks/comparison test ./...
go -C implementations/benchmarks/comparison test -c -o .artifacts/comparison.test.exe
go -C implementations/benchmarks/comparison run ./cmd/configcompare -generate ./fixtures
go -C implementations/go test ./...
go -C implementations/go test -c -o .artifacts/api.test.exe

python implementations/benchmarks/run-go.py --binary implementations/benchmarks/comparison/.artifacts/comparison.test.exe --module implementations/benchmarks/comparison --bench '^BenchmarkConfig$' --scope common-map --order seeded --isolate --samples 10 --benchtime 200ms --seed 20261024 --manifest implementations/benchmarks/comparison/fixtures-manifest.json --output (Join-Path $goReportsDirectory 'comparison-results.json') --raw (Join-Path $goReportsDirectory 'comparison-results.txt')
python implementations/benchmarks/run-go.py --binary implementations/go/.artifacts/api.test.exe --module implementations/go --bench '^Benchmark' --scope dotreg-api --order seeded --isolate --samples 10 --benchtime 200ms --seed 20261024 --output (Join-Path $goReportsDirectory 'go-api-results.json') --raw (Join-Path $goReportsDirectory 'go-api-results.txt')

Copy-Item -LiteralPath (Join-Path $goReportsDirectory 'comparison-results.json') -Destination implementations/benchmarks/comparison/results-2026-10-24.json
Copy-Item -LiteralPath (Join-Path $goReportsDirectory 'comparison-results.txt') -Destination implementations/benchmarks/comparison/results-2026-10-24.txt
Copy-Item -LiteralPath (Join-Path $goReportsDirectory 'go-api-results.json') -Destination implementations/benchmarks/go-api-2026-10-24.json
Copy-Item -LiteralPath (Join-Path $goReportsDirectory 'go-api-results.txt') -Destination implementations/benchmarks/go-api-2026-10-24.txt
```

[run-go.py](run-go.py) fixes the Go runtime environment, records the benchmark
flags, source and binary hashes, keeps raw output, and rejects a changing set
of operations between rounds. With `--isolate`, an untimed one-iteration
discovery process identifies the exact operation names; every measured child
must then report exactly its selected operation. Go's `testing.B` chooses the
iteration count for the requested duration. These Go timings exclude compiler
and process startup. Reports are written to local temporary storage during the
run and copied into the repository after completion, avoiding repeated writes
to the synchronised workspace while measurements are running. Report generation
and copying are outside the benchmark timer.

## Interpreting time and memory

The native table's time unit is milliseconds per full document pass; repeated
passes are divided by their fixed iteration count. The Go tables use time per
operation, whose scope is identified above. Compare like stages and include
the amount of semantic output when interpreting throughput.

The native [memory wrapper](measure.py) observes Windows
`PeakWorkingSetSize` or Linux `/proc` `VmHWM` every 5 ms. This high-water process
measure includes interpreter/VM memory, JIT code, reader buffers, startup,
warmup and allocator behaviour. Polling may miss the last rise before a short
process exits. Unsupported systems report unavailable memory as JSON `null`.
Process wall time includes startup and warmup; native `elapsed_seconds` covers
only the timed passes.

Streaming retains less historical parse output, but runtime GC and heap growth
can make observed RSS equal to or larger than collecting RSS in a sample.
Go's `B/op`, Go `TotalAlloc` deltas and C# allocation counters instead describe
cumulative allocation volume; they are neither live retained heap nor peak RSS.
Do not compare one metric's number as if it were another memory measure.

All measurements are observations on a single host. Power policy, CPU
scheduling, thermal behaviour and desktop activity are not completely
controlled. Cross-language results include each runtime's cost. Small timing
differences, one winning input or a single median do not establish a universal
language, parser or format ranking.

## Validation and CI

The native runner checks complete decoded occurrence output for every document
and engine before timing, then verifies the measured document hash, byte count,
entry count, iteration count, mode and engine on every process result. The format
comparison checks complete decoded maps against independent expectations. The
Go consumer fixture checks ordered values and selections, including boundary
and no-match cases, before timing.

Shared syntax/decoding [conformance tests](../conformance/README.md), native
API tests and the Go consumer tests remain separate correctness checks. CI
uses small benchmark smoke operations to verify harness execution and counts;
it has no time, allocation or process-memory threshold. Published performance
observations come from the serial runs described above.

## Historical single-fixture baseline

The [earlier single-fixture Windows report](2026-10-24-windows.json) is retained
unchanged as a historical snapshot. It used three process samples per engine
and mode, one 20,000-value fixture, two warmups and five timed passes. The current
20,000-value fixture is byte-for-byte identical; the harness source, pass count,
sample count and wider campaign differ. The earlier numbers are retained as a
historical observation and are not reused as current measurements or as an
otherwise controlled before/after performance comparison.

The original single-input tools remain available: [fixture.mjs](fixture.mjs)
generates that older document, [suite.py](suite.py) repeats one configured input,
and [measure.py](measure.py) records one child process's observed memory. For
example, the original fixture can still be generated with:

```powershell
node implementations/benchmarks/fixture.mjs (Join-Path $env:TEMP 'dotreg-legacy-benchmark.txt') 20000
```

Its commands and raw observations preserve the earlier baseline. Use the
current generators and source manifests above for present-specification
results.
