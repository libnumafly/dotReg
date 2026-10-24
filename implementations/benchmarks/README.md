# Benchmark results

This directory contains benchmark reports for the
[dotReg specification](../../docs/config-format.md) and its
[sample implementations](../README.md). Each suite measures a different
output contract, so its results are presented separately.

## Contents

- [Reports and data](#reports-and-data)
- [Available reports](#available-reports)
- [Measurement scope](#measurement-scope)
- [Reproduction and tooling](#reproduction-and-tooling)
- [Interpreting time and memory](#interpreting-time-and-memory)
- [Validation](#validation)

## Reports and data

The reports below present the measurements available for each series. Each
report identifies the source snapshot it measured through its recorded source
hashes and commit references.

A paired report's baseline and candidate are two conditions of the same
measurement. Both conditions, their observations and validation evidence are
included so the comparison can be assessed as a whole.

Later implementation changes are not represented unless they are included in
the source snapshot identified by a report.

## Available reports

| Measurement series | Report | Recorded data |
|---|---|---|
| Native parser APIs across seven engines | [Public sample parser adoption](public-rollin-2026-10-24/README.md) | [Paired process observations and validation](public-rollin-2026-10-24/report.json) |
| Equivalent decoded configuration maps | [Go multiline and allocation results](multiline-allocation-2026-10-24/README.md) | [Paired comparison observations](multiline-allocation-2026-10-24/comparison.json), [raw output](multiline-allocation-2026-10-24/comparison.txt) |
| Go parser, decoder, consumer and prefix-selection stages | [Go multiline and allocation results](multiline-allocation-2026-10-24/README.md) | [Paired API observations](multiline-allocation-2026-10-24/go-api.json), [raw output](multiline-allocation-2026-10-24/go-api.txt) |
| Focused multiline assembly and ordinary Go loading | [Go multiline and allocation results](multiline-allocation-2026-10-24/README.md) | [Paired assembly observations](multiline-allocation-2026-10-24/assembly.json), [raw output](multiline-allocation-2026-10-24/assembly.txt) |

The native report includes seven engines and eight language interfaces:
TypeScript shares the JavaScript engine. It retains 840 timed process
observations and 84 independent complete-output validations. The Go reports
retain 720 configuration-format observations, 560 API-stage observations and
320 focused assembly observations, giving 1,600 Go observations in total.
These measurements describe the recorded workloads and source snapshots; they
do not establish performance for every workload or later implementation.

The report pages contain the result tables, variation, source identity,
toolchains, environment and reproduction commands.

## Measurement scope

| Suite | Timed work | Result contract |
|---|---|---|
| Native parser APIs | Syntax plus common string decoding, with buffered file reads | Streaming consumption or a collected sequence of entries |
| Other configuration formats | Loading equivalent string settings with selected Go libraries | A fresh two-level map of decoded strings |
| Go consumer APIs | Parsing, decoding, configuration construction or selection, according to the benchmark name | A built configuration or selected ordered occurrences |
| Focused Go assembly and loading | Multiline assembly plus decoding, collected ordinary loads, or parsing plus configuration construction | Decoded entries or a built configuration, according to the operation name |

Prefix selection belongs to the Go consumer API. It is outside both the native
parser timings and the other-format comparison. Stage names and output
ownership matter when comparing timings: syntax-only parsing, decoded maps and
ordered consumer objects perform different work.

See [configuration comparison methodology](comparison/README.md) for equivalent
semantic fixtures and pinned libraries, and
[native workload methodology](native-2026-10-24/README.md) for document shapes,
toolchains, process sampling and independent expected output.

## Reproduction and tooling

Use the source snapshot, flags and commands recorded in the corresponding
report. Prepare binaries and verify complete output before timing. Run measured
jobs serially, without overlapping builds, tests or other benchmark suites.

- [Paired native runner](run-paired-native.py) compares baseline and candidate
  engines using the [native fixtures](native-2026-10-24/fixtures.py),
  [Windows preparation helper](native-2026-10-24/prepare_windows.py) and
  [process-memory wrapper](measure.py).
- [Paired Go runner](run-paired-go.py) uses the
  [isolated Go measurement support](run-go.py) and the
  [configuration comparison module](comparison/README.md).
- [Focused assembly harness](assembly/assembly_test.go) provides reusable
  measurements for block construction and loading. The Go report records the
  same instrumentation overlaid on both production snapshots.
- [Paired Go result validation](paired-go-results.py) supplies shared observation
  and summary checks for the report table renderer. Validation reads
  retained results without rerunning measurements.
- [Single-input fixture generator](fixture.mjs),
  [suite controller](suite.py) and [process-memory wrapper](measure.py)
  support additional measurements.

New measurements require a source snapshot, validated observations and recorded
provenance.

## Interpreting time and memory

Native timings are time per complete document pass; Go stage timings are time
per named operation. Include the decoded output size and the work performed
when interpreting throughput. Encoded bytes per second alone are not a fair
comparison between serialisations of different sizes.

Native process RSS includes runtime or VM memory, JIT code, buffers, startup,
warmup and allocator behaviour. Polling a high-water metric can miss a final
increase before a short process exits. Go allocation volume reports total
allocated bytes per operation; it is not peak live memory or process RSS.

Reports retain every observation and summarise variation. Runtime defaults,
garbage collection, host activity and thermal conditions can affect a finite
campaign. These measurements do not measure electrical energy and do not
establish that one language or parser is universally faster.

## Validation

The native runner checks complete syntax and decoded occurrence output against
independent expectations before timing. The configuration comparison checks
complete decoded maps. Go consumer fixtures check ordered values and selections
before measuring their named stages.

[Shared conformance checks](../conformance/README.md), native API tests and Go
consumer tests provide separate correctness evidence. Benchmark harnesses do
not enforce timing, allocation or process-memory thresholds.
