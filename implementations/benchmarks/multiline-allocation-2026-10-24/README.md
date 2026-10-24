# Go multiline assembly and allocation results

This campaign measures multiline construction and remaining allocation costs
in ordinary Go loading. It compares the public sample at
[`47a4879`](https://github.com/libnumafly/dotReg/commit/47a48799b2cf6caa1fe26b45a53fff2c284b4674)
with the candidate implementation described below. The current
[specification](../../../docs/config-format.md), syntax, public APIs, decoded
values, output ownership, occurrence order and diagnostic priority are retained.
The changes apply to the Go sample only.

This report measures the Go comparison and API stages. Its paired baseline is
the public implementation before these changes. The separate
[native parser report](../public-rollin-2026-10-24/README.md) records seven-engine
process measurements for its own source snapshots. Those native results cover a
different source snapshot and measurement scope.

## Contents

- [Implementation changes](#implementation-changes)
- [Findings](#findings)
- [Results](#results)
  - [Direct loading and the fastest other implementation](#direct-loading-and-the-fastest-other-implementation)
  - [All configuration-format observations](#all-configuration-format-observations)
  - [All Go API stages](#all-go-api-stages)
  - [Focused assembly and ordinary-load observations](#focused-assembly-and-ordinary-load-observations)
- [Conditions and validation](#conditions-and-validation)
- [Reproduction](#reproduction)

## Implementation changes

- Triple-quoted assembly retains up to eight lines of metadata on the stack.
  While reading, it records the eventual body size and the minimum nonempty
  margin. Once the closing marker supplies the actual margin, it allocates the
  completed raw output size. Physical-line errors retain their original priority
  over margin errors; escape decoding remains a separate operation.
- Pipe assembly borrows short physical lines from the buffered reader, copying
  their payload before the next read. The shortest bodies use a 256-byte stack
  output buffer. Up to eight rows and 4,096 original source bytes, including
  discarded indentation and endings, can remain buffered; payloads beyond the
  stack prefix use owned parts and a final size reservation. Larger bodies
  continue through ordinary incremental builder growth. Long
  physical lines use owned spill storage, and non-pipe lookahead becomes owned
  before it is replayed after the current visitor. Returned entries retain
  independent storage.
- Scope storage is cleared when scopes close and reused for later headings.
  This avoids retaining closed path nodes and repeatedly allocating the same
  shallow scope slice.
- Internal path IDs reserve their exact encoded size. A stack digit buffer
  writes byte-length prefixes without temporary decimal strings, including
  long and Unicode element names. The length-prefixed representation and its
  element-boundary matching are unchanged.
- `Config` owns settings once in an array in source order. Its key map stores
  the first occurrence's index and compact integer indices for repetitions.
  Repeated occurrences share the first immutable path ID, and growing the
  repetition index no longer copies full stored settings. `GetAt` retains
  constant-time indexing. Prefix selection reads a setting's path ID before
  copying its value, and returns independent path slices for matches.

These changes are in the normal public parser and consumer paths. The
[assembly benchmark module](../assembly/assembly_test.go) uses those public
APIs and does not provide fixture counts or preallocated output to the parser.

## Findings

<!-- findings:start -->

The clearest complete-load allocation improvement is the 512-setting multiline
case: **181,808 to 136,504 B/op (-24.9%)** and
**3,470 to 1,903 allocs/op**. Its time changes from
209.09 to 185.248 µs/op (-11.4% in the median). The collected path also reduces allocation traffic
from 312,640 to 267,336 B/op (-14.5%).

- `PipeLiteral32KiB`: 194,704 to 157,344 B/op (-19.2%);
  805 to 26 allocs/op; 53.806 to 47.703 µs/op (-11.3% in the median).
- `TriplePlain32KiB`: 266,248 to 144,264 B/op (-45.8%);
  621 to 602 allocs/op; 51.234 to 41.737 µs/op (-18.5% in the median).
- `TripleEscaped32KiB`: 264,920 to 144,264 B/op (-45.5%);
  816 to 797 allocs/op; 63.139 to 52.922 µs/op (-16.2% in the median).

Ordinary consumer construction saves memory traffic even when time is similar.
`BuildUntyped512` changes from 228,440 to 179,192 B/op (-21.6%),
with 917 to 917 allocs/op; time is
94.843 to 93.674 µs/op (-1.2% in the median). `ParseBuild512` changes from
411,672 to 362,424 B/op (-12.0%), with time
181.535 to 184.811 µs/op (+1.8% in the median). This is an allocation-byte saving;
it does not imply that ordinary loading becomes faster in every workload.

`SelectPrefix` with 512 stored occurrences and no match changes from
0.888 to 0.976 µs/op (+9.9% in the median); its observed ranges are
[0.88, 0.897] and
[0.892, 1] µs/op.
`SelectPrefix` with 8,192 stored occurrences and no match changes from
14.828 to 14.684 µs/op (-1.0% in the median); its observed ranges are
[14.715, 15.078] and
[14.285, 15.138] µs/op.
The source-order array removes a map lookup for matched values, but its larger
record stride remains a locality tradeoff for scans that do not match.

The focused all-distinct load changes from 131.982 to 129.51 µs/op (-1.9% in the median);
allocation traffic changes from 382,040 to 332,792 B/op (-12.9%). Its ranges
are [111.528, 138.295] and
[110.849, 136.809] µs/op. The distinct-load and no-match
results are retained as limits of the storage change, rather than omitted from
the allocation wins. Small timing changes and overlapping ranges are not
evidence of a consistent speed gain.

The focused repeated-key load has a slower median: `ParseBuildRepeated512`
changes from 103.510 to 112.523 µs/op (+8.7%), while allocation traffic falls
from 275,920 to 232,792 B/op (-15.6%) and the count falls from 1,669 to 1,662.
The existing mixed ordered-load fixtures have lower candidate medians, so
repeated-key loading as a whole is not claimed to be uniformly faster.

For multiline common-map loading, the fastest other candidate implementation
is TOML at 172.698 µs/load; dotReg is 1.073× that median.
The baseline ratio for the same implementations is 1.272×. The unchanged
TOML control itself moves by +5.1%, so the change in that gap cannot
be attributed solely to dotReg. Full control observations and ranges appear
below, including the ordinary cases where dotReg is close to another library.

The eight-row buffered-pipe guard retains 12,472 to 12,472 B/op (+0.0%)
and 16 to 16 allocs/op. Long-line, CRLF and large-discarded-margin
cases are also included in the report.

The measured benefit is lower allocation traffic, especially for multiline
bodies and consumer storage. Retained heap, process peak memory and electrical
energy were not measured in this campaign.

<!-- findings:end -->

## Results

<!-- results:start -->

All **1,600 timed observations** are retained: 720 in the format campaign,
560 in the Go API campaign and 320 in the focused assembly campaign.
Each operation has ten baseline and ten candidate observations in serial,
jointly shuffled fresh processes. No observations are removed.

Times below are microseconds per complete operation, shown as median
[minimum, maximum] in the detailed tables. Ranges describe observations,
not confidence intervals. Allocation columns show median B/op and allocs/op;
every recorded numeric metric retains its median, minimum and maximum in
the JSON reports. Negative time change means a lower candidate median.

### Direct loading and the fastest other implementation

The fastest other column selects the lowest candidate median among the
unchanged JSON, TOML, YAML and INI operations in this campaign. The ratio
divides candidate dotReg direct time by that median. This compares the
recorded implementations and spellings; it is not a universal format ranking.

| Case | Baseline µs/load | Candidate µs/load | Time change | B/op, baseline → candidate | allocs/op, baseline → candidate | Fastest other candidate median | Candidate / other |
|---|---:|---:|---:|---:|---:|---:|---:|
| Small ASCII, 32 settings | 6.2155 | 6.043 | -2.775% | 8,840 → 8,816 | 99 → 96 | JSON 5.9645 µs | 1.013161× |
| Medium ASCII, 512 settings | 107.1295 | 108.308 | +1.100% | 120,368 → 120,120 | 1,422 → 1,391 | JSON 109.092 µs | 0.992813× |
| Large ASCII, 8,192 settings | 1,933.437 | 1,890.9495 | -2.198% | 1,885,553 → 1,884,537 | 18,450 → 18,323 | JSON 1,928.5675 µs | 0.980494× |
| Unicode, 512 settings | 138.006 | 141.839 | +2.777% | 137,008 → 136,760 | 1,422 → 1,391 | JSON 161.9755 µs | 0.875682× |
| Escaped, 512 settings | 180.8145 | 170.268 | -5.833% | 169,520 → 169,272 | 1,934 → 1,903 | JSON 186.9595 µs | 0.910721× |
| Multiline, 512 settings | 209.0905 | 185.2475 | -11.403% | 181,808 → 136,504 | 3,470 → 1,903 | TOML 172.698 µs | 1.072667× |

### All configuration-format observations

The unchanged other-library rows show the spread possible within this
campaign without changing their implementations or dependencies. Small
differences and overlapping ranges do not establish a stable speed gain.

| Operation | Baseline µs/op, median [min, max] | Candidate µs/op, median [min, max] | B/op, baseline → candidate | allocs/op, baseline → candidate |
|---|---:|---:|---:|---:|
| small / dotReg direct | 6.2155 [5.876, 7.562] | 6.043 [5.971, 7.029] | 8,840 → 8,816 | 99 → 96 |
| small / dotReg collected | 7.091 [6.89, 7.967] | 6.9355 [6.811, 7.786] | 16,792 → 16,768 | 105 → 102 |
| small / JSON | 5.963 [5.681, 6.404] | 5.9645 [5.664, 7.056] | 3,104 → 3,104 | 89 → 89 |
| small / TOML | 7.935 [6.503, 9.013] | 7.858 [6.693, 9.221] | 3,144 → 3,144 | 75 → 75 |
| small / YAML | 47.432 [39.714, 55.6] | 41.7865 [38.586, 49.912] | 27,840 → 27,840 | 512 → 512 |
| small / INI | 8.4755 [8.222, 9.749] | 8.8425 [8.28, 9.778] | 19,336 → 19,336 | 202 → 202 |
| medium / dotReg direct | 107.1295 [100.386, 113.683] | 108.308 [101.063, 131.888] | 120,368 → 120,120 | 1,422 → 1,391 |
| medium / dotReg collected | 112.6185 [107.318, 119.756] | 109.6725 [106.962, 124.322] | 251,200 → 250,952 | 1,432 → 1,401 |
| medium / JSON | 102.8255 [98.959, 111.746] | 109.092 [99.247, 123.602] | 93,867 → 93,867.5 | 1,356 → 1,356 |
| medium / TOML | 154.502 [131.547, 171.135] | 156.2835 [133.554, 170.357] | 96,404 → 96,404 | 1,258 → 1,258 |
| medium / YAML | 658.562 [596.507, 711.48] | 630.7235 [582.395, 801.887] | 379,400 → 379,400.5 | 7,374 → 7,374 |
| medium / INI | 183.393 [179.24, 198.166] | 187.074 [181.936, 208.814] | 361,672 → 361,672 | 2,990 → 2,990 |
| large / dotReg direct | 1,933.437 [1,741.27, 2,174.142] | 1,890.9495 [1,737.64, 2,161.886] | 1,885,553 → 1,884,537 | 18,450 → 18,323 |
| large / dotReg collected | 2,897.542 [2,816.986, 3,410.161] | 2,921.417 [2,760.066, 3,469.447] | 6,407,304 → 6,406,287 | 18,469 → 18,342 |
| large / JSON | 1,890.762 [1,829.858, 2,446.739] | 1,928.5675 [1,883.524, 2,135.506] | 1,546,522 → 1,546,522 | 18,192 → 18,192 |
| large / TOML | 3,492.6555 [3,413.735, 3,983.828] | 3,539.642 [3,432.222, 3,959.003] | 1,605,889 → 1,605,887.5 | 17,806 → 17,806 |
| large / YAML | 11,796.7765 [11,216.015, 12,985.712] | 11,926.9875 [11,420.795, 13,665.281] | 5,880,788 → 5,880,784 | 110,004 → 110,004 |
| large / INI | 4,169.0835 [3,992.115, 4,590.79] | 4,262.978 [4,043.272, 5,157.369] | 5,851,694 → 5,851,694 | 38,201 → 38,201 |
| unicode / dotReg direct | 138.006 [132.849, 151.855] | 141.839 [132.854, 151.327] | 137,008 → 136,760 | 1,422 → 1,391 |
| unicode / dotReg collected | 146.125 [138.997, 164.646] | 143.758 [139.548, 179.392] | 267,840 → 267,592 | 1,432 → 1,401 |
| unicode / JSON | 166.5065 [153.059, 182.811] | 161.9755 [149.368, 168.096] | 106,412 → 106,412 | 1,356 → 1,356 |
| unicode / TOML | 186.182 [175.23, 209.881] | 186.8065 [175.688, 218.509] | 104,596 → 104,596 | 1,258 → 1,258 |
| unicode / YAML | 773.872 [693.693, 916.57] | 756.3545 [689.19, 810.514] | 506,632 → 506,632 | 10,446 → 10,446 |
| unicode / INI | 210.2405 [197.121, 226.349] | 203.074 [200.346, 265.958] | 386,760 → 386,760 | 2,990 → 2,990 |
| escaped / dotReg direct | 180.8145 [168.187, 210.222] | 170.268 [160.276, 190.29] | 169,520 → 169,272 | 1,934 → 1,903 |
| escaped / dotReg collected | 177.5665 [173.889, 191.092] | 176.6325 [167.517, 204.827] | 300,352 → 300,104 | 1,944 → 1,913 |
| escaped / JSON | 183.838 [178.991, 195.974] | 186.9595 [177.041, 215.625] | 143,021 → 143,021.5 | 1,868 → 1,868 |
| escaped / TOML | 204.398 [175.435, 231.98] | 203.45 [174.311, 221.579] | 145,558 → 145,558 | 1,770 → 1,770 |
| escaped / YAML | 852.7195 [755.217, 972.287] | 803.252 [730.997, 879.101] | 543,240 → 543,240 | 11,470 → 11,470 |
| escaped / INI | 194.752 [185.3, 219.986] | 197.4255 [188.892, 219.091] | 402,632 → 402,632 | 2,990 → 2,990 |
| multiline / dotReg direct | 209.0905 [195.834, 214.921] | 185.2475 [177.216, 208.704] | 181,808 → 136,504 | 3,470 → 1,903 |
| multiline / dotReg collected | 240.3255 [200.492, 267.296] | 209.8545 [185.51, 217.458] | 312,640 → 267,336 | 3,480 → 1,913 |
| multiline / JSON | 198.7505 [185.615, 206.637] | 186.986 [178.378, 233.109] | 143,021 → 143,021 | 1,868 → 1,868 |
| multiline / TOML | 164.3345 [148.405, 191.178] | 172.698 [149.95, 181.921] | 112,788.5 → 112,788.5 | 1,258 → 1,258 |
| multiline / YAML | 743.6735 [697.84, 813.951] | 688.5405 [677.84, 812.43] | 420,360 → 420,360 | 7,374 → 7,374 |
| multiline / INI | 279.7185 [264.084, 302.79] | 277.7085 [267.63, 305.949] | 439,496 → 439,496 | 6,062 → 6,062 |

### All Go API stages

These 28 operations retain their original stage boundaries. Prefix selection
starts from an already built configuration. Build and selection produce a
richer result than the common-map loads and should be assessed within the
same operation rather than ranked against other-format load times.

| Operation | Baseline µs/op, median [min, max] | Candidate µs/op, median [min, max] | B/op, baseline → candidate | allocs/op, baseline → candidate |
|---|---:|---:|---:|---:|
| `ParseEach/BareMixed512` | 56.6595 [55.749, 58.633] | 58.061 [56.729, 60.703] | 46,192 → 46,192 | 1,065 → 1,065 |
| `ParseEach/QuotedMixed512` | 86.636 [85.848, 88.957] | 92.7855 [87.939, 102.665] | 54,448 → 54,448 | 1,585 → 1,585 |
| `ParseEach/LongRaw1MiB` | 216.4885 [208.83, 259.909] | 218.5885 [201.801, 249.225] | 2,125,304 → 2,125,304 | 271 → 271 |
| `ParseEach/TriplePlain32KiB` | 51.234 [50.593, 61.85] | 41.7375 [40.909, 42.955] | 266,248 → 144,264 | 621 → 602 |
| `ParseEach/TripleEscaped32KiB` | 63.1395 [61.991, 65.545] | 52.9225 [50.708, 54.085] | 264,920 → 144,264 | 816 → 797 |
| `ParseEach/PipeLiteral32KiB` | 53.806 [52.712, 65.638] | 47.7035 [47.101, 48.495] | 194,704 → 157,344 | 805 → 26 |
| `ParseDecode512` | 114.201 [112.269, 125.629] | 116.5125 [114.512, 126.968] | 187,328 → 187,328 | 1,723 → 1,723 |
| `DecodeTriple32KiB/Plain` | 0.9816 [0.9765, 1.069] | 0.98385 [0.9747, 1.048] | 0 → 0 | 0 → 0 |
| `DecodeTriple32KiB/Escaped` | 43.9295 [43.411, 44.798] | 43.9235 [43.065, 48.963] | 113,400 → 113,400 | 19 → 19 |
| `ParseBuild512` | 181.5345 [179.919, 199.789] | 184.811 [180.787, 196.285] | 411,672 → 362,424 | 2,384 → 2,384 |
| `BuildUntyped512` | 94.843 [92.303, 102.591] | 93.674 [91.709, 100.717] | 228,440 → 179,192 | 917 → 917 |
| `ParseBuildOrdered/Occurrences512` | 147.947 [144.593, 159.869] | 141.353 [137.505, 150.506] | 319,168 → 272,264 | 2,670 → 2,665 |
| `ParseBuildOrdered/Occurrences8192` | 3,901.4235 [3,753.558, 4,072.04] | 3,821.8565 [3,538.644, 3,993.235] | 7,451,968 → 6,653,448 | 41,127 → 41,125 |
| `SelectPrefix/Occurrences512/All` | 63.3865 [60.686, 84.199] | 54.6615 [54.335, 75.096] | 205,600 → 205,600 | 2,218 → 2,218 |
| `SelectPrefix/Occurrences512/EmptyPrefix` | 62.772 [60.673, 81.353] | 55.5895 [53.317, 66.491] | 205,600 → 205,600 | 2,218 → 2,218 |
| `SelectPrefix/Occurrences512/BaseAndDescendants` | 33.396 [32.696, 39.514] | 28.9055 [28.472, 34.962] | 103,728 → 103,728 | 1,290 → 1,290 |
| `SelectPrefix/Occurrences512/SelectiveDescendant` | 4.727 [4.641, 5.023] | 4.0405 [3.997, 4.16] | 11,576 → 11,576 | 135 → 135 |
| `SelectPrefix/Occurrences512/NoMatch` | 0.8881 [0.8798, 0.8972] | 0.9759 [0.8917, 0.9998] | 16 → 16 | 1 → 1 |
| `SelectPrefix/Occurrences512/QuotedElementBoundary` | 4.5855 [4.493, 4.718] | 3.992 [3.909, 4.102] | 12,088 → 12,088 | 167 → 167 |
| `SelectPrefix/Occurrences512/EmptyName` | 7.985 [7.859, 8.174] | 6.8215 [6.659, 7.075] | 22,312 → 22,312 | 264 → 264 |
| `SelectPrefix/Occurrences8192/All` | 2,116.838 [2,074.451, 2,174.959] | 1,984.3995 [1,945.916, 2,027.705] | 4,422,944 → 4,422,944 | 35,346 → 35,346 |
| `SelectPrefix/Occurrences8192/EmptyPrefix` | 2,130.877 [2,047.835, 2,418.582] | 1,991.1115 [1,958.334, 2,099.331] | 4,422,944 → 4,422,944 | 35,346 → 35,346 |
| `SelectPrefix/Occurrences8192/BaseAndDescendants` | 1,125.012 [1,083.651, 1,207.471] | 1,037.3425 [1,018.813, 1,081.373] | 2,383,152 → 2,383,152 | 20,497 → 20,497 |
| `SelectPrefix/Occurrences8192/SelectiveDescendant` | 112.119 [108.641, 117.497] | 103.175 [101.396, 105.816] | 195,896 → 195,896 | 2,059 → 2,059 |
| `SelectPrefix/Occurrences8192/NoMatch` | 14.8275 [14.715, 15.078] | 14.6845 [14.285, 15.138] | 16 → 16 | 1 → 1 |
| `SelectPrefix/Occurrences8192/QuotedElementBoundary` | 111.343 [110.059, 115.6] | 102.409 [101.245, 117.494] | 204,088 → 204,088 | 2,571 → 2,571 |
| `SelectPrefix/Occurrences8192/EmptyName` | 191.044 [185.225, 233.441] | 177.49 [172.262, 184.403] | 343,336 → 343,336 | 4,108 → 4,108 |
| `SelectPrefix/EmptyConfig` | 0.003551 [0.0035, 0.003803] | 0.005171 [0.005112, 0.005385] | 0 → 0 | 0 → 0 |

### Focused assembly and ordinary-load observations

The twelve block and boundary cases time `ParseEach` followed by
`DecodeString` for each value. The two `CollectDecode` cases time `Parse`
then decoding; the two `ParseBuild` cases time `Parse` then `Build`.
Fixture creation, independent full-entry and decoded-value checks, and
Build's occurrence-order checks remain outside the timers. The same new
measurement module is overlaid on both production snapshots and recorded
in their source manifests; it is common instrumentation, not a baseline
production change.

| Operation | Baseline µs/op, median [min, max] | Candidate µs/op, median [min, max] | B/op, baseline → candidate | allocs/op, baseline → candidate |
|---|---:|---:|---:|---:|
| `PipeShort3LF` | 0.76055 [0.7524, 0.8372] | 0.73275 [0.7211, 0.7807] | 4,352 → 4,304 | 11 → 8 |
| `TripleShort3LF` | 0.90335 [0.8768, 1.06] | 0.79985 [0.7854, 0.8217] | 4,568 → 4,344 | 16 → 12 |
| `PipeManyShortLF` | 117.2455 [109.373, 124.476] | 105.8655 [102.458, 112.437] | 207,784 → 158,632 | 2,074 → 26 |
| `PipeManyShortCRLF` | 122.6095 [120.186, 134.26] | 116.8 [113.704, 123.573] | 207,784 → 158,632 | 2,074 → 26 |
| `TripleManyShortLF` | 131.7655 [129.967, 142.909] | 125.9725 [118.939, 150.179] | 400,880 → 278,848 | 2,087 → 2,066 |
| `TripleManyShortCRLF` | 145.367 [142.569, 167.368] | 137.858 [131.255, 167.621] | 400,880 → 287,040 | 2,087 → 2,066 |
| `PipeLong1MiB` | 317.788 [304.885, 340.247] | 318.0635 [302.049, 412.783] | 3,173,898 → 3,173,898 | 273 → 273 |
| `TripleLong1MiB` | 330.1535 [313.573, 344.47] | 331.72 [328.234, 363.796] | 3,173,905 → 3,173,905 | 274 → 274 |
| `PipeDiscardedMargin` | 21.267 [20.964, 23.628] | 21.2145 [21.087, 23.144] | 109,184 → 109,184 | 30 → 30 |
| `TripleDiscardedMargin` | 38.871 [38.285, 40.856] | 27.777 [27.331, 29.034] | 144,336 → 144,136 | 39 → 36 |
| `PipeBoundary1MiB` | 269.597 [257.06, 287.925] | 269.753 [260.105, 294.682] | 2,125,408 → 2,125,360 | 277 → 274 |
| `PipeBuffered8Rows` | 1.94 [1.875, 2.087] | 1.9925 [1.882, 2.039] | 12,472 → 12,472 | 16 → 16 |
| `Bare512CollectDecode` | 60.9065 [58.376, 75.082] | 62.945 [61.162, 75.599] | 163,840 → 163,840 | 1,043 → 1,043 |
| `Quoted512CollectDecode` | 133.7105 [112.902, 137.432] | 132.4795 [113.491, 139.351] | 188,416 → 188,416 | 2,067 → 2,067 |
| `ParseBuildDistinct512` | 131.982 [111.528, 138.295] | 129.5095 [110.849, 136.809] | 382,040 → 332,792 | 1,576 → 1,576 |
| `ParseBuildRepeated512` | 103.5095 [96.773, 125.033] | 112.523 [93.378, 122.783] | 275,920 → 232,792 | 1,669 → 1,662 |

These measurements report time and allocation traffic, not retained heap,
process peak memory or electrical energy. The protocol and limits in this
document apply to all tables above.

<!-- results:end -->

## Conditions and validation

The three completed paired campaigns retain **1,600 timed observations**:
720 for the equivalent-map comparison, 560 for existing Go API stages, and
320 for the sixteen added assembly and ordinary-load operations. Each operation
runs in both variants in each of ten rounds, with a 200 ms Go benchmark timing
target. All observations are retained without outlier removal.

The baseline production sources are the unchanged
`47a48799b2cf6caa1fe26b45a53fff2c284b4674` sources. Both variants use the same
assembly and public-API regression instrumentation. Per-file source hashes
identify the measured bytes rather than relying on a commit name alone.

Both variants are built before timing with the same Go toolchain and build
flags. The [paired controller](../run-paired-go.py) runs one exact operation per
fresh test-binary process, serially. Within each round it jointly shuffles all
variant/operation pairs using the recorded seed. Runtime settings are
`GOMAXPROCS=1`, `GOGC=100`, `GOMEMLIMIT=off` and `-test.cpu=1`. Go's benchmark
runner performs its ordinary calibration and garbage collection; no forced
collection is added inside timed loops. Input generation and independent
expected-output checks are outside the Go timers. Discovery passes are also
recorded separately and are excluded from the timed-observation count.

The comparison preserves all six original cases and the same encoded fixtures,
decoded common-map contract, library versions and dependency files. JSON,
TOML, YAML and INI execute in both variants using unchanged code. See the
[comparison method](../comparison/README.md) and
[fixture manifest](../comparison/fixtures-manifest.json) for their syntax and
payload boundaries. This comparison constructs a fresh
`map[string]map[string]string`; it does not call the sample consumer's `Build`.
The separate Go API and assembly campaigns exercise consumer construction and
ordered repeated-value retrieval.

The assembly module covers short and many-row bodies, LF and CRLF, long lines,
large discarded margins, a reader-buffer boundary, ordinary bare and quoted
collection/decoding, and consumer construction with distinct or repeated keys.
Its expected entries and decoded strings are specified independently of either
parser's output. Shared regression checks cover retained visitor results,
scope reuse, key-identity boundaries, interleaved occurrences, profile copying,
conversion order and failure behaviour. Reader-specific checks compare borrowed
and ordinary physical-line reads, including fragmented input, EOF and errors
returned alongside data.

Each completed JSON report retains per-observation metrics, process wall times, ordering,
runtime settings, source and binary hashes, and summaries. The controller
requires all expected observations and unchanged recorded source/binary bytes
before marking a campaign complete. Times in generated tables are medians
with observed minimum and maximum, not confidence intervals. Ratios between
medians describe this campaign and do not establish statistical significance
or a universal format ranking. Fresh process setup and calibration contribute
to recorded process wall times but are excluded from `ns/op`.

`B/op` and `allocs/op` measure Go heap allocation traffic per operation. This
campaign does not measure retained heap size, process peak RSS or electricity
consumption. Allocation reductions alone therefore do not establish an energy
saving. The completed [native process-memory report](../public-rollin-2026-10-24/README.md)
applies to its own recorded source snapshots and workloads.

The complete observations and raw Go output are retained in this directory:

| Campaign | Machine-readable report | Raw output |
|---|---|---|
| Equivalent decoded configurations | [comparison.json](comparison.json) | [comparison.txt](comparison.txt) |
| Existing Go API stages | [go-api.json](go-api.json) | [go-api.txt](go-api.txt) |
| Added assembly and ordinary loads | [assembly.json](assembly.json) | [assembly.txt](assembly.txt) |

The [host record](host.json) retains the preparation-time CPU, memory, operating
system, power and toolchain observations, plus hashes of the shared test and
benchmark overlays. This Windows/amd64 campaign used Go 1.27.0 and CPython
3.12.14 on an Intel Core i7-1370P with 14 physical cores, 20 logical processors
and 16,819,617,792 bytes of physical memory. Background desktop and operating
system activity was uncontrolled; builds, tests and benchmark suites did not
overlap the timed jobs. Power and battery observations were point-in-time
metadata, not continuously enforced conditions.

The recorded source hashes identify the exact candidate snapshot; commit
references alone do not identify every measured source file. Reproduction must
use the candidate sources recorded in the report. Both variants completed the
shared public-API regressions and conformance tests. The final candidate also
passed its reader tests, `go vet`, `go build` and a five-second parser fuzz run.

## Reproduction

Run from the repository root and record the Go and CPython versions in the
reports. Set `$python` to a working CPython executable. The PowerShell example
exports the full baseline, adds the shared instrumentation, builds every test
binary before measuring, and writes to a fresh temporary directory. The full
archive also preserves `conformance/cases.json` for separate Go test runs.
Keep sources unchanged and avoid concurrent CPU workloads during measurement.

```powershell
$python = "python"
$candidateRoot = (Get-Location).Path
$baselineCommit = "47a48799b2cf6caa1fe26b45a53fff2c284b4674"
$campaignId = [guid]::NewGuid().ToString("N")
$baselineRoot = Join-Path $env:TEMP "dotreg-assembly-before-$campaignId"
$baselineArchive = "$baselineRoot.zip"
$reportRoot = Join-Path $env:TEMP "dotreg-assembly-results-$campaignId"
New-Item -ItemType Directory -Path $reportRoot | Out-Null
git archive --format=zip --output=$baselineArchive $baselineCommit
if ($LASTEXITCODE -ne 0) { throw "Baseline export failed" }
Expand-Archive -LiteralPath $baselineArchive -DestinationPath $baselineRoot

$baselineAssembly = Join-Path $baselineRoot "implementations/benchmarks/assembly"
New-Item -ItemType Directory -Path $baselineAssembly | Out-Null
foreach ($name in @("go.mod", "assembly_test.go")) {
    Copy-Item -LiteralPath (Join-Path $candidateRoot "implementations/benchmarks/assembly/$name") `
      -Destination (Join-Path $baselineAssembly $name)
}
foreach ($name in @("assembly_optimisation_test.go", "identity_optimisation_test.go", `
                    "storage_test.go", "pipe_assembly_optimisation_test.go")) {
    Copy-Item -LiteralPath (Join-Path $candidateRoot "implementations/go/$name") `
      -Destination (Join-Path $baselineRoot "implementations/go/$name")
}

$modules = @{
    "comparison" = "implementations/benchmarks/comparison"
    "go-api" = "implementations/go"
    "assembly" = "implementations/benchmarks/assembly"
}
foreach ($variant in @("baseline", "candidate")) {
    $variantRoot = if ($variant -eq "baseline") { $baselineRoot } else { $candidateRoot }
    foreach ($suite in @("comparison", "go-api", "assembly")) {
        Push-Location (Join-Path $variantRoot $modules[$suite])
        try {
            $binary = Join-Path $reportRoot "$variant-$suite.test.exe"
            go test -c -trimpath -o $binary .
            if ($LASTEXITCODE -ne 0) { throw "$variant $suite build failed" }
        } finally {
            Pop-Location
        }
    }
}
```

Run the campaigns sequentially, retaining their separate reports:

```powershell
& $python implementations/benchmarks/run-paired-go.py `
  --baseline-root $baselineRoot `
  --baseline-binary (Join-Path $reportRoot "baseline-comparison.test.exe") `
  --candidate-root $candidateRoot `
  --candidate-binary (Join-Path $reportRoot "candidate-comparison.test.exe") `
  --baseline-commit $baselineCommit `
  --module implementations/benchmarks/comparison `
  --bench '^BenchmarkConfig$' --samples 10 --benchtime 200ms --seed 20261024 `
  --manifest implementations/benchmarks/comparison/fixtures-manifest.json `
  --output (Join-Path $reportRoot "comparison.json") `
  --raw (Join-Path $reportRoot "comparison.txt")
if ($LASTEXITCODE -ne 0) { throw "Paired format comparison failed" }

& $python implementations/benchmarks/run-paired-go.py `
  --baseline-root $baselineRoot `
  --baseline-binary (Join-Path $reportRoot "baseline-go-api.test.exe") `
  --candidate-root $candidateRoot `
  --candidate-binary (Join-Path $reportRoot "candidate-go-api.test.exe") `
  --baseline-commit $baselineCommit `
  --module implementations/go `
  --bench '^Benchmark' --samples 10 --benchtime 200ms --seed 20261024 `
  --output (Join-Path $reportRoot "go-api.json") `
  --raw (Join-Path $reportRoot "go-api.txt")
if ($LASTEXITCODE -ne 0) { throw "Paired Go API campaign failed" }

& $python implementations/benchmarks/run-paired-go.py `
  --baseline-root $baselineRoot `
  --baseline-binary (Join-Path $reportRoot "baseline-assembly.test.exe") `
  --candidate-root $candidateRoot `
  --candidate-binary (Join-Path $reportRoot "candidate-assembly.test.exe") `
  --baseline-commit $baselineCommit `
  --module implementations/benchmarks/assembly `
  --bench '^BenchmarkAssembly$' --samples 10 --benchtime 200ms --seed 20261024 `
  --output (Join-Path $reportRoot "assembly.json") `
  --raw (Join-Path $reportRoot "assembly.txt")
if ($LASTEXITCODE -ne 0) { throw "Paired assembly campaign failed" }
```

All three JSON reports must have `"status": "complete"`. Verify their source,
binary and shared-instrumentation hashes before copying the six files into
this report directory. An interrupted report retains `"status": "running"`
and cannot serve as a complete campaign. Use new output paths for each run.

Generate and check the tables from those retained reports:

```powershell
& $python implementations/benchmarks/multiline-allocation-2026-10-24/render.py
if ($LASTEXITCODE -ne 0) { throw "Assembly table generation failed" }
& $python implementations/benchmarks/multiline-allocation-2026-10-24/render.py --check
if ($LASTEXITCODE -ne 0) { throw "Assembly table verification failed" }
& $python implementations/benchmarks/multiline-allocation-2026-10-24/render.py --check --check-current-sources
if ($LASTEXITCODE -ne 0) { throw "Current measured source verification failed" }
```

The renderer validates observations, recorded summaries and source records,
then replaces only the marked result block. It does not modify raw reports or
launch benchmarks. `--check-current-sources` additionally compares the current
checkout with the recorded candidate; ordinary `--check` validates the completed
report independently of later source changes. Shared validation and formatting
live in [paired-go-results.py](../paired-go-results.py); rendering does not
depend on a superseded result directory.
