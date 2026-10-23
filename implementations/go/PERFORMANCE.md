# Parser refactor measurements

These are historical measurements from before support for repeated value keys,
mandatory file property declarations and fixed two-space hierarchy indentation.
The current implementation retains every occurrence, requires file properties
and uses two ASCII spaces per heading level. The figures below are not
measurements of the current implementation. Current benchmark fixtures follow
the current syntax and do not reproduce the historical input byte for byte.

This records the Go parser refactor on 2026-10-23. The comparison uses an exact
working-tree snapshot taken immediately before the refactor, including both
multiline forms. It does not use the repository's older Git HEAD as the baseline.
The notation, public API and accepted/rejected input rules are unchanged.

## Contents

- [Measurement method](#measurement-method)
- [Results](#results)
- [Implementation changes](#implementation-changes)
- [Compatibility verification](#compatibility-verification)

## Measurement method

- Windows/amd64, Intel Core i7-1370P, Go 1.27.0.
- Identical `benchmark_test.go` in the before and after copies.
- Seven samples per benchmark, one Go scheduler processor, 100 ms target per sample.
- Times below are sample medians. They are local observations, not a statistical
  significance claim or a guarantee for other machines.
- `B/op` is cumulative bytes allocated per operation; it is not peak heap or RSS.
- Input generation and fixture validation are outside timed loops.
- Raw measurements: [before](benchmarks/2026-10-23-before.txt) and
  [after](benchmarks/2026-10-23-after.txt).
- [Baseline source hashes and line counts](benchmarks/2026-10-23-baseline-sources.json)
  identify the four production files used for the comparison.

Run from this directory:

```sh
go test -run '^$' -bench . -benchmem -benchtime=100ms -count=7 -cpu=1
```

## Results

The 512-setting fixtures mix headings, declared types, undecorated values and
quoted values. The quoted-key fixture includes escaped dots and Unicode escapes.
Block fixtures contain 32 KiB of undecoded body text.

| Benchmark | Time, µs/op before → after | Time change | B/op before → after | Allocs/op before → after |
|---|---:|---:|---:|---:|
| `BuildUntyped512` | 904.493 → 758.059 | -16.2% | 242,488 → 217,912 | 2,836 → 1,300 |
| `DecodeTriple32KiB/Escaped` | 464.963 → 438.948 | -5.6% | 113,400 → 113,400 | 19 → 19 |
| `DecodeTriple32KiB/Plain` | 361.632 → 66.743 | -81.5% | 154,360 → 0 | 20 → 0 |
| `ParseBuild512` | 2258.026 → 2041.055 | -9.6% | 564,016 → 498,280 | 7,661 → 4,060 |
| `ParseDecode512` | 1520.168 → 1395.834 | -8.2% | 328,696 → 287,536 | 5,465 → 3,400 |
| `ParseEach/BareMixed512` | 166.493 → 146.193 | -12.2% | 178,280 → 145,376 | 4,159 → 2,614 |
| `ParseEach/LongRaw1MiB` | 3478.453 → 1523.933 | -56.2% | 6,269,664 → 2,125,352 | 27 → 272 |
| `ParseEach/PipeLiteral32KiB` | 480.402 → 446.857 | -7.0% | 232,216 → 194,752 | 1,588 → 806 |
| `ParseEach/QuotedMixed512` | 199.738 → 181.415 | -9.2% | 194,792 → 153,632 | 5,199 → 3,134 |
| `ParseEach/TripleEscaped32KiB` | 714.250 → 584.994 | -18.1% | 378,216 → 264,968 | 1,601 → 817 |
| `ParseEach/TriplePlain32KiB` | 530.416 → 485.438 | -8.5% | 314,008 → 266,296 | 1,210 → 622 |

The 1 MiB line trades more small allocations for less total allocation and less
elapsed time: allocations rise from 27 to 272, while cumulative allocated bytes
fall by about 66% and the median time falls by about 56%. This is the standard
reader's fragment handling, and is included here rather than hidden by an
aggregate score. The escaped-body-only decoder has unchanged allocation totals;
the no-escape body avoids allocation entirely after source validation.

## Implementation changes

- Delegate unbounded physical-line assembly to `bufio.Reader.ReadString`,
  retaining original EOLs, partial-read errors and deferred lookahead validation.
- Share the ordinary quoted-string scanner between delimiter validation and
  actual decoding. A nil output builder validates without allocating discarded
  decoded text. Escape validation still happens at its original stage; this is
  not a claim that the entire key is scanned only once.
- Finalise value kind, raw RHS and unset state in one place for typed and untyped
  headers. Remove redundant branches.
- Store only text and EOL in buffered triple-quoted rows. Derive their diagnostic
  line numbers from the opening line and physical row index.
- Copy literal spans together during string decoding. Return the already
  validated triple-quoted body directly when it has no escapes.
- Reserve a lower-bound capacity for length-prefixed key IDs, without changing
  their encoding or element boundaries.
- Avoid boxing an unused converted value for undeclared settings.

The implementation introduces no dependency or public API. Across
`parser.go`, `value.go`, `profile.go` and `errors.go`, nonblank, non-comment
lines change from 797 to 796. Total lines change from 921 to 923 and source bytes
from 23,937 to 24,246. The source size is essentially maintained; the simplification
is shared grammar and less temporary work, not a large reduction in source text.
Tests and benchmark fixtures are outside these production counts.

## Compatibility verification

- Existing tests and added refactor regression tests pass.
- The new regression tests also pass against the saved implementation, checking
  that their expectations describe the original contract.
- A temporary comparison harness checked 12,636 deterministic inputs against the
  saved implementation. It compared Parse results, decoded values, Build/Get,
  callback stops and mutation isolation, errors with physical line numbers, and
  manually constructed Entry forms. All observations matched.
- Nine partial-input/read-error cases also matched the saved implementation.
- Regression cases cover competing malformed-key/type errors, long partial-read
  failures after a pipe block, quote-block boundaries, retained callback results
  and independent validation of manually constructed entries.
- A 10-second fuzz target completed 153,865 generated executions without failure.
- `go vet ./...`, `go build ./...` and formatting checks pass.
- All three CLI modes are compared against the saved implementation on the
  repository's sample configuration.

Required independent checks remain in DecodeString and Build. Raw text, explicit
empty versus unset, original LF/CRLF, Unicode identity, key-element boundaries,
unbounded format lengths, callback order and diagnostic privacy are preserved.
