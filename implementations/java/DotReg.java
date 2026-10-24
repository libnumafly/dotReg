import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.ByteBuffer;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.CharsetDecoder;
import java.nio.charset.CodingErrorAction;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/** Dependency-free dotReg syntax and common string decoding, Java 17+. */
public final class DotReg {
    private DotReg() {}

    public static final class ParseException extends Exception {
        public final int line;
        public final String reason;
        public ParseException(int line, String reason) {
            super("dotReg line " + line + ": " + reason);
            this.line = line;
            this.reason = reason;
        }
    }

    /** Paths retain decoded element boundaries. Raw excludes block markers. */
    public record Entry(int line, String kind, List<String> path, String raw,
                        boolean unset, String type, boolean hasType, String form) {
        private Entry body(String text, String valueForm) {
            return new Entry(line, kind, path, text, false, type, hasType, valueForm);
        }
    }

    @FunctionalInterface
    public interface Visitor {
        /** Return false to stop successfully; thrown exceptions pass unchanged. */
        boolean visit(Entry entry) throws Exception;
    }

    /** Collect complete results; a failure returns no partial list. */
    public static List<Entry> parse(InputStream source) throws Exception {
        List<Entry> entries = new ArrayList<>();
        parseEach(source, entry -> { entries.add(entry); return true; });
        return entries;
    }

    /** Read incrementally without closing source. Each call has fresh scopes. */
    public static void parseEach(InputStream source, Visitor visitor) throws Exception {
        if (source == null || visitor == null) throw problem(0, "input");
        Lines lines = new Lines(source);
        if (lines.properties()) return;
        List<Node> scopes = new ArrayList<>();
        while (true) {
            Physical physical = lines.next();
            String text = physical.validate(lines.decoder);
            boolean eof = physical.eof;
            String trimmed = trim(text);
            if (!trimmed.isEmpty() && !trimmed.startsWith("#")) {
                int indent = spaces(text);
                String body = trimRight(text.substring(indent));
                boolean relative = body.startsWith(".");
                if (body.startsWith("\t") || indent % 2 != 0 || relative != (indent > 0))
                    throw problem(physical.number, "syntax");
                Split split = splitLine(body, physical.number);
                Entry entry = split.entry;
                String key = split.key;
                Node parent = null;
                if (relative) {
                    int depth = indent / 2;
                    if (depth > scopes.size()) throw problem(physical.number, "syntax");
                    scopes.subList(depth, scopes.size()).clear();
                    parent = scopes.get(depth - 1);
                    key = key.substring(1);
                } else scopes.clear();
                List<String> path;
                if (entry.kind.equals("heading")) {
                    Node node = new Node(parent, parsePath(key, physical.number));
                    path = node.elements();
                    scopes.add(node);
                } else {
                    path = parent == null ? new ArrayList<>() : parent.elements();
                    if (!(relative && key.isEmpty())) parsePath(key, physical.number, path);
                }
                entry = new Entry(entry.line, entry.kind, path, entry.raw,
                        entry.unset, entry.type, entry.hasType, entry.form);
                if (!entry.kind.equals("heading")) {
                    Body completed = lines.body(entry, indent, eof);
                    entry = completed.entry;
                    eof = completed.eof;
                }
                if (!visitor.visit(entry)) return;
            }
            if (eof) return;
        }
    }

    /** Persistent scopes retain suffixes, rather than copying all ancestors. */
    private static final class Node {
        final Node parent;
        final List<String> suffix;
        final int count;
        Node(Node parent, List<String> suffix) {
            this.parent = parent;
            this.suffix = List.copyOf(suffix);
            this.count = suffix.size() + (parent == null ? 0 : parent.count);
        }
        List<String> elements() {
            String[] result = new String[count];
            int end = count;
            for (Node node = this; node != null; node = node.parent) {
                int start = end - node.suffix.size();
                for (int i = 0; i < node.suffix.size(); i++) result[start + i] = node.suffix.get(i);
                end = start;
            }
            return new ArrayList<>(Arrays.asList(result));
        }
    }

    // A physical line borrows storage until next() is called again. Ordinary
    // consumers decode first; a pending boundary prevents another source read.
    private record Physical(byte[] data, int offset, int length, String ending,
                            int number, boolean eof, boolean failed) {
        String validate(CharsetDecoder decoder) throws ParseException {
            if (failed) throw problem(number, "input");
            String text;
            try {
                text = decoder.reset().decode(ByteBuffer.wrap(data, offset, length)).toString();
            } catch (CharacterCodingException error) { throw problem(number, "syntax"); }
            if (text.indexOf('\r') >= 0) throw problem(number, "syntax");
            return text;
        }
    }

