"""Generate the native workload matrix and independent expected syntax results.

All files are valid current-spec UTF-8/LF dotReg, including the declaration and
blank separator. No parser imports are used to construct the expected results.
"""
import argparse
import hashlib
import json
from pathlib import Path


class Fixture:
    def __init__(self, name, iterations, description):
        self.name, self.iterations, self.description = name, iterations, description
        self.lines, self.expected = ["@dotReg 1", ""], []

    def entry(self, source, path, *, kind="value", raw="", decoded="", unset=False,
              annotation=None, form=None):
        item = {"line": len(self.lines) + 1, "kind": kind, "path": path,
                "raw": raw, "unset": unset, "has_type": annotation is not None}
        if annotation is not None:
            item["type"] = annotation
        if form:
            item["form"] = form
        if kind == "value":
            item["decoded"] = decoded
        self.expected.append(item)
        self.lines.extend(source)

    @property
    def data(self):
        return ("\n".join(self.lines) + "\n").encode("utf-8")


def mixed(count, iterations):
    result = Fixture(f"mixed-{count}", iterations,
                     f"{count} occurrences, repeated keys, opaque annotations, Unicode, unset, pipe/triple blocks")
    for i in range(count):
        group = [f"Group{i // 256}"]
        if i % 256 == 0:
            result.entry([group[0] + "."], group, kind="heading")
        choice = i % 8
        if choice == 0:
            result.entry(["  .Tag:001"], group + ["Tag"], raw="001", decoded="001")
        elif choice == 1:
            raw = '"hello\\tworld \\u{1F600}"'
            result.entry(["  .Tag(string):" + raw], group + ["Tag"], raw=raw,
                         decoded="hello\tworld \U0001f600", annotation="string")
        elif choice == 2:
            raw = r"C:\temp\new"
            result.entry(["  .Path:" + raw], group + ["Path"], raw=raw, decoded=raw)
        elif choice == 3:
            result.entry(['  ."名称.B":日本語'], group + ["名称.B"], raw="日本語", decoded="日本語")
        elif choice == 4:
            result.entry(["  .Unset:"], group + ["Unset"], unset=True)
        elif choice == 5:
            raw = ' literal \\q "quotes"\n|second line'
            result.entry(["  .Literal:", '   | literal \\q "quotes"', "   ||second line"],
                         group + ["Literal"], raw=raw, decoded=raw, form="pipe")
        elif choice == 6:
            raw = "first\\nline\n  second"
            result.entry(['  .Message:"""', "   first\\nline", "     second", '   """'],
                         group + ["Message"], raw=raw, decoded="first\nline\n  second", form="triple_quoted")
        else:
            raw = "x" * 256
            result.entry(["  .Long:" + raw], group + ["Long"], raw=raw, decoded=raw)
    return result


def workloads():
    cases = [mixed(32, 1000), mixed(512, 50), mixed(20000, 3)]
    large = Fixture("line-1mib", 3, "One literal value with a 1,048,576-byte payload on one physical line")
    value = "x" * (1024 * 1024)
    large.entry(["Long:" + value], ["Long"], raw=value, decoded=value)
    cases.append(large)
    blocks = Fixture("blocks-32kib", 20, "One pipe and one triple-quoted block, each with 32,768 ASCII x bytes plus 31 LF separators (32,799 decoded bytes)")
    value = "\n".join(["x" * 1024] * 32)
    blocks.entry(["Pipe:"] + [" |" + "x" * 1024] * 32, ["Pipe"], raw=value, decoded=value, form="pipe")
    blocks.entry(['Triple:"""'] + [" " + "x" * 1024] * 32 + [' """'], ["Triple"],
                 raw=value, decoded=value, form="triple_quoted")
    cases.append(blocks)
    deep = Fixture("depth-256", 10, "256 written heading levels and one value per level; two-space structural indentation")
    for depth in range(256):
        path = [f"H{i}" for i in range(depth + 1)]
        deep.entry(["  " * depth + ("." if depth else "") + f"H{depth}."], path, kind="heading")
        deep.entry(["  " * (depth + 1) + ".Value:001"], path + ["Value"], raw="001", decoded="001")
    cases.append(deep)
    return cases


def canonical(value):
    return json.dumps(value, ensure_ascii=True, separators=(",", ":"), sort_keys=True).encode("utf-8")


def generate(directory):
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    manifest = []
    for fixture in workloads():
        filename = fixture.name + ".reg"
        (directory / filename).write_bytes(fixture.data)
        manifest.append({"name": fixture.name, "filename": filename,
                         "description": fixture.description, "iterations": fixture.iterations,
                         "bytes": len(fixture.data), "entries": len(fixture.expected),
                         "value_occurrences": sum(e["kind"] == "value" for e in fixture.expected),
                         "sha256": hashlib.sha256(fixture.data).hexdigest(),
                         "expected_entries_sha256": hashlib.sha256(canonical(fixture.expected)).hexdigest()})
    (directory / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    return manifest


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory")
    args = parser.parse_args()
    print(json.dumps(generate(args.directory), indent=2))
