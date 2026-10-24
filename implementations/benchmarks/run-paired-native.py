"""Compare native sample snapshots in serial, balanced fresh-process rounds.

Configs come from native-2026-10-24/prepare_windows.py. Absolute executable
paths remain in those local configs; published results contain their hashes,
runtime descriptions and relative source manifests. No build runs here.
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
import tempfile


HERE = Path(__file__).resolve().parent
CONTROLLER_ROOT = HERE.parent.parent
MEASURE = HERE / "measure.py"
FIXTURES = HERE / "native-2026-10-24/fixtures.py"
LANGUAGES = {"go", "python", "javascript", "java", "csharp", "rust", "cpp"}
EXCLUDED = {"bin", "obj", "target", "__pycache__", "node_modules", ".artifacts"}
CODE_SUFFIXES = {".go", ".py", ".mjs", ".mts", ".ts", ".java", ".cs", ".csproj", ".rs", ".cpp", ".hpp", ".h"}
CODE_NAMES = {"go.mod", "go.sum", "Cargo.toml", "Cargo.lock", "CMakeLists.txt"}


def timestamp():
    return datetime.now(timezone.utc).isoformat()


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def relative_file(root, name):
    """Accept repository-relative names only, including on Windows."""
    candidate = Path(name)
    if candidate.is_absolute() or ".." in candidate.parts or "\\" in name:
        raise ValueError("source names must be repository-relative POSIX paths")
    resolved = (root / candidate).resolve()
    resolved.relative_to(root)
    if not resolved.is_file():
        raise ValueError("recorded source is not a file")
    return resolved


def source_snapshot(root, engines):
    names = {"docs/config-format.md", "implementations/conformance/cases.json"}
    names.update(name for engine in engines.values() for name in engine["sources"])
    # Include native tests and the shared TypeScript interface as well as each
    # engine's recorded build inputs. Generated outputs are deliberately absent.
    for language in LANGUAGES | {"typescript"}:
        directory = root / "implementations" / language
        for source in directory.rglob("*"):
            relative = source.relative_to(root)
            if (source.is_file() and not EXCLUDED.intersection(relative.parts)
                    and (source.suffix in CODE_SUFFIXES or source.name in CODE_NAMES)):
                names.add(relative.as_posix())
    return {name: sha256(relative_file(root, name)) for name in sorted(names)}


def load_config(path):
    config = json.loads(path.read_text(encoding="utf-8-sig"))
    engines = {}
    for engine in config["engines"]:
        language = engine["language"]
        if language in engines:
            raise ValueError("duplicate engine language")
        for key in ("command", "cli", "sources"):
            value = engine[key]
            if (not isinstance(value, list) or not value
                    or any(not isinstance(item, str) or not item for item in value)):
                raise ValueError("engine commands and sources must be nonempty string arrays")
        binaries = engine["binary_sha256"]
        if not isinstance(binaries, dict):
            raise ValueError("binary hash manifest must be an object")
        for name, digest in binaries.items():
            if (Path(name).name != name or "/" in name or "\\" in name
                    or not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest)):
                raise ValueError("invalid binary hash manifest")
        engines[language] = engine
    if set(engines) != LANGUAGES:
        raise ValueError("both configs must contain the same seven native engines")
    return config, engines


def binary_snapshot(root, engines):
    """Locate only the build artifacts named by the prepared config."""
    result = {}
    for language, engine in sorted(engines.items()):
        hashes = {}
        for name, expected in sorted(engine["binary_sha256"].items()):
            matches = set()
            for argument in engine["command"] + engine["cli"]:
                # Java classpaths can contain multiple directory entries.
                for part in argument.split(os.pathsep):
                    path = Path(part)
                    if not path.is_absolute():
                        path = root / path
                    if path.is_file() and path.name == name:
                        matches.add(path.resolve())
                    elif path.is_dir() and (path / name).is_file():
                        matches.add((path / name).resolve())
            if len(matches) != 1:
                raise ValueError(f"cannot uniquely locate recorded {language} artifact {name}")
            actual = sha256(matches.pop())
            if actual != expected:
                raise ValueError(f"prepared {language} artifact hash mismatch: {name}")
            hashes[name] = actual
        result[language] = hashes
    return result


def public_engines(engines):
    return [{key: engine[key] for key in ("language", "runtime", "build", "sources", "binary_sha256")}
            for _, engine in sorted(engines.items())]


def check_runtime_defaults():
    names = {"GOGC", "GOMEMLIMIT", "GOMAXPROCS", "GODEBUG", "NODE_OPTIONS",
             "JAVA_TOOL_OPTIONS", "JDK_JAVA_OPTIONS", "_JAVA_OPTIONS", "PYTHONMALLOC"}
    overrides = sorted(name for name, value in os.environ.items() if value and
                       (name.upper() in names or name.upper().startswith(("DOTNET_GC", "COMPLUS_GC"))))
    if overrides:
        raise ValueError("clear runtime tuning environment variables before measuring: " + ", ".join(overrides))


def atomic_write(output, report):
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", newline="\n",
                                         dir=output.parent, prefix=output.name + ".", suffix=".tmp",
                                         delete=False) as handle:
            temporary = Path(handle.name)
            json.dump(report, handle, ensure_ascii=True, indent=2, allow_nan=False)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, output)
    finally:
        if temporary is not None and temporary.exists():
            temporary.unlink()


def run_json(command, root, label, timeout):
    child = subprocess.run(command, cwd=root, capture_output=True, timeout=timeout)
    if child.returncode:
        # Commands, runtime errors and source values stay in local diagnostics,
        # rather than being copied into the public report.
        raise RuntimeError(f"{label} process failed with exit code {child.returncode}")
    if child.stderr.strip():
        raise RuntimeError(f"{label} process emitted unexpected diagnostics")
    return json.loads(child.stdout)


def finite_number(value, *, positive=False):
    return (isinstance(value, (int, float)) and not isinstance(value, bool)
            and math.isfinite(value) and (value > 0 if positive else value >= 0))


def check_measurement(row, engine, case, mode):
    if not isinstance(row, dict):
        raise ValueError("measurement must be a JSON object")
    expected = {"language": engine["language"], "mode": mode,
                "iterations": case["iterations"], "input_bytes": case["bytes"],
                "entries": case["entries"], "fixture_sha256": case["sha256"]}
    if any(type(row.get(key)) is not type(value) or row[key] != value for key, value in expected.items()):
        raise ValueError("measurement fixture, engine or iteration mismatch")
    if (not finite_number(row.get("elapsed_seconds"), positive=True)
            or not finite_number(row.get("process_wall_seconds"), positive=True)
            or not isinstance(row.get("rss_samples"), int) or isinstance(row["rss_samples"], bool)
            or row["rss_samples"] < 1):
        raise ValueError("invalid measurement time or RSS sampling count")
    rss = row.get("observed_peak_rss_bytes")
    if rss is not None and (not isinstance(rss, int) or isinstance(rss, bool) or rss <= 0):
        raise ValueError("invalid observed peak RSS")
    if "allocated_bytes" in row and not finite_number(row["allocated_bytes"]):
        raise ValueError("invalid allocation count")
    if "warmup_iterations" in row and row["warmup_iterations"] != 2:
        raise ValueError("unexpected warm-up pass count")


def distribution(values):
    if not values:
        return {"samples": 0, "median": None, "min": None, "max": None, "mad": None}
    median = statistics.median(values)
    return {"samples": len(values), "median": median, "min": min(values), "max": max(values),
            "mad": statistics.median(abs(value - median) for value in values)}


def summarise(results, conditions, samples):
    summary = []
    for variant, language, case, mode in conditions:
        rows = [row for row in results if row["variant"] == variant and row["language"] == language
                and row["fixture"] == case["name"] and row["mode"] == mode]
        if len(rows) != samples or {row["sample"] for row in rows} != set(range(1, samples + 1)):
            raise ValueError("unbalanced or duplicate condition samples")
        item = {"variant": variant, "language": language, "fixture": case["name"], "mode": mode,
                "samples": len(rows),
                "milliseconds_per_pass": distribution([row["milliseconds_per_pass"] for row in rows]),
                "observed_peak_rss_bytes": distribution([row["observed_peak_rss_bytes"] for row in rows
                                                         if row["observed_peak_rss_bytes"] is not None]),
                "process_wall_seconds": distribution([row["process_wall_seconds"] for row in rows])}
        allocation = [row["allocated_bytes"] / row["iterations"] for row in rows if "allocated_bytes" in row]
        if allocation:
            if len(allocation) != samples:
                raise ValueError("allocation counter absent from some condition samples")
            item["allocated_bytes_per_pass"] = distribution(allocation)
        summary.append(item)
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("baseline-root", "baseline-config", "candidate-root", "candidate-config",
                 "baseline-commit", "fixtures", "output"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--samples", type=int, default=5)
    parser.add_argument("--seed", type=int, default=20261024)
    parser.add_argument("--verify-only", action="store_true", help="Validate artifacts and all CLI results without timing")
    args = parser.parse_args()
    if args.samples < 1:
        parser.error("samples must be positive")
    if not re.fullmatch(r"[0-9a-f]{40}", args.baseline_commit):
        parser.error("baseline-commit must be a full lower-case Git commit ID")
    check_runtime_defaults()
    output = Path(args.output).resolve()
    if output.exists():
        parser.error("output already exists; choose a new report filename")

    # Import this fixed generator by file path; no sample parser is imported.
    spec = importlib.util.spec_from_file_location("dotreg_native_fixed_fixtures", FIXTURES)
    fixture_module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(fixture_module)
    fixtures = Path(args.fixtures).resolve()
    manifest = fixture_module.generate(fixtures)
    expected = {case.name: case.expected for case in fixture_module.workloads()}
    variants = {}
    for variant in ("baseline", "candidate"):
        root = Path(getattr(args, variant + "_root")).resolve()
        config_path = Path(getattr(args, variant + "_config")).resolve()
        config, engines = load_config(config_path)
        variants[variant] = {"root": root, "config_path": config_path, "config": config, "engines": engines,
                             "config_sha256": sha256(config_path),
                             "source_sha256": source_snapshot(root, engines),
                             "verified_binary_sha256": binary_snapshot(root, engines)}
    if variants["baseline"]["source_sha256"]["docs/config-format.md"] != variants["candidate"]["source_sha256"]["docs/config-format.md"]:
        raise ValueError("the two snapshots must use the same specification")

    controller_paths = [Path(__file__).resolve(), MEASURE, FIXTURES]
    controller_hashes = {path.relative_to(CONTROLLER_ROOT).as_posix(): sha256(path) for path in controller_paths}
    started = timestamp()
    report = {
        "version": 1, "status": "running", "started_at_utc": started,
        "baseline_commit": args.baseline_commit,
        "baseline_commit_role": "Requested unchanged baseline commit; exact source manifests and prepared artifact hashes identify both measured snapshots.",
        "platform": platform.platform(), "machine": platform.machine(),
        "controller_python": platform.python_version(), "controller_sha256": controller_hashes,
        "samples_per_condition": args.samples, "order_seed": args.seed,
        "warmup_passes_per_process": 2, "runtime_tuning_environment": {}, "variants": {}, "fixtures": manifest,
        "method": "Both snapshots use identical UTF-8/LF fixture bytes and fixed iterations per fixture. Independent complete CLI entry arrays are checked before timing. All baseline/candidate/language/fixture/mode conditions are jointly shuffled once per balanced sample round. One unchanged measure.py wrapper launches one fresh harness process per job, serially. Parsing, common string decoding and cached buffered file reads are timed; CLI JSON and independent validation are excluded. Runtime/GC defaults are inherited without adjustments by this controller; two untimed warm-ups, finite-pass JIT behaviour, not guaranteed steady state. RSS includes startup, warm-up, buffers and runtime; 5 ms high-water polling can miss final growth. Allocation volume is a separate optional native counter, not peak RSS.",
        "validation": [], "results": [], "summary": [],
        "expected_validation_count": len(variants) * len(LANGUAGES) * len(manifest),
        "expected_result_count": len(variants) * len(LANGUAGES) * len(manifest) * 2 * args.samples,
    }
    for variant, data in variants.items():
        report["variants"][variant] = {"config_sha256": data["config_sha256"],
                                       "source_sha256": data["source_sha256"],
                                       "verified_binary_sha256": data["verified_binary_sha256"],
                                       "environment": data["config"].get("environment", {}),
                                       "tool_versions": data["config"].get("tool_versions", {}),
                                       "engines": public_engines(data["engines"])}

    def save():
        atomic_write(output, report)

    def check_end_hashes():
        if any(sha256(path) != controller_hashes[path.relative_to(CONTROLLER_ROOT).as_posix()]
               for path in controller_paths):
            raise ValueError("controller source changed during campaign")
        for variant, data in variants.items():
            if (sha256(data["config_path"]) != data["config_sha256"]
                    or source_snapshot(data["root"], data["engines"]) != data["source_sha256"]
                    or binary_snapshot(data["root"], data["engines"]) != data["verified_binary_sha256"]):
                raise ValueError(f"{variant} source, config or artifacts changed during campaign")
        for case in manifest:
            if sha256(fixtures / case["filename"]) != case["sha256"]:
                raise ValueError("fixture changed during campaign")
        report["end_hash_verification"] = "All variant source/config/artifact, controller and fixture hashes match preparation."

    save()
    try:
        for variant, data in variants.items():
            for language, engine in sorted(data["engines"].items()):
                for case in manifest:
                    entries = run_json(engine["cli"] + ["--mode", "strings", str(fixtures / case["filename"])],
                                       data["root"], f"{variant} {language} validation", 300)
                    if entries != expected[case["name"]]:
                        raise ValueError(f"independent entry-array mismatch: {variant} {language} {case['name']}")
                    report["validation"].append({"variant": variant, "language": language,
                                                 "fixture": case["name"], "entries": len(entries),
                                                 "expected_entries_sha256": hashlib.sha256(fixture_module.canonical(entries)).hexdigest()})
                    save()
                    print(f"verified {variant} {language} {case['name']}", flush=True)
        if len(report["validation"]) != report["expected_validation_count"]:
            raise ValueError("incomplete independent validation")
        save()
        if args.verify_only:
            check_end_hashes()
            report.update({"status": "verified-only", "finished_at_utc": timestamp()})
            save()
            return

        conditions = [(variant, language, case, mode) for variant in variants
                      for language in sorted(LANGUAGES) for case in manifest for mode in ("stream", "collect")]
        rng = random.Random(args.seed)
        sequence = 0
        for sample in range(1, args.samples + 1):
            ordered = conditions.copy()
            rng.shuffle(ordered)
            for variant, language, case, mode in ordered:
                sequence += 1
                data = variants[variant]
                engine = data["engines"][language]
                command = engine["command"] + [str(fixtures / case["filename"]), str(case["iterations"]), mode]
                before = timestamp()
                measurement = run_json([sys.executable, str(MEASURE), "--command", json.dumps(command)],
                                       data["root"], f"{variant} {language} measurement", 330)
                check_measurement(measurement, engine, case, mode)
                row = dict(measurement)
                row.update({"variant": variant, "fixture": case["name"], "sample": sample,
                            "execution_sequence": sequence, "started_at_utc": before,
                            "finished_at_utc": timestamp(),
                            "milliseconds_per_pass": 1000 * measurement["elapsed_seconds"] / case["iterations"],
                            "measurement": measurement})
                report["results"].append(row)
                save()
                print(f"{sequence}/{report['expected_result_count']}: {variant} {language} {case['name']} {mode} "
                      f"sample {sample}: {row['milliseconds_per_pass']:.5f} ms/pass", flush=True)
        if len(report["results"]) != report["expected_result_count"]:
            raise ValueError("incomplete measurement campaign")
        report["summary"] = summarise(report["results"], conditions, args.samples)
        if len(report["summary"]) != len(conditions):
            raise ValueError("incomplete summary")
        check_end_hashes()
        report.update({"status": "complete", "finished_at_utc": timestamp()})
        save()
    except BaseException as error:
        report.update({"status": "failed", "finished_at_utc": timestamp(),
                       "failure": "Campaign stopped without accepting a complete result: " + type(error).__name__})
        save()
        raise


if __name__ == "__main__":
    main()
