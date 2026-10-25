using System.Text;
using DotReg;

var tests = new (string Name, Action Run)[]
{
    ("declarations and physical positions", () =>
    {
        var entries = Read("\ufeff@dotReg\t0.1 \t\r\n \t\r\n# comment\nA:001\r\n", false);
        Equal(1, entries.Count);
        Equal(4, entries[0].Line);
        Equal("001", Parser.DecodeString(entries[0]).Value);
    }),
    ("empty declared body", () => Equal(0, Read("").Count)),
    ("declaration failures", () =>
    {
        foreach (var source in new[] { "", "@dotReg 0.1\n", "@dotReg 0.1", "\n@dotReg 0.1\n\n",
            "# secret\n@dotReg 0.1\n\n", " @dotReg 0.1\n\n", "@Unknown 1\n\n", "@dotReg 2\n\n",
            "@dotReg 0.1\n@dotReg 0.1\n\n", "@dotReg\n\n" })
            Fails(() => Read(source, false));
    }),
    ("quoted paths Unicode and empty names", () =>
    {
        var entries = Read("A.\"B\\.C\".\"\": value\n\"\\uD83D\\uDE00\".\"\\u{1f600}\": hi\n");
        Sequence(new[] { "A", "B.C", "" }, entries[0].Path);
        Sequence(new[] { "😀", "😀" }, entries[1].Path);
    }),
    ("hierarchy counts headings rather than path elements", () =>
    {
        var entries = Read("A.B.\n  .C.D.\n    .E: child\n  .: parent\n  .F: sibling\nZ: root\n");
        Sequence(new[] { "A", "B", "C", "D", "E" }, entries[2].Path);
        Sequence(new[] { "A", "B" }, entries[3].Path);
        Sequence(new[] { "A", "B", "F" }, entries[4].Path);
    }),
    ("indentation failures", () =>
    {
        foreach (var body in new[] { " .A:x", "\tA:x", "  A:x", ".A:x", "A.\n    .B:x",
            "A.\n  .B.\n  .C:x\n    .D:x", "A.\nB:x\n  .C:x" }) Fails(() => Read(body));
    }),
    ("key failures", () =>
    {
        foreach (var key in new[] { "A..B", "A.", "", "A B", "A\\B", "A#B", "A\u0085B",
            "\"A\"B", "A.\"\\q\"", "\"\\uD800\"", "\"\\u{110000}\"" })
            Fails(() => Read(key + ": hidden"));
    }),
    ("opaque annotations", () =>
    {
        var entry = Read("A (custom(nested:kind)) : 001")[0];
        Equal("custom(nested:kind)", entry.Type);
        Equal(true, entry.HasType);
        Equal("001", Parser.DecodeString(entry).Value);
        foreach (var body in new[] { "A():v", "A(t x):v", "A(t):v\nB(t x):v", "A\t(t):v", "A(t)\t:v",
            "A(\"t\"):v", "A(\\t):v", "A(t:v", "A(t)tail:v", "A(t)." }) Fails(() => Read(body));
    }),
    ("repeated occurrence and parent coexistence", () =>
    {
        var entries = Read("A:root\nA.B:one\nA.\"B\":one\nA.B:two");
        Equal(4, entries.Count);
        Equal("one", entries[1].Raw);
        Equal("one", entries[2].Raw);
        Equal("two", entries[3].Raw);
    }),
    ("one-time common decoding", () =>
    {
        var entries = Read("A: \"\\.\\\"\\\\\\/\\b\\f\\n\\r\\t\\u0041\\u{1F600}\"\nB: ab\"\\q\nC: \"\\\\u0041\"\nD:\nE: \"\"");
        Equal(".\"\\/\b\f\n\r\tA😀", Parser.DecodeString(entries[0]).Value);
        Equal("ab\"\\q", Parser.DecodeString(entries[1]).Value);
        Equal("\\u0041", Parser.DecodeString(entries[2]).Value);
        Equal(new Text("", true), Parser.DecodeString(entries[3]));
        Equal(new Text("", false), Parser.DecodeString(entries[4]));
    }),
    ("raw syntax can retain undecodable quoted values", () =>
    {
        foreach (var raw in new[] { "\"open", "\"\\q\"", "\"x\"tail", "\"a\" \"b\"", "\"\\uD800\"",
            "\"\\uDC00\"", "\"\\u{d800}\"", "\"\\u{}\"", "\"\\u{0000000}\"", "\"\\uZZZZ\"" })
        {
            var entries = Read("A:" + raw);
            Equal(raw, entries[0].Raw);
            Fails(() => Parser.DecodeString(entries[0]));
        }
    }),
    ("mixed block line endings preserved", () =>
    {
        var entry = Read("A: \"\"\"\r\n  first\\nnext\r\n\n   third\n  \"\"\"\r\n")[0];
        Equal("triple_quoted", entry.Form);
        Equal("first\\nnext\r\n\n third", entry.Raw);
        Equal("first\nnext\r\n\n third", Parser.DecodeString(entry).Value);
    }),
    ("triple odd margin and empty body", () =>
    {
        var entries = Read("A:\"\"\"\n   text\n   \"\"\"\nB:\"\"\"\n\"\"\"");
        Equal("text", entries[0].Raw);
        Equal("", entries[1].Raw);
        Equal(false, entries[1].Unset);
        Fails(() => Read("A:\"\"\"\n x\n  \"\"\""));
        Fails(() => Read("A:\"\"\"\ntext"));
        Fails(() => Read("A:\"\"\"\n \n  \"\"\""));
    }),
    ("literal pipes preserve content and scope", () =>
    {
        var entries = Read("A.\n  .B:\n   |C:\\temp\\new\r\n     |\"\"\" \\q\n   |\n  .C:x");
        Equal("pipe", entries[1].Form);
        Equal("C:\\temp\\new\r\n\"\"\" \\q\n", Parser.DecodeString(entries[1]).Value);
        Sequence(new[] { "A", "C" }, entries[2].Path);
    }),
    ("pipes inspect only the immediate next line", () =>
    {
        Equal(true, Read("A:\n\n|: ordinary")[0].Unset);
        Equal(true, Read("A:\n# end\n|: ordinary")[0].Unset);
        Equal("|", Parser.DecodeString(Read("A:|")[0]).Value);
        Equal("", Parser.DecodeString(Read("A:\n | ")[0]).Value.Trim());
    }),
    ("physical UTF8 validation includes comments and blocks", () =>
    {
        foreach (var invalid in new byte[][] { [0xff], [0xc0, 0x80], [0xed, 0xa0, 0x80], [0xf4, 0x90, 0x80, 0x80], [0xe2, 0x82] })
            foreach (var prefix in new[] { "A:", "#", "A:\n |", "A:\"\"\"\n" })
                Fails(() => Parser.Parse(Bytes(Encoding.UTF8.GetBytes("@dotReg 0.1\n\n" + prefix).Concat(invalid).ToArray())));
        Fails(() => Read("# comment\rhidden"));
        Fails(() => Read("A:\n |hidden\rdata"));
    }),
    ("recipient early stop has exception identity", () =>
    {
        var sentinel = new InvalidOperationException("recipient error");
        var calls = 0;
        try
        {
            Parser.ParseEach(Source("A:value\ninvalid later"), _ => { calls++; throw sentinel; });
            throw new Exception("did not stop");
        }
        catch (InvalidOperationException error) { Equal(true, ReferenceEquals(sentinel, error)); }
        Equal(1, calls);
    }),
    ("invalid non-pipe lookahead waits for recipient", () =>
    {
        foreach (var prefix in new[] { "A:\n", "A:\n |text\n" })
        {
            var source = Encoding.UTF8.GetBytes("@dotReg 0.1\n\n" + prefix).Concat(new byte[] { 0xff }).ToArray();
            var sentinel = new InvalidOperationException("stop before invalid lookahead");
            try { Parser.ParseEach(Bytes(source), _ => throw sentinel); throw new Exception("did not stop"); }
            catch (InvalidOperationException error) { Equal(true, ReferenceEquals(sentinel, error)); }
        }
    }),
    ("invalid qualifying pipe body fails before notification", () =>
    {
        foreach (var prefix in new[] { "A:\n |", "A:\n |valid\n |" })
        {
            var source = Encoding.UTF8.GetBytes("@dotReg 0.1\n\n" + prefix).Concat(new byte[] { 0xff }).ToArray();
            var calls = 0;
            Fails(() => Parser.ParseEach(Bytes(source), _ => { calls++; throw new InvalidOperationException("recipient must not run"); }));
            Equal(0, calls);
        }
        var ioCalls = 0;
        Fails(() => Parser.ParseEach(new ErrorStream("@dotReg 0.1\n\nA:\n |partial"), _ => { ioCalls++; }));
        Equal(0, ioCalls);
    }),
    ("heading array mutation does not mutate scope", () =>
    {
        string[]? original = null;
        Parser.ParseEach(Source("A.B.\n  .C:x"), entry =>
        {
            if (entry.Kind == "heading") { original = entry.Path; entry.Path[0] = "changed"; entry.Path = ["replaced"]; }
            else { Sequence(new[] { "A", "B", "C" }, entry.Path); Equal(false, ReferenceEquals(original, entry.Path)); }
        });
    }),
    ("value paths own relative suffixes and self paths", () =>
    {
        var paths = new List<string[]>();
        var exported = new List<string[]>();
        Parser.ParseEach(Source("A.B.\n  .C.D:first\n  .:self\n  .\"\":empty\n  .E:next\nZ.Y:root"), entry =>
        {
            paths.Add(entry.Path.ToArray());
            foreach (var previous in exported) Equal(false, ReferenceEquals(previous, entry.Path));
            exported.Add(entry.Path);
            entry.Path[0] = "changed";
            entry.Path = [];
        });
        var expected = new[] { new[] { "A", "B" }, new[] { "A", "B", "C", "D" },
            new[] { "A", "B" }, new[] { "A", "B", "" }, new[] { "A", "B", "E" }, new[] { "Z", "Y" } };
        Equal(expected.Length, paths.Count);
        for (var i = 0; i < expected.Length; i++) Sequence(expected[i], paths[i]);
    }),
    ("short reads and source ownership", () =>
    {
        using var source = new ShortStream(Encoding.UTF8.GetBytes("@dotReg 0.1\r\n\r\n\"店舗😀\":value"));
        var entries = Parser.Parse(source);
        Equal("店舗😀", entries[0].Path[0]);
        Equal(true, source.CanRead);
    }),
    ("I/O errors are safe and collection returns no partial result", () =>
    {
        List<Entry>? result = null;
        try { result = Parser.Parse(new ErrorStream("@dotReg 0.1\n\nA:value\n")); throw new Exception("did not fail"); }
        catch (ParseException error) { Equal(false, error.Message.Contains("secret")); Equal(4, error.Line); }
        Equal<List<Entry>?>(null, result);
    }),
    ("I/O lookahead defers failure until after notification", () =>
    {
        var calls = 0;
        var sentinel = new InvalidOperationException("recipient");
        try { Parser.ParseEach(new ErrorStream("@dotReg 0.1\n\nA:\n"), _ => { calls++; throw sentinel; }); }
        catch (InvalidOperationException error) { Equal(true, ReferenceEquals(sentinel, error)); }
        Equal(1, calls);
    }),
    ("fresh read scopes and required arguments", () =>
    {
        Read("A.\n  .B:x");
        Fails(() => Read("  .C:x"));
        Fails(() => Parser.ParseEach(null!, _ => { }));
        Fails(() => Parser.ParseEach(Source("A:x"), null!));
    }),
    ("unbounded line and non-recursive deep headings", () =>
    {
        var large = new string('x', 200000);
        Equal(large, Read("A:" + large)[0].Raw);
        var body = new StringBuilder("Root.\n");
        for (var depth = 1; depth <= 700; depth++) body.Append(' ', depth * 2).Append(".Child.\n");
        body.Append(' ', 1402).Append(".:last");
        var count = 0;
        Parser.ParseEach(Source(body.ToString()), entry =>
        {
            count++;
            if (entry.Kind == "value") Equal(701, entry.Path.Length);
        });
        Equal(702, count);
    }),
};

