"""Validate and render the published Go multiline and allocation campaign.

All three completed JSON reports and their three raw-output files are required;
an empty template is not a completed result. The shared paired-Go result module
supplies the ten-round observation validation and formatting helpers. Reports
are validated against their own source records. --check-current-sources
optionally compares current production bytes.
No parser, compiler, benchmark, Git or network process is launched.
"""

import argparse
import hashlib
import importlib.util
import json
import math
from pathlib import Path
import re
import sys


HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent.parent
LEGACY_PATH = HERE.parent / "paired-go-results.py"
BEGIN = "<!-- results:start -->"
END = "<!-- results:end -->"
VARIANTS = ("baseline", "candidate")
ASSEMBLY_CASES = (
    "PipeShort3LF", "TripleShort3LF", "PipeManyShortLF", "PipeManyShortCRLF",
    "TripleManyShortLF", "TripleManyShortCRLF", "PipeLong1MiB", "TripleLong1MiB",
    "PipeDiscardedMargin", "TripleDiscardedMargin", "PipeBoundary1MiB", "PipeBuffered8Rows",
    "Bare512CollectDecode", "Quoted512CollectDecode",
    "ParseBuildDistinct512", "ParseBuildRepeated512",
)
ASSEMBLY_NAMES = {"BenchmarkAssembly/" + case for case in ASSEMBLY_CASES}
OVERLAY_SOURCES = ("implementations/benchmarks/assembly/go.mod",
                   "implementations/benchmarks/assembly/assembly_test.go")
MODULES = {"comparison": "implementations/benchmarks/comparison",
           "go-api": "implementations/go", "assembly": "implementations/benchmarks/assembly"}
OBSERVATIONS = 1600


def load_legacy():
    spec = importlib.util.spec_from_file_location("dotreg_paired_go_results", LEGACY_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


legacy = load_legacy()
require = legacy.require
ReportError = legacy.ReportError


def valid_digest(value):
    return isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value) is not None


def relative_source(name):
    require(isinstance(name, str) and name and "\\" not in name and ":" not in name,
            "Source names must be repository-relative POSIX paths")
    relative = Path(name)
    require(not relative.is_absolute() and ".." not in relative.parts and relative.parts,
            "Source names must be repository-relative paths without parent traversal")
    return relative


def validate_sources(reports):
    for label, report in reports.items():
        require(report.get("module") == MODULES[label], f"{label}: unexpected measured module")
        controllers = report.get("controller_source_sha256", {})
        require(set(controllers) == {"implementations/benchmarks/run-paired-go.py",
                                     "implementations/benchmarks/run-go.py"}
                and all(valid_digest(digest) for digest in controllers.values()),
                f"{label}: invalid controller hash record")
        for name in controllers:
            relative_source(name)
        for variant in VARIANTS:
            record = report["variants"][variant]
            require(isinstance(record.get("source_state"), str) and record["source_state"].strip(),
                    f"{label}/{variant}: missing explanation of measured source state")
            require(valid_digest(record.get("binary_sha256")), f"{label}/{variant}: invalid binary hash")
            manifest = record.get("source_sha256")
            require(isinstance(manifest, dict) and manifest, f"{label}/{variant}: missing source manifest")
            for name, digest in manifest.items():
                relative_source(name)
                require(valid_digest(digest), f"{label}/{variant}: invalid recorded source digest: {name}")
            required = {"docs/config-format.md", "implementations/go/parser.go",
                        "implementations/go/profile.go", "implementations/go/value.go",
                        MODULES[label] + "/go.mod"}
            require(required <= manifest.keys(), f"{label}/{variant}: incomplete measured source manifest")
        require(report["variants"]["baseline"].get("go_environment") ==
                report["variants"]["candidate"].get("go_environment"),
                f"{label}: Go build environment differs between variants")
    labels = tuple(reports)
    for i, left in enumerate(labels):
        for right in labels[i + 1:]:
            legacy.validate_shared_sources(reports[left], reports[right])
            for variant in VARIANTS:
                require(reports[left]["variants"][variant].get("go_environment") ==
                        reports[right]["variants"][variant].get("go_environment"),
                        "Go build environment differs between campaigns")
    # The added measurement module is common instrumentation. It is not a
    # production change in the baseline, whose commit predates this module.
    assembly = reports["assembly"]["variants"]
    for name in OVERLAY_SOURCES:
        require(name in assembly["baseline"]["source_sha256"] and
                name in assembly["candidate"]["source_sha256"],
                f"Common measurement overlay is missing: {name}")
        require(assembly["baseline"]["source_sha256"][name] ==
                assembly["candidate"]["source_sha256"][name],
                f"Measurement instrumentation differs between variants: {name}")