    private record Body(Entry entry, boolean eof) {}

    // Expose the assembled bytes to Physical without a final toByteArray copy.
    // The buffer is local to one spanning line and is never reused afterwards.
    private static final class LineBuffer extends ByteArrayOutputStream {
        LineBuffer() { super(8192); }
        byte[] storage() { return buf; }
    }

    private static final class Lines {
        final InputStream source;
        final byte[] buffer = new byte[8192];
        final CharsetDecoder decoder = StandardCharsets.UTF_8.newDecoder()
                .onMalformedInput(CodingErrorAction.REPORT)
                .onUnmappableCharacter(CodingErrorAction.REPORT);
        int position, available, number = 1;
        boolean eof;
        Physical pending;
        Lines(InputStream source) { this.source = source; }

        Physical next() {
            if (pending != null) {
                Physical line = pending;
                pending = null;
                return line;
            }
            LineBuffer output = null;
            boolean failed = false;
            while (true) {
                if (position == available) {
                    position = available = 0;
                    if (eof) break;
                    try {
                        available = source.read(buffer);
                        if (available < 0) { available = 0; eof = true; break; }
                        // InputStream forbids zero progress for a non-empty buffer.
                        if (available == 0) { failed = true; break; }
                    } catch (IOException | RuntimeException error) { failed = true; break; }
                }
                int end = position;
                while (end < available && buffer[end] != '\n') end++;
                int take = end < available ? end + 1 - position : available - position;
                if (end < available && output == null) {
                    int start = position;
                    position += take;
                    return finish(buffer, start, take, false, false);
                }
                if (output == null) output = new LineBuffer();
                output.write(buffer, position, take);
                position += take;
                if (end < available) return finish(output.storage(), 0, output.size(), false, false);
            }
            return output == null ? finish(buffer, 0, 0, eof, failed)
                    : finish(output.storage(), 0, output.size(), eof, failed);
        }

        private Physical finish(byte[] data, int start, int length, boolean ended, boolean failed) {
            int end = start + length;
            String ending = "";
            if (end > start && data[end - 1] == '\n') {
                end--; ending = "\n";
                if (end > start && data[end - 1] == '\r') { end--; ending = "\r\n"; }
            }
            if (number == 1 && end - start >= 3 && data[start] == (byte) 0xef
                    && data[start + 1] == (byte) 0xbb && data[start + 2] == (byte) 0xbf) start += 3;
            return new Physical(data, start, end - start, ending, number++, ended && ending.isEmpty(), failed);
        }

        boolean properties() throws ParseException {
            boolean declared = false;
            while (true) {
                Physical line = next();
                String text = trimRight(line.validate(decoder));
                if (text.isEmpty()) {
                    if (!declared || (line.length == 0 && line.ending.isEmpty()))
                        throw problem(line.number, "syntax");
                    return line.eof;
                }
                int split = 0;
                while (split < text.length() && text.charAt(split) != ' ' && text.charAt(split) != '\t') split++;
                if (split == 0 || split == text.length() || text.charAt(0) != '@')
                    throw problem(line.number, "syntax");
                String name = text.substring(0, split), value = trimLeft(text.substring(split));
                if (value.isEmpty()) throw problem(line.number, "syntax");
                if (!name.equals("@dotReg")) throw problem(line.number, "unsupported property");
                if (declared) throw problem(line.number, "duplicate property");
                if (!value.equals("1")) throw problem(line.number, "unsupported version");
                declared = true;
                if (line.eof) throw problem(line.number + 1, "syntax");
            }
        }

