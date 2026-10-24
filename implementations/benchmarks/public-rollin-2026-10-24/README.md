# Public sample parser adoption

This benchmark measures direct value-path construction in the public sample
parsers. Active headings retain parent/suffix nodes, while each value builds an
independent output path without a temporary scope node. The change covers
Python, JavaScript, Java, C#, Rust and C++; TypeScript shares the JavaScript
engine. Go is the unchanged control for this comparison. The separate
[Go multiline and allocation report](../multiline-allocation-2026-10-24/README.md)
measures file loading and Go API stages, so its results cover a different
measurement scope.

The [benchmark overview](../README.md) links to the benchmark reports
for each suite. The [native methodology guide](../native-2026-10-24/README.md)
describes the workload and reproduction tools used by this campaign.

The purpose is to remove intermediate objects and copies while preserving the
current [specification](../../../docs/config-format.md), APIs, entry order,
error precedence, streaming behaviour and output ownership. Each notified path
remains independent: changing a delivered path cannot change a later resolved
path or the parser's heading state.

## Contents

- [Path construction](#path-construction)
- [Findings and limits](#findings-and-limits)
- [Results](#results)
- [Conditions and validation](#conditions-and-validation)
- [Recorded source and reproduction](#recorded-source-and-reproduction)

## Path construction

The six engines now distinguish heading construction from value construction.
Headings retain their private parent/suffix nodes and separately materialised
public paths. Values materialise an owned parent path plus their decoded key
suffix, or use the fresh parsed absolute path where appropriate. A value
assigned to the parent itself receives an independent copy of that parent's
resolved path.

Python and JavaScript combine a parent and suffix into one sized output list or
array; Python converts a suffix to a private tuple only for a heading. C#
constructs a final sized array without a value node or an intermediate suffix
array. Java appends decoded key elements into the output list. Rust and C++
append decoded suffix elements into the owned output vector, reserving space
for the common single-element relative suffix. Deep-scope release remains
iterative in Rust and C++.

The Go-specific whitespace, ASCII validation, quoted-string sizing and pipe-body
changes were not transferred as a bundle. The other runtimes have different
string and container costs; this adoption covers only value-path construction.
The syntax specification, common decoding rules and benchmark harnesses are
unchanged. The Go consumer and prefix-selection APIs are outside this change.

## Findings and limits

The clearest timing gains are in the mixed documents, where every value needs
a resolved path:

- Rust's medians fell by **12.6–24.4%** across the three mixed sizes and both
  modes; C++ fell by **12.8–19.6%**. Their baseline and candidate timing ranges
  are separated in all six mixed conditions in this campaign.
- Python's mixed medians fell by **4.2–7.4%**. The smaller fixtures show the
  clearest separation; the large-input ranges overlap.
- C# allocates **72 fewer bytes per value** in these fixtures, including
  1,440,000 fewer allocated bytes per 20,000-value pass in each mode. The
  512-value collecting median fell from 0.737 to 0.688 ms, with separated
  ranges. Its large collecting RSS nevertheless rose from 51.12 to 55.49 MiB;
  lower allocation traffic does not guarantee a lower process peak.
- Java's large mixed RSS medians fell from **187.76 to 175.43 MiB** in
  streaming and **195.39 to 188.34 MiB** in collecting. Its large collecting
  time is effectively unchanged, 22.098 → 22.104 ms, with overlapping ranges.

JavaScript/TypeScript removes a temporary value node, but the timings do not
establish a broad speed gain: the large mixed medians change by +2.6% in
streaming and -1.8% in collecting, with overlapping ranges. The depth workload
also has higher candidate medians in Rust collecting (+14.9%) and C++ streaming
(+10.9%), again with overlapping ranges. This adoption therefore claims less
intermediate path-construction work, with the observed timing and memory
benefits scoped to the conditions above; it does not claim that every condition
became faster or used less process memory.

The tables below summarise all five samples for each condition through their
medians and ranges; the raw report retains each sample and median absolute deviation.
Changes are the candidate median divided by the baseline median, minus one.
A negative time change means a lower observed time. Overlapping ranges and
small differences do not establish a stable speed improvement.

Go is an unchanged control in this campaign. Movement in its observations
illustrates the variation possible without a parser change. A single large
line and two large blocks spend little work constructing paths, so this change
does not target their dominant cost. Cross-language comparisons also include
the runtime and the sample implementation; these tables are not a general
language ranking.

Process RSS includes the runtime, startup, warm-up, buffers and allocator/GC
behaviour. It is distinct from cumulative allocation traffic, which is
available only for Go and C# here. Fewer intermediate objects need not lower
the observed process high-water mark. Java's output lists can also retain spare
capacity after appending suffix elements; the depth workload is retained to
make that trade-off inspectable. These measurements do not measure joules or
prove a reduction in electrical energy consumption.

## Results

<!-- results:start -->

All 840 observations and 84 independent complete-entry validations are retained in [report.json](report.json).
Every condition has five fresh-process observations. Times are milliseconds per pass; ranges are observed minima and maxima.
Time change is `(candidate / baseline - 1) × 100%`; a negative value means a shorter elapsed time.
RSS is the median observed peak process memory in MiB and includes runtime, startup and warm-up activity.
The report also retains the median absolute deviation (MAD), complete RSS ranges and process wall-time distributions.

### Large mixed document

The `mixed-20000` fixture contains 20,000 value occurrences, including repeated keys, annotations, Unicode and multiline values.
Go is the unchanged control. JavaScript and TypeScript use the same parser engine.

| Engine | Mode | Baseline ms/pass | Candidate ms/pass | Time change | Baseline RSS MiB | Candidate RSS MiB |
|---|---|---:|---:|---:|---:|---:|
| Go | stream | 4.587 | 4.470 | -2.6% | 12.10 | 12.25 |
| Go | collect | 6.720 | 6.731 | +0.2% | 21.04 | 21.01 |
| Python | stream | 178.407 | 169.768 | -4.8% | 15.64 | 15.64 |
| Python | collect | 165.441 | 156.307 | -5.5% | 22.86 | 22.76 |
| JavaScript / TypeScript | stream | 29.246 | 30.017 | +2.6% | 55.03 | 55.05 |
| JavaScript / TypeScript | collect | 33.772 | 33.149 | -1.8% | 125.18 | 126.39 |
| Java | stream | 20.680 | 19.935 | -3.6% | 187.76 | 175.43 |
| Java | collect | 22.098 | 22.104 | +0.0% | 195.39 | 188.34 |
| C# | stream | 21.147 | 22.061 | +4.3% | 45.66 | 45.14 |
| C# | collect | 31.442 | 30.596 | -2.7% | 51.12 | 55.49 |
| Rust | stream | 9.380 | 7.540 | -19.6% | 5.08 | 5.07 |
| Rust | collect | 13.313 | 11.193 | -15.9% | 12.59 | 12.62 |
| C++ | stream | 10.027 | 8.066 | -19.6% | 4.57 | 4.57 |
| C++ | collect | 14.404 | 11.746 | -18.5% | 10.59 | 10.60 |

### All workloads

The timing columns show median [minimum, maximum]. The RSS column shows baseline → candidate median MiB.

#### mixed-32

32 occurrences, repeated keys, opaque annotations, Unicode, unset, pipe/triple blocks.
1,911 input bytes; 33 entries; 1,000 timed passes per process.

| Engine | Mode | Baseline ms/pass [min, max] | Candidate ms/pass [min, max] | Time change | RSS MiB, baseline → candidate |
|---|---|---:|---:|---:|---:|
| Go | stream | 0.012 [0.011, 0.015] | 0.012 [0.011, 0.016] | -5.7% | 12.09 → 11.93 |
| Go | collect | 0.017 [0.013, 0.020] | 0.014 [0.014, 0.021] | -17.6% | 12.10 → 12.45 |
| Python | stream | 0.298 [0.294, 0.308] | 0.281 [0.280, 0.284] | -5.5% | 15.61 → 15.59 |
| Python | collect | 0.289 [0.286, 0.302] | 0.268 [0.267, 0.272] | -7.4% | 15.68 → 15.57 |
| JavaScript / TypeScript | stream | 0.141 [0.140, 0.145] | 0.146 [0.144, 0.160] | +3.2% | 53.68 → 53.59 |
| JavaScript / TypeScript | collect | 0.150 [0.143, 0.162] | 0.153 [0.141, 0.160] | +2.3% | 53.62 → 53.66 |
| Java | stream | 0.089 [0.087, 0.092] | 0.093 [0.087, 0.100] | +5.0% | 122.52 → 118.49 |
| Java | collect | 0.093 [0.085, 0.099] | 0.085 [0.083, 0.092] | -8.7% | 123.94 → 117.77 |
| C# | stream | 0.048 [0.048, 0.054] | 0.048 [0.048, 0.050] | -0.2% | 39.33 → 39.32 |
| C# | collect | 0.050 [0.048, 0.056] | 0.048 [0.047, 0.052] | -5.0% | 39.38 → 39.34 |
| Rust | stream | 0.020 [0.018, 0.021] | 0.015 [0.015, 0.016] | -24.4% | 5.04 → 5.03 |
| Rust | collect | 0.020 [0.019, 0.022] | 0.017 [0.017, 0.018] | -13.1% | 5.16 → 5.15 |
| C++ | stream | 0.020 [0.020, 0.020] | 0.017 [0.016, 0.019] | -15.7% | 4.56 → 4.56 |
| C++ | collect | 0.021 [0.021, 0.025] | 0.018 [0.018, 0.021] | -15.2% | 4.57 → 4.57 |

#### mixed-512

512 occurrences, repeated keys, opaque annotations, Unicode, unset, pipe/triple blocks.
30,299 input bytes; 514 entries; 50 timed passes per process.

| Engine | Mode | Baseline ms/pass [min, max] | Candidate ms/pass [min, max] | Time change | RSS MiB, baseline → candidate |
|---|---|---:|---:|---:|---:|
| Go | stream | 0.141 [0.126, 0.168] | 0.141 [0.139, 0.152] | -0.2% | 11.59 → 11.64 |
| Go | collect | 0.171 [0.162, 0.231] | 0.165 [0.156, 0.215] | -3.7% | 12.30 → 12.25 |
| Python | stream | 4.470 [4.422, 4.772] | 4.255 [4.225, 4.320] | -4.8% | 15.55 → 15.63 |
| Python | collect | 4.267 [4.148, 4.818] | 4.089 [3.904, 4.195] | -4.2% | 15.85 → 15.83 |
| JavaScript / TypeScript | stream | 1.067 [1.042, 1.081] | 1.066 [1.059, 1.084] | -0.1% | 56.53 → 55.78 |
| JavaScript / TypeScript | collect | 1.075 [1.060, 1.178] | 1.099 [1.071, 1.168] | +2.2% | 56.54 → 55.83 |
| Java | stream | 1.359 [1.268, 1.403] | 1.344 [1.252, 1.471] | -1.1% | 107.45 → 107.06 |
| Java | collect | 1.357 [1.315, 1.504] | 1.332 [1.281, 1.403] | -1.8% | 108.67 → 107.02 |
| C# | stream | 0.690 [0.682, 0.792] | 0.701 [0.695, 0.707] | +1.6% | 39.25 → 39.25 |
| C# | collect | 0.737 [0.725, 0.758] | 0.688 [0.673, 0.694] | -6.6% | 43.36 → 39.33 |
| Rust | stream | 0.245 [0.240, 0.273] | 0.198 [0.195, 0.224] | -19.1% | 5.04 → 5.04 |
| Rust | collect | 0.333 [0.321, 0.376] | 0.291 [0.276, 0.311] | -12.6% | 5.30 → 5.26 |
| C++ | stream | 0.265 [0.265, 0.275] | 0.218 [0.215, 0.252] | -17.8% | 4.56 → 4.56 |
| C++ | collect | 0.360 [0.357, 0.407] | 0.314 [0.304, 0.350] | -12.8% | 4.57 → 4.57 |

#### mixed-20000

20000 occurrences, repeated keys, opaque annotations, Unicode, unset, pipe/triple blocks.
1,183,212 input bytes; 20,079 entries; 3 timed passes per process.

| Engine | Mode | Baseline ms/pass [min, max] | Candidate ms/pass [min, max] | Time change | RSS MiB, baseline → candidate |
|---|---|---:|---:|---:|---:|
| Go | stream | 4.587 [4.283, 4.731] | 4.470 [4.439, 4.530] | -2.6% | 12.10 → 12.25 |
| Go | collect | 6.720 [6.385, 7.273] | 6.731 [5.776, 7.164] | +0.2% | 21.04 → 21.01 |
| Python | stream | 178.407 [173.906, 182.022] | 169.768 [166.188, 179.779] | -4.8% | 15.64 → 15.64 |
| Python | collect | 165.441 [163.427, 167.375] | 156.307 [154.526, 168.308] | -5.5% | 22.86 → 22.76 |
| JavaScript / TypeScript | stream | 29.246 [28.988, 30.335] | 30.017 [29.181, 31.343] | +2.6% | 55.03 → 55.05 |
| JavaScript / TypeScript | collect | 33.772 [32.263, 34.689] | 33.149 [32.608, 33.509] | -1.8% | 125.18 → 126.39 |
| Java | stream | 20.680 [19.358, 21.039] | 19.935 [19.558, 21.608] | -3.6% | 187.76 → 175.43 |
| Java | collect | 22.098 [21.110, 24.445] | 22.104 [20.431, 24.655] | +0.0% | 195.39 → 188.34 |
| C# | stream | 21.147 [20.975, 22.681] | 22.061 [20.801, 24.237] | +4.3% | 45.66 → 45.14 |
| C# | collect | 31.442 [29.480, 32.730] | 30.596 [28.931, 39.748] | -2.7% | 51.12 → 55.49 |
| Rust | stream | 9.380 [9.111, 13.443] | 7.540 [7.477, 8.732] | -19.6% | 5.08 → 5.07 |
| Rust | collect | 13.313 [13.090, 14.168] | 11.193 [10.263, 11.939] | -15.9% | 12.59 → 12.62 |
| C++ | stream | 10.027 [9.900, 11.106] | 8.066 [7.960, 9.297] | -19.6% | 4.57 → 4.57 |
| C++ | collect | 14.404 [13.201, 14.599] | 11.746 [11.018, 12.335] | -18.5% | 10.59 → 10.60 |

#### line-1mib

One literal value with a 1,048,576-byte payload on one physical line.
1,048,593 input bytes; 1 entries; 3 timed passes per process.

| Engine | Mode | Baseline ms/pass [min, max] | Candidate ms/pass [min, max] | Time change | RSS MiB, baseline → candidate |
|---|---|---:|---:|---:|---:|
| Go | stream | 0.981 [0.899, 1.613] | 0.909 [0.586, 1.064] | -7.4% | 12.70 → 12.70 |
| Go | collect | 0.943 [0.767, 1.322] | 0.860 [0.721, 1.192] | -8.8% | 12.67 → 12.69 |
| Python | stream | 46.478 [44.917, 48.587] | 46.792 [46.106, 50.450] | +0.7% | 18.61 → 18.61 |
| Python | collect | 45.932 [45.299, 48.095] | 46.553 [45.251, 48.088] | +1.4% | 18.55 → 18.59 |
| JavaScript / TypeScript | stream | 4.125 [3.940, 4.509] | 4.029 [3.921, 4.826] | -2.3% | 56.04 → 55.91 |
| JavaScript / TypeScript | collect | 4.173 [4.021, 4.379] | 4.324 [4.069, 4.542] | +3.6% | 56.07 → 55.99 |
| Java | stream | 5.063 [3.923, 5.353] | 4.887 [3.795, 5.506] | -3.5% | 84.90 → 85.19 |
| Java | collect | 5.205 [4.129, 5.365] | 5.168 [3.763, 5.489] | -0.7% | 84.95 → 85.04 |
| C# | stream | 2.215 [2.144, 2.430] | 2.219 [1.963, 2.550] | +0.2% | 47.36 → 47.75 |
| C# | collect | 2.284 [1.882, 2.410] | 2.333 [2.040, 2.493] | +2.1% | 47.32 → 47.34 |
| Rust | stream | 1.748 [1.612, 1.782] | 1.735 [1.709, 1.809] | -0.8% | 8.72 → 8.71 |
| Rust | collect | 1.742 [1.718, 1.776] | 1.730 [1.673, 1.767] | -0.6% | 7.72 → 7.71 |
| C++ | stream | 4.511 [4.232, 4.546] | 4.391 [4.156, 4.487] | -2.7% | 7.92 → 7.92 |
| C++ | collect | 4.448 [4.361, 4.537] | 4.370 [4.193, 4.501] | -1.8% | 6.93 → 6.93 |

#### blocks-32kib

One pipe and one triple-quoted block, each with 32,768 ASCII x bytes plus 31 LF separators (32,799 decoded bytes).
65,729 input bytes; 2 entries; 20 timed passes per process.

| Engine | Mode | Baseline ms/pass [min, max] | Candidate ms/pass [min, max] | Time change | RSS MiB, baseline → candidate |
|---|---|---:|---:|---:|---:|
| Go | stream | 0.191 [0.168, 0.227] | 0.147 [0.136, 0.280] | -23.3% | 12.44 → 12.29 |
| Go | collect | 0.184 [0.165, 0.206] | 0.166 [0.160, 0.467] | -9.9% | 12.13 → 12.50 |
| Python | stream | 6.856 [6.727, 7.095] | 6.799 [6.708, 7.168] | -0.8% | 15.71 → 15.71 |
| Python | collect | 6.789 [6.679, 6.836] | 7.002 [6.743, 7.739] | +3.1% | 15.73 → 15.68 |
| JavaScript / TypeScript | stream | 0.441 [0.401, 0.469] | 0.414 [0.402, 0.434] | -6.2% | 44.90 → 44.96 |
| JavaScript / TypeScript | collect | 0.439 [0.432, 0.450] | 0.448 [0.425, 0.469] | +2.1% | 44.54 → 44.73 |
| Java | stream | 0.882 [0.828, 0.937] | 0.826 [0.793, 0.876] | -6.3% | 58.58 → 58.50 |
| Java | collect | 0.839 [0.812, 0.875] | 0.854 [0.832, 0.904] | +1.7% | 58.11 → 58.15 |
| C# | stream | 0.276 [0.261, 0.295] | 0.267 [0.260, 0.304] | -3.2% | 38.84 → 38.86 |
| C# | collect | 0.281 [0.272, 0.304] | 0.267 [0.260, 0.292] | -4.8% | 38.88 → 38.91 |
| Rust | stream | 0.104 [0.095, 0.111] | 0.103 [0.097, 0.105] | -1.6% | 5.43 → 5.40 |
| Rust | collect | 0.093 [0.087, 0.111] | 0.094 [0.090, 0.101] | +0.2% | 5.45 → 5.17 |
| C++ | stream | 0.254 [0.250, 0.306] | 0.264 [0.260, 0.269] | +3.9% | 4.63 → 4.62 |
| C++ | collect | 0.266 [0.262, 0.318] | 0.268 [0.256, 0.298] | +0.8% | 4.62 → 4.62 |

#### depth-256

256 written heading levels and one value per level; two-space structural indentation.
135,580 input bytes; 512 entries; 10 timed passes per process.

| Engine | Mode | Baseline ms/pass [min, max] | Candidate ms/pass [min, max] | Time change | RSS MiB, baseline → candidate |
|---|---|---:|---:|---:|---:|
| Go | stream | 0.712 [0.645, 0.851] | 0.784 [0.620, 0.883] | +10.1% | 12.63 → 12.61 |
| Go | collect | 0.938 [0.762, 1.051] | 0.852 [0.760, 1.264] | -9.2% | 12.90 → 12.93 |
| Python | stream | 6.187 [6.107, 6.970] | 6.106 [5.995, 6.361] | -1.3% | 15.68 → 15.72 |
| Python | collect | 6.457 [6.311, 6.637] | 6.372 [6.313, 7.174] | -1.3% | 16.22 → 16.23 |
| JavaScript / TypeScript | stream | 2.137 [2.045, 2.319] | 1.987 [1.950, 2.216] | -7.0% | 49.54 → 49.57 |
| JavaScript / TypeScript | collect | 2.305 [2.198, 2.380] | 2.284 [2.133, 2.350] | -0.9% | 54.02 → 54.26 |
| Java | stream | 2.549 [2.453, 2.668] | 2.552 [2.490, 2.728] | +0.1% | 72.62 → 72.84 |
| Java | collect | 2.567 [2.399, 2.874] | 2.682 [2.469, 3.232] | +4.4% | 73.10 → 73.04 |
| C# | stream | 1.647 [1.617, 1.916] | 1.685 [1.618, 1.862] | +2.3% | 38.88 → 38.84 |
| C# | collect | 1.670 [1.634, 1.796] | 1.670 [1.653, 1.881] | +0.0% | 39.10 → 39.18 |
| Rust | stream | 2.781 [2.777, 3.134] | 2.760 [2.731, 3.149] | -0.7% | 5.50 → 5.48 |
| Rust | collect | 3.580 [3.531, 4.027] | 4.114 [3.565, 4.410] | +14.9% | 7.71 → 7.72 |
| C++ | stream | 0.947 [0.928, 1.093] | 1.050 [0.908, 1.059] | +10.9% | 4.82 → 4.82 |
| C++ | collect | 1.670 [1.539, 1.740] | 1.555 [1.495, 1.745] | -6.9% | 5.84 → 5.86 |

### Allocation traffic

These native harness counters report bytes allocated during timed passes, divided by the pass count. They do not measure retained memory or peak RSS.
Go uses `runtime.MemStats.TotalAlloc`; C# uses `GC.GetAllocatedBytesForCurrentThread`.
Values are medians of five observations. Differences in the unchanged Go control help show run-to-run variation.

| Workload | Engine | Mode | Baseline B/pass | Candidate B/pass | Allocation change |
|---|---|---|---:|---:|---:|
| mixed-32 | Go | stream | 8,409.9 | 8,410.0 | +0.0% |
| mixed-32 | Go | collect | 16,377.9 | 16,377.4 | -0.0% |
| mixed-32 | C# | stream | 72,336.0 | 70,032.0 | -3.2% |
| mixed-32 | C# | collect | 73,448.0 | 71,144.0 | -3.1% |
| mixed-512 | Go | stream | 70,201.3 | 70,202.6 | +0.0% |
| mixed-512 | Go | collect | 201,250.1 | 201,253.3 | +0.0% |
| mixed-512 | C# | stream | 650,664.0 | 613,800.0 | -5.7% |
| mixed-512 | C# | collect | 667,232.0 | 630,368.0 | -5.5% |
| mixed-20000 | Go | stream | 2,555,864.0 | 2,556,837.3 | +0.0% |
| mixed-20000 | Go | collect | 12,604,520.0 | 12,606,186.7 | +0.0% |
| mixed-20000 | C# | stream | 24,146,456.0 | 22,706,456.0 | -6.0% |
| mixed-20000 | C# | collect | 24,671,048.0 | 23,231,048.0 | -5.8% |
| line-1mib | Go | stream | 2,130,528.0 | 2,128,829.3 | -0.1% |
| line-1mib | Go | collect | 2,130,602.7 | 2,128,957.3 | -0.1% |
| line-1mib | C# | stream | 8,389,176.0 | 8,389,104.0 | -0.0% |
| line-1mib | C# | collect | 8,389,232.0 | 8,389,160.0 | -0.0% |
| blocks-32kib | Go | stream | 384,032.0 | 384,023.2 | -0.0% |
| blocks-32kib | Go | collect | 384,115.6 | 384,365.6 | +0.1% |
| blocks-32kib | C# | stream | 598,584.0 | 598,440.0 | -0.0% |
| blocks-32kib | C# | collect | 598,640.0 | 598,496.0 | -0.0% |
| depth-256 | Go | stream | 1,304,401.6 | 1,304,484.8 | +0.0% |
| depth-256 | Go | collect | 1,435,769.6 | 1,435,712.8 | -0.0% |
| depth-256 | C# | stream | 1,125,856.0 | 1,107,424.0 | -1.6% |
| depth-256 | C# | collect | 1,134,208.0 | 1,115,776.0 | -1.6% |

<!-- results:end -->

## Conditions and validation

The baseline is commit `6c7fa4cd4ef0c6c8019b82664d6d5997bfb5246a`, including
the already tuned Go parser. The candidate differs in the six parser engines
and their native regression tests. Both use the same specification, fixed
[fixture generator](../native-2026-10-24/fixtures.py), native benchmark
harnesses and [memory wrapper](../measure.py). This paired report retains all
baseline and candidate observations together.

The six UTF-8/LF inputs retain the required declaration and blank separator:

| Fixture | Input bytes | Entries | Timed passes per process |
|---|---:|---:|---:|
| `mixed-32` | 1,911 | 33 | 1,000 |
| `mixed-512` | 30,299 | 514 | 50 |
| `mixed-20000` | 1,183,212 | 20,079 | 3 |
| `line-1mib` | 1,048,593 | 1 | 3 |
| `blocks-32kib` | 65,729 | 2 | 20 |
| `depth-256` | 135,580 | 512 | 10 |

Both snapshots' complete decoded CLI entry arrays are checked against
independently generated expectations before timing: **84 validations**. This
covers paths, occurrence order, source lines, raw values, unset/type/form
metadata and decoded strings. The shared corpus additionally passed all
378 raw/decoded CLI checks for each changed engine. Native API regressions
cover mixed relative/absolute paths, multiple suffix elements, quoted empty
elements, self assignment and mutation of previously delivered paths. The
new regressions also pass on the unchanged baseline. TypeScript's strict
NodeNext type check passed against the shared JavaScript interface.

There are **840 fresh-process observations**: two snapshots × seven engines ×
six fixtures × two modes × five rounds. Each round contains every condition
once. One recorded seed (`20261024`) jointly shuffles both snapshots and all
conditions; jobs run serially with no concurrent builds or tests. No samples
are removed. Each native process performs two untimed warm-up passes followed
by the same fixed pass count for its fixture. Both snapshots use identical
toolchains, build flags and default runtime/GC settings. No CPU affinity,
priority change or forced collection is applied.

The normal buffered file reader rewinds a previously opened file on each pass;
warm-up normally leaves it in the operating system's cache. Timed work includes
parsing and common string decoding. Streaming consumes complete entries;
collecting retains syntax entries before decoding. CLI JSON output, independent
validation, compilation and report generation are outside the native timer.
Two warm-ups and finite pass counts do not guarantee JIT steady state.

The host is Windows 11 Enterprise 10.0.26200 AMD64 on an
Intel Core i7-1370P, 14 physical cores, 20 logical processors,
16,819,617,792 bytes of physical memory. The existing custom power scheme was
retained. Preparation records timestamped host, battery and power observations;
desktop/OS activity and thermal state were not continuously controlled. Exact
tool versions, build descriptions and both environment snapshots are retained
in [report.json](report.json).

RSS is the process high-water mark observed through Windows
`PeakWorkingSetSize` polling every 5 ms. Sampling can miss final growth before
exit. Allocation counters cover the timed interval and are normalised per
document pass; they do not describe retained heap or process RSS. All medians,
ranges and deviations in the tables use the complete accepted report.

## Recorded source and reproduction

[report.json](report.json) contains every raw wrapper result, its order and
timestamps, fixture and independent expectation hashes, source/test manifests,
compiler/runtime descriptions and prepared binary hashes. The source manifests
identify the measured snapshots; commit references alone do not identify every
measured source file. The controller verifies all recorded source, input,
controller, configuration and prepared-artifact hashes again at the end. The
accepted report has status `complete`, all 84 validations, all 840 observations
and all 168 condition summaries.

Prepare separate baseline and candidate directories using the unchanged
[Windows preparation helper](../native-2026-10-24/prepare_windows.py), supplying
the same explicit tool paths to both, as described in the
[native guide](../native-2026-10-24/README.md#toolchains-and-reproduction).
Inspect both local configurations to confirm matching tool versions, build
flags, host and power policy. Complete correctness checks before timing.

Run the controller from the candidate checkout with suitable local paths:

```sh
python implementations/benchmarks/run-paired-native.py --baseline-root BASELINE --baseline-config TEMP/before/toolchains.json --candidate-root CANDIDATE --candidate-config TEMP/after/toolchains.json --baseline-commit 6c7fa4cd4ef0c6c8019b82664d6d5997bfb5246a --fixtures TEMP/fixtures --output TEMP/paired-report.json --samples 5 --seed 20261024
```

Add `--verify-only` for the 84 untimed independent output checks. Reports are
written atomically to the selected output path during the run. The
[controller](../run-paired-native.py) rejects changing inputs or sources,
mismatched measurement metadata and incomplete sample rounds. Runtime tuning
environment overrides must be cleared first.

[render.py](render.py) validates the retained raw observations and regenerates
only the marked result tables in this document. `python
implementations/benchmarks/public-rollin-2026-10-24/render.py --check` checks
the tables and recorded source manifests against the completed report. The
measured snapshots remain identified by their recorded hashes as the public
parsers continue to change. To additionally require a checkout's current
production sources to match the measured candidate, pass
`--check --check-current-sources`. Correctness checks validate the recorded
report; performance thresholds are not imposed.