def check_current_sources(reports):
    manifest = {}
    for report in reports.values():
        manifest.update(report["variants"]["candidate"]["source_sha256"])
    names = {"docs/config-format.md", "implementations/go/go.mod"}
    names.update(name for name in manifest if name.startswith("implementations/go/")
                 and name.endswith(".go") and not name.endswith("_test.go"))
    for name in sorted(names):
        require(name in manifest, f"Current production source record is missing: {name}")
        source = (ROOT / relative_source(name)).resolve()
        require(source.is_relative_to(ROOT) and source.is_file(), f"Current production source is missing: {name}")
        require(hashlib.sha256(source.read_bytes()).hexdigest() == manifest[name],
                f"Current production bytes differ from the measured candidate: {name}")


def validate_comparisons(label, report, computed, expected_names):
    comparisons = report.get("comparison", [])
    require(len(comparisons) == len(expected_names), f"{label}: incomplete comparison records")
    seen = set()
    for comparison in comparisons:
        name = comparison.get("benchmark")
        require(name in expected_names and name not in seen, f"{label}: duplicate or unexpected comparison")
        seen.add(name)
        before, after = legacy.pair(computed, name)
        metrics = comparison.get("metrics", {})
        require(set(metrics) == set(before), f"{label}: comparison metric set differs from observations")
        for metric in before:
            baseline = before[metric]["median"]
            candidate = after[metric]["median"]
            expected = {"candidate_over_baseline_medians": candidate / baseline if baseline else None,
                        "change_percent": (candidate / baseline - 1) * 100 if baseline else None}
            require(set(metrics[metric]) == set(expected), f"{label}: unexpected comparison fields")
            for key, value in expected.items():
                actual = metrics[metric][key]
                require(actual is None if value is None else
                        isinstance(actual, (int, float)) and math.isclose(actual, value, rel_tol=1e-12, abs_tol=1e-12),
                        f"{label}: recorded comparison differs from raw observations")


