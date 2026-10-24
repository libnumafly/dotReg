# Native sample parser measurements, 24 October 2026

This is the initial seven-engine campaign. The later
[Go tuning follow-up](../tuning-2026-10-24/README.md) changes the Go implementation
and measures its decoded-map and API costs separately. The native observations
below remain tied to their original source manifest.

This run measures the current dotReg syntax and common string decoding in all seven native engines. TypeScript shares the JavaScript engine. It covers the mandatory `@dotReg 1` declaration, two-space heading indentation, quoted element boundaries, repeated occurrences, annotations, Unicode, unset values and explicit multiline values. Long-line and deep-heading cases exercise the format's absence of fixed size/depth limits at the specified sizes. The consuming application's `Build`, type profile, retrieval and prefix-selection operations are outside the timed work.

The [raw report](report.json) contains every process sample, execution order, UTC timestamps, fixture and source SHA-256 hashes, native binary hashes, versions and build flags. Its base commit identifies the starting checkout; the working-tree source hashes identify the actual snapshot, including the current Go consumer sources and a benchmark harness correction that consumes decoded string lengths.

## Conditions and validation

The host was Windows 11 Enterprise 10.0.26200 x64 on an Intel Core i7-1370P (14 cores, 20 logical processors), with 16,819,617,792 bytes of physical memory. The existing custom power scheme `MY` was retained. Host and battery observations are snapshots, rather than continuous controls. Agent builds and other benchmark jobs were paused during this serial run; other desktop and OS activity was uncontrolled. No CPU affinity, process-priority change, forced GC or heap budget was applied.

The run started at `2026-10-24T02:23:55.292418+00:00` and finished at `2026-10-24T02:26:18.233722+00:00`. Each of the 84 conditions has 5 independent process samples (420 processes total). Each process performs two untimed warm-up passes and then a fixed number of timed passes. Iteration counts are identical across engines and modes for each fixture. Each sample round shuffles all 84 conditions with PRNG seed `20261024`; the report retains the actual order.

Before timing, all 42 engine/fixture CLI outputs were compared with independently generated expected entry arrays. The oracle checks every entry's physical line number, kind, resolved element array, raw text, unset state, annotation and form, plus its decoded string. Every native benchmark sample also checks its fixture byte count, SHA-256, entry count, iteration count and mode. These fixture checks complement the separate shared conformance suite; they do not establish correctness for arbitrary files.

Each timed pass seeks the same open file back to its beginning, reads through the native buffered input API, parses syntax and decodes common strings. OS-cached file reads are included; file opening, fixture generation, process startup, the two warm-up passes and entry JSON serialisation are outside the timed interval. Decoded output is consumed and discarded. Streaming consumes entries as delivered. Collecting retains all syntax entries before decoding, then discards the collection. Decoded length sinks use native string units and are not cross-language performance metrics.

The following results describe these sample implementations, toolchains and finite pass counts. Two warm-up passes do not guarantee JVM, V8 or .NET steady state. Native file buffering, GC, JIT and string representations differ. The comparisons neither isolate language overhead nor predict every consuming application.

## Workloads

| Fixture | Bytes | Entries | Values | Timed passes per process |
|---|---:|---:|---:|---:|
| mixed-32 | 1,911 | 33 | 32 | 1000 |
| mixed-512 | 30,299 | 514 | 512 | 50 |
| mixed-20000 | 1,183,212 | 20,079 | 20,000 | 3 |
| line-1mib | 1,048,593 | 1 | 1 | 3 |
| blocks-32kib | 65,729 | 2 | 2 | 20 |
| depth-256 | 135,580 | 512 | 256 | 10 |

The three mixed fixtures use the same generation pattern as the earlier single-fixture baseline. The 20,000-value fixture remains byte-for-byte identical (`dd4b8b98991735cdb7c35e2d3470af3f63058760367c436ab12a9abf81153860`). The block fixture has one pipe and one triple-quoted value; each contains 32,768 ASCII `x` bytes and 31 LF separators, for 32,799 decoded bytes. The depth fixture has 256 headings and one value at each level. All fixtures use UTF-8 and LF; this run does not compare CRLF, invalid inputs or cancellation paths.