        Body body(Entry entry, int indent, boolean valueEOF) throws ParseException {
            if (entry.raw.equals("\"\"\"")) {
                List<String> texts = new ArrayList<>(), endings = new ArrayList<>();
                while (true) {
                    Physical line = next();
                    String text = line.validate(decoder);
                    int margin = spaces(text);
                    if (trimRight(text.substring(margin)).equals("\"\"\"")) {
                        StringBuilder raw = new StringBuilder();
                        for (int i = 0; i < texts.size(); i++) {
                            String part = texts.get(i);
                            if (!part.isEmpty()) {
                                if (spaces(part) < margin) throw problem(entry.line + i + 1, "syntax");
                                part = part.substring(margin);
                            }
                            if (i > 0) raw.append(endings.get(i - 1));
                            raw.append(part);
                        }
                        return new Body(entry.body(raw.toString(), "triple_quoted"), line.eof);
                    }
                    if (line.eof) throw problem(entry.line, "syntax");
                    texts.add(text); endings.add(line.ending);
                }
            }
            if (entry.unset && !valueEOF) {
                StringBuilder raw = new StringBuilder();
                boolean found = false, blockEOF = false;
                String previousEnding = "";
                while (true) {
                    Physical line = next();
                    int prefix = 0;
                    while (prefix < line.length && line.data[line.offset + prefix] == ' ') prefix++;
                    if (prefix <= indent || prefix == line.length || line.data[line.offset + prefix] != '|') {
                        pending = line;
                        break;
                    }
                    String text = line.validate(decoder);
                    if (found) raw.append(previousEnding);
                    raw.append(text, prefix + 1, text.length());
                    found = true; previousEnding = line.ending;
                    if (line.eof) { blockEOF = true; break; }
                }
                if (found) return new Body(entry.body(raw.toString(), "pipe"), blockEOF);
            }
            return new Body(entry, valueEOF);
        }
    }

    private record Split(Entry entry, String key) {}

    private static Split splitLine(String body, int line) throws ParseException {
        for (int i = 0; i < body.length();) {
            char c = body.charAt(i);
            if (c == '"') {
                try { i = quoted(body, i, null); }
                catch (IllegalArgumentException error) { throw problem(line, "syntax"); }
                continue;
            }
            String key, rhs, type = "";
            boolean hasType = false;
            if (c == ':') { key = trim(body.substring(0, i)); rhs = body.substring(i + 1); }
            else if (c == '(') {
                int keyEnd = i;
                while (keyEnd > 0 && body.charAt(keyEnd - 1) == ' ') keyEnd--;
                key = body.substring(0, keyEnd);
                if (key.endsWith("\t")) throw problem(line, "unsupported type syntax");
                int close = i + 1, depth = 1;
                while (close < body.length()) {
                    int t = body.codePointAt(close);
                    if (white(t)) throw problem(line, "syntax");
                    if (t == '"' || t == '\\') throw problem(line, "unsupported type syntax");
                    if (t == '(') depth++;
                    else if (t == ')') depth--;
                    if (depth == 0) break;
                    close += Character.charCount(t);
                }
                if (depth != 0) throw problem(line, "syntax");
                type = body.substring(i + 1, close);
                if (type.isEmpty()) throw problem(line, "unsupported type syntax");
                int tail = close + 1;
                while (tail < body.length() && body.charAt(tail) == ' ') tail++;
                if (tail < body.length() && body.charAt(tail) == '\t')
                    throw problem(line, "unsupported type syntax");
                if (tail == body.length() || body.charAt(tail) != ':') throw problem(line, "syntax");
                hasType = true; rhs = body.substring(tail + 1);
            } else { i++; continue; }
            String raw = trim(rhs);
            return new Split(new Entry(line, "value", List.of(), raw, raw.isEmpty(), type, hasType, ""), key);
        }
        if (body.endsWith(".")) return new Split(new Entry(line, "heading", List.of(), "", false, "", false, ""),
                body.substring(0, body.length() - 1));
        throw problem(line, "syntax");
    }

    private static List<String> parsePath(String key, int line) throws ParseException {
        return parsePath(key, line, new ArrayList<>());
    }

    private static List<String> parsePath(String key, int line, List<String> path) throws ParseException {
        int start = 0;
        while (true) {
            if (start >= key.length()) throw problem(line, "syntax");
            int end;
            String element;
            if (key.charAt(start) == '"') {
                StringBuilder decoded = new StringBuilder();
                try { end = quoted(key, start, decoded); }
                catch (IllegalArgumentException error) { throw problem(line, "syntax"); }
                element = decoded.toString();
                if (end < key.length() && key.charAt(end) != '.') throw problem(line, "syntax");
            } else {
                end = key.indexOf('.', start);
                if (end < 0) end = key.length();
                element = key.substring(start, end);
                if (element.isEmpty()) throw problem(line, "syntax");
                for (int i = 0; i < element.length();) {
                    int c = element.codePointAt(i);
                    if (white(c) || Character.isISOControl(c) || ":()#\"\\".indexOf(c) >= 0)
                        throw problem(line, "syntax");
                    i += Character.charCount(c);
                }
            }
            path.add(element);
            if (end == key.length()) return path;
            start = end + 1;
        }
    }

