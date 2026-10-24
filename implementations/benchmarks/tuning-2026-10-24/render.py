"""Render tuning tables from the completed paired reports, without altering data.

Run this script after comparison.json and go-api.json have been copied beside
it. --check validates that README.md already contains the reproducible tables.
"""

import argparse
from collections import defaultdict
import json
import math
from pathlib import Path
import statistics
import sys


BEGIN = "<!-- BEGIN GENERATED TUNING RESULTS -->"
END = "<!-- END GENERATED TUNING RESULTS -->"
VARIANTS = ("baseline", "candidate")
CASES = ("small", "medium", "large", "unicode", "escaped", "multiline")
OPERATIONS = ("dotreg-direct", "dotreg-collected", "json", "toml", "yaml", "ini")
OTHER_OPERATIONS = ("json", "toml", "yaml", "ini")
LABELS = {"dotreg-direct": "dotReg direct", "dotreg-collected": "dotReg collected",
          "json": "JSON", "toml": "TOML", "yaml": "YAML", "ini": "INI"}
CASE_LABELS = {"small": "Small ASCII, 32 settings", "medium": "Medium ASCII, 512 settings",
               "large": "Large ASCII, 8,192 settings", "unicode": "Unicode, 512 settings",
               "escaped": "Escaped, 512 settings", "multiline": "Multiline, 512 settings"}
PARSE_EACH_CASES = ("BareMixed512", "QuotedMixed512", "LongRaw1MiB", "TriplePlain32KiB",
                    "TripleEscaped32KiB", "PipeLiteral32KiB")
PREFIX_CASES = ("All", "EmptyPrefix", "BaseAndDescendants", "SelectiveDescendant",
                "NoMatch", "QuotedElementBoundary", "EmptyName")
COMPARISON_NAMES = {f"BenchmarkConfig/{case}/{operation}" for case in CASES for operation in OPERATIONS}
API_GROUPS = (
    ("Parser and decoder stages", tuple(f"BenchmarkParseEach/{case}" for case in PARSE_EACH_CASES)
     + ("BenchmarkParseDecode512", "BenchmarkDecodeTriple32KiB/Plain", "BenchmarkDecodeTriple32KiB/Escaped")),
    ("Configuration construction", ("BenchmarkParseBuild512", "BenchmarkBuildUntyped512",
                                    "BenchmarkParseBuildOrdered/Occurrences512", "BenchmarkParseBuildOrdered/Occurrences8192")),
    ("Prefix selection: 512 stored occurrences", tuple(f"BenchmarkSelectPrefix/Occurrences512/{case}" for case in PREFIX_CASES)),
    ("Prefix selection: 8,192 stored occurrences", tuple(f"BenchmarkSelectPrefix/Occurrences8192/{case}" for case in PREFIX_CASES)),
    ("Prefix selection: empty configuration", ("BenchmarkSelectPrefix/EmptyConfig",)),
)
API_NAMES = {name for _, names in API_GROUPS for name in names}


class ReportError(ValueError):
    pass


def require(condition, message):
    if not condition:
        raise ReportError(message)


