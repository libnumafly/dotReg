"""Run serial Go benchmark rounds and retain raw output plus a parsed report.

Build a test binary with go test -c first. Every round uses a fresh process,
GOMAXPROCS=1, one testing.B sample per operation and a recorded shuffle seed.
Run from the repository root; --module identifies the source module for hashes.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import statistics
import subprocess


def timestamp():
    return datetime.now(timezone.utc).isoformat()


def parse_rows(output):
    rows = []
    for line in output.splitlines():
        match = re.match(r"^(Benchmark\S+)\s+(\d+)\s+(.+)$", line)
        if not match:
            continue
        fields = match[3].split()
        if len(fields) % 2:
            raise ValueError("incomplete benchmark metric pair")
        metrics = {fields[i + 1]: float(fields[i]) for i in range(0, len(fields), 2)}
        if metrics.get("ns/op", 0) <= 0 or "B/op" not in metrics or "allocs/op" not in metrics:
            raise ValueError("missing benchmark time/allocation counters")
        rows.append({"benchmark": re.sub(r"-1$", "", match[1]),
                     "iterations": int(match[2]), "metrics": metrics})
    if not rows or len({row["benchmark"] for row in rows}) != len(rows):
        raise ValueError("empty or duplicate benchmark output")
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True)
    parser.add_argument("--module", required=True)
    parser.add_argument("--bench", default="^BenchmarkConfig$")
    parser.add_argument("--samples", type=int, default=10)
    parser.add_argument("--benchtime", default="100ms")
    parser.add_argument("--seed", type=int, default=20261024)
    parser.add_argument("--output", required=True)
    parser.add_argument("--raw", required=True)
    parser.add_argument("--manifest")
    parser.add_argument("--scope", choices=("common-map", "dotreg-api"), default="common-map")
    parser.add_argument("--order", choices=("seeded", "declared"), default="seeded")
    args = parser.parse_args()
    if args.samples < 3:
        parser.error("at least three serial process rounds required")
    binary = Path(args.binary).resolve()
    module = Path(args.module)
    production = [p for p in Path("implementations/go").rglob("*.go") if "cmd" not in p.parts]
    sources = sorted(set([Path("docs/config-format.md"), Path(__file__), module / "go.mod"]
                         + list(module.rglob("*.go")) + production
                         + ([module / "go.sum"] if (module / "go.sum").exists() else [])))
    # Keep source records relative and omit generated build outputs.
    sources = [p for p in sources if not any(part in ("bin", "obj", "target", "__pycache__") for part in p.parts)]
    source_hashes = {p.resolve().relative_to(Path.cwd().resolve()).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest() for p in sources}
    go_env = json.loads(subprocess.check_output(
        ["go", "env", "-json", "GOVERSION", "GOOS", "GOARCH", "GOEXPERIMENT", "GOFLAGS", "CGO_ENABLED"], text=True))
    flags = ["-test.run=^$", "-test.bench=" + args.bench, "-test.benchmem",
             "-test.benchtime=" + args.benchtime, "-test.cpu=1", "-test.count=1", "-test.timeout=5m"]
    report = {
        "version": 1, "started_at_utc": timestamp(),
        "source_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
        "source_state": "The source hash manifest identifies the measured working tree, including changes awaiting the benchmark publication commit.",
        "source_sha256": source_hashes,
        "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
        "go_environment": go_env, "platform": platform.platform(), "machine": platform.machine(),
        "samples_per_benchmark": args.samples, "base_order_seed": args.seed,
        "benchmark_flags": flags,
        "runtime_environment": {"GOMAXPROCS": "1", "GOGC": "100", "GOMEMLIMIT": "off",
                                "inherited_GODEBUG": os.environ.get("GODEBUG", "")},
        "scope": args.scope,
        "operation_order": "BenchmarkConfig shuffles all case/operation pairs with the recorded per-round seed" if args.order == "seeded" else "Declared benchmark order; the seed is passed but these API benchmarks do not shuffle operations",
        "method": "Serial fresh test-binary processes; GOMAXPROCS=1, GOGC=100, GOMEMLIMIT=off, no forced GC within timed loops. testing.B calibration and GC between samples follow the Go test runner. Inputs and independent equivalence checks are outside the timed region. The common-map tier includes decoded common-map construction; the dotReg API tier measures the stage identified by each benchmark name. B/op and allocs/op are cumulative allocation volume, not peak memory. All observations retained, no outliers discarded.",
        "results": [], "summary": [],
    }
    if args.manifest:
        report["fixture_manifest"] = json.loads(Path(args.manifest).read_text(encoding="utf-8"))
    environment = os.environ.copy()
    environment.update({"GOMAXPROCS": "1", "GOGC": "100", "GOMEMLIMIT": "off"})
    # Make experimental Go compiler/runtime settings explicit and stable.
    expected_names = None
    with Path(args.raw).open("w", encoding="utf-8", newline="\n") as raw:
        raw.write("# Serial Go benchmark rounds; command flags and source hashes in the JSON report.\n")
        for sample in range(1, args.samples + 1):
            seed = args.seed + sample - 1
            environment["DOTREG_BENCH_SEED"] = str(seed)
            started = timestamp()
            child = subprocess.run([str(binary)] + flags, capture_output=True, text=True,
                                   encoding="utf-8", env=environment, timeout=330)
            raw.write(f"\n# sample={sample} order_seed={seed} started_at_utc={started}\n")
            raw.write(child.stdout)
            if child.stderr:
                raw.write("# stderr\n" + child.stderr)
            raw.flush()
            if child.returncode:
                raise RuntimeError(f"benchmark round {sample} failed; see raw output")
            rows = parse_rows(child.stdout)
            names = {row["benchmark"] for row in rows}
            if expected_names is None:
                expected_names = names
            elif names != expected_names:
                raise ValueError("benchmark operation set changed between rounds")
            for row in rows:
                row.update({"sample": sample, "order_seed": seed,
                            "started_at_utc": started, "finished_at_utc": timestamp()})
                report["results"].append(row)
            Path(args.output).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
            print(f"Go benchmark round {sample}/{args.samples}: {len(rows)} operations", flush=True)
    for name in sorted(expected_names):
        rows = [r for r in report["results"] if r["benchmark"] == name]
        metric_names = set.intersection(*(set(r["metrics"]) for r in rows))
        summary = {"benchmark": name, "samples": len(rows), "metrics": {}}
        for metric in sorted(metric_names):
            values = [r["metrics"][metric] for r in rows]
            summary["metrics"][metric] = {"median": statistics.median(values), "min": min(values), "max": max(values)}
        if all("values/op" in r["metrics"] for r in rows):
            speeds = [r["metrics"]["values/op"] * 1e9 / r["metrics"]["ns/op"] for r in rows]
            summary["values_per_second"] = {"median": statistics.median(speeds), "min": min(speeds), "max": max(speeds)}
        report["summary"].append(summary)
    report["finished_at_utc"] = timestamp()
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