    private record Escape(int scalar, int end) {}

    private static Escape escape(String text, int start) {
        if (start + 1 >= text.length()) throw new IllegalArgumentException();
        int end = start + 2;
        char c = text.charAt(start + 1);
        int basic = switch (c) {
            case '.', '"', '\\', '/' -> c;
            case 'b' -> '\b'; case 'f' -> '\f'; case 'n' -> '\n';
            case 'r' -> '\r'; case 't' -> '\t'; default -> -1;
        };
        if (basic >= 0) return new Escape(basic, end);
        if (c != 'u' || end >= text.length()) throw new IllegalArgumentException();
        int value;
        if (text.charAt(end) == '{') {
            int close = text.indexOf('}', end + 1);
            if (close < 0 || close - end < 2 || close - end > 7) throw new IllegalArgumentException();
            value = hex(text, end + 1, close); end = close + 1;
        } else {
            value = hex(text, end, end + 4); end += 4;
            if (value >= 0xd800 && value <= 0xdbff) {
                if (!text.startsWith("\\u", end)) throw new IllegalArgumentException();
                int low = hex(text, end + 2, end + 6);
                if (low < 0xdc00 || low > 0xdfff) throw new IllegalArgumentException();
                value = 0x10000 + (value - 0xd800) * 1024 + low - 0xdc00; end += 6;
            }
        }
        if (value > 0x10ffff || (value >= 0xd800 && value <= 0xdfff)) throw new IllegalArgumentException();
        return new Escape(value, end);
    }

    private static int hex(String text, int start, int end) {
        if (start >= end || end > text.length()) throw new IllegalArgumentException();
        int value = 0;
        for (int i = start; i < end; i++) {
            char c = text.charAt(i);
            int digit = c >= '0' && c <= '9' ? c - '0' :
                    c >= 'a' && c <= 'f' ? c - 'a' + 10 : c >= 'A' && c <= 'F' ? c - 'A' + 10 : -1;
            if (digit < 0) throw new IllegalArgumentException();
            value = value * 16 + digit;
        }
        return value;
    }

    private static int quoted(String text, int start, StringBuilder decoded) {
        if (start >= text.length() || text.charAt(start) != '"') throw new IllegalArgumentException();
        int literal = start + 1;
        for (int i = start + 1; i < text.length();) {
            char c = text.charAt(i);
            if (c == '"') {
                if (decoded != null) decoded.append(text, literal, i);
                return i + 1;
            }
            if (c == '\r' || c == '\n') throw new IllegalArgumentException();
            if (c == '\\') {
                Escape escaped = escape(text, i);
                if (decoded != null) decoded.append(text, literal, i).appendCodePoint(escaped.scalar);
                i = literal = escaped.end;
            } else i++;
        }
        throw new IllegalArgumentException();
    }

    /** Common string decoding; type annotations remain opaque. */
    public static String decodeString(Entry entry) throws ParseException {
        if (entry == null) throw problem(0, "string decoding");
        if (!entry.kind.equals("value") || !validScalars(entry.raw)) throw problem(entry.line, "string decoding");
        String raw = entry.raw;
        if (!entry.form.isEmpty()) {
            if (entry.unset) throw problem(entry.line, "string decoding");
            for (int i = 0; i < raw.length(); i++)
                if (raw.charAt(i) == '\r' && (i + 1 == raw.length() || raw.charAt(i + 1) != '\n'))
                    throw problem(entry.line, "string decoding");
            if (entry.form.equals("pipe")) return raw;
            if (!entry.form.equals("triple_quoted")) throw problem(entry.line, "string decoding");
            int escapeStart = raw.indexOf('\\');
            if (escapeStart < 0) return raw;
            StringBuilder result = new StringBuilder();
            int literal = 0;
            try {
                while (escapeStart >= 0) {
                    Escape escaped = escape(raw, escapeStart);
                    result.append(raw, literal, escapeStart).appendCodePoint(escaped.scalar);
                    literal = escaped.end;
                    escapeStart = raw.indexOf('\\', literal);
                }
            } catch (IllegalArgumentException error) { throw problem(entry.line, "string decoding"); }
            return result.append(raw, literal, raw.length()).toString();
        }
        raw = trim(raw);
        if (raw.indexOf('\r') >= 0 || raw.indexOf('\n') >= 0 || entry.unset != raw.isEmpty())
            throw problem(entry.line, "string decoding");
        if (raw.isEmpty() || raw.charAt(0) != '"') return raw;
        StringBuilder decoded = new StringBuilder();
        try {
            if (quoted(raw, 0, decoded) != raw.length()) throw new IllegalArgumentException();
        } catch (IllegalArgumentException error) { throw problem(entry.line, "string decoding"); }
        return decoded.toString();
    }

