# Java sample parser

This independent, standard-library-only sample requires JDK 17 or later. It
implements the current [format syntax](../../docs/config-format.md), common
string decoding and an incremental visitor. Type expressions remain opaque;
this sample supplies no type profile or typed retrieval.

From the repository root, Java source-file mode runs the CLI without a build:

```console
java implementations/java/DotReg.java --mode strings examples/sample.txt
java implementations/java/DotReg.java --mode raw < settings.reg
```

For repeated runs, compile once to a separate output directory:

```console
javac -encoding UTF-8 -d /tmp/dotreg-java implementations/java/DotReg.java implementations/java/DotRegTest.java
java -ea -cp /tmp/dotreg-java DotRegTest
java -cp /tmp/dotreg-java DotReg --mode strings examples/sample.txt
```

Use a suitable temporary output path on Windows. The CLI reads UTF-8 bytes from
standard input when no filename is supplied. It emits one JSON array only after
the entire parse and requested common decoding succeed. `--mode strings` (the
default) adds `decoded` to each value; `--mode raw` retains undecoded right-hand
sides. Headings remain separate entries. Failure means a non-zero exit status,
empty standard output and a safe diagnostic on standard error. Diagnostics do
not include right-hand sides, filenames or underlying input error strings.

```java
try (var source = java.nio.file.Files.newInputStream(java.nio.file.Path.of("settings.reg"))) {
    DotReg.parseEach(source, entry -> {
        if (entry.kind().equals("value")) {
            String value = DotReg.decodeString(entry);
            // Consume this occurrence here. Return false to stop successfully.
        }
        return true;
    });
}
```

`parse(InputStream)` returns a list or throws without returning partial results.
`parseEach(InputStream, Visitor)` delivers complete entries in source order,
including headings and repeated identical values. A visitor exception passes
through unchanged before the next entry is parsed. Returning `false` stops
successfully. Visitor side effects already performed are the application's
responsibility. Each call starts with fresh scopes, rejects missing sources and
visitors and leaves its input stream open.

`Entry.path()` retains decoded element boundaries, empty names and dots within
names. Its list is detached from the persistent heading nodes; mutation cannot
change later resolution. `decodeString(entry)` returns the common decoded
string, with `entry.unset()` distinguishing unset from explicit empty values.
Annotations do not bypass common decoding.

The visitor retains an 8 KiB byte buffer, active heading suffixes, the current
physical line and, when necessary, a block body plus one boundary line. Lines
within the buffer borrow a byte slice until their next read; spanning lines use
an append buffer without an additional final byte-array copy. BOM and line-ending
removal adjust offsets rather than copying line bytes. Each parse reuses one
strict UTF-8 decoder. UTF-8 and LF/CRLF are validated per physical line. A non-body lookahead line is
validated only after the preceding visitor succeeds. Original internal LF and
CRLF are retained in blocks. Persistent parent nodes avoid storing every full
ancestor path at each depth; materialising a delivered path still costs its
number of elements. `StringBuilder` assembles block text and decoded escapes.

The format imposes no fixed size or depth limit. A long individual line or block
needs memory proportional to its content; total use remains subject to the JVM's
heap and array limits. Use the visitor to avoid retaining all occurrences. The
collecting API and JSON CLI deliberately retain the complete result. Compiled
execution avoids source-mode compilation overhead when measuring repeated work.

The optional native benchmark harness measures parsing plus common decoding:

```console
javac -encoding UTF-8 -d /tmp/dotreg-java implementations/java/DotReg.java implementations/java/DotRegBenchmark.java
java -cp /tmp/dotreg-java DotRegBenchmark fixture.reg 5 stream
java -cp /tmp/dotreg-java DotRegBenchmark fixture.reg 5 collect
```

It reuses one file stream and seeks its channel back to zero between passes. Two
warm-up passes precede timing. The reported seconds cover the requested number
of passes; `entries` is the count in one pass. A volatile sink consumes decoded
lengths and counts are compared across passes. No per-entry JSON is timed. Record
JVM version and flags alongside externally sampled process memory; retained JVM
heap, garbage collection and runtime baseline affect that measurement.
