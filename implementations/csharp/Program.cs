using System.Text.Json;
using DotReg;

return Run(args);

static int Run(string[] args)
{
    var mode = "strings";
    string? filename = null;
    for (var i = 0; i < args.Length; i++)
    {
        if (args[i] == "--mode" && i + 1 < args.Length)
        {
            mode = args[++i];
            if (mode is not ("raw" or "strings")) return Usage();
        }
        else if (args[i].StartsWith('-') && args[i] != "-") return Usage();
        else if (filename is null) filename = args[i];
        else return Usage();
    }

    try
    {
        using var source = filename is null or "-"
            ? Console.OpenStandardInput()
            : File.OpenRead(filename);
        var entries = Parser.Parse(source);
        if (mode == "strings")
            foreach (var entry in entries)
                if (entry.Kind == "value") entry.Decoded = Parser.DecodeString(entry).Value;
        // Serialisation also completes before stdout is touched.
        var output = JsonSerializer.SerializeToUtf8Bytes(entries);
        using var stdout = Console.OpenStandardOutput();
        stdout.Write(output);
        stdout.WriteByte((byte)'\n');
        return 0;
    }
    catch (ParseException error)
    {
        Console.Error.WriteLine(error.Message);
        return 1;
    }
    catch (Exception)
    {
        // Filenames, source text and platform exception strings can be sensitive.
        Console.Error.WriteLine("dotReg: input or output failure");
        return 1;
    }
}

static int Usage()
{
    Console.Error.WriteLine("usage: DotReg.Sample [--mode raw|strings] [file|-]");
    return 2;
}
