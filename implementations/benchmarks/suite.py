"""Run isolated benchmark samples serially from a local toolchain config.

Config: {"fixture":"path", "iterations":5, "samples":3,
"engines":[{"language":"go", "runtime":"Go version and flags",
"command":["path/to/configbench"], "sources":["relative/source.go"]}]}
Commands receive fixture, iterations and stream|collect appended to their argv.
Local commands/paths are not copied into the report. See README.md for limits.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text(encoding="utf-8-sig"))
    fixture = str(Path(config["fixture"]).resolve())
    iterations, samples = config.get("iterations", 5), config.get("samples", 3)
    if type(iterations) is not int or type(samples) is not int or min(iterations, samples) < 1:
        parser.error("iterations and samples must be positive integers")
    results, sources = [], {}
    for engine in config["engines"]:
        for source in engine.get("sources", []):
            sources[source] = hashlib.sha256(Path(source).read_bytes()).hexdigest()
        for mode in ("stream", "collect"):
            for sample in range(1, samples + 1):
                command = engine["command"] + [fixture, str(iterations), mode]
                child = subprocess.run(
                    [sys.executable, str(Path(__file__).with_name("measure.py")), "--command", json.dumps(command)],
                    capture_output=True, text=True, timeout=330, check=True,
                )
                row = json.loads(child.stdout)
                if row["language"] != engine["language"] or row["mode"] != mode or row["iterations"] != iterations:
                    raise ValueError("unexpected harness result")
                if row["input_bytes"] != Path(fixture).stat().st_size or row["entries"] < 1 or row["elapsed_seconds"] <= 0:
                    raise ValueError("invalid harness counters")
                row.update({"sample": sample, "runtime": engine["runtime"]})
                results.append(row)
                print(f'{row["language"]} {mode} sample {sample}: {row["elapsed_seconds"]:.6f}s, observed peak {row["observed_peak_rss_bytes"]}', file=sys.stderr)
    if len({row["entries"] for row in results}) != 1:
        raise ValueError("entry counts differ across engines")
    for filename in ("fixture.mjs", "measure.py", "suite.py"):
        source = Path(__file__).with_name(filename)
        sources[source.as_posix() if not source.is_absolute() else "implementations/benchmarks/" + filename] = hashlib.sha256(source.read_bytes()).hexdigest()
    report = {
        "version": 1, "measured_at_utc": datetime.now(timezone.utc).isoformat(),
        "method": "Serial isolated processes; two warm-up passes, then timed parse plus common decoding; no entry JSON; process RSS includes runtime and warm-up; high-water polling may miss a final increase.",
        "source_sha256": sources, "results": results,
    }
    Path(args.output).write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
