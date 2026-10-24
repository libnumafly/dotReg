"""Measure two Go implementations in balanced, serial, isolated rounds.

Build both test binaries before running this controller. Every operation in
both variants runs once per round, in a jointly shuffled order, in a fresh
process. Published source and binary records contain no absolute paths.
"""

import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import platform
import random
import re
import statistics
import subprocess
import sys
import time


VARIANTS = ("baseline", "candidate")
IGNORED_PARTS = {"bin", "obj", "target", "__pycache__", ".artifacts"}
CONTROLLER = Path(__file__).resolve()
PARSER_SOURCE = CONTROLLER.with_name("run-go.py")


def timestamp():
    return datetime.now(timezone.utc).isoformat()


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_row_parser():
    # The original measured controller remains unchanged. Share its numeric
    # metric parser rather than silently introducing a different interpretation.
    spec = importlib.util.spec_from_file_location("dotreg_recorded_go_driver", PARSER_SOURCE)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.parse_rows


def source_manifest(root, module):
    go_root = root / "implementations/go"
    paths = {root / "docs/config-format.md", root / "implementations/benchmarks/run-go.py",
             module / "go.mod", go_root / "go.mod"}
    paths.update(module.rglob("*.go"))
    paths.update(p for p in go_root.rglob("*.go") if "cmd" not in p.relative_to(go_root).parts)
    for directory in {module, go_root}:
        if (directory / "go.sum").exists():
            paths.add(directory / "go.sum")
    paths = [p for p in paths if not IGNORED_PARTS.intersection(p.relative_to(root).parts)]
    return {p.relative_to(root).as_posix(): sha256(p) for p in sorted(paths)}


def git_head(root):
    # An exported baseline may have no Git metadata; its supplied commit and
    # exact source hashes still identify it. Do not inherit an enclosing repo.
    try:
        top = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], cwd=root,
                                      text=True, stderr=subprocess.DEVNULL).strip()
        if Path(top).resolve() == root:
            return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root,
                                           text=True, stderr=subprocess.DEVNULL).strip()
    except subprocess.CalledProcessError:
        pass
    return None


def go_environment(module):
    result = json.loads(subprocess.check_output(
        ["go", "env", "-json", "GOVERSION", "GOOS", "GOARCH", "GOEXPERIMENT", "GOFLAGS", "CGO_ENABLED"],
        cwd=module, text=True))
    # GOFLAGS may contain a user-specific -modfile or overlay path. Its value
    # affects builds, not execution of these already-built test binaries.
    flags = result.get("GOFLAGS", "")
    if re.search(r"(?:[A-Za-z]:[/\\]|(?:^|[=\s])/[^\s])", flags):
        result["GOFLAGS"] = "[absolute path omitted]"
        result["GOFLAGS_sha256"] = hashlib.sha256(flags.encode()).hexdigest()
    return result


def selected_flags(flags, operation):
    # The Go runner splits selections at slashes, so anchor every segment.
    pattern = "/".join("^" + re.escape(part) + "$" for part in operation.split("/"))
    return [f for f in flags if not f.startswith("-test.bench=")] + ["-test.bench=" + pattern]


