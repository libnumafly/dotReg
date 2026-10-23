# Python sample parser

This independent, standard-library-only sample requires Python 3.10 or later.
It implements the current [format syntax](../../docs/config-format.md), common
string decoding and an incremental visitor. Type expressions are retained as
opaque annotations; this sample supplies no type profile or typed retrieval.

From the repository root:

```console
python implementations/python/dotreg.py --mode strings examples/sample.txt
python implementations/python/dotreg.py --mode raw < settings.reg
python -m unittest discover -s implementations/python -v
```

Without a filename, the CLI reads UTF-8 bytes from standard input. It emits one JSON array only after
the whole file succeeds. `--mode strings` (the default) adds `decoded` to values;
`--mode raw` retains undecoded right-hand sides, including quoted values whose
escapes cannot be decoded. Headings remain separate entries. A failure has a
non-zero exit status, empty standard output and a diagnostic on standard error.
Diagnostics exclude right-hand sides, filenames and underlying input error text.

```python
import sys
sys.path.insert(0, "implementations/python")
from dotreg import parse_each, decode_string

with open("settings.reg", "rb") as source:
    def visit(entry):
        if entry.kind == "value":
            value = decode_string(entry)
            # Consume this occurrence here. Return False to stop successfully.
        return True
    parse_each(source, visit)
```

`parse(source)` returns a list or raises without returning partial results.
`parse_each(source, visitor)` delivers complete entries in source order, including
headings and repeated identical values. A visitor exception is raised unchanged
before the next entry is parsed. Returning exactly `False` stops successfully;
`None`, as returned by `list.append`, continues. Already performed visitor side
effects remain the application's responsibility. Each call starts with fresh
scope state and leaves its binary source open. Missing sources and visitors fail.

Paths are lists of decoded elements, preserving dots within names and empty
names. Visitor paths are detached from internal heading nodes, so mutating one
cannot change later resolution. `decode_string(entry)` returns a string while
`entry.unset` distinguishes an unset value from an explicit empty string. The
function never interprets an annotation as permission to skip common decoding.

The visitor keeps an 8 KiB byte buffer, active heading suffixes, the current
physical line and, when needed, a block body plus one boundary line. UTF-8 and
line endings are validated per physical line, with boundary lookahead validation
deferred until the preceding visitor succeeds. Source LF and CRLF inside blocks
are retained. Long lines and blocks grow with their content; there is no format
size limit. Persistent parent nodes avoid retaining a full ancestor path at every
heading depth. Each delivered resolved path still costs its number of elements.

Use the visitor to process large files without collecting all occurrences. The
collecting API and JSON CLI intentionally retain the entire result. This design
reduces avoidable allocations and retention. A single large entry still needs
memory proportional to its content. The source must implement `read(size)` returning `bytes`;
text streams are rejected to avoid newline translation and implicit encoding.

The benchmark harness times parsing plus common decoding through either API:

```console
python implementations/python/benchmark.py fixture.reg 5 stream
python implementations/python/benchmark.py fixture.reg 5 collect
```

It seeks the same binary file back to zero for every pass, warms up twice, then
reports elapsed seconds for the requested number of passes. `entries` is the
count in one pass. Decoded string lengths are consumed and compared across passes.
No per-entry JSON is included in the timed work. Process memory should be measured
externally with the sampling method and interpreter version recorded separately.
