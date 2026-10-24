"""Verify the published native campaign and render its README result block.

Copy the completed campaign to report.json beside this script, then run it.
--check leaves files untouched and checks the recorded observations, source
manifests and generated block. The completed report does not require today's
parser sources to match its measured snapshot. --check-current-sources adds
that explicit check. No parser, benchmark, compiler or Git process is launched.
"""

import argparse
from collections import defaultdict
import hashlib
import json
import math
from pathlib import Path
import statistics
import sys


HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent.parent
BEGIN = "<!-- results:start -->"
END = "<!-- results:end -->"
VARIANTS = ("baseline", "candidate")
LANGUAGES = ("go", "python", "javascript", "java", "csharp", "rust", "cpp")
LABELS = {"go": "Go", "python": "Python", "javascript": "JavaScript / TypeScript",
          "java": "Java", "csharp": "C#", "rust": "Rust", "cpp": "C++"}
CASES = ("mixed-32", "mixed-512", "mixed-20000", "line-1mib", "blocks-32kib", "depth-256")
MODES = ("stream", "collect")
SAMPLES = 5
CONDITIONS = len(VARIANTS) * len(LANGUAGES) * len(CASES) * len(MODES)
RESULTS = CONDITIONS * SAMPLES
VALIDATIONS = len(VARIANTS) * len(LANGUAGES) * len(CASES)
PRODUCTION = (
    "implementations/python/dotreg.py", "implementations/javascript/dotreg.mjs",
    "implementations/java/DotReg.java", "implementations/csharp/Parser.cs",
    "implementations/rust/src/lib.rs", "implementations/cpp/dotreg.hpp",
)


class ReportError(ValueError):
    pass


def require(condition, message):
    if not condition:
        raise ReportError(message)


def number(value, positive=False):
    return (isinstance(value, (int, float)) and not isinstance(value, bool)
            and math.isfinite(value) and (value > 0 if positive else value >= 0))


def equal_number(actual, expected, label):
    if expected is None:
        require(actual is None, f"{label}: expected null")
    else:
        require(number(actual) and math.isclose(actual, expected, rel_tol=1e-12, abs_tol=1e-12),
                f"{label}: differs from retained observations")


def distribution(values):
    if not values:
        return {"samples": 0, "median": None, "min": None, "max": None, "mad": None}
    median = statistics.median(values)
    return {"samples": len(values), "median": median, "min": min(values), "max": max(values),
            "mad": statistics.median(abs(value - median) for value in values)}


def check_distribution(actual, expected, label):
    require(isinstance(actual, dict) and set(actual) == set(expected), f"{label}: invalid distribution fields")
    require(type(actual["samples"]) is int and actual["samples"] == expected["samples"],
            f"{label}: sample count differs")
    for key in ("median", "min", "max", "mad"):
        equal_number(actual[key], expected[key], f"{label} {key}")