    private static boolean validScalars(String text) {
        for (int i = 0; i < text.length(); i++) {
            char c = text.charAt(i);
            if (Character.isHighSurrogate(c)) {
                if (++i >= text.length() || !Character.isLowSurrogate(text.charAt(i))) return false;
            } else if (Character.isLowSurrogate(c)) return false;
        }
        return true;
    }

    private static int spaces(String text) {
        int n = 0;
        while (n < text.length() && text.charAt(n) == ' ') n++;
        return n;
    }
    private static boolean white(int c) {
        return c >= 9 && c <= 13 || c == 32 || c == 133 || c == 160 || c == 5760
                || c >= 8192 && c <= 8202 || c == 8232 || c == 8233 || c == 8239 || c == 8287 || c == 12288;
    }
    private static String trimRight(String text) {
        int end = text.length();
        while (end > 0 && (text.charAt(end - 1) == ' ' || text.charAt(end - 1) == '\t')) end--;
        return text.substring(0, end);
    }
    private static String trimLeft(String text) {
        int start = 0;
        while (start < text.length() && (text.charAt(start) == ' ' || text.charAt(start) == '\t')) start++;
        return text.substring(start);
    }
    private static String trim(String text) { return trimLeft(trimRight(text)); }
    private static ParseException problem(int line, String reason) { return new ParseException(line, reason); }

    private static void jsonString(StringBuilder output, String text) {
        output.append('"');
        final char[] hex = "0123456789abcdef".toCharArray();
        for (int i = 0; i < text.length(); i++) {
            char c = text.charAt(i);
            if (c == '"' || c == '\\') output.append('\\').append(c);
            else if (c < 32 || c > 126) output.append("\\u").append(hex[c >> 12])
                    .append(hex[c >> 8 & 15]).append(hex[c >> 4 & 15]).append(hex[c & 15]);
            else output.append(c);
        }
        output.append('"');
    }

    /** Shared CLI JSON shape. Output is assembled only after all decoding succeeds. */
    public static String toJson(List<Entry> entries, boolean strings) throws ParseException {
        StringBuilder out = new StringBuilder("[");
        for (int i = 0; i < entries.size(); i++) {
            Entry entry = entries.get(i);
            if (i > 0) out.append(',');
            out.append("{\"line\":").append(entry.line).append(",\"kind\":"); jsonString(out, entry.kind);
            out.append(",\"path\":[");
            for (int j = 0; j < entry.path.size(); j++) {
                if (j > 0) out.append(',');
                jsonString(out, entry.path.get(j));
            }
            out.append("],\"raw\":"); jsonString(out, entry.raw);
            out.append(",\"unset\":").append(entry.unset).append(",\"has_type\":").append(entry.hasType);
            if (!entry.type.isEmpty()) { out.append(",\"type\":"); jsonString(out, entry.type); }
            if (!entry.form.isEmpty()) { out.append(",\"form\":"); jsonString(out, entry.form); }
            if (strings && entry.kind.equals("value")) {
                out.append(",\"decoded\":"); jsonString(out, decodeString(entry));
            }
            out.append('}');
        }
        return out.append(']').toString();
    }

    public static void main(String[] args) {
        try {
            String mode = "strings", file = null;
            for (int i = 0; i < args.length; i++) {
                if (args[i].equals("--mode") && i + 1 < args.length) mode = args[++i];
                else if (!args[i].startsWith("-") && file == null) file = args[i];
                else throw problem(0, "arguments");
            }
            if (!mode.equals("strings") && !mode.equals("raw")) throw problem(0, "arguments");
            List<Entry> entries;
            if (file == null) entries = parse(System.in);
            else try (InputStream source = Files.newInputStream(Path.of(file))) { entries = parse(source); }
            System.out.println(toJson(entries, mode.equals("strings")));
        } catch (ParseException error) {
            System.err.println(error.getMessage()); System.exit(1);
        } catch (Exception error) {
            System.err.println("dotReg: input or output failure"); System.exit(1);
        }
    }
}