def validate_raw_text(path, report):
    """Cross-check every timed JSON row against its separately retained text."""
    header = re.compile(r"^# variant=(baseline|candidate) sample=(\d+) order_seed=(\d+) "
                        r"sequence=(\d+) operation=(Benchmark\S+) started_at_utc=(\S+)$")
    numeric = re.compile(r"^(Benchmark\S+)\s+(\d+)\s+(.+)$")
    rows = {row["execution_sequence"]: row for row in report["results"]}
    seen = set()
    current = None
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("# discovery "):
            current = None
            continue
        match = header.fullmatch(line)
        if match:
            variant, sample, seed, sequence, name, timestamp = match.groups()
            sequence = int(sequence)
            require(sequence in rows and sequence not in seen,
                    f"{path.name}: unexpected or duplicate timed process header")
            current = rows[sequence]
            require((variant, int(sample), int(seed), name, timestamp) ==
                    (current["variant"], current["sample"], current["order_seed"],
                     current["benchmark"], current["started_at_utc"]),
                    f"{path.name}: raw process header differs from JSON observation")
            continue
        match = numeric.fullmatch(line)
        if current is None or not match:
            continue
        name, iterations, fields = match.groups()
        fields = fields.split()
        require(len(fields) % 2 == 0, f"{path.name}: incomplete raw metric pair")
        metrics = {fields[i + 1]: float(fields[i]) for i in range(0, len(fields), 2)}
        require(len(metrics) == len(fields) // 2, f"{path.name}: duplicate raw metric name")
        require(name == current["benchmark"] and int(iterations) == current["iterations"]
                and metrics == current["metrics"], f"{path.name}: timed raw numbers differ from JSON observation")
        sequence = current["execution_sequence"]
        require(sequence not in seen, f"{path.name}: multiple numeric observations in one isolated child")
        seen.add(sequence)
    require(seen == set(rows), f"{path.name}: missing separately retained raw observations")


def detailed_row(label, before, after):
    return legacy.detailed_row(label, before, after)


def detailed_header():
    return [line.replace("Tuned", "Candidate").replace("tuned", "candidate")
            for line in legacy.detailed_header()]


def render(metrics):
    comparison, api, assembly = (metrics[key] for key in ("comparison", "go-api", "assembly"))
    lines = ["All **1,600 timed observations** are retained: 720 in the format campaign,",
             "560 in the Go API campaign and 320 in the focused assembly campaign.",
             "Each operation has ten baseline and ten candidate observations in serial,",
             "jointly shuffled fresh processes. No observations are removed.", "",
             "Times below are microseconds per complete operation, shown as median",
             "[minimum, maximum] in the detailed tables. Ranges describe observations,",
             "not confidence intervals. Allocation columns show median B/op and allocs/op;",
             "every recorded numeric metric retains its median, minimum and maximum in",
             "the JSON reports. Negative time change means a lower candidate median.", "",
             "### Direct loading and the fastest other implementation", "",
             "The fastest other column selects the lowest candidate median among the",
             "unchanged JSON, TOML, YAML and INI operations in this campaign. The ratio",
             "divides candidate dotReg direct time by that median. This compares the",
             "recorded implementations and spellings; it is not a universal format ranking.", "",
             "| Case | Baseline µs/load | Candidate µs/load | Time change | B/op, baseline → candidate | allocs/op, baseline → candidate | Fastest other candidate median | Candidate / other |",
             "|---|---:|---:|---:|---:|---:|---:|---:|"]
    for case in legacy.CASES:
        before, after = legacy.pair(comparison, f"BenchmarkConfig/{case}/dotreg-direct")
        best = min(legacy.OTHER_OPERATIONS,
                   key=lambda operation: comparison[("candidate", f"BenchmarkConfig/{case}/{operation}")]["ns/op"]["median"])
        best_time = comparison[("candidate", f"BenchmarkConfig/{case}/{best}")]["ns/op"]["median"]
        before_time, after_time = before["ns/op"]["median"], after["ns/op"]["median"]
        lines.append(f"| {legacy.CASE_LABELS[case]} | {legacy.number(before_time / 1000)} | "
                     f"{legacy.number(after_time / 1000)} | {legacy.percent(legacy.delta(before_time, after_time))} | "
                     f"{legacy.transition(before, after, 'B/op')} | {legacy.transition(before, after, 'allocs/op')} | "
                     f"{legacy.LABELS[best]} {legacy.number(best_time / 1000)} µs | {legacy.number(after_time / best_time)}× |")
    lines += ["", "### All configuration-format observations", "",
              "The unchanged other-library rows show the spread possible within this",
              "campaign without changing their implementations or dependencies. Small",
              "differences and overlapping ranges do not establish a stable speed gain.", ""]
    lines += detailed_header()
    for case in legacy.CASES:
        for operation in legacy.OPERATIONS:
            before, after = legacy.pair(comparison, f"BenchmarkConfig/{case}/{operation}")
            lines.append(detailed_row(f"{case} / {legacy.LABELS[operation]}", before, after))
    lines += ["", "### All Go API stages", "",
              "These 28 operations retain their original stage boundaries. Prefix selection",
              "starts from an already built configuration. Build and selection produce a",
              "richer result than the common-map loads and should be assessed within the",
              "same operation rather than ranked against other-format load times.", ""]
    lines += detailed_header()
    for _, names in legacy.API_GROUPS:
        for name in names:
            before, after = legacy.pair(api, name)
            lines.append(detailed_row("`" + name.removeprefix("Benchmark") + "`", before, after))
    lines += ["", "### Focused assembly and ordinary-load observations", "",
              "The twelve block and boundary cases time `ParseEach` followed by",
              "`DecodeString` for each value. The two `CollectDecode` cases time `Parse`",
              "then decoding; the two `ParseBuild` cases time `Parse` then `Build`.",
              "Fixture creation, independent full-entry and decoded-value checks, and",
              "Build's occurrence-order checks remain outside the timers. The same new",
              "measurement module is overlaid on both production snapshots and recorded",
              "in their source manifests; it is common instrumentation, not a baseline",
              "production change.", ""]
    lines += detailed_header()
    for case in ASSEMBLY_CASES:
        before, after = legacy.pair(assembly, "BenchmarkAssembly/" + case)
        lines.append(detailed_row("`" + case + "`", before, after))
    lines += ["", "These measurements report time and allocation traffic, not retained heap,",
              "process peak memory or electrical energy. The protocol and limits in this",
              "document apply to all tables above."]
    return "\n".join(lines) + "\n"


def replace_results(source, generated):
    require(source.count(BEGIN) == 1 and source.count(END) == 1,
            "README must contain exactly one generated-results marker pair")
    start, end = source.index(BEGIN) + len(BEGIN), source.index(END)
    require(start < end, "Generated-results markers are out of order")
    return source[:start] + "\n\n" + generated + "\n" + source[end:]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, default=HERE)
    parser.add_argument("--check", action="store_true", help="Validate retained data and existing tables without editing")
    parser.add_argument("--check-current-sources", action="store_true",
                        help="Additionally compare current production bytes with the measured candidate")
    args = parser.parse_args()
    try:
        reports, metrics = {}, {}
        expected = {"comparison": legacy.COMPARISON_NAMES, "go-api": legacy.API_NAMES, "assembly": ASSEMBLY_NAMES}
        for label, names in expected.items():
            report, computed = legacy.read_report(args.directory / (label + ".json"), names)
            validate_comparisons(label, report, computed, names)
            validate_raw_text(args.directory / (label + ".txt"), report)
            reports[label], metrics[label] = report, computed
        validate_sources(reports)
        require(sum(len(report["results"]) for report in reports.values()) == OBSERVATIONS,
                "The three campaigns do not retain all 1,600 observations")
        if args.check_current_sources:
            check_current_sources(reports)
        readme = args.directory / "README.md"
        with readme.open(encoding="utf-8", newline="") as source:
            original = source.read()
        updated = replace_results(original, render(metrics))
        if args.check:
            require(updated == original, "README tables are stale; run render.py after validating the retained reports")
            print("Verified all 1,600 JSON/text observations, recorded summaries, ratios, source manifests and result tables.")
        else:
            with readme.open("w", encoding="utf-8", newline="") as output:
                output.write(updated)
            print("Updated result tables from all 1,600 verified JSON/text observations.")
    except (ReportError, OSError, ValueError, KeyError, TypeError, ZeroDivisionError) as error:
        print("Cannot render multiline and allocation results: " + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