def read_report(path):
    require(path.is_file(), "Missing report.json; copy the completed paired native campaign before rendering.")
    report = json.loads(path.read_text(encoding="utf-8"))
    require(report.get("status") == "complete", "The native campaign is not complete")
    require(report.get("samples_per_condition") == SAMPLES, "Exactly five observations per condition are required")
    require(report.get("expected_result_count") == RESULTS, "Unexpected recorded result count")
    require(report.get("expected_validation_count") == VALIDATIONS, "Unexpected recorded validation count")
    require(set(report.get("variants", {})) == set(VARIANTS), "Both measured snapshots are required")
    require(report.get("warmup_passes_per_process") == 2, "Unexpected warm-up protocol")
    require(report.get("runtime_tuning_environment") == {}, "Unexpected runtime tuning environment")
    require(report.get("end_hash_verification"), "Campaign end hash verification is missing")
    require(report["variants"]["baseline"].get("tool_versions") == report["variants"]["candidate"].get("tool_versions"),
            "The two snapshots use different tool versions")
    for variant in VARIANTS:
        engines = report["variants"][variant].get("engines", [])
        require(len(engines) == len(LANGUAGES)
                and {engine.get("language") for engine in engines} == set(LANGUAGES),
                f"{variant}: incomplete engine metadata")
        require(report["variants"][variant].get("verified_binary_sha256") ==
                {engine["language"]: engine["binary_sha256"] for engine in engines},
                f"{variant}: prepared and verified binary manifests differ")
    baseline_engines = {engine["language"]: engine for engine in report["variants"]["baseline"]["engines"]}
    for engine in report["variants"]["candidate"]["engines"]:
        before = baseline_engines[engine["language"]]
        require(all(before[key] == engine[key] for key in ("runtime", "build")),
                f"{engine['language']}: runtime or build description differs between snapshots")
    source_key = "docs/config-format.md"
    require(report["variants"]["baseline"]["source_sha256"][source_key] ==
            report["variants"]["candidate"]["source_sha256"][source_key], "The specification differs between snapshots")

    fixtures = report.get("fixtures", [])
    require(len(fixtures) == len(CASES) and {case.get("name") for case in fixtures} == set(CASES),
            "Unexpected fixture matrix")
    cases = {case["name"]: case for case in fixtures}
    validations = report.get("validation", [])
    require(len(validations) == VALIDATIONS, "Expected 84 independent full-entry validations")
    seen_validations = set()
    for row in validations:
        key = (row.get("variant"), row.get("language"), row.get("fixture"))
        require(key[0] in VARIANTS and key[1] in LANGUAGES and key[2] in CASES,
                "Unknown independent validation condition")
        require(key not in seen_validations, "Duplicate independent validation condition")
        seen_validations.add(key)
        case = cases[key[2]]
        require(row.get("entries") == case["entries"] and
                row.get("expected_entries_sha256") == case["expected_entries_sha256"],
                "Independent validation result differs from the fixture oracle")

    rows = report.get("results", [])
    require(len(rows) == RESULTS, "Expected all 840 retained observations")
    require({row.get("execution_sequence") for row in rows} == set(range(1, RESULTS + 1)),
            "Missing or duplicate execution sequence")
    grouped = defaultdict(list)
    seen_rows = set()
    for row in rows:
        key = (row.get("variant"), row.get("language"), row.get("fixture"), row.get("mode"))
        sample = row.get("sample")
        require(key[0] in VARIANTS and key[1] in LANGUAGES and key[2] in CASES and key[3] in MODES,
                "Unknown measured condition")
        require(type(sample) is int and sample in range(1, SAMPLES + 1), "Invalid sample round")
        require((*key, sample) not in seen_rows, "Duplicate measured condition and round")
        seen_rows.add((*key, sample))
        raw = row.get("measurement")
        require(isinstance(raw, dict), "Original wrapper measurement is missing")
        require(all(row.get(field) == value for field, value in raw.items()),
                "Retained row differs from its original wrapper measurement")
        case = cases[key[2]]
        expected = {"language": key[1], "mode": key[3], "input_bytes": case["bytes"],
                    "iterations": case["iterations"], "entries": case["entries"], "fixture_sha256": case["sha256"]}
        require(all(type(raw.get(field)) is type(value) and raw[field] == value for field, value in expected.items()),
                "Measurement input, mode, engine or iteration count differs from its fixture")
        require(number(raw.get("elapsed_seconds"), positive=True)
                and number(raw.get("process_wall_seconds"), positive=True), "Invalid measured time")
        require(type(raw.get("rss_samples")) is int and raw["rss_samples"] > 0, "Invalid RSS polling count")
        rss = raw.get("observed_peak_rss_bytes")
        require(rss is None or (type(rss) is int and rss > 0), "Invalid observed peak RSS")
        if "allocated_bytes" in raw:
            require(number(raw["allocated_bytes"]), "Invalid allocation traffic")
        if "warmup_iterations" in raw:
            require(raw["warmup_iterations"] == 2, "Unexpected native warm-up count")
        expected_ms = raw["elapsed_seconds"] * 1000 / raw["iterations"]
        equal_number(row.get("milliseconds_per_pass"), expected_ms, "Derived milliseconds per pass")
        grouped[key].append(row)
    require(len(grouped) == CONDITIONS, "Expected all 168 measured conditions")

    saved = report.get("summary", [])
    require(len(saved) == CONDITIONS, "Expected all 168 condition summaries")
    summaries = {}
    for item in saved:
        key = (item.get("variant"), item.get("language"), item.get("fixture"), item.get("mode"))
        require(key in grouped and key not in summaries, "Unknown or duplicate summary condition")
        observations = grouped[key]
        require(len(observations) == SAMPLES and {row["sample"] for row in observations} == set(range(1, SAMPLES + 1)),
                "Unbalanced condition observations")
        require(type(item.get("samples")) is int and item["samples"] == SAMPLES, "Invalid condition sample count")
        recomputed = {
            "milliseconds_per_pass": distribution([row["measurement"]["elapsed_seconds"] * 1000 /
                                                    row["measurement"]["iterations"] for row in observations]),
            "observed_peak_rss_bytes": distribution([row["measurement"]["observed_peak_rss_bytes"]
                                                      for row in observations
                                                      if row["measurement"]["observed_peak_rss_bytes"] is not None]),
            "process_wall_seconds": distribution([row["measurement"]["process_wall_seconds"] for row in observations]),
        }
        allocated = [row["measurement"]["allocated_bytes"] / row["measurement"]["iterations"]
                     for row in observations if "allocated_bytes" in row["measurement"]]
        if allocated:
            require(len(allocated) == SAMPLES, "Some allocation counters are missing")
            recomputed["allocated_bytes_per_pass"] = distribution(allocated)
        require(("allocated_bytes_per_pass" in item) == bool(allocated), "Unexpected allocation summary")
        for metric, expected_distribution in recomputed.items():
            check_distribution(item.get(metric), expected_distribution, f"{' / '.join(key)} {metric}")
        summaries[key] = recomputed
    return report, summaries


