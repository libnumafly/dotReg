"""Measure a native harness in a fresh process; standard library only.

Usage: python measure.py --command '["exe", "benchmark", "fixture", "10", "stream"]'
Linux /proc and Windows GetProcessMemoryInfo report process high-water marks.
Polling includes runtime, warm-up, buffers and output; it is not live heap size.
"""
import argparse
import ctypes
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import tempfile
import time


def sampler(pid):
    if os.name == "nt":
        from ctypes import wintypes

        class Counters(ctypes.Structure):
            _fields_ = [("cb", wintypes.DWORD), ("PageFaultCount", wintypes.DWORD)] + [
                (name, ctypes.c_size_t) for name in (
                    "PeakWorkingSetSize", "WorkingSetSize", "QuotaPeakPagedPoolUsage",
                    "QuotaPagedPoolUsage", "QuotaPeakNonPagedPoolUsage", "QuotaNonPagedPoolUsage",
                    "PagefileUsage", "PeakPagefileUsage",
                )
            ]
        kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        psapi = ctypes.WinDLL("psapi", use_last_error=True)
        kernel.OpenProcess.argtypes = [wintypes.DWORD, wintypes.BOOL, wintypes.DWORD]
        kernel.OpenProcess.restype = wintypes.HANDLE
        kernel.CloseHandle.argtypes = [wintypes.HANDLE]
        psapi.GetProcessMemoryInfo.argtypes = [wintypes.HANDLE, ctypes.POINTER(Counters), wintypes.DWORD]
        handle = kernel.OpenProcess(0x410, False, pid)
        if not handle:
            raise RuntimeError("cannot open process memory counters")

        def read():
            counters = Counters()
            counters.cb = ctypes.sizeof(counters)
            return counters.PeakWorkingSetSize if psapi.GetProcessMemoryInfo(handle, ctypes.byref(counters), counters.cb) else 0
        return read, lambda: kernel.CloseHandle(handle), "Windows PeakWorkingSetSize, sampled every 5 ms"
    if platform.system() == "Linux":
        def read():
            try:
                for line in Path(f"/proc/{pid}/status").read_text().splitlines():
                    if line.startswith("VmHWM:"):
                        return int(line.split()[1]) * 1024
            except (OSError, ValueError):
                pass
            return 0
        return read, lambda: None, "Linux /proc VmHWM, sampled every 5 ms"
    return lambda: 0, lambda: None, "unavailable on this platform"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--command", required=True, help="JSON argv including fixture, iterations and mode")
    args = parser.parse_args()
    command = json.loads(args.command)
    if not isinstance(command, list) or not command or any(not isinstance(x, str) for x in command):
        parser.error("--command must be a nonempty JSON string array")
    fixture = Path(command[-3])
    # Temporary files prevent output pipe deadlocks and exclude output from RSS sampling.
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        started = time.perf_counter()
        child = subprocess.Popen(command, stdout=out, stderr=err, stdin=subprocess.DEVNULL)
        peak = samples = 0
        try:
            read, close, method = sampler(child.pid)
            try:
                while child.poll() is None:
                    peak = max(peak, read())
                    samples += 1
                    if time.perf_counter() - started > 300:
                        raise TimeoutError("benchmark exceeded five minutes")
                    time.sleep(0.005)
                peak = max(peak, read())
            finally:
                close()
        finally:
            if child.poll() is None:
                child.kill()
                child.wait()
        wall = time.perf_counter() - started
        if child.returncode:
            raise RuntimeError("benchmark process failed")
        out.seek(0)
        result = json.load(out)
    result.update({
        "fixture_sha256": hashlib.sha256(fixture.read_bytes()).hexdigest(),
        "platform": platform.platform(), "machine": platform.machine(),
        "observed_peak_rss_bytes": peak or None, "rss_method": method,
        "rss_samples": samples, "process_wall_seconds": wall,
    })
    print(json.dumps(result, ensure_ascii=True))


if __name__ == "__main__":
    main()
