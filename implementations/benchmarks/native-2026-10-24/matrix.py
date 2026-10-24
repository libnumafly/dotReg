"""Serial, balanced native API benchmarks; run from the repository root.

Local config contains engines with language/runtime/command/cli/sources/build
fields. Commands receive fixture, fixed fixture iterations, and mode. CLI
commands receive --mode strings and fixture. See config.example.json.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import platform
import random
import statistics
import subprocess
import sys

from fixtures import canonical, generate, workloads


def timestamp():
    return datetime.now(timezone.utc).isoformat()


def checked(command, timeout=300):
    child = subprocess.run(command, capture_output=True, timeout=timeout)
    if child.returncode:
        raise RuntimeError(f"process failed ({command[0]}): {child.stderr.decode('utf-8', 'replace')[-2000:]}")
    return child.stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--fixtures", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--samples", type=int, default=5)
    parser.add_argument("--seed", type=int, default=20261024)
    parser.add_argument("--verify-only", action="store_true")
    args = parser.parse_args()
    if args.samples < 3:
        parser.error("at least three independent process samples are required")
    config = json.loads(Path(args.config).read_text(encoding="utf-8-sig"))
    started = timestamp()
    manifest = generate(args.fixtures)
    engines = config["engines"]
    expected = {case.name: case.expected for case in workloads()}
    validation = []
    for case in manifest:
        fixture = str((Path(args.fixtures) / case["filename"]).resolve())
        for engine in engines:
            entries = json.loads(checked(engine["cli"] + ["--mode", "strings", fixture]))
            if entries != expected[case["name"]]:
                for index, (actual, wanted) in enumerate(zip(entries, expected[case["name"]])):
                    if actual != wanted:
                        raise ValueError(f"independent expected-result mismatch: {engine['language']} {case['name']} entry {index}")
                raise ValueError(f"entry-count mismatch: {engine['language']} {case['name']}")
            validation.append({"fixture": case["name"], "language": engine["language"],
                               "entries": len(entries), "expected_entries_sha256": hashlib.sha256(canonical(entries)).hexdigest()})
            print(f"verified {engine['language']} {case['name']} {len(entries)} entries", flush=True)
    sources = {}
    for source in sorted(set(["docs/config-format.md", "implementations/benchmarks/measure.py"]
                             + [p.resolve().relative_to(Path.cwd().resolve()).as_posix() for p in Path(__file__).parent.glob("*.py")]
                             + [p for e in engines for p in e["sources"]])):
        sources[source] = hashlib.sha256(Path(source).read_bytes()).hexdigest()
    report = {"version": 2, "started_at_utc": started,
              "source_commit": checked(["git", "rev-parse", "HEAD"]).decode().strip(),
              "source_commit_role": "Base commit. This benchmark uses the working-tree snapshot identified by source_sha256, including the decoded-length Go harness correction and the current Go prefix-selection consumer sources. Consumer Build/prefix selection is outside the timed work.",
              "platform": platform.platform(), "machine": platform.machine(),
              "environment": config.get("environment", {}),
              "tool_versions": config.get("tool_versions", {}),
              "previous_attempt": "An earlier campaign was aborted after a Windows Errno22 report-write failure in the synchronised checkout. All accepted samples below belong to a fresh complete run using temporary local report storage. No samples were selected or removed on timing grounds.",
              "samples_per_condition": args.samples, "order_seed": args.seed,
              "warmup_passes_per_process": 2,
              "method": "All engines receive identical UTF-8/LF fixture bytes. Fixed iterations per fixture across engines and modes. Serial fresh processes, balanced sample rounds with seeded shuffled condition order; syntax plus common string decoding and buffered file reads; entry JSON excluded. Two untimed warm-ups per process. Runtime default GC; finite-pass JIT behaviour, not guaranteed steady state. Process RSS includes startup, warm-up, buffers and runtime; Windows high-water sampled every 5ms, may miss final growth.",
              "engines": [{k: v for k, v in e.items() if k not in ("command", "cli")} for e in engines],
              "fixtures": manifest, "validation": validation,
              "source_sha256": sources, "results": [], "summary": []}
    output = Path(args.output)
    def save():
        output.write_text(json.dumps(report, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")
    save()
    if args.verify_only:
        return
    rng = random.Random(args.seed)
    conditions = [(case, engine, mode) for case in manifest for engine in engines for mode in ("stream", "collect")]
    sequence = 0
    for sample in range(1, args.samples + 1):
        ordered = conditions.copy()
        rng.shuffle(ordered)
        for case, engine, mode in ordered:
            sequence += 1
            fixture = str((Path(args.fixtures) / case["filename"]).resolve())
            command = engine["command"] + [fixture, str(case["iterations"]), mode]
            before = timestamp()
            child = subprocess.run([sys.executable, str(Path(__file__).parent.parent / "measure.py"),
                                    "--command", json.dumps(command)], capture_output=True, timeout=330)
            if child.returncode:
                raise RuntimeError(f"measurement failed: {case['name']} {engine['language']} {mode}: "
                                   + child.stderr.decode('utf-8', 'replace')[-2000:])
            row = json.loads(child.stdout)
            if (row["language"] != engine["language"] or row["mode"] != mode
                or row["iterations"] != case["iterations"] or row["input_bytes"] != case["bytes"]
                or row["entries"] != case["entries"] or row["fixture_sha256"] != case["sha256"]
                or row["elapsed_seconds"] <= 0):
                raise ValueError("unexpected native harness result")
            row.update({"fixture": case["name"], "sample": sample, "execution_sequence": sequence,
                        "started_at_utc": before, "finished_at_utc": timestamp(),
                        "milliseconds_per_pass": 1000 * row["elapsed_seconds"] / case["iterations"]})
            report["results"].append(row)
            save()
            print(f"{sequence}/{len(conditions) * args.samples}: {case['name']} {engine['language']} {mode} "
                  f"sample {sample}: {row['milliseconds_per_pass']:.5f} ms/pass", flush=True)
    for case, engine, mode in conditions:
        rows = [r for r in report["results"] if r["fixture"] == case["name"]
                and r["language"] == engine["language"] and r["mode"] == mode]
        elapsed = [r["milliseconds_per_pass"] for r in rows]
        rss = [r["observed_peak_rss_bytes"] for r in rows if r["observed_peak_rss_bytes"] is not None]
        item = {"fixture": case["name"], "language": engine["language"], "mode": mode,
                "samples": len(rows), "median_ms_per_pass": statistics.median(elapsed),
                "min_ms_per_pass": min(elapsed), "max_ms_per_pass": max(elapsed),
                "median_absolute_deviation_ms": statistics.median(abs(e - statistics.median(elapsed)) for e in elapsed),
                "median_peak_rss_bytes": statistics.median(rss) if rss else None}
        allocation = [r["allocated_bytes"] / r["iterations"] for r in rows if "allocated_bytes" in r]
        if allocation:
            item["median_allocated_bytes_per_pass"] = statistics.median(allocation)
        report["summary"].append(item)
    report["finished_at_utc"] = timestamp()
    save()


if __name__ == "__main__":
    main()
