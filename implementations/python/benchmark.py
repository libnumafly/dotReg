"""Measure warmed parsing plus common decoding, without per-entry JSON."""

import argparse
import json
import os
import time

from dotreg import decode_string, parse, parse_each


def measure(source, mode):
    source.seek(0)
    entries = 0
    decoded_characters = 0

    def consume(entry):
        nonlocal entries, decoded_characters
        entries += 1
        if entry.kind == "value":
            decoded_characters += len(decode_string(entry))

    if mode == "stream":
        parse_each(source, consume)
    else:
        collected = parse(source)
        for entry in collected:
            consume(entry)
    return entries, decoded_characters


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("fixture")
    parser.add_argument("iterations", type=int)
    parser.add_argument("mode", choices=("stream", "collect"))
    args = parser.parse_args()
    if args.iterations < 1:
        parser.error("iterations must be positive")
    with open(args.fixture, "rb") as source:
        expected = measure(source, args.mode)
        assert measure(source, args.mode) == expected
        started = time.perf_counter()
        for _ in range(args.iterations):
            if measure(source, args.mode) != expected:
                raise RuntimeError("inconsistent benchmark result")
        elapsed = time.perf_counter() - started
    print(json.dumps({"language": "python", "mode": args.mode, "iterations": args.iterations,
                      "input_bytes": os.path.getsize(args.fixture), "entries": expected[0],
                      "elapsed_seconds": elapsed}, separators=(",", ":")))


if __name__ == "__main__":
    main()
