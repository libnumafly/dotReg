using System.Buffers;
using System.Text;
using System.Text.Json.Serialization;

namespace DotReg;

/// <summary>A source occurrence; Path retains decoded element boundaries.</summary>
public sealed class Entry
{
    [JsonPropertyName("line")] public int Line { get; init; }
    [JsonPropertyName("kind")] public string Kind { get; init; } = "value";
    [JsonPropertyName("path")] public string[] Path { get; set; } = [];
    [JsonPropertyName("raw")] public string Raw { get; set; } = "";
    [JsonPropertyName("unset")] public bool Unset { get; set; }
    [JsonPropertyName("type"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? Type { get; init; }
    [JsonPropertyName("has_type")] public bool HasType { get; init; }
    [JsonPropertyName("form"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? Form { get; set; }
    [JsonPropertyName("decoded"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? Decoded { get; set; }
}

public readonly record struct Text(string Value, bool Unset);

/// <summary>A safe diagnostic which never incorporates input or an I/O error string.</summary>
public sealed class ParseException : Exception
{
    public int Line { get; }
    internal ParseException(int line, string cause)
        : base(line > 0 ? $"dotReg line {line}: {cause}" : $"dotReg: {cause}") => Line = line;
}

public static class Parser
{
    private static readonly UTF8Encoding Utf8 = new(false, true);

    /// <summary>Collect all entries, or throw without returning partial results.</summary>
    public static List<Entry> Parse(Stream source)
    {
        var entries = new List<Entry>();
        ParseEach(source, entries.Add);
        return entries;
    }

    /// <summary>
    /// Visit complete occurrences in order. A recipient exception propagates unchanged
    /// before the following entry is validated or parsed. The source remains open.
    /// </summary>
    public static void ParseEach(Stream source, Action<Entry> visit)
    {
        if (source is null || visit is null) throw Problem(0, "input required");
        var lines = new SourceLines(source);
        if (lines.Properties()) return;
        var scopes = new List<PathNode>();
        while (true)
        {
            var physical = lines.Next();
            var text = physical.Validate();
            var eof = physical.Eof;
            var trimmed = Trim(text);
            if (trimmed.Length != 0 && trimmed[0] != '#')
            {
                var indent = LeadingSpaces(text);
                var body = text[indent..].TrimEnd(' ', '\t');
                if (body[0] == '\t' || indent % 2 != 0) throw Syntax(physical.Number);
                var relative = body[0] == '.';
                if (relative != (indent > 0)) throw Syntax(physical.Number);
                var (entry, key) = SplitLine(body, physical.Number);
                PathNode? parent = null;
                if (relative)
                {
                    var depth = indent / 2;
                    if (depth > scopes.Count || !key.StartsWith('.')) throw Syntax(entry.Line);
                    if (scopes.Count > depth) scopes.RemoveRange(depth, scopes.Count - depth);
                    parent = scopes[depth - 1];
                    key = key[1..];
                }
                else scopes.Clear();
                if (entry.Kind == "heading")
                {
                    var node = new PathNode(parent, ParsePath(key, entry.Line).ToArray());
                    entry.Path = node.Elements();
                    scopes.Add(node);
                }
                else
                {
                    if (relative && key.Length == 0) entry.Path = parent!.Elements();
                    else
                    {
                        var suffix = ParsePath(key, entry.Line);
                        entry.Path = parent is null ? suffix.ToArray() : parent.Elements(suffix);
                    }
                    eof = lines.ValueBody(entry, indent, eof);
                }
                visit(entry);
            }
            if (eof) return;
        }
    }

    /// <summary>Decode form-specific strings without interpreting type expressions.</summary>
    public static Text DecodeString(Entry entry)
    {
        if (entry is null) throw Problem(0, "value required");
        try
        {
            if (entry.Kind != "value") throw Syntax(entry.Line);
            if (entry.Form is not null and not "")
            {
                if (entry.Unset || !ValidText(entry.Raw, true)) throw Syntax(entry.Line);
                return entry.Form switch
                {
                    "pipe" => new Text(entry.Raw, false),
                    "triple_quoted" => new Text(DecodeBlock(entry.Raw), false),
                    _ => throw Syntax(entry.Line)
                };
            }
            var raw = Trim(entry.Raw);
            if (!ValidText(raw, false) || entry.Unset != (raw.Length == 0)) throw Syntax(entry.Line);
            if (entry.Unset) return new Text("", true);
            if (raw[0] != '"') return new Text(raw, false);
            var (value, end) = DecodeQuoted(raw, 0);
            if (end != raw.Length) throw Syntax(entry.Line);
            return new Text(value, false);
        }
        catch (ParseException) { throw Problem(entry.Line, "invalid string"); }
    }

    private static bool ValidText(string text, bool block)
    {
        for (var i = 0; i < text.Length; i++)
        {
            var c = text[i];
            if (char.IsHighSurrogate(c))
            {
                if (++i >= text.Length || !char.IsLowSurrogate(text[i])) return false;
            }
            else if (char.IsLowSurrogate(c)) return false;
            else if (c == '\r')
            {
                if (!block || i + 1 >= text.Length || text[i + 1] != '\n') return false;
            }
            else if (c == '\n' && !block) return false;
        }
        return true;
    }

    private static (Entry, string) SplitLine(string body, int line)
    {
        for (var i = 0; i < body.Length;)
        {
            switch (body[i])
            {
                case '"':
                    try { i = ScanQuoted(body, i, null); }
                    catch (ParseException) { throw Syntax(line); }
                    continue;
                case ':': return Value(Trim(body[..i]), body[(i + 1)..], null);
                case '(':
                    var key = body[..i].TrimEnd(' ');
                    if (key.EndsWith('\t')) throw Problem(line, "unsupported type syntax");
                    var close = TypeEnd(body, i, line);
                    var type = body[(i + 1)..close];
                    if (type.Length == 0) throw Problem(line, "unsupported type syntax");
                    var tail = body[(close + 1)..].TrimStart(' ');
                    if (tail.StartsWith('\t')) throw Problem(line, "unsupported type syntax");
                    if (tail.Length == 0 || tail[0] != ':') throw Syntax(line);
                    return Value(key, tail[1..], type);
                default: i++; continue;
            }
        }
        if (body.EndsWith('.')) return (new Entry { Line = line, Kind = "heading" }, body[..^1]);
        throw Syntax(line);

        (Entry, string) Value(string key, string rhs, string? type)
        {
            var raw = Trim(rhs);
            return (new Entry { Line = line, Raw = raw, Unset = raw.Length == 0,
                Type = type, HasType = type is not null }, key);
        }
    }

    private static int TypeEnd(string body, int open, int line)
    {
        var depth = 1;
        for (var i = open + 1; i < body.Length;)
        {
            var c = Rune.GetRuneAt(body, i);
            if (Rune.IsWhiteSpace(c)) throw Syntax(line);
            if (c.Value is '"' or '\\') throw Problem(line, "unsupported type syntax");
            if (c.Value == '(') depth++;
            else if (c.Value == ')' && --depth == 0) return i;
            i += c.Utf16SequenceLength;
        }
        throw Syntax(line);
    }

    private static List<string> ParsePath(string key, int line)
    {
        var elements = new List<string>();
        for (var i = 0;;)
        {
            if (i == key.Length) throw Syntax(line);
            string element;
            if (key[i] == '"')
            {
                try { (element, i) = DecodeQuoted(key, i); }
                catch (ParseException) { throw Syntax(line); }
                if (i < key.Length && key[i] != '.') throw Syntax(line);
            }
            else
            {
                var end = key.IndexOf('.', i);
                if (end < 0) end = key.Length;
                if (end == i) throw Syntax(line);
                element = key[i..end];
                foreach (var c in element.EnumerateRunes())
                    if (Rune.IsWhiteSpace(c) || Rune.IsControl(c) || ":()#\"\\".Contains((char)c.Value)
                        && c.Value <= char.MaxValue) throw Syntax(line);
                i = end;
            }
            elements.Add(element);
            if (i == key.Length) return elements;
            i++; // Unquoted element separator.
        }
    }

    private static (string Value, int End) DecodeQuoted(string text, int start)
    {
        var output = new StringBuilder();
        var end = ScanQuoted(text, start, output);
        return (output.ToString(), end);
    }

    private static int ScanQuoted(string text, int start, StringBuilder? output)
    {
        if (start >= text.Length || text[start] != '"') throw Syntax(0);
        var literal = start + 1;
        for (var i = literal; i < text.Length; i++)
        {
            switch (text[i])
            {
                case '"': output?.Append(text, literal, i - literal); return i + 1;
                case '\r': case '\n': throw Syntax(0);
                case '\\':
                    var (scalar, end) = Escape(text, i);
                    output?.Append(text, literal, i - literal);
                    if (output is not null) AppendScalar(output, scalar);
                    literal = end;
                    i = end - 1;
                    break;
            }
        }
        throw Syntax(0);
    }

    private static string DecodeBlock(string text)
    {
        var escape = text.IndexOf('\\');
        if (escape < 0) return text;
        var output = new StringBuilder(text.Length);
        var literal = 0;
        while (escape >= 0)
        {
            output.Append(text, literal, escape - literal);
            var (scalar, end) = Escape(text, escape);
            AppendScalar(output, scalar);
            literal = end;
            escape = text.IndexOf('\\', end);
        }
        output.Append(text, literal, text.Length - literal);
        return output.ToString();
    }

    private static (int Scalar, int End) Escape(string text, int start)
    {
        if (start + 1 >= text.Length) throw Syntax(0);
        var end = start + 2;
        var c = text[start + 1];
        var scalar = c switch
        {
            '.' or '"' or '\\' or '/' => c,
            'b' => '\b', 'f' => '\f', 'n' => '\n', 'r' => '\r', 't' => '\t',
            'u' => -1, _ => throw Syntax(0)
        };
        if (scalar >= 0) return (scalar, end);
        if (end == text.Length) throw Syntax(0);
        if (text[end] == '{')
        {
            var value = 0;
            var digits = 0;
            for (var i = end + 1; i < text.Length; i++)
            {
                if (text[i] == '}')
                {
                    if (digits == 0 || !Rune.IsValid(value)) throw Syntax(0);
                    return (value, i + 1);
                }
                if (++digits > 6) throw Syntax(0);
                value = value * 16 + Hex(text[i]);
            }
            throw Syntax(0);
        }
        scalar = FourHex(text, end);
        end += 4;
        if (scalar is >= 0xd800 and <= 0xdbff)
        {
            if (text.Length - end < 6 || text[end] != '\\' || text[end + 1] != 'u') throw Syntax(0);
            var low = FourHex(text, end + 2);
            if (low is < 0xdc00 or > 0xdfff) throw Syntax(0);
            return (0x10000 + (scalar - 0xd800) * 0x400 + low - 0xdc00, end + 6);
        }
        if (!Rune.IsValid(scalar)) throw Syntax(0);
        return (scalar, end);
    }

    private static int FourHex(string text, int start)
    {
        if (text.Length - start < 4) throw Syntax(0);
        var value = 0;
        for (var i = start; i < start + 4; i++) value = value * 16 + Hex(text[i]);
        return value;
    }

    private static int Hex(char c) => c switch
    {
        >= '0' and <= '9' => c - '0', >= 'a' and <= 'f' => c - 'a' + 10,
        >= 'A' and <= 'F' => c - 'A' + 10, _ => throw Syntax(0)
    };
    private static void AppendScalar(StringBuilder output, int scalar)
    {
        if (scalar <= char.MaxValue) output.Append((char)scalar);
        else output.Append(char.ConvertFromUtf32(scalar));
    }
    private static string Trim(string text) => text.Trim(' ', '\t');
    private static int LeadingSpaces(string text)
    {
        var i = 0;
        while (i < text.Length && text[i] == ' ') i++;
        return i;
    }
    private static ParseException Syntax(int line) => Problem(line, "invalid syntax");
    private static ParseException Problem(int line, string cause) => new(line, cause);

    // A heading retains only its suffix and its parent. Exported arrays are separate.
    private sealed class PathNode(PathNode? parent, string[] suffix)
    {
        private readonly int count = suffix.Length + (parent?.count ?? 0);
        public string[] Elements(List<string>? tail = null)
        {
            var result = new string[count + (tail?.Count ?? 0)];
            tail?.CopyTo(result, count);
            var end = count;
            for (PathNode? node = this; node is not null; node = node.Parent)
            {
                end -= node.Suffix.Length;
                node.Suffix.CopyTo(result, end);
            }
            return result;
        }
        private PathNode? Parent => parent;
        private string[] Suffix => suffix;
    }

    // A line borrows the input/scratch storage until Next is called again. All
    // ordinary consumers decode it first; pending lookahead prevents another read.
    private readonly struct PhysicalLine(ReadOnlyMemory<byte> bytes, string ending, int number, bool eof, bool readError)
    {
        public ReadOnlyMemory<byte> Bytes { get; } = bytes;
        public string Ending { get; } = ending;
        public int Number { get; } = number;
        public bool Eof { get; } = eof;
        public string Validate()
        {
            if (readError) throw Problem(Number, "input failure");
            if (Bytes.Span.IndexOf((byte)'\r') >= 0) throw Syntax(Number);
            try { return Utf8.GetString(Bytes.Span); }
            catch (DecoderFallbackException) { throw Syntax(Number); }
        }
        public int PrefixSpaces()
        {
            var i = 0;
            while (i < Bytes.Length && Bytes.Span[i] == ' ') i++;
            return i;
        }
    }

    private sealed class SourceLines(Stream stream)
    {
        private readonly byte[] buffer = new byte[32768];
        private readonly ArrayBufferWriter<byte> scratch = new();
        private int position, length, nextNumber = 1;
        private PhysicalLine? pending;

        public PhysicalLine Next()
        {
            if (pending is { } pendingLine)
            {
                pending = null;
                return pendingLine;
            }
            scratch.Clear();
            var eof = false;
            var readError = false;
            while (true)
            {
                if (position == length)
                {
                    position = 0;
                    try { length = stream.Read(buffer, 0, buffer.Length); }
                    catch (Exception)
                    { readError = true; break; }
                    if (length == 0) { eof = true; break; }
                }
                var rest = buffer.AsMemory(position, length - position);
                var newline = rest.Span.IndexOf((byte)'\n');
                var take = newline < 0 ? rest.Length : newline + 1;
                if (newline >= 0 && scratch.WrittenCount == 0)
                {
                    position += take;
                    return Finish(rest[..take], false, false);
                }
                rest.Span[..take].CopyTo(scratch.GetSpan(take));
                scratch.Advance(take);
                position += take;
                if (newline >= 0) break;
            }
            return Finish(scratch.WrittenMemory, eof, readError);
        }

        private PhysicalLine Finish(ReadOnlyMemory<byte> bytes, bool eof, bool readError)
        {
            var ending = "";
            if (bytes.Length > 0 && bytes.Span[^1] == '\n')
            {
                bytes = bytes[..^1];
                ending = "\n";
                if (bytes.Length > 0 && bytes.Span[^1] == '\r') { bytes = bytes[..^1]; ending = "\r\n"; }
            }
            if (nextNumber == 1 && bytes.Span.StartsWith(new byte[] { 0xef, 0xbb, 0xbf })) bytes = bytes[3..];
            return new PhysicalLine(bytes, ending, nextNumber++, eof, readError);
        }

        public bool Properties()
        {
            var declared = false;
            while (true)
            {
                var physical = Next();
                var source = physical.Validate();
                var text = source.TrimEnd(' ', '\t');
                if (text.Length == 0)
                {
                    if (!declared || source.Length == 0 && physical.Ending.Length == 0) throw Syntax(physical.Number);
                    return physical.Eof;
                }
                var separator = text.IndexOfAny([' ', '\t']);
                if (separator <= 0 || text[0] != '@') throw Syntax(physical.Number);
                var name = text[..separator];
                var value = text[separator..].TrimStart(' ', '\t');
                if (value.Length == 0) throw Syntax(physical.Number);
                if (name != "@dotReg") throw Problem(physical.Number, "unsupported property");
                if (declared) throw Problem(physical.Number, "duplicate property");
                if (value != "1") throw Problem(physical.Number, "unsupported format version");
                declared = true;
                if (physical.Eof) throw Syntax(physical.Number + 1);
            }
        }

        public bool ValueBody(Entry entry, int indent, bool eof)
        {
            if (entry.Raw == "\"\"\"")
            {
                (entry.Raw, eof) = TripleBody(entry.Line);
                entry.Form = "triple_quoted";
                entry.Unset = false;
            }
            else if (entry.Unset && !eof)
            {
                var (raw, blockEof, found) = PipeBody(indent);
                if (found)
                {
                    entry.Raw = raw; entry.Form = "pipe"; entry.Unset = false;
                    eof = blockEof;
                }
            }
            return eof;
        }

        private (string Raw, bool Eof) TripleBody(int opening)
        {
            var body = new List<(string Text, string Ending, int Number)>();
            while (true)
            {
                var physical = Next();
                var text = physical.Validate();
                var margin = LeadingSpaces(text);
                if (text[margin..].TrimEnd(' ', '\t') == "\"\"\"")
                {
                    var output = new StringBuilder();
                    for (var i = 0; i < body.Count; i++)
                    {
                        var part = body[i];
                        if (part.Text.Length > 0 && LeadingSpaces(part.Text) < margin) throw Syntax(part.Number);
                        if (i > 0) output.Append(body[i - 1].Ending);
                        output.Append(part.Text.Length == 0 ? "" : part.Text[margin..]);
                    }
                    return (output.ToString(), physical.Eof);
                }
                if (physical.Eof) throw Syntax(opening);
                body.Add((text, physical.Ending, physical.Number));
            }
        }

        private (string Raw, bool Eof, bool Found) PipeBody(int indent)
        {
            var output = new StringBuilder();
            var previousEnding = "";
            var found = false;
            while (true)
            {
                var physical = Next();
                var prefix = physical.PrefixSpaces();
                if (prefix <= indent || prefix == physical.Bytes.Length || physical.Bytes.Span[prefix] != '|')
                {
                    pending = physical;
                    return (output.ToString(), false, found);
                }
                var text = physical.Validate();
                if (found) output.Append(previousEnding);
                output.Append(text, prefix + 1, text.Length - prefix - 1);
                found = true;
                previousEnding = physical.Ending;
                if (physical.Eof) return (output.ToString(), true, true);
            }
        }
    }
}