## Time and process memory

Time columns show the median milliseconds per complete file pass, followed by the minimum and maximum across five process samples in brackets. Each sample time is the average of its timed passes. The range describes observed spread, not a confidence interval. The raw report also records median absolute deviation.

Memory columns show median observed process peak working set in MiB (1 MiB = 1,048,576 bytes). Windows `PeakWorkingSetSize` is sampled every 5 ms. This includes the runtime, startup, warm-up, input buffers, allocator behaviour and JIT activity. It is neither retained parser heap nor memory per pass. Polling can miss final growth before a process exits; short processes have fewer observations. Streaming may retain less result data without producing a smaller process peak.

### mixed-32

32 occurrences, repeated keys, opaque annotations, Unicode, unset, pipe/triple blocks.

| Engine | Stream ms/pass [min, max] | Collect ms/pass [min, max] | Stream peak MiB | Collect peak MiB |
|---|---:|---:|---:|---:|
| Go | 0.0152 [0.0137, 0.0180] | 0.0170 [0.0164, 0.0193] | 12.08 | 12.34 |
| Python | 0.3006 [0.2978, 0.3191] | 0.2923 [0.2828, 0.2953] | 15.55 | 15.56 |
| JavaScript / TypeScript | 0.1491 [0.1419, 0.1617] | 0.1444 [0.1384, 0.1467] | 53.64 | 53.76 |
| Java | 0.0958 [0.0895, 0.1021] | 0.0978 [0.0925, 0.1173] | 121.23 | 122.82 |
| C# | 0.0496 [0.0486, 0.0547] | 0.0498 [0.0478, 0.0545] | 39.35 | 39.33 |
| Rust | 0.0182 [0.0174, 0.0209] | 0.0207 [0.0193, 0.0237] | 5.04 | 5.16 |
| C++ | 0.0199 [0.0196, 0.0213] | 0.0212 [0.0209, 0.0243] | 4.56 | 4.57 |

### mixed-512

512 occurrences, repeated keys, opaque annotations, Unicode, unset, pipe/triple blocks.

| Engine | Stream ms/pass [min, max] | Collect ms/pass [min, max] | Stream peak MiB | Collect peak MiB |
|---|---:|---:|---:|---:|
| Go | 0.2197 [0.1819, 0.2856] | 0.2384 [0.2027, 0.3045] | 11.98 | 12.43 |
| Python | 4.6249 [4.4886, 4.9418] | 4.2636 [4.1289, 4.3617] | 15.55 | 15.74 |
| JavaScript / TypeScript | 1.0826 [1.0719, 1.1743] | 1.1047 [1.0927, 1.1177] | 56.44 | 56.54 |
| Java | 1.3713 [1.3122, 1.4538] | 1.4387 [1.3360, 1.5944] | 106.24 | 107.84 |
| C# | 0.7077 [0.6916, 0.7123] | 0.7608 [0.7233, 0.8382] | 39.25 | 43.36 |
| Rust | 0.2503 [0.2400, 0.2713] | 0.3220 [0.3193, 0.3838] | 5.04 | 5.30 |
| C++ | 0.2672 [0.2647, 0.2847] | 0.3592 [0.3508, 0.4049] | 4.57 | 4.58 |

### mixed-20000

20000 occurrences, repeated keys, opaque annotations, Unicode, unset, pipe/triple blocks.

| Engine | Stream ms/pass [min, max] | Collect ms/pass [min, max] | Stream peak MiB | Collect peak MiB |
|---|---:|---:|---:|---:|
| Go | 6.3321 [5.9681, 6.7186] | 8.7721 [8.4684, 9.0399] | 12.50 | 21.75 |
| Python | 176.4969 [174.9348, 184.0998] | 166.8567 [162.0827, 173.3573] | 15.59 | 22.79 |
| JavaScript / TypeScript | 30.0166 [29.2678, 32.7819] | 34.4042 [33.3582, 34.7507] | 55.04 | 125.12 |
| Java | 23.7966 [22.5014, 25.2358] | 21.2985 [20.6387, 26.5617] | 185.88 | 196.36 |
| C# | 21.9389 [21.2685, 22.7774] | 31.6624 [30.4516, 32.5458] | 45.71 | 55.12 |
| Rust | 9.2529 [9.0854, 10.7751] | 13.4816 [11.8599, 14.1284] | 5.08 | 12.28 |
| C++ | 10.0108 [9.8721, 11.7619] | 13.6869 [13.1623, 14.4684] | 4.57 | 10.62 |

