using System.Text;

namespace DotReg;

public readonly record struct WriteValue(Text Text, string? Type = null);

/// <summary>Source-preserving write helpers; their style policy is sample-specific.</summary>
public static class Writer
{
    private static readonly UTF8Encoding Utf8 = new(false, true);

    /// <summary>Append an occurrence under the deepest matching heading, or as a full key.</summary>
    public static string AppendValue(string source, IReadOnlyList<string> path, WriteValue value)
    {
        var entries = ParseSource(source);
        return AppendParsed(source, entries, path, value);
    }

    /// <summary>Replace a unique occurrence, or append an untyped value when absent.</summary>
    public static string SetText(string source, IReadOnlyList<string> path, Text text)
    {
        var entries = ParseSource(source);
        var matches = MatchingValues(entries, path);
        if (matches.Count == 0) return AppendParsed(source, entries, path, new WriteValue(text));
        if (matches.Count > 1) throw new ParseException(matches[1].Line, "multiple values");
        return ReplaceText(source, matches[0], text);
    }

    /// <summary>Replace one zero-based occurrence of a repeated key.</summary>
    public static string SetTextAt(string source, IReadOnlyList<string> path, int occurrence, Text text)
    {
        var entries = ParseSource(source);
        if (occurrence < 0) throw new ParseException(0, "input");
        var matches = MatchingValues(entries, path);
        if (occurrence >= matches.Count) throw new ParseException(0, "missing");
        return ReplaceText(source, matches[occurrence], text);
    }

    private static List<Entry> ParseSource(string source)
    {
        if (source is null) throw new ParseException(0, "input");
        try
        {
            using var input = new MemoryStream(Utf8.GetBytes(source));
            return Parser.Parse(input);
        }
        catch (EncoderFallbackException) { throw new ParseException(0, "input"); }
    }

    private static List<Entry> MatchingValues(List<Entry> entries, IReadOnlyList<string> path)
    {
        ValidatePath(path);
        return entries.Where(entry => entry.Kind == "value" && PathEquals(entry.Path, path)).ToList();
    }

    private static void ValidatePath(IReadOnlyList<string> path)
    {
        if (path is null || path.Count == 0) throw new ParseException(0, "input");
        try
        {
            foreach (var part in path)
            {
                if (part is null) throw new EncoderFallbackException();
                _ = Utf8.GetBytes(part);
            }
        }
        catch (EncoderFallbackException) { throw new ParseException(0, "input"); }
    }

    private static bool PathEquals(IReadOnlyList<string> left, IReadOnlyList<string> right)
    {
        if (left.Count != right.Count) return false;
        for (var i = 0; i < left.Count; i++)
            if (!StringComparer.Ordinal.Equals(left[i], right[i])) return false;
        return true;
    }

    private static bool HasPrefix(IReadOnlyList<string> path, IReadOnlyList<string> prefix)
    {
        if (prefix.Count > path.Count) return false;
        for (var i = 0; i < prefix.Count; i++)
            if (!StringComparer.Ordinal.Equals(path[i], prefix[i])) return false;
        return true;
    }

    private static Entry? DeepestHeadingAncestor(List<Entry> entries, IReadOnlyList<string> path)
    {
        Entry? best = null;
        foreach (var entry in entries)
        {
            if (entry.Kind != "heading" || !HasPrefix(path, entry.Path)) continue;
            if (best is null || entry.Path.Length >= best.Path.Length) best = entry;
        }
        return best;
    }

    private static string AppendParsed(string source, List<Entry> entries, IReadOnlyList<string> path, WriteValue value)
    {
        ValidatePath(path);
        if (value.Text.Value is null || value.Text.Unset && value.Text.Value.Length != 0 ||
            value.Type is { Length: 0 }) throw new ParseException(0, "input");
        string key = EncodeKeyPath(path);
        string line = EncodeWriteLine(key, value);
        int insertAt = source.Length, indent = 0;
        Entry? heading = DeepestHeadingAncestor(entries, path);
        if (heading is not null)
        {
            var lines = WriterLines(source);
            indent = LeadingSpaces(lines[heading.Line - 1].Text) + 2;
            string suffix = path.Count == heading.Path.Length ? "" :
                EncodeKeyPath(path.Skip(heading.Path.Length).ToArray());
            line = "." + suffix + line[key.Length..];
            insertAt = HeadingSectionEnd(entries, lines, heading);
        }
        line = new string(' ', indent) + line;
        string updated = InsertSourceLine(source, insertAt, line, LineEndingAt(source, insertAt));
        _ = ParseSource(updated);
        return updated;
    }

    private static string ReplaceText(string source, Entry entry, Text text)
    {
        if (text.Value is null || text.Unset && text.Value.Length != 0)
            throw new ParseException(0, "input");
        try { _ = Utf8.GetBytes(text.Value); }
        catch (EncoderFallbackException) { throw new ParseException(0, "input"); }
        var lines = WriterLines(source);
        int start = entry.Line - 1;
        if (start < 0 || start >= lines.Count) throw new ParseException(entry.Line, "syntax");
        int end = EntryEndLine(lines, entry);
        var first = lines[start];
        int separator = ValueSeparator(first.Text);
        if (separator < 0) throw new ParseException(entry.Line, "syntax");
        string rhs = text.Unset ? "" : " " + EncodeQuotedValue(text.Value);
        var last = lines[end];
        string replacement = source[first.Start..(first.Start + separator + 1)] + rhs +
            source[last.ContentEnd..last.End];
        string updated = source[..first.Start] + replacement + source[last.End..];
        _ = ParseSource(updated);
        return updated;
    }

