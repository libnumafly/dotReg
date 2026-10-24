"""Render the checked-in native result tables from the raw report."""
import hashlib
import json
from pathlib import Path


def main():
    directory = Path(__file__).resolve().parent
    report = json.loads((directory / "report.json").read_text(encoding="utf-8"))
    assert len(report["results"]) == len(report["fixtures"]) * len(report["engines"]) * 2 * report["samples_per_condition"]
    names = {"go": "Go", "python": "Python", "javascript": "JavaScript / TypeScript",
             "java": "Java", "csharp": "C#", "rust": "Rust", "cpp": "C++"}
    summary = {(r["fixture"], r["language"], r["mode"]): r for r in report["summary"]}
    text = ["# Native sample parser measurements, 24 October 2026", "",
        "This run measures the current dotReg syntax and common string decoding in all seven native engines. "
        "TypeScript shares the JavaScript engine. It covers the mandatory `@dotReg 1` declaration, "
        "two-space heading indentation, quoted element boundaries, repeated occurrences, annotations, "
        "Unicode, unset values and explicit multiline values. Long-line and deep-heading cases exercise "
        "the format's absence of fixed size/depth limits at the specified sizes. "
        "The consuming application's `Build`, type profile, retrieval and prefix-selection operations "
        "are outside the timed work.", "",
        "The [raw report](report.json) contains every process sample, execution order, UTC timestamps, "
        "fixture and source SHA-256 hashes, native binary hashes, versions and build flags. "
        "Its base commit identifies the starting checkout; the working-tree source hashes identify "
        "the actual snapshot, including the current Go consumer sources and a benchmark harness "
        "correction that consumes decoded string lengths.", "",
        "## Conditions and validation", "",
        "The host was Windows 11 Enterprise 10.0.26200 x64 on an Intel Core i7-1370P "
        "(14 cores, 20 logical processors), with 16,819,617,792 bytes of physical memory. "
        "The existing custom power scheme `MY` was retained. Host and battery observations are "
        "snapshots, rather than continuous controls. Agent builds and other benchmark jobs were "
        "paused during this serial run; other desktop and OS activity was uncontrolled. "
        "No CPU affinity, process-priority change, forced GC or heap budget was applied.", "",
        f"The run started at `{report['started_at_utc']}` and finished at `{report['finished_at_utc']}`. "
        f"Each of the 84 conditions has {report['samples_per_condition']} independent process samples "
        "(420 processes total). Each process performs two untimed warm-up passes and then a fixed "
        "number of timed passes. Iteration counts are identical across engines and modes for each fixture. "
        f"Each sample round shuffles all 84 conditions with PRNG seed `{report['order_seed']}`; "
        "the report retains the actual order.", "",
        "Before timing, all 42 engine/fixture CLI outputs were compared with independently generated "
        "expected entry arrays. The oracle checks every entry's physical line number, kind, resolved "
        "element array, raw text, unset state, annotation and form, plus its decoded string. "
        "Every native benchmark sample also checks its fixture byte count, SHA-256, entry count, "
        "iteration count and mode. These fixture checks complement the separate shared conformance "
        "suite; they do not establish correctness for arbitrary files.", "",
        "Each timed pass seeks the same open file back to its beginning, reads through the native "
        "buffered input API, parses syntax and decodes common strings. OS-cached file reads are included; "
        "file opening, fixture generation, process startup, the two warm-up passes and entry JSON "
        "serialisation are outside the timed interval. Decoded output is consumed and discarded. "
        "Streaming consumes entries as delivered. Collecting retains all syntax entries before decoding, "
        "then discards the collection. Decoded length sinks use native string units and are not "
        "cross-language performance metrics.", "",
        "The following results describe these sample implementations, toolchains and finite pass counts. "
        "Two warm-up passes do not guarantee JVM, V8 or .NET steady state. Native file buffering, GC, "
        "JIT and string representations differ. The comparisons neither isolate language overhead nor "
        "predict every consuming application.", "",
        "## Workloads", "",
        "| Fixture | Bytes | Entries | Values | Timed passes per process |", "|---|---:|---:|---:|---:|"]
    for case in report["fixtures"]:
        text.append(f"| {case['name']} | {case['bytes']:,} | {case['entries']:,} | {case['value_occurrences']:,} | {case['iterations']} |")
    text += ["", "The three mixed fixtures use the same generation pattern as the earlier single-fixture "
             "baseline. The 20,000-value fixture remains byte-for-byte identical "
             "(`dd4b8b98991735cdb7c35e2d3470af3f63058760367c436ab12a9abf81153860`). "
             "The block fixture has one pipe and one triple-quoted value; each contains 32,768 ASCII "
             "`x` bytes and 31 LF separators, for 32,799 decoded bytes. The depth fixture has 256 "
             "headings and one value at each level. All fixtures use UTF-8 and LF; this run does not "
             "compare CRLF, invalid inputs or cancellation paths.", "",
             "## Time and process memory", "",
             "Time columns show the median milliseconds per complete file pass, followed by the "
             "minimum and maximum across five process samples in brackets. Each sample time is the "
             "average of its timed passes. The range describes observed spread, not a confidence interval. "
             "The raw report also records median absolute deviation.", "",
             "Memory columns show median observed process peak working set in MiB "
             "(1 MiB = 1,048,576 bytes). Windows `PeakWorkingSetSize` is sampled every 5 ms. "
             "This includes the runtime, startup, warm-up, input buffers, allocator behaviour and "
             "JIT activity. It is neither retained parser heap nor memory per pass. Polling can "
             "miss final growth before a process exits; short processes have fewer observations. "
             "Streaming may retain less result data without producing a smaller process peak.", ""]
    for case in report["fixtures"]:
        text += [f"### {case['name']}", "", case["description"] + ".", "",
                 "| Engine | Stream ms/pass [min, max] | Collect ms/pass [min, max] | Stream peak MiB | Collect peak MiB |",
                 "|---|---:|---:|---:|---:|"]
        for engine in report["engines"]:
            language = engine["language"]
            stream, collect = (summary[(case["name"], language, mode)] for mode in ("stream", "collect"))
            def elapsed(row):
                return f"{row['median_ms_per_pass']:.4f} [{row['min_ms_per_pass']:.4f}, {row['max_ms_per_pass']:.4f}]"
            def memory(row):
                value = row["median_peak_rss_bytes"]
                return f"{value / 1048576:.2f}" if value is not None else "unavailable"
            text.append(f"| {names[language]} | {elapsed(stream)} | {elapsed(collect)} | {memory(stream)} | {memory(collect)} |")
        text.append("")
    text += ["## Allocation volume", "",
             "Go and C# additionally expose cumulative allocation counters for the timed interval. "
             "The following medians are normalised to KiB per pass (1 KiB = 1,024 bytes). "
             "Go uses `runtime.MemStats.TotalAlloc`; C# uses `GC.GetAllocatedBytesForCurrentThread`. "
             "Their scope differs, and neither measures peak memory. C# also records GC collection "
             "counts in the raw samples. These values help inspect each implementation's streaming "
             "and collecting paths; other engines report process memory only.", "",
             "| Fixture | Go stream KiB/pass | Go collect KiB/pass | C# stream KiB/pass | C# collect KiB/pass |",
             "|---|---:|---:|---:|---:|"]
    for case in report["fixtures"]:
        values = [summary[(case["name"], language, mode)]["median_allocated_bytes_per_pass"] / 1024
                  for language in ("go", "csharp") for mode in ("stream", "collect")]
        text.append("| " + case["name"] + " | " + " | ".join(f"{v:.2f}" for v in values) + " |")
    text += ["", "## Toolchains and reproduction", "",
             "The compiled engines were rebuilt from the source snapshot before validation and timing. "
             "The C# source tree was copied into temporary storage before building, preserving its "
             "project references and avoiding generated outputs in the checkout. "
             "The Rust GNU target uses its bundled MinGW GCC and self-contained CRT; the C++ build "
             "uses LLVM-MinGW with its usual static release flags.", "",
             "| Engine | Runtime / compiler | Build and runtime flags |", "|---|---|---|"]
    for engine in report["engines"]:
        text.append(f"| {names[engine['language']]} | {engine['runtime']} | {engine['build']} |")
    text += ["", "Run from the repository root. Generate fixtures outside the checkout:", "",
             "```sh", "python implementations/benchmarks/native-2026-10-24/fixtures.py TEMP/fixtures", "```", "",
             "Build the native benchmark and CLI commands following the language guides. "
             "On Windows, `prepare_windows.py` accepts explicit installed tool paths with "
             "`--directory`, `--python`, `--node`, `--jdk`, `--dotnet`, `--rustc` and `--cxx`. "
             "It writes a local `toolchains.json` in the selected temporary directory. "
             "No toolchains are downloaded by this helper. Alternatively start from "
             "[config.example.json](config.example.json), add all engines, record exact versions "
             "and flags, and provide an environment snapshot. Local executable paths stay in "
             "the local configuration and are omitted from the published report.", "",
             "```sh", "python implementations/benchmarks/native-2026-10-24/matrix.py --config TEMP/toolchains.json --fixtures TEMP/fixtures --output TEMP/report.json --samples 5 --seed 20261024", "```", "",
             "Use `--verify-only` to run the 42 independent expected-result checks without "
             "timing. Supply a suitable temporary path on the target platform. "
             "The six fixture hashes and generated expectation hashes are in `report.json`; "
             "the large generated fixtures are deliberately omitted from Git. "
             "`render.py` regenerates this document's data tables from the checked-in report. "
             "The [main benchmark guide](../README.md) distinguishes these native file-input "
             "measurements from the separate resident-input comparison of configuration notations.", ""]
    (directory / "README.md").write_text("\n".join(text), encoding="utf-8")


if __name__ == "__main__":
    main()