### line-1mib

One literal value with a 1,048,576-byte payload on one physical line.

| Engine | Stream ms/pass [min, max] | Collect ms/pass [min, max] | Stream peak MiB | Collect peak MiB |
|---|---:|---:|---:|---:|
| Go | 1.2465 [0.9015, 1.8701] | 1.3522 [1.1125, 2.0386] | 12.64 | 12.49 |
| Python | 46.5765 [45.6653, 48.4193] | 48.0959 [45.6950, 49.8147] | 18.64 | 18.50 |
| JavaScript / TypeScript | 4.0276 [3.7904, 4.6734] | 4.1329 [4.0524, 4.1869] | 56.00 | 56.01 |
| Java | 5.0213 [4.9522, 5.0970] | 5.1008 [3.9692, 5.2002] | 84.90 | 84.89 |
| C# | 2.2850 [2.2106, 2.3958] | 2.3543 [2.2261, 2.6626] | 47.37 | 47.38 |
| Rust | 1.7890 [1.6633, 1.8124] | 1.7389 [1.6599, 1.7881] | 8.71 | 7.72 |
| C++ | 4.5050 [4.4838, 4.6691] | 4.4990 [4.3963, 4.5333] | 7.93 | 6.93 |

### blocks-32kib

One pipe and one triple-quoted block, each with 32,768 ASCII x bytes plus 31 LF separators (32,799 decoded bytes).

| Engine | Stream ms/pass [min, max] | Collect ms/pass [min, max] | Stream peak MiB | Collect peak MiB |
|---|---:|---:|---:|---:|
| Go | 0.1877 [0.1675, 0.2700] | 0.1674 [0.1618, 0.1995] | 12.32 | 12.31 |
| Python | 6.8577 [6.7201, 7.3984] | 6.8831 [6.8320, 7.6711] | 15.68 | 15.69 |
| JavaScript / TypeScript | 0.4047 [0.3931, 0.4317] | 0.4332 [0.4210, 0.4684] | 44.71 | 44.71 |
| Java | 0.8567 [0.8279, 0.9257] | 0.8502 [0.8255, 0.9172] | 58.27 | 58.21 |
| C# | 0.2809 [0.2725, 0.3201] | 0.2808 [0.2738, 0.2949] | 38.84 | 38.89 |
| Rust | 0.0980 [0.0931, 0.1040] | 0.0930 [0.0807, 0.1058] | 5.45 | 5.46 |
| C++ | 0.2594 [0.2531, 0.2854] | 0.2646 [0.2564, 0.2887] | 4.63 | 4.62 |

### depth-256

256 written heading levels and one value per level; two-space structural indentation.

| Engine | Stream ms/pass [min, max] | Collect ms/pass [min, max] | Stream peak MiB | Collect peak MiB |
|---|---:|---:|---:|---:|
| Go | 0.9146 [0.7934, 1.2033] | 1.0747 [0.8772, 1.1380] | 12.81 | 13.00 |
| Python | 6.2815 [6.1686, 6.8252] | 6.5049 [6.3186, 7.3581] | 15.63 | 16.23 |
| JavaScript / TypeScript | 2.0899 [2.0727, 2.2205] | 2.2897 [2.1954, 2.3270] | 49.55 | 54.06 |
| Java | 2.5493 [2.2791, 2.6207] | 2.6466 [2.5147, 3.1043] | 72.93 | 73.10 |
| C# | 1.7393 [1.6292, 1.8762] | 1.8909 [1.6393, 1.9757] | 38.91 | 39.06 |
| Rust | 3.1170 [2.8619, 3.2054] | 3.5967 [3.5387, 3.7686] | 5.42 | 7.76 |
| C++ | 0.9390 [0.9314, 0.9472] | 1.6800 [1.5746, 1.7247] | 4.91 | 5.82 |

## Allocation volume

