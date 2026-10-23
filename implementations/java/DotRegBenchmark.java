import java.io.FileInputStream;
import java.util.List;
import java.util.Locale;

/** Warmed API benchmark: fixture iterations stream|collect. */
public final class DotRegBenchmark {
    private static volatile long sink;
    private record Counts(long entries, long decodedCharacters) {}

    private static Counts measure(FileInputStream source, String mode) throws Exception {
        source.getChannel().position(0);
        long[] counts = new long[2];
        DotReg.Visitor consume = entry -> {
            counts[0]++;
            if (entry.kind().equals("value")) counts[1] += DotReg.decodeString(entry).length();
            return true;
        };
        if (mode.equals("stream")) DotReg.parseEach(source, consume);
        else {
            List<DotReg.Entry> entries = DotReg.parse(source);
            for (DotReg.Entry entry : entries) consume.visit(entry);
        }
        sink = counts[1];
        return new Counts(counts[0], counts[1]);
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 3) throw new IllegalArgumentException("fixture iterations stream|collect");
        int iterations = Integer.parseInt(args[1]);
        String mode = args[2];
        if (iterations < 1 || !(mode.equals("stream") || mode.equals("collect")))
            throw new IllegalArgumentException("invalid benchmark arguments");
        try (FileInputStream source = new FileInputStream(args[0])) {
            long inputBytes = source.getChannel().size();
            Counts expected = measure(source, mode);
            if (!measure(source, mode).equals(expected)) throw new IllegalStateException("inconsistent benchmark result");
            long started = System.nanoTime();
            for (int i = 0; i < iterations; i++)
                if (!measure(source, mode).equals(expected)) throw new IllegalStateException("inconsistent benchmark result");
            double elapsed = (System.nanoTime() - started) / 1e9;
            System.out.printf(Locale.ROOT, "{\"language\":\"java\",\"mode\":\"%s\",\"iterations\":%d,"
                    + "\"input_bytes\":%d,\"entries\":%d,\"elapsed_seconds\":%.9f}%n",
                    mode, iterations, inputBytes, expected.entries, elapsed);
        }
    }
}