def check_source_records(report):
    for variant in VARIANTS:
        manifest = report["variants"][variant].get("source_sha256")
        require(isinstance(manifest, dict) and manifest, f"{variant}: missing source manifest")
        engines = report["variants"][variant]["engines"]
        names = set(PRODUCTION) | {"docs/config-format.md"}
        names.update(name for engine in engines for name in engine["sources"])
        require(names <= manifest.keys(), f"{variant}: required measured sources are missing")
        for name, digest in manifest.items():
            relative = Path(name)
            require(not relative.is_absolute() and ".." not in relative.parts and "\\" not in name
                    and bool(relative.parts) and ":" not in name,
                    "Source manifest name must be repository-relative")
            require(isinstance(digest, str) and len(digest) == 64
                    and all(c in "0123456789abcdef" for c in digest),
                    f"{variant}: invalid recorded SHA-256: {name}")
    # Go is an unchanged control in this campaign. Check that fact between the
    # retained snapshots, rather than requiring future checkouts to stay frozen.
    baseline = report["variants"]["baseline"]["source_sha256"]
    candidate = report["variants"]["candidate"]["source_sha256"]
    go_engine = next(engine for engine in report["variants"]["candidate"]["engines"]
                     if engine["language"] == "go")
    for name in go_engine["sources"]:
        require(baseline[name] == candidate[name], f"Recorded Go control differs between snapshots: {name}")


def check_candidate_sources(report):
    candidate = report["variants"]["candidate"]
    manifest = candidate["source_sha256"]
    go_engine = next(engine for engine in candidate["engines"] if engine["language"] == "go")
    names = set(PRODUCTION) | set(go_engine["sources"]) | {"docs/config-format.md"}
    for name in sorted(names):
        require(name in manifest, f"Candidate source hash missing: {name}")
        relative = Path(name)
        require(not relative.is_absolute() and ".." not in relative.parts and "\\" not in name,
                "Source manifest name must be repository-relative")
        source = (ROOT / relative).resolve()
        require(source.is_relative_to(ROOT) and source.is_file(), f"Candidate source missing: {name}")
        require(hashlib.sha256(source.read_bytes()).hexdigest() == manifest[name],
                f"Current source differs from the measured candidate: {name}")


def percent(before, after):
    return f"{100 * (after / before - 1):+.1f}%"


def timing(item, ranges=False):
    value = item["milliseconds_per_pass"]
    if ranges:
        return f"{value['median']:.3f} [{value['min']:.3f}, {value['max']:.3f}]"
    return f"{value['median']:.3f}"


def rss(item):
    value = item["observed_peak_rss_bytes"]["median"]
    return "unavailable" if value is None else f"{value / (1024 * 1024):.2f}"


def pair(summaries, language, case, mode):
    return tuple(summaries[(variant, language, case, mode)] for variant in VARIANTS)


