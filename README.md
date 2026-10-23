# dotReg

**Readable configuration, built on dotted keys.**

dotReg is a line-oriented configuration notation. Write full keys directly, or group them beneath a shared heading. Values start as text; your application decides how to interpret them.

```text
@dotReg 1

A.
  .Code:001
  .Message: "Hello, dotReg"
  .Group.
    .Enabled:true
```

The grouped entries resolve to `A.Code`, `A.Message` and `A.Group.Enabled`. Each heading level adds exactly two ASCII spaces. The code `001` stays text, with its leading zeroes intact.

Every file starts with the exact declaration `@dotReg 1` and a required blank separator. Declarations are file properties, separate from settings. The shorter notation examples below show body fragments; prepend the same declaration and blank line when parsing them. See [file properties](docs/config-format.md#11-file-property-declarations) for validation rules.

For a single-line value, only an initial double quote after outer ASCII spaces and tabs are removed selects quoted syntax and escapes. Other values are entirely literal: `A.Message: say "hello"` keeps its quotes, and `A.Path: C:\temp\new` keeps its backslashes. Quotes encountered later never switch modes.

Repeat a key to record multiple values in source order:

```text
A.Tag:red
A.Tag:green
A.Tag:red
```

All three occurrences remain, including the repeated `red`. Later values do not overwrite earlier ones. Your application can require a single value for particular keys through its configuration schema.

Multiline values support triple quotes with the same escapes as ordinary quoted values, or a pipe block whose text is preserved literally:

```text
A.Message: """
  first line
  say \"hello\"
  """

A.Path:
 |C:\temp\new
 |another line
```

The closing triple quotes set the indentation margin to remove. In a pipe block, everything after the first `|` belongs to the value, including spaces and backslashes. Both forms preserve line endings between body lines. To include a final newline, add an empty body line before the closing triple quotes, or an empty final pipe line (` |`). See the [value rules](docs/config-format.md#8-preserving-right-hand-sides-and-interpreting-values).

[Get started](docs/getting-started.md) · [Documentation](docs/README.md) · [Specification](docs/config-format.md) · [Sample parsers](implementations/README.md)

Try the notation in Go, Python, JavaScript, TypeScript, Java, C#, Rust or C++.
The [parser set](implementations/README.md) provides dependency-free syntax and
common string decoding, incremental APIs for bounded retention, shared
conformance cases and [reproducible time and memory measurements](implementations/benchmarks/README.md).
TypeScript shares the JavaScript engine through a typed interface.

Website development and GitHub Pages setup: [Starlight website guide](pages/README.md).

**In development.** The specification defines the core rules. [Type profiles and candidate types](docs/type-profile.md) records type-specific representations and candidate names; [undecided matters](docs/open-issues.md) lists the questions still open.

The specification, examples and sample implementations are available under the [MIT License](LICENSE).