def read_report(path, expected_names):
    require(path.is_file(), f"Missing {path.name}; complete and copy the paired campaign reports before rendering.")
    report = json.loads(path.read_text(encoding="utf-8"))
    label = path.name
    require(report.get("status") == "complete", f"{label}: campaign status is not complete")
    require(report.get("samples_per_benchmark_per_variant") == 10, f"{label}: exactly ten rounds per variant are required")
    require(report.get("isolate_each_operation") is True, f"{label}: operations were not isolated")
    require(set(report.get("variants", {})) == set(VARIANTS), f"{label}: baseline/candidate records are missing")
    flags = set(report.get("benchmark_flags", []))
    require({"-test.cpu=1", "-test.count=1", "-test.benchmem", "-test.benchtime=200ms"} <= flags,
            f"{label}: benchmark timing or process flags differ from the reported protocol")
    environment = report.get("runtime_environment", {})
    require(all(environment.get(k) == v for k, v in {"GOMAXPROCS": "1", "GOGC": "100", "GOMEMLIMIT": "off"}.items()),
            f"{label}: runtime settings differ from the reported protocol")
    for variant in VARIANTS:
        discovered = report.get("discovery", {}).get(variant, {})
        require(set(discovered.get("operations", [])) == expected_names,
                f"{label}: discovery operation names differ for {variant}")
        require(discovered.get("operation_count") == len(expected_names), f"{label}: invalid discovery count")
    rows = report.get("results", [])
    expected_count = len(expected_names) * len(VARIANTS) * 10
    require(len(rows) == expected_count, f"{label}: expected {expected_count} retained observations, found {len(rows)}")
    require({row.get("execution_sequence") for row in rows} == set(range(1, expected_count + 1)),
            f"{label}: missing or duplicate execution sequences")
    grouped = defaultdict(list)
    seen = set()
    base_seed = report.get("base_order_seed")
    require(isinstance(base_seed, int), f"{label}: missing shuffle seed")
    for row in rows:
        variant, name, sample = row.get("variant"), row.get("benchmark"), row.get("sample")
        require(variant in VARIANTS and name in expected_names and sample in range(1, 11),
                f"{label}: unexpected variant, operation or round")
        key = (variant, name, sample)
        require(key not in seen, f"{label}: duplicate variant/operation/round")
        seen.add(key)
        require(row.get("order_seed") == base_seed + sample - 1, f"{label}: inconsistent per-round shuffle seed")
        metrics = row.get("metrics", {})
        require({"ns/op", "B/op", "allocs/op"} <= set(metrics), f"{label}: missing time/allocation metric")
        require(all(isinstance(v, (int, float)) and math.isfinite(v) for v in metrics.values()),
                f"{label}: invalid metric value")
        require(metrics["ns/op"] > 0 and metrics["B/op"] >= 0 and metrics["allocs/op"] >= 0,
                f"{label}: invalid time or allocation value")
        grouped[(variant, name)].append(row)
    computed = {}
    for variant in VARIANTS:
        for name in expected_names:
            samples = grouped[(variant, name)]
            require(len(samples) == 10 and {s["sample"] for s in samples} == set(range(1, 11)),
                    f"{label}: incomplete samples for {variant}/{name}")
            metric_names = set(samples[0]["metrics"])
            require(all(set(s["metrics"]) == metric_names for s in samples), f"{label}: changing metric set")
            computed[(variant, name)] = {
                metric: {"median": statistics.median(values), "min": min(values), "max": max(values)}
                for metric in metric_names
                for values in [[s["metrics"][metric] for s in samples]]
            }
    recorded = report.get("summary", [])
    require(len(recorded) == len(expected_names) * len(VARIANTS), f"{label}: incomplete summary records")
    summary_keys = set()
    for summary in recorded:
        key = (summary.get("variant"), summary.get("benchmark"))
        require(key in computed and key not in summary_keys, f"{label}: unexpected or duplicate summary")
        summary_keys.add(key)
        require(summary.get("samples") == 10, f"{label}: invalid summary sample count")
        require(set(summary.get("metrics", {})) == set(computed[key]), f"{label}: stale summary metric set")
        for metric, values in computed[key].items():
            for statistic, expected in values.items():
                actual = summary["metrics"][metric].get(statistic)
                require(isinstance(actual, (int, float)) and math.isclose(actual, expected, rel_tol=1e-12, abs_tol=1e-12),
                        f"{label}: recorded summary differs from retained observations")
    for name in expected_names:
        require(set(computed[("baseline", name)]) == set(computed[("candidate", name)]),
                f"{label}: variants report different metrics for {name}")
    hashes = []
    for variant in VARIANTS:
        source_hashes = report["variants"][variant].get("source_sha256", {})
        specification = source_hashes.get("docs/config-format.md")
        require(isinstance(specification, str) and len(specification) == 64, f"{label}: missing specification hash")
        hashes.append(specification)
    require(len(set(hashes)) == 1, f"{label}: baseline and candidate specifications differ")
    return report, computed