def write_report(path, report):
    # A partial report is useful after interruption, but never masquerades as a
    # completed run. Write between child processes and replace atomically.
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_text(json.dumps(report, indent=2, allow_nan=False) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def validate_rows(parse_rows, output):
    rows = parse_rows(output)
    for row in rows:
        if row["iterations"] <= 0:
            raise ValueError("benchmark iteration count must be positive")
        if any(not math.isfinite(value) for value in row["metrics"].values()):
            raise ValueError("non-finite benchmark metric")
    return rows


def summarise(report, operations, samples):
    for variant in VARIANTS:
        for name in sorted(operations):
            rows = [r for r in report["results"] if r["variant"] == variant and r["benchmark"] == name]
            if len(rows) != samples or {r["sample"] for r in rows} != set(range(1, samples + 1)):
                raise ValueError("missing or duplicate samples for " + variant + "/" + name)
            metric_names = set(rows[0]["metrics"])
            if any(set(row["metrics"]) != metric_names for row in rows):
                raise ValueError("benchmark metric set changed between samples")
            summary = {"variant": variant, "benchmark": name, "samples": samples, "metrics": {}}
            for metric in sorted(metric_names):
                values = [row["metrics"][metric] for row in rows]
                summary["metrics"][metric] = {"median": statistics.median(values),
                                               "min": min(values), "max": max(values)}
            if "values/op" in metric_names:
                speeds = [row["metrics"]["values/op"] * 1e9 / row["metrics"]["ns/op"] for row in rows]
                summary["values_per_second"] = {"median": statistics.median(speeds),
                                                  "min": min(speeds), "max": max(speeds)}
            report["summary"].append(summary)
    for name in sorted(operations):
        pair = {s["variant"]: s for s in report["summary"] if s["benchmark"] == name}
        if set(pair["baseline"]["metrics"]) != set(pair["candidate"]["metrics"]):
            raise ValueError("baseline and candidate benchmark metric sets differ")
        ratios = {}
        for metric in sorted(set(pair["baseline"]["metrics"]) & set(pair["candidate"]["metrics"])):
            before = pair["baseline"]["metrics"][metric]["median"]
            after = pair["candidate"]["metrics"][metric]["median"]
            ratios[metric] = {"candidate_over_baseline_medians": after / before if before else None,
                              "change_percent": (after / before - 1) * 100 if before else None}
        report["comparison"].append({"benchmark": name, "metrics": ratios})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for variant in VARIANTS:
        parser.add_argument("--" + variant + "-root", required=True)
        parser.add_argument("--" + variant + "-binary", required=True)
    parser.add_argument("--baseline-commit", required=True)
    parser.add_argument("--module", required=True, help="Repository-relative module directory")
    parser.add_argument("--bench", default="^BenchmarkConfig$")
    parser.add_argument("--samples", type=int, default=10)
    parser.add_argument("--benchtime", default="200ms")
    parser.add_argument("--seed", type=int, default=20261024)
    parser.add_argument("--output", required=True)
    parser.add_argument("--raw", required=True)
    parser.add_argument("--manifest")
    args = parser.parse_args()
    if args.samples < 3:
        parser.error("at least three balanced process rounds required")
    module_relative = Path(args.module)
    if module_relative.is_absolute() or ".." in module_relative.parts:
        parser.error("--module must be a repository-relative path without parent traversal")
    output, raw_path = Path(args.output).resolve(), Path(args.raw).resolve()
    if output == raw_path:
        parser.error("--output and --raw must be distinct files")
    for destination in (output, raw_path):
        destination.parent.mkdir(parents=True, exist_ok=True)
    roots = {v: Path(getattr(args, v + "_root")).resolve() for v in VARIANTS}
    binaries = {v: Path(getattr(args, v + "_binary")).resolve() for v in VARIANTS}
    modules = {v: roots[v] / module_relative for v in VARIANTS}
    if any(output == p or raw_path == p for p in binaries.values()):
        parser.error("report destinations must not overwrite a benchmark binary")
    parse_rows = load_row_parser()
    controller_hashes = {"implementations/benchmarks/run-paired-go.py": sha256(CONTROLLER),
                         "implementations/benchmarks/run-go.py": sha256(PARSER_SOURCE)}
    source_records = {}
    for variant in VARIANTS:
        head = git_head(roots[variant])
        if variant == "baseline" and head and not head.startswith(args.baseline_commit):
            parser.error("baseline root HEAD does not match --baseline-commit")
        source_records[variant] = {
            "source_commit": args.baseline_commit if variant == "baseline" else head,
            "observed_git_head": head,
            "source_state": ("The supplied commit identifies the baseline production reference; exact source hashes identify the measured tree and any shared test or benchmark instrumentation." if variant == "baseline"
                             else "Source hashes identify the candidate working tree, including uncommitted tuning changes."),
            "source_sha256": source_manifest(roots[variant], modules[variant]),
            "binary_sha256": sha256(binaries[variant]),
            "go_environment": go_environment(modules[variant]),
        }
    if source_records["baseline"]["source_sha256"]["docs/config-format.md"] != source_records["candidate"]["source_sha256"]["docs/config-format.md"]:
        raise ValueError("baseline and candidate specifications differ")
    flags = ["-test.run=^$", "-test.bench=" + args.bench, "-test.benchmem",
             "-test.benchtime=" + args.benchtime, "-test.cpu=1", "-test.count=1", "-test.timeout=5m"]
    environment = os.environ.copy()
    environment.update({"GOMAXPROCS": "1", "GOGC": "100", "GOMEMLIMIT": "off"})
    report = {
        "version": 1, "status": "running", "started_at_utc": timestamp(),
        "module": module_relative.as_posix(), "variants": source_records,
        "controller_source_sha256": controller_hashes,
        "system": {"platform": platform.platform(), "machine": platform.machine(),
                   "processor": platform.processor(), "logical_cpus": os.cpu_count(),
                   "byteorder": sys.byteorder,
                   "python_implementation": platform.python_implementation(),
                   "python_version": platform.python_version(),
                   "perf_counter_resolution_seconds": time.get_clock_info("perf_counter").resolution},
        "samples_per_benchmark_per_variant": args.samples, "base_order_seed": args.seed,
        "benchmark_flags": flags,
        "runtime_environment": {"GOMAXPROCS": "1", "GOGC": "100", "GOMEMLIMIT": "off",
                                "inherited_GODEBUG": os.environ.get("GODEBUG", "")},
        "isolate_each_operation": True,
        "operation_order": "Shuffle all (variant, operation) pairs jointly with the recorded per-round seed; each pair appears exactly once per round.",
        "method": "Serial fresh test-binary processes. One exact operation per process, testing.B calibration with a fixed timing target, CPU=1, GOMAXPROCS=1, GOGC=100, GOMEMLIMIT=off. One 1x discovery pass per variant precedes measured rounds. Input generation and independent oracle checks remain outside the Go timers. No forced GC inside timed loops; the Go test runner controls calibration and GC. All observations retained with no outlier removal. B/op and allocs/op measure allocation traffic, not peak or retained memory. Source and binary hashes must remain unchanged throughout the campaign. Ratios compare medians and do not express statistical significance.",
        "discovery": {}, "results": [], "summary": [], "comparison": [],
    }
    if args.manifest:
        manifest = Path(args.manifest)
        report["fixture_manifest"] = json.loads(manifest.read_text(encoding="utf-8"))
        report["fixture_manifest_sha256"] = sha256(manifest)
    expected_names = None
    with raw_path.open("w", encoding="utf-8", newline="\n") as raw:
        raw.write("# Balanced baseline/candidate Go campaign; exact flags and hashes are in the JSON report.\n")
        discovery_flags = [f for f in flags if not f.startswith("-test.benchtime=")] + ["-test.benchtime=1x"]
        environment["DOTREG_BENCH_SEED"] = str(args.seed)
        for variant in VARIANTS:
            started, wall = timestamp(), time.perf_counter()
            child = subprocess.run([str(binaries[variant])] + discovery_flags, cwd=modules[variant],
                                   capture_output=True, text=True, encoding="utf-8", env=environment, timeout=330)
            elapsed = time.perf_counter() - wall
            raw.write(f"\n# discovery variant={variant} started_at_utc={started}\n" + child.stdout)
            if child.stderr:
                raw.write("# stderr\n" + child.stderr)
            raw.flush()
            if child.returncode:
                raise RuntimeError("discovery failed for " + variant + "; see raw output")
            rows = validate_rows(parse_rows, child.stdout)
            names = {row["benchmark"] for row in rows}
            if expected_names is not None and names != expected_names:
                raise ValueError("baseline and candidate discovered operation sets differ")
            expected_names = names
            report["discovery"][variant] = {"flags": discovery_flags, "operations": sorted(names),
                                              "operation_count": len(names), "output": child.stdout,
                                              "started_at_utc": started, "finished_at_utc": timestamp(),
                                              "process_wall_seconds": elapsed}
        write_report(output, report)
        sequence = 0
        for sample in range(1, args.samples + 1):
            seed = args.seed + sample - 1
            environment["DOTREG_BENCH_SEED"] = str(seed)
            pairs = [(v, name) for v in VARIANTS for name in sorted(expected_names)]
            random.Random(seed).shuffle(pairs)
            observed = set()
            for variant, operation in pairs:
                sequence += 1
                operation_flags = selected_flags(flags, operation)
                started, wall = timestamp(), time.perf_counter()
                child = subprocess.run([str(binaries[variant])] + operation_flags, cwd=modules[variant],
                                       capture_output=True, text=True, encoding="utf-8", env=environment, timeout=330)
                elapsed = time.perf_counter() - wall
                finished = timestamp()
                raw.write(f"\n# variant={variant} sample={sample} order_seed={seed} sequence={sequence} operation={operation} started_at_utc={started}\n")
                raw.write(child.stdout)
                if child.stderr:
                    raw.write("# stderr\n" + child.stderr)
                raw.flush()
                if child.returncode:
                    raise RuntimeError(f"{variant} benchmark round {sample} failed; see raw output")
                rows = validate_rows(parse_rows, child.stdout)
                if len(rows) != 1 or rows[0]["benchmark"] != operation:
                    raise ValueError("isolated child did not execute exactly the selected operation")
                if (variant, operation) in observed:
                    raise ValueError("duplicate variant/operation pair in a round")
                observed.add((variant, operation))
                rows[0].update({"variant": variant, "sample": sample, "order_seed": seed,
                                "execution_sequence": sequence, "selection_flags": operation_flags,
                                "started_at_utc": started, "finished_at_utc": finished,
                                "process_wall_seconds": elapsed})
                report["results"].append(rows[0])
                write_report(output, report)
            if observed != set(pairs):
                raise ValueError("incomplete balanced round")
            print(f"Paired Go round {sample}/{args.samples}: {len(observed)} isolated variant/operation pairs", flush=True)
    for variant in VARIANTS:
        if source_manifest(roots[variant], modules[variant]) != source_records[variant]["source_sha256"]:
            raise ValueError(variant + " source files changed during measurement")
        if sha256(binaries[variant]) != source_records[variant]["binary_sha256"]:
            raise ValueError(variant + " binary changed during measurement")
    if {"implementations/benchmarks/run-paired-go.py": sha256(CONTROLLER),
        "implementations/benchmarks/run-go.py": sha256(PARSER_SOURCE)} != controller_hashes:
        raise ValueError("benchmark controller changed during measurement")
    expected_rows = len(expected_names) * len(VARIANTS) * args.samples
    if len(report["results"]) != expected_rows:
        raise ValueError("total observation count differs from the requested complete campaign")
    summarise(report, expected_names, args.samples)
    report["finished_at_utc"] = timestamp()
    report["status"] = "complete"
    write_report(output, report)


if __name__ == "__main__":
    main()