def render(report, summaries):
    lines = [
        "All 840 observations and 84 independent complete-entry validations are retained in [report.json](report.json).",
        "Every condition has five fresh-process observations. Times are milliseconds per pass; ranges are observed minima and maxima.",
        "Time change is `(candidate / baseline - 1) × 100%`; a negative value means a shorter elapsed time.",
        "RSS is the median observed peak process memory in MiB and includes runtime, startup and warm-up activity.",
        "The report also retains the median absolute deviation (MAD), complete RSS ranges and process wall-time distributions.",
        "",
        "### Large mixed document",
        "",
        "The `mixed-20000` fixture contains 20,000 value occurrences, including repeated keys, annotations, Unicode and multiline values.",
        "Go is the unchanged control. JavaScript and TypeScript use the same parser engine.",
        "",
        "| Engine | Mode | Baseline ms/pass | Candidate ms/pass | Time change | Baseline RSS MiB | Candidate RSS MiB |",
        "|---|---|---:|---:|---:|---:|---:|",
    ]
    for language in LANGUAGES:
        for mode in MODES:
            before, after = pair(summaries, language, "mixed-20000", mode)
            change = percent(before["milliseconds_per_pass"]["median"], after["milliseconds_per_pass"]["median"])
            lines.append(f"| {LABELS[language]} | {mode} | {timing(before)} | {timing(after)} | {change} | {rss(before)} | {rss(after)} |")
    lines.extend(["", "### All workloads", "",
                  "The timing columns show median [minimum, maximum]. The RSS column shows baseline → candidate median MiB."])
    fixture_metadata = {case["name"]: case for case in report["fixtures"]}
    for case in CASES:
        metadata = fixture_metadata[case]
        lines.extend(["", f"#### {case}", "", metadata["description"] + ".",
                      f"{metadata['bytes']:,} input bytes; {metadata['entries']:,} entries; {metadata['iterations']:,} timed passes per process.", "",
                      "| Engine | Mode | Baseline ms/pass [min, max] | Candidate ms/pass [min, max] | Time change | RSS MiB, baseline → candidate |",
                      "|---|---|---:|---:|---:|---:|"])
        for language in LANGUAGES:
            for mode in MODES:
                before, after = pair(summaries, language, case, mode)
                change = percent(before["milliseconds_per_pass"]["median"], after["milliseconds_per_pass"]["median"])
                lines.append(f"| {LABELS[language]} | {mode} | {timing(before, True)} | {timing(after, True)} | {change} | {rss(before)} → {rss(after)} |")
    lines.extend(["", "### Allocation traffic", "",
                  "These native harness counters report bytes allocated during timed passes, divided by the pass count. They do not measure retained memory or peak RSS.",
                  "Go uses `runtime.MemStats.TotalAlloc`; C# uses `GC.GetAllocatedBytesForCurrentThread`.",
                  "Values are medians of five observations. Differences in the unchanged Go control help show run-to-run variation.", "",
                  "| Workload | Engine | Mode | Baseline B/pass | Candidate B/pass | Allocation change |",
                  "|---|---|---|---:|---:|---:|"])
    for case in CASES:
        for language in ("go", "csharp"):
            for mode in MODES:
                before, after = pair(summaries, language, case, mode)
                require("allocated_bytes_per_pass" in before and "allocated_bytes_per_pass" in after,
                        f"Missing expected native allocation counter: {language} {case} {mode}")
                baseline = before["allocated_bytes_per_pass"]["median"]
                candidate = after["allocated_bytes_per_pass"]["median"]
                change = percent(baseline, candidate) if baseline else "unavailable"
                lines.append(f"| {case} | {LABELS[language]} | {mode} | {baseline:,.1f} | {candidate:,.1f} | {change} |")
    return "\n".join(lines) + "\n"


def result_block(readme, generated):
    require(readme.count(BEGIN) == 1 and readme.count(END) == 1, "README must contain exactly one pair of result markers")
    start = readme.index(BEGIN) + len(BEGIN)
    finish = readme.index(END)
    require(start < finish, "README result markers are out of order")
    return readme[:start] + "\n\n" + generated + "\n" + readme[finish:]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="Verify the recorded result block without editing")
    parser.add_argument("--check-current-sources", action="store_true",
                        help="Additionally require current production sources to match the measured candidate")
    parser.add_argument("--report", type=Path, default=HERE / "report.json")
    parser.add_argument("--readme", type=Path, default=HERE / "README.md")
    args = parser.parse_args()
    try:
        report, summaries = read_report(args.report)
        check_source_records(report)
        if args.check_current_sources:
            check_candidate_sources(report)
        require(args.readme.is_file(), "README.md is missing")
        with args.readme.open(encoding="utf-8", newline="") as source:
            original = source.read()
        expected = result_block(original, render(report, summaries))
        if args.check:
            require(original == expected, "README result block is stale; run render.py without --check")
            print("Verified 840 observations, 84 independent validations, 168 summaries, recorded source manifests and generated result block.")
        else:
            with args.readme.open("w", encoding="utf-8", newline="") as output:
                output.write(expected)
            print("Updated the README result block from 840 verified observations.")
    except (ReportError, OSError, ValueError, KeyError, TypeError, ZeroDivisionError) as error:
        print(f"Cannot render native results: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
