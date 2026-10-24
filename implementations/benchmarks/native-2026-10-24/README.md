# Native workload methodology

This guide describes the fixed native workloads, timing boundaries and
reproduction helpers for the [native parser report](../public-rollin-2026-10-24/README.md).
The [Go multiline and allocation report](../multiline-allocation-2026-10-24/README.md)
separately measures resident-input decoded-map loads, Go API costs and focused
assembly workloads. Its Go source snapshot differs from this native campaign's
recorded Go control; the two reports identify their own measured sources.

The native workload covers dotReg syntax and common string decoding in seven
engines; TypeScript shares the JavaScript engine. It includes the mandatory
`@dotReg 1` declaration, two-space heading indentation, quoted element
boundaries, repeated occurrences, opaque annotations, Unicode, unset values
and explicit multiline values. Long-line and deep-heading cases exercise the
absence of fixed size and depth limits at the specified sizes. The consuming
application's `Build`, type profile, retrieval and prefix-selection operations
are outside the timed work.

## Conditions and validation

Record the CPU, physical and logical core counts, memory, OS, power policy,
background workload and exact compiler/runtime versions for each campaign.
Point-in-time host and battery observations are snapshots rather than
continuous controls. Run benchmark jobs serially and keep other builds and
CPU-intensive tasks idle. The standard protocol uses default runtime and GC
settings, with no CPU affinity, process-priority change, forced GC or heap
budget. Record any departure from that policy rather than combining unlike
conditions.

