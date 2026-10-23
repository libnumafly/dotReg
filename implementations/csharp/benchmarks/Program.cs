using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text.Json;
using DotReg;

if (args.Length != 3 || !int.TryParse(args[1], out var iterations) || iterations < 1
    || args[2] is not ("stream" or "collect"))
{
    Console.Error.WriteLine("usage: DotReg.Benchmark file iterations stream|collect");
    return 2;
}
var mode = args[2];
try
{
    using var source = File.OpenRead(args[0]);
    // Reuse the FileStream, not a complete in-memory copy of the fixture.
    // Length/seek, filesystem reads and common decoding are included per pass.
    (long Entries, long Characters) Visit()
    {
        source.Position = 0;
        long entries = 0, characters = 0;
        Parser.ParseEach(source, entry =>
        {
            entries++;
            if (entry.Kind == "value") characters += Parser.DecodeString(entry).Value.Length;
        });
        return (entries, characters);
    }
    (long Entries, long Characters) Collect()
    {
        source.Position = 0;
        var entries = Parser.Parse(source);
        long characters = 0;
        foreach (var entry in entries)
            if (entry.Kind == "value") characters += Parser.DecodeString(entry).Value.Length;
        return (entries.Count, characters);
    }
    // A separate process runs each mode so an external RSS sampler can compare
    // streaming and collection without retaining the other mode's high-water mark.
    Func<(long Entries, long Characters)> action = mode == "stream" ? Visit : Collect;
    var expected = action();
    if (action() != expected) throw new InvalidOperationException();
    var gen0 = GC.CollectionCount(0);
    var gen1 = GC.CollectionCount(1);
    var gen2 = GC.CollectionCount(2);
    var before = GC.GetAllocatedBytesForCurrentThread();
    var start = Stopwatch.GetTimestamp();
    for (var i = 0; i < iterations; i++)
        if (action() != expected) throw new InvalidOperationException();
    var elapsedTicks = Stopwatch.GetTimestamp() - start;
    var allocated = GC.GetAllocatedBytesForCurrentThread() - before;
    Console.WriteLine(JsonSerializer.Serialize(new
    {
        language = "csharp",
        mode,
        runtime = RuntimeInformation.FrameworkDescription,
        iterations,
        warmup_iterations = 2,
        input_bytes = source.Length,
        entries = expected.Entries,
        decoded_utf16_code_units = expected.Characters,
        elapsed_seconds = (double)elapsedTicks / Stopwatch.Frequency,
        allocated_bytes = allocated,
        allocated_bytes_per_iteration = (double)allocated / iterations,
        collections = new[] { GC.CollectionCount(0) - gen0, GC.CollectionCount(1) - gen1, GC.CollectionCount(2) - gen2 },
        memory_method = "GC.GetAllocatedBytesForCurrentThread: allocation volume, not retained heap or process peak memory",
        timing_method = "Stopwatch; reused FileStream and common decoding included; fixture opening and JSON excluded"
    }));
    return 0;
}
catch (ParseException error) { Console.Error.WriteLine(error.Message); return 1; }
catch (Exception) { Console.Error.WriteLine("dotReg benchmark: input or measurement failure"); return 1; }
