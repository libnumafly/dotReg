# Getting started

dotReg describes settings as key paths and values. Each key and value header occupies one line; explicit blocks allow a value to span several lines. You can write each key in full or group related settings beneath a heading.

This guide introduces the notation. The [reference specification](config-format.md) defines the complete rules, and [section 12](config-format.md#12-undecided-matters) records what remains undecided.

## Start with a full key

A colon separates a key from its value. Dots separate the elements of a key path:

```text
A.Code:001
A.Count:42
```

These lines assign strings to `A.Code` and `A.Count`. Values start as strings: `001` keeps its leading zeroes, and `42` does not automatically become a number. A key can also consist of a single element.

## Group related settings

A heading ends with a dot. It supplies a prefix without assigning a value. Relative lines begin with a dot and are indented beneath their heading:

```text
A.
 .Code:001
 .Count:42
```

This is an alternative input to the first example: the resolved keys and values are the same. Try the examples separately when comparing their notation; combining them records two occurrences of each key and retains both values.

Use one additional **ASCII space** for each written heading level. Tabs cannot replace these spaces. Indentation follows the headings you write, rather than the number of dot-separated key elements:

```text
A.B.
 .C.
  .Code:001
```

Here `A.B.` occupies one heading level, so `.C.` has one leading space. `.Code` has two and resolves to `A.B.C.Code`. A value line does not create another heading level.

## Keep repeated values

A key can appear repeatedly. Every value occurrence is kept in source order, even when the same value appears again:

```text
A.Tag:red
A.Tag:green
A.Tag:red
```

The values of `A.Tag` are `red`, `green`, `red`. No occurrence is overwritten or deduplicated. Fully qualified keys, relative keys and equivalent quoted spellings use the same key identity. A heading alone adds no value. Each occurrence keeps its own type annotation and unset state; an application may require a particular key to occur only once through its schema.

See [repeated values and read state](config-format.md#9-repeated-values-and-read-state). The [Go sample](../implementations/go/README.md#using-the-library) offers collection and indexed retrieval; its exact API remains provisional.

## Write strings naturally

Unquoted values preserve backslashes. Double quotes preserve surrounding spaces and allow escapes, including Unicode escapes:

```text
A.Path: C:\temp\new
A.Message: "  hello  "
A.Emoji: "\u{1F600}"
A."Display.Name": "Caf\u{E9}"
```

`A.Path` contains the literal Windows path `C:\temp\new`; `\t` and `\n` are not escapes outside quotes. `A.Message` retains its two spaces at each end. `A.Emoji` decodes to `😀`, and the final value decodes to `Café`.

Quote an entire key element when its name contains a dot or another syntax character. The last key has two elements, `A` and `Display.Name`, rather than three. Key names are case-sensitive, and Unicode normalisation is not performed.

See [key paths](config-format.md#3-key-paths) and [common escapes](config-format.md#34-common-escapes-within-quotes) for the detailed rules.

## Write multiline strings

Use triple quotes when a multiline value should support the same escapes as an ordinary quoted string:

```text
A.Message: """
  Hello, \u{1F600}
    second line
  """
```

The opening marker has no inline content. The closing marker occupies its own line, and its two leading ASCII spaces set the margin removed from each non-empty body line. With LF source endings, this value is `Hello, 😀`, one LF and two spaces followed by `second line`. Empty source lines are exempt from margin removal; non-empty lines with too little margin are errors. A zero-space closing margin is allowed when pasting text directly.

Use a pipe block for literal text, including paths, backslashes or quote characters:

```text
A.Literal:
 |C:\temp\new
 |"quotes" and \q stay literal
 |# this is content
A.Next: value
```

Only the physical line immediately after an empty header can start this block. It must begin with more ASCII spaces than the header, followed by `|`. Consecutive lines meeting that rule belong to the body. Their leading spaces and first pipe are removed; all remaining characters are preserved without escapes or trimming. `A.Next` ends the block and is read normally. Blank lines and comments are not skipped to find a pipe block. `A:|` is simply the single-line value `|`.

Both forms preserve the original LF or CRLF between body lines. They exclude boundary line endings and add no terminal newline automatically. Add an empty body line before a triple-quoted closing marker, or add an empty final pipe, when a terminal newline is needed:

```text
A.WithNewline:
 |text
 |
A.EmptyBlock:
 |
```

`A.WithNewline` is `text` followed by one source line ending. `A.EmptyBlock` is an explicit empty string, not unset. In a pipe block, spaces after the marker are content: ` |  text` retains two leading spaces. Multiline bodies neither create headings nor change their scope. Type annotations and assignment to a heading itself work with both forms.

See [triple-quoted values](config-format.md#83-triple-quoted-multiline-values) and [pipe blocks](config-format.md#84-literal-pipe-blocks) for exact closing-marker and indentation rules.

## Distinguish unset from empty

An empty right-hand side leaves a value unset unless its immediately following line starts a pipe block. An empty quoted string or empty multiline body specifies a value explicitly:

```text
A.Missing:
A.Empty: ""
A.NullText: null
```

All three keys are present. `A.Missing` is unset, `A.Empty` is an explicit empty string, and `A.NullText` is the string `null`. When retrieval as a string is permitted, unset is equivalent to an empty string; its unset state remains available separately.

An omitted key and a heading alone are different again: neither assigns a value. Your application's schema decides which keys are required and whether unset or empty values are acceptable. See [value interpretation](config-format.md#8-preserving-right-hand-sides-and-interpreting-values).

## Add types when a consumer supports them

A value line can include a type annotation between its key and colon:

```text
A.Count(int):42
```

This requests conversion according to a declared type expression. Type conversion follows syntax parsing and common string decoding; it is a separate processing stage. The meaning of `int` here comes from the Go sample's experimental profile. Standard type names and individual conversion rules remain undecided.

The [type annotation rules](config-format.md#6-type-annotations) define the syntax, while the [Go guide](../implementations/go/README.md#type-conversion-and-the-callers-configuration-schema) explains that sample's profile and retrieval behaviour.

## Try the Go sample

With Go 1.22 or later, run these commands from the repository root:

```sh
cd implementations/go
go run ./cmd/configsample ../../examples/sample.txt
```

The default mode prints JSON showing parsed entries, their unset state and decoded strings. It does not convert annotated values. Use the [Go sample guide](../implementations/go/README.md) for the experimental conversion mode, standard input and library API. The [example configuration](../examples/sample.txt) provides a larger input to explore.

Return to the [documentation index](README.md) or the [project introduction](../README.md).