Each engine/fixture/mode combination runs in a fresh process in every sample
round. The single-snapshot matrix uses five rounds by default, with two untimed
warm-up passes and the fixture's fixed number of timed passes in each process.
Iteration counts stay identical across engines and modes for a fixture. A
recorded seed shuffles all conditions once per balanced round; retain the
actual order and timestamps. Paired comparisons jointly balance the snapshots
as described in the [published report's protocol](../public-rollin-2026-10-24/README.md#conditions-and-validation).

Before timing, compare every engine/fixture CLI output with independently
generated expected entry arrays. Check each entry's physical line number, kind,
resolved element array, raw text, unset state, annotation and form, plus its
decoded string. Each native benchmark sample must also match its fixture byte
count, SHA-256, entry count, iteration count and mode. These fixture checks
complement the shared conformance suite and language-native API tests; they do
not establish correctness for arbitrary files.

Each timed pass seeks the same open file back to its beginning, reads through
the native buffered input API, parses syntax and decodes common strings.
OS-cached file reads are included. File opening, fixture generation, process
startup, warm-up and entry JSON serialisation are outside the timed interval.
Decoded output is consumed and discarded. Streaming consumes entries as
delivered; collecting retains all syntax entries before decoding and then
discards the collection. Decoded length sinks use native string units and are
not cross-language performance metrics.

Two warm-up passes do not guarantee JVM, V8 or .NET steady state. Native file
buffering, GC, JIT and string representations differ. Results describe the
selected implementations, toolchains and finite pass counts; they neither
isolate language overhead nor predict every consuming application.

## Workloads

The [fixture generator](fixtures.py) defines these six UTF-8/LF inputs and
independent expected entry arrays. Byte counts and entry counts identify the
workload, rather than measured performance.

| Fixture | Bytes | Entries | Values | Timed passes per process |
|---|---:|---:|---:|---:|
| `mixed-32` | 1,911 | 33 | 32 | 1,000 |
| `mixed-512` | 30,299 | 514 | 512 | 50 |
| `mixed-20000` | 1,183,212 | 20,079 | 20,000 | 3 |
| `line-1mib` | 1,048,593 | 1 | 1 | 3 |
| `blocks-32kib` | 65,729 | 2 | 2 | 20 |
| `depth-256` | 135,580 | 512 | 256 | 10 |

The three mixed fixtures use the same pattern of repeated keys, opaque
annotations, Unicode, unset values and pipe/triple-quoted blocks at different
sizes. The line fixture contains one literal 1,048,576-byte value on one
physical line. The block fixture has one pipe and one triple-quoted value;
each contains 32,768 ASCII `x` bytes and 31 LF separators, for 32,799 decoded
bytes. The depth fixture has 256 headings and one value at each level. All
fixtures retain the required declaration and blank separator. These workloads
do not compare CRLF, invalid inputs or cancellation paths.

## Metrics and interpretation

Report time as milliseconds per complete file pass. Each process sample's time
is the average of its timed passes; summaries report the median, minimum,
maximum and median absolute deviation across independent process samples.
Observed ranges describe spread rather than confidence intervals.

Process memory uses the observed peak working set in MiB
(1 MiB = 1,048,576 bytes). The [memory wrapper](../measure.py) samples Windows
`PeakWorkingSetSize` every 5 ms. This includes the runtime, startup, warm-up,
input buffers, allocator behaviour and JIT activity. It is neither retained
parser heap nor memory per pass. Polling can miss final growth before a process
exits, and short processes provide fewer observations. Streaming may retain
less result data without producing a smaller process peak. Record the actual
memory measurement method on every target platform.

Go and C# additionally expose cumulative allocation counters for the timed
interval: `runtime.MemStats.TotalAlloc` and
`GC.GetAllocatedBytesForCurrentThread`, respectively. Divide these counters by
the number of timed passes. Their scope differs; neither measures retained
heap or peak process memory. C# also records GC collection counts. These
counters help inspect each implementation's streaming and collecting paths;
other engines report process memory only.

## Toolchains and reproduction

Build native benchmark and CLI commands from the exact source snapshot before
validation and timing. Record source/test hashes, fixture and independent
expectation hashes, prepared binary hashes, compiler/runtime descriptions and
build flags with the report. A base commit alone may not identify the exact
source snapshot, so record hashes for every measured source. Keep baseline and candidate builds in separate temporary
directories when comparing snapshots, using matching toolchains and flags.

The [Windows preparation helper](prepare_windows.py) accepts explicit installed
tool paths with `--directory`, `--python`, `--node`, `--jdk`, `--dotnet`,
`--rustc` and `--cxx`; Go must be on `PATH`. It downloads no toolchains. It
copies the C# source tree into temporary storage before building, preserving
project references and keeping generated outputs outside the checkout. Its
Rust GNU build uses the toolchain's bundled MinGW GCC and self-contained CRT;
the C++ build uses LLVM-MinGW with static release flags.

| Engine | Preparation policy |
|---|---|
| Go | `go build -trimpath`; default optimisation and runtime settings |
| Python | Installed interpreter; default flags |
| JavaScript / TypeScript | Installed Node.js; default V8 flags; shared engine |
| Java | JDK 17 or later; `javac -encoding UTF-8`; default JVM/GC flags |
| C# | .NET 8 SDK; `dotnet build -c Release --nologo`; default .NET/GC flags |
| Rust | GNU Windows toolchain; `rustc --edition=2021 -C opt-level=3 -C link-self-contained=yes` |
| C++ | LLVM-MinGW; `clang++ -std=c++17 -O2 -Wall -Wextra -pedantic -static` |

The helper writes a local `toolchains.json` into the chosen temporary
directory. Alternatively, start from [config.example.json](config.example.json),
add all engines, record exact versions and flags, and provide an environment
snapshot. Local executable paths stay in the local configuration and are
omitted from published reports. Inspect the configuration before running it.

Run from the repository root, using suitable temporary paths on the target
platform:

```sh
python implementations/benchmarks/native-2026-10-24/fixtures.py TEMP/fixtures
python implementations/benchmarks/native-2026-10-24/matrix.py --config TEMP/toolchains.json --fixtures TEMP/fixtures --output TEMP/report.json --samples 5 --seed 20261024
```

Use `--verify-only` for the independent output checks without timing. With all
seven engines and six fixtures configured, there are 42 such checks. The
matrix report records fixture and expectation hashes alongside each sample's
metadata; large generated inputs remain outside Git. A partial report or a
verification-only run is not a completed measurement campaign.

For a before/after measurement, use the
[paired controller and validation guide](../public-rollin-2026-10-24/README.md#recorded-source-and-reproduction).

The [main benchmark guide](../README.md) distinguishes native file-input
measurements from the separate resident-input comparison of configuration
notations.
