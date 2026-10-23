# C# sample parser

This independent, standard-library-only .NET 8 implementation reads dotReg version
`1`. It resolves headings and quoted key elements, retains every occurrence in
source order, and preserves each multiline body's physical LF or CRLF endings.
Type annotations remain opaque text: this sample performs common string decoding
and does not introduce a type profile or implicit conversions.

The format specification is [../../docs/config-format.md](../../docs/config-format.md).
These public types and JSON fields are sample API choices rather than a standard
retrieval API.

Build once, then invoke the compiled programme to avoid a build on every read:

```sh
dotnet build implementations/csharp/DotReg.Sample.csproj -c Release
dotnet implementations/csharp/bin/Release/net8.0/DotReg.Sample.dll settings.reg
dotnet implementations/csharp/bin/Release/net8.0/DotReg.Sample.dll --mode raw settings.reg
dotnet implementations/csharp/bin/Release/net8.0/DotReg.Sample.dll --mode strings < settings.reg
```

An omitted filename, or `-`, reads UTF-8 bytes from standard input. The default
`strings` mode adds `decoded` to each value; `raw` returns syntax results without
common value decoding. Both modes return a single JSON array. On failure the
programme exits non-zero, leaves stdout empty for input and decoding failures,
and prints a diagnostic which excludes values, filenames and underlying I/O
exception strings. A late stdout-device failure can of course leave partial
output at that device.

## Native API

```csharp
using DotReg;

using var source = File.OpenRead("settings.reg");
Parser.ParseEach(source, entry =>
{
    if (entry.Kind == "value")
    {
        Text text = Parser.DecodeString(entry);
        // text.Value is a string; text.Unset retains the unset distinction.
    }
});
```

`ParseEach(Stream, Action<Entry>)` visits each complete heading or value before
parsing the next entry. It propagates a recipient's exception as the same
exception object. A possible pipe boundary can require one unvalidated physical
line of lookahead, and the input buffer may read bytes ahead. An invalid
non-pipe lookahead line is validated only after the preceding recipient returns.
Multiline values are collected before notification. Recipient side effects are
not rolled back. The stream stays open; a null stream or recipient is rejected.

`Parse(Stream)` returns all entries only on success and throws on failure without
returning partial results. `DecodeString(Entry)` applies ordinary quoted and
triple-quoted escapes, keeps pipe bodies literal, and reports invalid strings.
Raw syntax parsing intentionally retains an undecodable quoted value until this
decoding step. `Entry.Path` is a separate array on each notification, so mutating
it cannot alter subsequent heading resolution. Compare complete element sequences
to identify keys; joining elements with dots loses their boundaries.

## Memory and work

The parser scans the input once using a 32 KiB byte buffer. It validates UTF-8
strictly and does not normalise Unicode or line endings. Persistent parent/suffix
nodes share heading prefixes rather than retaining a full expanded path at every
scope. Path expansion is iterative, so deep headings do not consume call-stack
depth. Parsing and common decoding do not use regular expressions, an object
tree, a repeated-key index, or third-party packages.

Streaming retention consists of active heading suffixes, the current physical
line or multiline value, one possible lookahead line, the input buffer and the
largest line scratch buffer seen. Each recipient receives a fresh expanded path;
creating that path costs work proportional to its element count, as required by
this API. Triple-quoted margins are known only at the closing line, so that
block's source lines remain until its closing marker. Collection and the CLI also
retain all results, and CLI serialisation temporarily retains its complete JSON
output to prevent input failures from producing partial JSON.

The sample imposes no fixed format size or depth cap; available process memory
and .NET's string/array limits still bound an individual parse. These structural
choices reduce repeated work and retention. They do not establish a measured
speed, memory, or electrical-energy advantage over another implementation.

## Validation

The native tests require no test framework or NuGet packages:

```sh
dotnet run --project implementations/csharp/tests/DotReg.Tests.csproj -c Release
```

They cover declarations, key identity, repeated occurrences, indentation, opaque
annotations, common Unicode decoding, raw decoding failures, multiline margins
and mixed endings, strict source encoding, short reads, recipient exception
identity, deferred lookahead errors, mutation isolation, safe I/O diagnostics,
fresh parse state, long lines and deep headings. The repository's shared
conformance runner additionally compares the CLI with the other samples.

## Native benchmark

The separate benchmark executable times the streaming API plus common decoding
and the collecting API plus common decoding against the same supplied file:

```sh
dotnet build implementations/csharp/benchmarks/DotReg.Benchmark.csproj -c Release
dotnet implementations/csharp/benchmarks/bin/Release/net8.0/DotReg.Benchmark.dll fixture.reg 20 stream
dotnet implementations/csharp/benchmarks/bin/Release/net8.0/DotReg.Benchmark.dll fixture.reg 20 collect
```

Each invocation measures one API mode, warms that path twice, reuses a `FileStream`, verifies occurrence counts
and decoded UTF-16 code-unit totals, and emits JSON. Timed passes include parsing,
common decoding, seeking and reads from the already warmed filesystem cache;
fixture opening and JSON serialisation are excluded. `Stopwatch` supplies elapsed
time. `GC.GetAllocatedBytesForCurrentThread` reports managed allocation volume,
including temporary objects, and collection counts are reported by generation.
That allocation volume is neither retained heap size nor peak process memory.
Running each mode in a separate process lets an external process-memory sampler
compare their peaks independently. The process is synchronous, so allocations from the parser and decoder occur on
the measured thread. Compare runs with the same runtime, fixture, iteration
count and machine; this benchmark does not measure electrical energy.
