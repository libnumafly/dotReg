# Time and memory measurements

The [parser set](../README.md) provides streaming and collecting APIs. This
directory measures both through the native APIs with the same UTF-8 fixture.
Each harness parses syntax and performs common string decoding, consumes the
results, then discards them. Entry JSON serialisation is outside the timed work.
TypeScript uses the measured JavaScript engine.

## Reproduce the workload

Run from the repository root with Node.js and Python available. Generate one
fixture and supply that exact file to every harness:

```sh
node implementations/benchmarks/fixture.mjs /tmp/dotreg-benchmark.txt 20000
```

Use a suitable temporary path on Windows. The default count is 20,000 value
occurrences, plus 79 headings: 20,079 entries in 1,183,212 bytes. The fixture mixes
repeated keys, opaque annotations, literal paths, Japanese, quoted/escaped keys
and values, unset values, both multiline forms and 256-character literal values.
Generation is outside the timed work. Its SHA-256 is
`dd4b8b98991735cdb7c35e2d3470af3f63058760367c436ab12a9abf81153860`.

Build compiled samples first using their language guides. Every benchmark
accepts `fixture iterations stream|collect`, with `--benchmark` before those
arguments on the Rust and C++ sample binaries:

| Engine | Command prefix before `fixture iterations mode` |
|---|---|
| Go | Build `implementations/go/cmd/configbench`, then run the binary |
| Python | `python implementations/python/benchmark.py` |
| JavaScript / TypeScript | `node implementations/javascript/benchmark.mjs` |
| Java | Compile `DotReg.java` and `DotRegBenchmark.java`, then `java -cp CLASSES DotRegBenchmark` |
| C# | Build `implementations/csharp/benchmarks/DotReg.Benchmark.csproj -c Release`, then `dotnet implementations/csharp/benchmarks/bin/Release/net8.0/DotReg.Benchmark.dll` |
| Rust | `implementations/rust/target/release/dotreg-sample --benchmark` |
| C++ | `./dotreg-cpp --benchmark` |

For example:

```sh
python implementations/python/benchmark.py /tmp/dotreg-benchmark.txt 5 stream
python implementations/python/benchmark.py /tmp/dotreg-benchmark.txt 5 collect
```

Each harness opens one file and reuses it, resetting its position before every
pass. It performs two untimed warm-up passes, then reports the total time for the
requested passes. `entries` is the count in one pass. Input is read through file
buffers during each pass. The fixture is not preloaded by the harness. The OS
file cache, compilation/JIT activity and runtime GC can affect the results.

The result JSON includes `language`, `mode`, `iterations`, `input_bytes`,
`entries` and `elapsed_seconds`. Some harnesses also report allocation counters
or consumed string units. Units reflect the language's native representation;
they are consumption sinks and are not cross-language comparison metrics.

## Measure process memory

Run each mode in a separate process. The standard-library Python wrapper accepts
a complete JSON argv array, including the final fixture/iteration/mode arguments:

```sh
python implementations/benchmarks/measure.py --command '["python","implementations/python/benchmark.py","/tmp/dotreg-benchmark.txt","5","stream"]'
```

It samples Windows `PeakWorkingSetSize` or Linux `/proc` `VmHWM` every 5 ms and
adds `observed_peak_rss_bytes`, its method, sample count, process wall time,
platform and fixture hash. Process RSS includes the interpreter or VM, JIT code,
buffers, allocator behaviour and warm-up. It is distinct from live parser heap,
retained values and cumulative allocation volume. Polling may miss the final
increase before a short process exits. Unsupported platforms report unavailable
memory as JSON `null`.

Go's `runtime.MemStats.TotalAlloc` delta and C#'s thread allocation counter measure
cumulative allocation volume over timed passes. These optional fields are
explicitly labelled in the output; they are not peak RSS. Process wall time also
includes startup and warm-up, whereas `elapsed_seconds` measures only the native
timed passes.

For repeated serial measurements, create a local JSON configuration:

```json
{
  "fixture": "/tmp/dotreg-benchmark.txt",
  "iterations": 5,
  "samples": 3,
  "engines": [
    {
      "language": "python",
      "runtime": "Record the exact Python version and flags here",
      "command": ["python", "implementations/python/benchmark.py"],
      "sources": ["implementations/python/dotreg.py", "implementations/python/benchmark.py"]
    }
  ]
}
```

```sh
python implementations/benchmarks/suite.py --config local-toolchains.json --output measurements.json
```

Add the other command prefixes from the table as engines. The suite runs one
mode and sample at a time, checks occurrence counts across engines, and records
runtime descriptions, source hashes and all individual samples. Local executable
paths are kept in the local configuration and are not copied into the report.
Runtime descriptions must match the binaries you actually built.

## Recorded observation

The [2026-10-24 Windows report](2026-10-24-windows.json) records three isolated
processes per engine and mode, each with two warm-ups and five timed passes.
It uses Windows 11 x64, the default 20,000-value fixture, default runtime/GC
settings and the exact source hashes and toolchains recorded in the report.
These observations establish a reproducible local baseline for the samples.
They do not compare against older parsers or establish performance guarantees.

The following are medians of the three process samples. Time is milliseconds
per full pass (total timed seconds divided by five). Memory is observed process
peak RSS in MiB, including the runtime and warm-up:

| Engine | Stream, ms/pass | Collect, ms/pass | Stream, MiB | Collect, MiB |
|---|---:|---:|---:|---:|
| Go | 6.12 | 8.84 | 12.49 | 21.94 |
| Python | 180.48 | 166.50 | 15.61 | 23.20 |
| JavaScript / TypeScript | 28.29 | 31.65 | 56.85 | 127.61 |
| Java | 20.74 | 19.42 | 242.43 | 239.51 |
| C# | 21.22 | 29.77 | 41.77 | 51.73 |
| Rust | 9.39 | 13.54 | 5.08 | 12.64 |
| C++ | 10.14 | 13.48 | 4.56 | 10.72 |

Python's collecting mode was faster on this fixture. Java's default JVM grew
its process heap differently between modes, and streaming did not lower its
observed RSS. The samples retain less result data when streaming, but this
measurement does not promise lower elapsed time or process memory in every
runtime. Applications with a JVM heap budget can include that budget in the
Java command, record the flags, and measure again.

The collecting API intentionally retains every syntax entry and expanded path
until decoding completes. Streaming drops each consumed occurrence. Both need
memory for the reader, active headings, current expanded path and complete
current multiline body; one very large line or block can dominate either mode.
Process-level GC and heap growth can make streaming RSS equal to or greater than
collecting RSS on a particular run, despite the smaller retained result set.
Inspect time and memory together, and measure the intended application workload.

The CI benchmark runs a small smoke workload to validate the harnesses and
counts. It has no speed or memory threshold. Shared syntax/decoding conformance
and native callback tests are separate checks; see [conformance](../conformance/README.md).