foreach (var test in tests)
{
    try { test.Run(); }
    catch (Exception error) { Console.Error.WriteLine($"FAIL {test.Name}: {error}"); return 1; }
}
Console.WriteLine($"C#: {tests.Length} native parser tests passed");
return 0;

static List<Entry> Read(string body, bool declarations = true) => Parser.Parse(declarations ? Source(body) : Bytes(Encoding.UTF8.GetBytes(body)));
static MemoryStream Source(string body) => Bytes(Encoding.UTF8.GetBytes("@dotReg 0.1\n\n" + body));
static MemoryStream Bytes(byte[] bytes) => new(bytes);
static void Equal<T>(T expected, T actual)
{
    if (!EqualityComparer<T>.Default.Equals(expected, actual)) throw new Exception($"expected {expected}, got {actual}");
}
static void Sequence(string[] expected, string[] actual)
{
    if (!expected.SequenceEqual(actual)) throw new Exception("path elements differ");
}
static void Fails(Action action)
{
    try { action(); }
    catch (ParseException) { return; }
    throw new Exception("expected a safe ParseException");
}

sealed class ShortStream(byte[] bytes) : MemoryStream(bytes)
{
    public override int Read(byte[] buffer, int offset, int count) => base.Read(buffer, offset, Math.Min(count, 1));
}

sealed class ErrorStream(string prefix) : Stream
{
    private readonly byte[] data = Encoding.UTF8.GetBytes(prefix);
    private int position;
    public override int Read(byte[] buffer, int offset, int count)
    {
        if (position == data.Length) throw new IOException("secret input system detail");
        var take = Math.Min(count, data.Length - position);
        data.AsSpan(position, take).CopyTo(buffer.AsSpan(offset));
        position += take;
        return take;
    }
    public override bool CanRead => true;
    public override bool CanSeek => false;
    public override bool CanWrite => false;
    public override long Length => throw new NotSupportedException();
    public override long Position { get => position; set => throw new NotSupportedException(); }
    public override void Flush() => throw new NotSupportedException();
    public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();
    public override void SetLength(long value) => throw new NotSupportedException();
    public override void Write(byte[] buffer, int offset, int count) => throw new NotSupportedException();
}
