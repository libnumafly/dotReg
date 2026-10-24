# Published benchmark results

This directory contains the latest benchmark reports pushed to the repository
for the [dotReg specification](../../docs/config-format.md) and its
[sample implementations](../README.md). The suites measure different output
contracts, so their timings are presented separately.

## Contents

- [Publication policy](#publication-policy)
- [Latest published reports](#latest-published-reports)
- [Measurement scope](#measurement-scope)
- [Reproduction and tooling](#reproduction-and-tooling)
- [Interpreting time and memory](#interpreting-time-and-memory)
- [Validation and CI](#validation-and-ci)

## Publication policy

**Latest means the most recent report already pushed to the main branch for
that measurement series.** A finished local measurement or an unpushed source
change does not replace the published result.

The repository working tree and Pages retain only that report version.
When publishing a replacement, update its rendered tables, raw observations,
source and fixture manifests, links and Pages registrations together, and remove
the superseded standalone results. Do not keep earlier result tables,
diagnostic runs or historical result pages beside the replacement.
Measurement tools and methodology remain available for reproduction.

A paired report's baseline and candidate are two conditions of the same
measurement. Keep both conditions, their observations and validation evidence
inside the latest report; removing one would remove the comparison's control.

Every report identifies the source snapshot it actually measured. Its source
hashes and immutable commit references take precedence over the current checkout
when reproducing it. A later implementation change does not turn an older
measurement into a measurement of the new code.

Unfinished campaigns and unpushed replacement reports stay outside the published
results and Pages navigation.

## Latest published reports

| Measurement series | Latest pushed report | Recorded data |
|---|---|---|
| Native parser APIs across seven engines | [Public sample parser adoption](public-rollin-2026-10-24/README.md) | [Paired process observations and validation](public-rollin-2026-10-24/report.json) |
| Equivalent decoded configuration maps | [Go parser tuning results](tuning-2026-10-24/README.md) | [Paired comparison observations](tuning-2026-10-24/comparison.json), [raw output](tuning-2026-10-24/comparison.txt) |
| Go parser, decoder, consumer and prefix-selection stages | [Go parser tuning results](tuning-2026-10-24/README.md) | [Paired API observations](tuning-2026-10-24/go-api.json), [raw output](tuning-2026-10-24/go-api.txt) |

The native report includes seven engines and eight language interfaces:
TypeScript shares the JavaScript engine. It retains 840 timed process
observations and 84 independent complete-output validations. The Go reports
retain 720 configuration-format observations and 560 API-stage observations.
These are the samples in the published campaigns, rather than claims about
every possible workload or later implementation revision.

The report pages contain the result tables, variation, source identity,
toolchains, environment and exact reproduction commands. This overview does
not repeat older tables.

## Measurement scope

| Suite | Timed work | Result contract |
|---|---|---|
| Native parser APIs | Syntax plus common string decoding, with buffered file reads | Streaming consumption or a collected sequence of entries |
| Other configuration formats | Loading equivalent string settings with selected Go libraries | A fresh two-level map of decoded strings |
| Go consumer APIs | Parsing, decoding, configuration construction or selection, according to the benchmark name | A built configuration or selected ordered occurrences |

Prefix selection belongs to the Go consumer API. It is outside both the native
parser timings and the other-format comparison. Stage names and output
ownership matter when comparing timings: syntax-only parsing, decoded maps and
ordered consumer objects perform different work.

See [configuration comparison methodology](comparison/README.md) for equivalent
semantic fixtures and pinned libraries, and
[native workload methodology](native-2026-10-24/README.md) for document shapes,
toolchains, process sampling and independent expected output.

## Reproduction and tooling

Use the source snapshots, flags and commands recorded in the latest report.
Prepare binaries and verify complete output before timing. Run measured jobs
serially, without overlapping builds, tests or other benchmark suites. Write
intermediate reports to temporary storage and publish them only after the
campaign and its validation have completed.

- [Paired native runner](run-paired-native.py) compares baseline and candidate
  engines using the [native fixtures](native-2026-10-24/fixtures.py),
  [Windows preparation helper](native-2026-10-24/prepare_windows.py) and
  [process-memory wrapper](measure.py).
- [Paired Go runner](run-paired-go.py) uses the
  [isolated Go measurement support](run-go.py) and the
  [configuration comparison module](comparison/README.md).
- [Single-input fixture generator](fixture.mjs),
  [suite controller](suite.py) and [process-memory wrapper](measure.py)
  remain available for independent local measurements.

Do not overwrite report metadata or relabel a measurement after changing its
source. A replacement needs its own validated observations and recorded
provenance, followed by a coordinated push of the new report and removal of the
superseded result version.

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

## Validation and CI

The native runner checks complete syntax and decoded occurrence output against
independent expectations before timing. The configuration comparison checks
complete decoded maps. Go consumer fixtures check ordered values and selections
before measuring their named stages.

[Shared conformance checks](../conformance/README.md), native API tests and Go
consumer tests remain separate correctness evidence. CI validates the rendered
tables of the published reports and runs small harness smoke checks; it does
not enforce timing, allocation or process-memory thresholds. Source preparation
and Pages publication happen through the repository's Runner workflows.