    private static string EncodeWriteLine(string key, WriteValue value)
    {
        string type = value.Type is null ? "" : $" ({value.Type})";
        return value.Text.Unset ? key + type + ":" :
            key + type + ": " + EncodeQuotedValue(value.Text.Value);
    }

    private static string EncodeKeyPath(IReadOnlyList<string> path)
    {
        ValidatePath(path);
        return string.Join(".", path.Select(EncodeKeyElement));
    }

    private static string EncodeKeyElement(string element)
    {
        bool plain = element.Length != 0;
        foreach (var rune in element.EnumerateRunes())
            if (rune.Value == '.' || Rune.IsWhiteSpace(rune) ||
                Rune.GetUnicodeCategory(rune) == System.Globalization.UnicodeCategory.Control ||
                rune.Value <= char.MaxValue && ":()#\"\\".Contains((char)rune.Value))
                plain = false;
        return plain ? element : EncodeQuotedValue(element);
    }

    private static string EncodeQuotedValue(string value)
    {
        var result = new StringBuilder("\"");
        foreach (var rune in value.EnumerateRunes())
        {
            switch (rune.Value)
            {
                case '"': result.Append("\\\""); break;
                case '\\': result.Append("\\\\"); break;
                case '\b': result.Append("\\b"); break;
                case '\f': result.Append("\\f"); break;
                case '\n': result.Append("\\n"); break;
                case '\r': result.Append("\\r"); break;
                case '\t': result.Append("\\t"); break;
                default:
                    if (Rune.GetUnicodeCategory(rune) == System.Globalization.UnicodeCategory.Control)
                        result.Append("\\u{").Append(rune.Value.ToString("x")).Append('}');
                    else result.Append(rune.ToString());
                    break;
            }
        }
        return result.Append('"').ToString();
    }

    private readonly record struct SourceLine(string Text, int Start, int ContentEnd, int End);

    private static List<SourceLine> WriterLines(string source)
    {
        var lines = new List<SourceLine>();
        for (int start = 0; start < source.Length;)
        {
            int newline = source.IndexOf('\n', start);
            int end = newline < 0 ? source.Length : newline + 1;
            int contentEnd = newline < 0 ? end : newline;
            if (contentEnd > start && source[contentEnd - 1] == '\r') contentEnd--;
            lines.Add(new SourceLine(source[start..contentEnd], start, contentEnd, end));
            start = end;
        }
        return lines;
    }

    private static int EntryEndLine(List<SourceLine> lines, Entry entry)
    {
        int start = entry.Line - 1;
        if (start < 0 || start >= lines.Count) throw new ParseException(entry.Line, "syntax");
        if (string.IsNullOrEmpty(entry.Form)) return start;
        if (entry.Form == "triple_quoted")
        {
            for (int i = start + 1; i < lines.Count; i++)
            {
                string text = lines[i].Text;
                int indent = LeadingSpaces(text);
                if (text[indent..].TrimEnd(' ', '\t') == "\"\"\"") return i;
            }
            throw new ParseException(entry.Line, "syntax");
        }
        if (entry.Form == "pipe")
        {
            int parentIndent = LeadingSpaces(lines[start].Text), end = start;
            for (int i = start + 1; i < lines.Count; i++)
            {
                string text = lines[i].Text;
                int indent = LeadingSpaces(text);
                if (indent <= parentIndent || indent == text.Length || text[indent] != '|') break;
                end = i;
            }
            if (end == start) throw new ParseException(entry.Line, "syntax");
            return end;
        }
        throw new ParseException(entry.Line, "syntax");
    }

    private static int HeadingSectionEnd(List<Entry> entries, List<SourceLine> lines, Entry heading)
    {
        int headingIndex = heading.Line - 1;
        int indent = LeadingSpaces(lines[headingIndex].Text), insertAt = lines[headingIndex].End;
        bool found = false;
        foreach (var entry in entries)
        {
            if (!found)
            {
                if (entry.Kind == "heading" && entry.Line == heading.Line) found = true;
                continue;
            }
            int index = entry.Line - 1;
            if (LeadingSpaces(lines[index].Text) <= indent) break;
            insertAt = lines[EntryEndLine(lines, entry)].End;
        }
        return insertAt;
    }

    private static int ValueSeparator(string line)
    {
        bool quoted = false;
        int depth = 0;
        for (int i = 0; i < line.Length; i++)
        {
            char c = line[i];
            if (c == '\\' && quoted) { i++; continue; }
            if (c == '"') { quoted = !quoted; continue; }
            if (quoted) continue;
            if (c == '(') depth++;
            else if (c == ')' && depth > 0) depth--;
            else if (c == ':' && depth == 0) return i;
        }
        return -1;
    }

    private static int LeadingSpaces(string line)
    {
        int i = 0;
        while (i < line.Length && line[i] == ' ') i++;
        return i;
    }

    private static string LineEndingAt(string source, int offset)
    {
        int newline = source.IndexOf('\n', Math.Max(0, offset));
        if (newline < 0 && offset > 0) newline = source.LastIndexOf('\n', Math.Min(source.Length, offset) - 1);
        return newline > 0 && source[newline - 1] == '\r' ? "\r\n" : "\n";
    }

    private static string InsertSourceLine(string source, int offset, string line, string ending)
    {
        bool previousBreak = offset == 0 || source[offset - 1] == '\n';
        bool following = offset < source.Length, finalBreak = source.EndsWith('\n');
        bool needsFollowingBreak = following || offset == source.Length && finalBreak;
        return source[..offset] + (previousBreak ? "" : ending) + line +
            (needsFollowingBreak ? ending : "") + source[offset..];
    }
}
