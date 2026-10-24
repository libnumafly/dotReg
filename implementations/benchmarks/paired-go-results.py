"""Validate retained paired Go observations and share result-formatting helpers.

These helpers have no report-directory, README, rendering-marker or generation
entry-point dependency. Callers supply report paths and expected operations.
They validate the retained observations against their recorded protocol and
summaries; they do not launch processes or compare historic sources with a
current checkout.
"""

from collections import defaultdict
import json
import math
import statistics


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


def validate_shared_sources(comparison, api):
    for variant in VARIANTS:
        comparison_sources = comparison["variants"][variant]["source_sha256"]
        api_sources = api["variants"][variant]["source_sha256"]
        for path in comparison_sources.keys() & api_sources.keys():
            require(comparison_sources[path] == api_sources[path],
                    f"The two campaigns measured different {variant} source bytes: {path}")
    require(comparison.get("controller_source_sha256") == api.get("controller_source_sha256"),
            "The two campaigns used different controller sources")
