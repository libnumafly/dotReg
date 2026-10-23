import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

/** Native API checks; run with assertions enabled, no test framework required. */
public final class DotRegTest {
    private static ByteArrayInputStream document(String body) {
        return document(body.getBytes(StandardCharsets.UTF_8));
    }
    private static ByteArrayInputStream document(byte[] body) {
        byte[] header = "@dotReg 1\n\n".getBytes(StandardCharsets.UTF_8);
        byte[] result = new byte[header.length + body.length];
        System.arraycopy(header, 0, result, 0, header.length);
        System.arraycopy(body, 0, result, header.length, body.length);
        return new ByteArrayInputStream(result);
    }
    @FunctionalInterface private interface Action { void run() throws Exception; }
    private static DotReg.ParseException fails(Action action) throws Exception {
        try { action.run(); }
        catch (DotReg.ParseException error) { return error; }
        throw new AssertionError("expected parse failure");
    }
    private static void entriesAndScopes() throws Exception {
        List<DotReg.Entry> entries = DotReg.parse(document("A.B.\n  .\"C.D\"(list(int)): 001\n  .: x\n  .: x\n\"\":\n"));
        assert entries.stream().map(DotReg.Entry::path).toList().equals(List.of(
                List.of("A", "B"), List.of("A", "B", "C.D"), List.of("A", "B"),
                List.of("A", "B"), List.of("")));
        assert entries.get(1).type().equals("list(int)");
        assert entries.get(1).hasType();
        assert entries.get(1).line() == 4;
        assert entries.get(4).unset();
        List<List<String>> visited = new ArrayList<>();
        DotReg.parseEach(document("A.\n  .B.\n    .C:x\n  .D:y"), entry -> {
            visited.add(new ArrayList<>(entry.path()));
            entry.path().clear(); entry.path().add("mutated");
            return true;
        });
        assert visited.equals(List.of(List.of("A"), List.of("A", "B"),
                List.of("A", "B", "C"), List.of("A", "D")));
    }
    private static void stopAndErrors() throws Exception {
        List<DotReg.Entry> visited = new ArrayList<>();
        DotReg.parseEach(document(new byte[]{'A', ':', 'x', '\n', (byte) 0xff}), entry -> {
            visited.add(entry); return false;
        });
        assert visited.size() == 1;
        Exception sentinel = new Exception("recipient-owned error");
        try {
            DotReg.parseEach(document("A:x\ninvalid"), entry -> { throw sentinel; });
            throw new AssertionError();
        } catch (Exception error) { assert error == sentinel; }
        for (String prefix : List.of("A:\n", "A:\n |body\n")) {
            byte[] start = prefix.getBytes(StandardCharsets.UTF_8);
            byte[] data = java.util.Arrays.copyOf(start, start.length + 1);
            data[start.length] = (byte) 0xff;
            visited.clear();
            DotReg.parseEach(document(data), entry -> { visited.add(entry); return false; });
            assert visited.size() == 1;
            fails(() -> DotReg.parse(document(data)));
        }
        visited.clear();
        DotReg.ParseException invalidPipe = fails(() -> DotReg.parseEach(
                document(new byte[]{'A', ':', '\n', ' ', '|', (byte) 0xff}),
                entry -> { visited.add(entry); return false; }));
        assert invalidPipe.line == 4 && visited.isEmpty();
        InputStream source = new InputStream() {
            public int read() throws IOException { throw new IOException("SECRET INPUT VALUE"); }
            public int read(byte[] buffer, int start, int count) throws IOException {
                throw new IOException("SECRET INPUT VALUE");
            }
        };
        assert !fails(() -> DotReg.parse(source)).getMessage().contains("SECRET");
        fails(() -> DotReg.parse(null));
        fails(() -> DotReg.parseEach(document("A:x"), null));
    }
    private static void multilineAndUnicode() throws Exception {
        List<DotReg.Entry> entries = DotReg.parse(document("A:\"\"\"\r\n  first\\nsecond\r\n    tail\n  \"\"\"\r\nB:\n |one\r\n |\nC: last"));
        assert entries.get(0).raw().equals("first\\nsecond\r\n  tail");
        assert DotReg.decodeString(entries.get(0)).equals("first\nsecond\r\n  tail");
        assert entries.get(1).raw().equals("one\r\n");
        assert entries.get(1).form().equals("pipe");
        assert !entries.get(1).unset();
        assert entries.get(2).path().equals(List.of("C"));
        entries = DotReg.parse(document("\"\\u{1F600}\".\"\\u304C\": \"\\uD83D\\uDE00\\.\\t\"\nA: literal\"\\q\nB:\"\"\nC:\n"));
        assert entries.get(0).path().equals(List.of("😀", "が"));
        assert DotReg.decodeString(entries.get(0)).equals("😀.\t");
        assert DotReg.decodeString(entries.get(1)).equals("literal\"\\q");
        assert !entries.get(2).unset() && entries.get(3).unset();
        DotReg.Entry invalid = DotReg.parse(document("A:\"\\q\"")).get(0);
        fails(() -> DotReg.decodeString(invalid));
        fails(() -> DotReg.decodeString(new DotReg.Entry(3, "value", List.of("A"),
                "\ud800", false, "", false, "")));
    }
    private static void invalidInputsAndFreshState() throws Exception {
        for (String text : List.of("", "@dotReg 1\n", "\n", "@dotReg 1\n@dotReg 1\n\n",
                "@dotReg 2\n\n", "@other 1\n\n", "@dotReg 1\n\nA:x\r",
                "@dotReg 1\n\nA:\"\"\"\n x\n  \"\"\"")) {
            fails(() -> DotReg.parse(new ByteArrayInputStream(text.getBytes(StandardCharsets.UTF_8))));
        }
        assert DotReg.parse(new ByteArrayInputStream("\ufeff@dotReg 1\n\n".getBytes(StandardCharsets.UTF_8))).isEmpty();
        String longValue = "x".repeat(200_000);
        assert DotReg.parse(document("A:" + longValue)).get(0).raw().equals(longValue);
        DotReg.parse(document("A."));
        fails(() -> DotReg.parse(document("  .B:x")));
        DotReg.Entry heading = DotReg.parse(document("A.")).get(0);
        fails(() -> DotReg.decodeString(heading));
    }
    private static void borrowedSlicesAndSpanningLines() throws Exception {
        String body = "#" + "x".repeat(8170) + "\r\nA:\r\n |one\r\n |two\r\n"
                + "B:\"😀\"\nC:" + "long".repeat(5000) + "\r\nD:tail";
        byte[] bytes = document(body).readAllBytes();
        String expected = DotReg.toJson(DotReg.parse(new ByteArrayInputStream(bytes)), true);
        for (int chunk : new int[]{1, 2, 7, 8191, 8192}) {
            InputStream source = new ByteArrayInputStream(bytes) {
                public synchronized int read(byte[] target, int start, int count) {
                    return super.read(target, start, Math.min(count, chunk));
                }
            };
            List<DotReg.Entry> entries = DotReg.parse(source);
            assert DotReg.toJson(entries, true).equals(expected);
            assert entries.get(0).raw().equals("one\r\ntwo");
            assert entries.get(1).path().equals(List.of("B"));
            assert entries.get(2).raw().length() == 20_000;
            assert entries.get(3).raw().equals("tail");
        }
    }
    public static void main(String[] args) throws Exception {
        boolean assertions = false;
        assert assertions = true;
        if (!assertions) throw new IllegalStateException("run with -ea");
        entriesAndScopes(); stopAndErrors(); multilineAndUnicode(); invalidInputsAndFreshState();
        borrowedSlicesAndSpanningLines();
        System.out.println("Java API tests passed");
    }
}
