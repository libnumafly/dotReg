"""Build fresh native benchmark/CLI binaries in a temporary directory.

Tool paths are explicit rather than downloaded. Requires Go on PATH and already
installed Python, Node, JDK17+, .NET8 SDK, Rust GNU, and LLVM-MinGW toolchains.
Only the local config contains absolute executable paths; the published report
retains version descriptions, build flags, source hashes and binary hashes.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess


def run(argv, env=None):
    subprocess.run([str(a) for a in argv], check=True, env=env)


def version(argv):
    return subprocess.run([str(a) for a in argv], check=True, capture_output=True,
                          text=True, encoding="utf-8", errors="replace").stdout.strip()


def environment_snapshot():
    command = """
$nativeCpu = Get-CimInstance Win32_Processor | Select-Object -First 1
$nativeOs = Get-CimInstance Win32_OperatingSystem
$nativeMemory = Get-CimInstance Win32_ComputerSystem
$nativeBattery = Get-CimInstance Win32_Battery | Select-Object -First 1
[ordered]@{
    cpu = $nativeCpu.Name.Trim()
    physical_cores = [int]$nativeCpu.NumberOfCores
    logical_processors = [int]$nativeCpu.NumberOfLogicalProcessors
    physical_memory_bytes = [long]$nativeMemory.TotalPhysicalMemory
    os = ($nativeOs.Caption + ' ' + $nativeOs.Version + ' ' + $nativeOs.OSArchitecture)
    battery_status_code = $nativeBattery.BatteryStatus
    battery_charge_percent = $nativeBattery.EstimatedChargeRemaining
} | ConvertTo-Json -Compress
"""
    data = json.loads(version(["powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command]))
    data["observed_at_utc"] = datetime.now(timezone.utc).isoformat()
    data["power_scheme_observation"] = version(["powercfg", "/getactivescheme"])
    data["observation_scope"] = "Point-in-time preparation observation; not continuously monitored or enforced during timing"
    data["background_workload"] = "Benchmark jobs serial; agents coordinated to avoid builds/tests during timing. Other desktop/OS activity uncontrolled."
    data["runtime_policy"] = "Default runtime/GC settings; no CPU pinning, forced GC, priority adjustment or power-plan change."
    return data


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("directory", "python", "node", "jdk", "dotnet", "rustc", "cxx"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    temp = Path(args.directory).resolve()
    temp.mkdir(parents=True, exist_ok=True)
    java = temp / "java"
    java.mkdir(exist_ok=True)
    csharp = temp / "csharp"
    shutil.copytree("implementations/csharp", csharp, dirs_exist_ok=True,
                    ignore=shutil.ignore_patterns("bin", "obj"))
    run(["go", "-C", "implementations/go", "build", "-trimpath", "-o", temp / "configbench.exe", "./cmd/configbench"])
    run(["go", "-C", "implementations/go", "build", "-trimpath", "-o", temp / "configsample.exe", "./cmd/configsample"])
    run([str(Path(args.jdk) / "bin/javac.exe"), "-encoding", "UTF-8", "-d", java,
         "implementations/java/DotReg.java", "implementations/java/DotRegBenchmark.java"])
    env = dict(os.environ, DOTNET_CLI_TELEMETRY_OPTOUT="1", DOTNET_NOLOGO="1")
    run([args.dotnet, "build", csharp / "benchmarks/DotReg.Benchmark.csproj", "-c", "Release", "--nologo"], env)
    run([args.cxx, "-std=c++17", "-O2", "-Wall", "-Wextra", "-pedantic", "-static",
         "implementations/cpp/main.cpp", "-o", temp / "dotreg-cpp.exe"])
    run([args.rustc, "--crate-name", "dotreg", "--crate-type", "rlib", "--edition=2021", "-C", "opt-level=3",
         "implementations/rust/src/lib.rs", "-o", temp / "libdotreg.rlib"])
    rust_linker = Path(args.rustc).parent.parent / "lib/rustlib/x86_64-pc-windows-gnu/bin/self-contained/x86_64-w64-mingw32-gcc.exe"
    run([args.rustc, "--edition=2021", "-C", "opt-level=3", "-C", "linker=" + str(rust_linker),
         "-C", "link-self-contained=yes",
         "--extern", "dotreg=" + str(temp / "libdotreg.rlib"), "implementations/rust/src/main.rs",
         "-o", temp / "dotreg-rust.exe"])
    def engine(language, runtime, build, command, cli, sources, binaries):
        return {"language": language, "runtime": runtime, "build": build,
                "command": [str(p) for p in command], "cli": [str(p) for p in cli],
                "sources": sources,
                "binary_sha256": {Path(p).name: hashlib.sha256(Path(p).read_bytes()).hexdigest() for p in binaries}}
    csharp_bench = csharp / "benchmarks/bin/Release/net8.0/DotReg.Benchmark.dll"
    csharp_cli = csharp / "bin/Release/net8.0/DotReg.Sample.dll"
    java_version = subprocess.run([str(Path(args.jdk) / "bin/java.exe"), "-version"], check=True, capture_output=True, text=True).stderr.strip()
    dotnet_runtimes = [" ".join(line.split()[:2]) for line in version([args.dotnet, "--list-runtimes"]).splitlines()]
    versions = {"go": version(["go", "version"]), "python": version([args.python, "--version"]),
                "node": version([args.node, "--version"]), "java": java_version,
                "dotnet_sdk": version([args.dotnet, "--version"]), "dotnet_runtimes": dotnet_runtimes,
                "rust": version([args.rustc, "--version"]), "cpp": version([args.cxx, "--version"]).splitlines()[0]}
    config = {"environment": environment_snapshot(), "tool_versions": versions, "engines": [
        engine("go", versions["go"], "go build -trimpath; default optimisation/runtime", [temp / "configbench.exe"], [temp / "configsample.exe"],
               [str(p.as_posix()) for p in Path("implementations/go").rglob("*.go") if not p.name.endswith("_test.go")]
               + ["implementations/go/go.mod"], [temp / "configbench.exe", temp / "configsample.exe"]),
        engine("python", versions["python"], "Interpreted, default flags", [args.python, "implementations/python/benchmark.py"],
               [args.python, "implementations/python/dotreg.py"], ["implementations/python/dotreg.py", "implementations/python/benchmark.py"], []),
        engine("javascript", "Node.js " + versions["node"] + "; TypeScript shares this engine", "Default V8 flags", [args.node, "implementations/javascript/benchmark.mjs"],
               [args.node, "implementations/javascript/configsample.mjs"], ["implementations/javascript/dotreg.mjs", "implementations/javascript/benchmark.mjs", "implementations/javascript/configsample.mjs"], []),
        engine("java", " / ".join(versions["java"].splitlines()), "javac -encoding UTF-8; default JVM/GC flags", [Path(args.jdk) / "bin/java.exe", "-cp", java, "DotRegBenchmark"],
               [Path(args.jdk) / "bin/java.exe", "-cp", java, "DotReg"], ["implementations/java/DotReg.java", "implementations/java/DotRegBenchmark.java"], list(java.glob("*.class"))),
        engine("csharp", ".NET SDK " + versions["dotnet_sdk"] + "; " + "; ".join(v for v in versions["dotnet_runtimes"] if v.startswith("Microsoft.NETCore.App")), "dotnet build -c Release --nologo; default .NET/GC flags",
               [args.dotnet, csharp_bench], [args.dotnet, csharp_cli], ["implementations/csharp/Parser.cs", "implementations/csharp/Program.cs", "implementations/csharp/DotReg.Sample.csproj",
               "implementations/csharp/benchmarks/Program.cs", "implementations/csharp/benchmarks/DotReg.Benchmark.csproj"], [csharp_bench, csharp_cli]),
        engine("rust", versions["rust"] + ", x86_64-pc-windows-gnu", "rustc --edition=2021 -C opt-level=3 -C link-self-contained=yes; GNU target linked with Rust toolchain's bundled MinGW GCC",
               [temp / "dotreg-rust.exe", "--benchmark"], [temp / "dotreg-rust.exe"], ["implementations/rust/src/lib.rs", "implementations/rust/src/main.rs", "implementations/rust/Cargo.toml"], [temp / "dotreg-rust.exe"]),
        engine("cpp", versions["cpp"] + "; x86_64-w64-windows-gnu, posix threads", "clang++ -std=c++17 -O2 -Wall -Wextra -pedantic -static",
               [temp / "dotreg-cpp.exe", "--benchmark"], [temp / "dotreg-cpp.exe"], ["implementations/cpp/dotreg.hpp", "implementations/cpp/main.cpp"], [temp / "dotreg-cpp.exe"])
    ]}
    filename = temp / "toolchains.json"
    filename.write_text(json.dumps(config, indent=2) + "\n", encoding="utf-8")
    print(filename)


if __name__ == "__main__":
    main()