def number(value, decimals=6):
    # Preserve half-unit medians and true zero allocations. Six decimal places
    # in microseconds retain sub-nanosecond resolution for tiny API operations.
    rendered = f"{value:,.{decimals}f}"
    return rendered.rstrip("0").rstrip(".") if "." in rendered else rendered


def percent(value):
    return f"{value:+.3f}%"


def delta(before, after):
    return (after / before - 1) * 100


def timing(metrics):
    values = metrics["ns/op"]
    return f"{number(values['median'] / 1000)} [{number(values['min'] / 1000)}, {number(values['max'] / 1000)}]"


def transition(before, after, metric):
    return f"{number(before[metric]['median'])} → {number(after[metric]['median'])}"


def pair(metrics, name):
    return metrics[("baseline", name)], metrics[("candidate", name)]


def detailed_header():
    return ["| Operation | Baseline µs/op, median [min, max] | Tuned µs/op, median [min, max] | B/op, baseline → tuned | allocs/op, baseline → tuned |",
            "|---|---:|---:|---:|---:|"]


def detailed_row(label, before, after):
    return f"| {label} | {timing(before)} | {timing(after)} | {transition(before, after, 'B/op')} | {transition(before, after, 'allocs/op')} |"


def generate(comparison, comparison_metrics, api, api_metrics):
    lines = ["## Results", "", "These tables are generated by [render.py](render.py) directly from the",
             "retained observations in the two complete paired reports. The format campaign",
             "contains **720 observations** and the Go API campaign **560 observations**:",
             "ten samples for each operation in each variant. No samples are removed.", "",
             "Times are microseconds per complete operation. Bracketed values are the",
             "observed minimum and maximum, not confidence intervals. Allocation figures",
             "are median bytes and allocation counts per operation. Fractional medians and",
             "zero-allocation results are retained. Time cells use up to six decimal places",
             "in microseconds so tiny operations such as `EmptyConfig` do not round to zero.", "",
             "### Direct loading against the fastest other library", "",
             "The other-library column selects the lowest candidate median among JSON,",
             "TOML, YAML and INI in this same campaign. The ratio is the tuned dotReg direct",
             "median divided by that median: above one is slower and below one is faster.",
             "This is a comparison of the selected implementations and serialisations.", "",
             "| Case | dotReg direct baseline µs/load | Tuned µs/load | Time change | Fastest other candidate median | Tuned / other | Direct B/op, baseline → tuned |",
             "|---|---:|---:|---:|---:|---:|---:|"]
    for case in CASES:
        before, after = pair(comparison_metrics, f"BenchmarkConfig/{case}/dotreg-direct")
        best = min(OTHER_OPERATIONS, key=lambda op: comparison_metrics[("candidate", f"BenchmarkConfig/{case}/{op}")]["ns/op"]["median"])
        best_time = comparison_metrics[("candidate", f"BenchmarkConfig/{case}/{best}")]["ns/op"]["median"]
        before_time, after_time = before["ns/op"]["median"], after["ns/op"]["median"]
        lines.append(f"| {CASE_LABELS[case]} | {number(before_time / 1000)} | {number(after_time / 1000)} | {percent(delta(before_time, after_time))} | {LABELS[best]} {number(best_time / 1000)} µs | {number(after_time / best_time)}× | {transition(before, after, 'B/op')} |")
    lines += ["", "### Medium ASCII: all operations and allocations", "",
              "This is the same 512-setting workload for all six operations. The full",
              "ranges remain visible alongside the medians; a small ordering difference",
              "with overlapping observations should not be treated as a stable ranking.", ""] + detailed_header()
    for operation in OPERATIONS:
        before, after = pair(comparison_metrics, f"BenchmarkConfig/medium/{operation}")
        lines.append(detailed_row(LABELS[operation], before, after))
    lines += ["", "### Variation in the unchanged controls", "",
              "JSON, TOML, YAML and INI use unchanged code and dependencies in the two",
              "variants. Their differences below are observations of campaign variation,",
              "not improvements caused by dotReg tuning. Each range summarises the six",
              "case-level changes between candidate and baseline medians. The timing",
              "ranges for every individual case follow in the detailed tables.", "",
              "| Unchanged operation | Time-median change across six cases, min to max | B/op-median change across six cases, min to max | allocs/op-median change across six cases, min to max |",
              "|---|---:|---:|---:|"]
    for operation in OTHER_OPERATIONS:
        cells = []
        for metric in ("ns/op", "B/op", "allocs/op"):
            changes = []
            for case in CASES:
                before, after = pair(comparison_metrics, f"BenchmarkConfig/{case}/{operation}")
                changes.append(delta(before[metric]["median"], after[metric]["median"]))
            cells.append(f"{percent(min(changes))} to {percent(max(changes))}")
        lines.append(f"| {LABELS[operation]} | " + " | ".join(cells) + " |")
    lines += ["", "### All configuration-format observations", "",
              "The following 36 operation rows retain baseline and candidate ranges and",
              "allocation medians for every format-comparison case."]
    for case in CASES:
        lines += ["", "#### " + CASE_LABELS[case], ""] + detailed_header()
        for operation in OPERATIONS:
            before, after = pair(comparison_metrics, f"BenchmarkConfig/{case}/{operation}")
            lines.append(detailed_row(LABELS[operation], before, after))
    lines += ["", "### All Go API observations", "",
              "These 28 operations measure different stages of the Go API. Selection starts",
              "from an already built configuration and does not parse source bytes.",
              "`Config` construction and selection retain a richer output contract than",
              "the two-level string map, so these times should be assessed within the",
              "same API operation rather than against the other-format load times."]
    for title, names in API_GROUPS:
        lines += ["", "#### " + title, ""] + detailed_header()
        for name in names:
            before, after = pair(api_metrics, name)
            lines.append(detailed_row("`" + name.removeprefix("Benchmark") + "`", before, after))
    lines += ["", "The report's [measurement protocol](#paired-measurement-protocol) and",
              "[interpretation notes](#reading-the-results) apply to all tables above."]
    return "\n".join(lines)