Go and C# additionally expose cumulative allocation counters for the timed interval. The following medians are normalised to KiB per pass (1 KiB = 1,024 bytes). Go uses `runtime.MemStats.TotalAlloc`; C# uses `GC.GetAllocatedBytesForCurrentThread`. Their scope differs, and neither measures peak memory. C# also records GC collection counts in the raw samples. These values help inspect each implementation's streaming and collecting paths; other engines report process memory only.

| Fixture | Go stream KiB/pass | Go collect KiB/pass | C# stream KiB/pass | C# collect KiB/pass |
|---|---:|---:|---:|---:|
| mixed-32 | 10.35 | 18.12 | 70.64 | 71.73 |
| mixed-512 | 102.56 | 230.63 | 635.41 | 651.59 |
| mixed-20000 | 3824.99 | 13635.72 | 23580.52 | 24092.82 |
| line-1mib | 2080.66 | 2080.74 | 8192.55 | 8192.61 |
| blocks-32kib | 375.15 | 375.23 | 584.55 | 584.61 |
| depth-256 | 1290.31 | 1419.14 | 1099.47 | 1107.62 |

## Toolchains and reproduction

The compiled engines were rebuilt from the source snapshot before validation and timing. The C# source tree was copied into temporary storage before building, preserving its project references and avoiding generated outputs in the checkout. The Rust GNU target uses its bundled MinGW GCC and self-contained CRT; the C++ build uses LLVM-MinGW with its usual static release flags.

| Engine | Runtime / compiler | Build and runtime flags |
|---|---|---|
| Go | go version go1.27.0 windows/amd64 | go build -trimpath; default optimisation/runtime |
| Python | Python 3.12.14 | Interpreted, default flags |
| JavaScript / TypeScript | Node.js v24.19.0; TypeScript shares this engine | Default V8 flags |
| Java | openjdk version "17.0.17" 2025-10-21 LTS / OpenJDK Runtime Environment Microsoft-12574423 (build 17.0.17+10-LTS) / OpenJDK 64-Bit Server VM Microsoft-12574423 (build 17.0.17+10-LTS, mixed mode, sharing) | javac -encoding UTF-8; default JVM/GC flags |
| C# | .NET SDK 8.0.426; Microsoft.NETCore.App 8.0.32 | dotnet build -c Release --nologo; default .NET/GC flags |
| Rust | rustc 1.99.1 (58cf8f9c3 2026-10-13), x86_64-pc-windows-gnu | rustc --edition=2021 -C opt-level=3 -C link-self-contained=yes; GNU target linked with Rust toolchain's bundled MinGW GCC |
| C++ | clang version 23.1.4 (https://github.com/llvm/llvm-project.git 7a941dfb2877a0b8cf30323fefa85ad019e0b0f5); x86_64-w64-windows-gnu, posix threads | clang++ -std=c++17 -O2 -Wall -Wextra -pedantic -static |

Run from the repository root. Generate fixtures outside the checkout:

```sh
python implementations/benchmarks/native-2026-10-24/fixtures.py TEMP/fixtures
```

Build the native benchmark and CLI commands following the language guides. On Windows, `prepare_windows.py` accepts explicit installed tool paths with `--directory`, `--python`, `--node`, `--jdk`, `--dotnet`, `--rustc` and `--cxx`. It writes a local `toolchains.json` in the selected temporary directory. No toolchains are downloaded by this helper. Alternatively start from [config.example.json](config.example.json), add all engines, record exact versions and flags, and provide an environment snapshot. Local executable paths stay in the local configuration and are omitted from the published report.

```sh
python implementations/benchmarks/native-2026-10-24/matrix.py --config TEMP/toolchains.json --fixtures TEMP/fixtures --output TEMP/report.json --samples 5 --seed 20261024
```

Use `--verify-only` to run the 42 independent expected-result checks without timing. Supply a suitable temporary path on the target platform. The six fixture hashes and generated expectation hashes are in `report.json`; the large generated fixtures are deliberately omitted from Git. `render.py` regenerates this document's data tables from the checked-in report. The [main benchmark guide](../README.md) distinguishes these native file-input measurements from the separate resident-input comparison of configuration notations.