def validate_shared_sources(comparison, api):
    for variant in VARIANTS:
        comparison_sources = comparison["variants"][variant]["source_sha256"]
        api_sources = api["variants"][variant]["source_sha256"]
        for path in comparison_sources.keys() & api_sources.keys():
            require(comparison_sources[path] == api_sources[path],
                    f"The two campaigns measured different {variant} source bytes: {path}")
    require(comparison.get("controller_source_sha256") == api.get("controller_source_sha256"),
            "The two campaigns used different controller sources")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, default=Path(__file__).resolve().parent)
    parser.add_argument("--check", action="store_true", help="Fail if README tables differ; never write files")
    args = parser.parse_args()
    try:
        comparison, comparison_metrics = read_report(args.directory / "comparison.json", COMPARISON_NAMES)
        api, api_metrics = read_report(args.directory / "go-api.json", API_NAMES)
        validate_shared_sources(comparison, api)
        readme = args.directory / "README.md"
        source = readme.read_text(encoding="utf-8")
        require(source.count(BEGIN) == 1 and source.count(END) == 1, "README must contain exactly one generated-results marker pair")
        start, end = source.index(BEGIN) + len(BEGIN), source.index(END)
        require(start < end, "README generated-results markers are out of order")
        replacement = "\n\n" + generate(comparison, comparison_metrics, api, api_metrics) + "\n\n"
        updated = source[:start] + replacement + source[end:]
        if args.check:
            require(updated == source, "README tables are stale; run render.py after validating the completed reports")
            print("Tuning README tables match all 1,280 retained paired observations.")
        else:
            readme.write_text(updated, encoding="utf-8", newline="\n")
            print("Updated tuning README tables from all 1,280 retained paired observations.")
    except (ReportError, json.JSONDecodeError, OSError) as error:
        print("Cannot render tuning results: " + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
