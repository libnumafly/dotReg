# Getting started

dotReg uses `key: value` lines. Dots separate key elements. Headings group related keys, and explicit blocks hold multiline values.

This guide introduces the notation. The [reference specification](config-format.md) defines the core rules, and [undecided matters](open-issues.md) lists the parts still open.

## Contents

- [Start with file properties](#start-with-file-properties)
- [Start with a full key](#start-with-a-full-key)
- [Group related settings](#group-related-settings)
- [Keep repeated values](#keep-repeated-values)
- [Write strings naturally](#write-strings-naturally)
- [Write multiline strings](#write-multiline-strings)
- [Distinguish unset from empty](#distinguish-unset-from-empty)
- [Add types when a consumer supports them](#add-types-when-a-consumer-supports-them)
- [Try the Go sample](#try-the-go-sample)

## Start with file properties

Every complete file begins with this declaration and a blank line:

```text
@dotReg 1

A.Code:001
```

The declaration and blank line are required. `@dotReg 1` is the only declaration currently supported. The first setting, `A.Code`, is on physical line 3.

The examples below are body fragments, so they omit the declaration and blank line. Add both when trying an example with a parser. See [file property declarations](config-format.md#11-file-property-declarations) for the detailed rules.

## Start with a full key

A colon separates a key from its value. Dots separate key elements:

```text
A.Code:001
A.Count:42
```

These lines set the strings `001` and `42` under `A.Code` and `A.Count`. Values do not become numbers automatically. A key can also have just one element, as in `A: root`.

To include a dot in one key element, quote the whole element: `A."Display.Name": value`. See [key paths](config-format.md#3-key-paths) for other quoted names and key rules.

## Group related settings

A heading ends with a dot and supplies a shared prefix. Relative lines begin with a dot and are indented beneath the heading:

```text
A.
  .Code:001
  .Count:42
```

This has the same result as the full-key example above. Include both examples in one file only when you want to record each value twice.

Use exactly two additional **ASCII spaces** for each heading level. Odd numbers of leading spaces are invalid on headings and value lines, and tabs cannot replace these spaces. The number of levels follows the headings you write, not the number of dots in a key:

```text
A.B.
  .C.
    .Code:001
```

`A.B.` is one heading level, so `.C.` has two leading spaces and `.Code` has four. The value belongs to `A.B.C.Code`.

## Keep repeated values

A key can appear more than once. Every value is retained in source order, including identical values:

```text
A.Tag:red
A.Tag:green
A.Tag:red
```

The values of `A.Tag` are `red`, `green`, `red`. An application can require a key to appear only once through its configuration schema. See [repeated values and read state](config-format.md#9-repeated-values-and-read-state). The [Go sample](../implementations/go/README.md#using-the-library) also shows collection and indexed retrieval.

## Write strings naturally

After outer spaces and tabs are trimmed, a value that starts with `"` must be quoted in full. Escapes are decoded inside it. Any other starting character makes the whole value literal, including later quotes and backslashes:

```text
A.Path: C:\temp\new
A.Message: "  hello  "
A.Saying: say "hello"
A.Emoji: "\u{1F600}"
```

`A.Path` keeps its backslashes, `A.Message` keeps its surrounding value spaces, and `A.Saying` keeps its embedded quotes. `A.Emoji` becomes `😀`. See [common escapes](config-format.md#34-common-escapes-within-quotes) for the full escape list and [single-line values](config-format.md#82-double-quoted-values-and-common-string-decoding) for quoting details.

## Write multiline strings

Use triple quotes for multiline text that needs the same escapes as an ordinary quoted string:

```text
A.Message: """
  Hello, \u{1F600}
    second line
  """
```

The closing marker goes on its own line. Its leading spaces set the margin removed from non-empty body lines. This margin is independent of heading indentation and may be odd or zero. The body decodes to `Hello, 😀`, a line break, and two spaces followed by `second line`.

Use a pipe block when the body should stay literal:

```text
A.Literal:
 |C:\temp\new
 |"quotes" and \q stay literal
 |# this is content
A.Next: value
```

The first pipe line must immediately follow the empty header and have more leading ASCII spaces than it. Pipe-body indentation is not subject to the two-space hierarchy rule: the single leading space in this example is sufficient. Each body line loses its indentation and first `|`; the remaining text is kept as written. `A.Next` is then read as a normal setting.

Neither block form adds a final line break automatically. See [triple-quoted values](config-format.md#83-triple-quoted-multiline-values) and [pipe blocks](config-format.md#84-literal-pipe-blocks) for exact indentation and line-ending rules.

## Distinguish unset from empty

An empty right-hand side leaves the value unset unless the next physical line starts a pipe block. An empty quoted value explicitly sets an empty string, and the word `null` is an ordinary string:

```text
A.Missing:
A.Empty: ""
A.NullText: null
```

All three keys exist. When a string is requested and the declaration permits it, `A.Missing` is returned as an empty string; its unset state is still available. An omitted key or a heading alone has no value. Your application's schema decides which keys are required and whether unset or empty values are allowed. See [value interpretation](config-format.md#8-preserving-right-hand-sides-and-interpreting-values).

## Add types when a consumer supports them

An optional type annotation goes between the key and colon:

```text
A.Count(int):42
A.Label:42
```

Values are decoded as strings first. A declaration converts that occurrence to its declared type. Without a declaration, a consumer may request conversion when it reads the value:

| Setting | Requested type | Result |
|---|---|---|
| `A.Count(int):42` | `int` | Use the declared `int` conversion |
| `A.Count(int):42` | `string` | Type mismatch |
| `A.Label:42` | `int` | Try converting the string when read; failure is a type mismatch |
| `A.Label:42` | `string` | Return the string |

Each occurrence has its own annotation. The names and behaviour of standard types are still undecided; `int` and `string` here illustrate the rules and the Go sample's experimental profile. See [type annotation rules](config-format.md#6-type-annotations), [type profiles and candidate types](type-profile.md) and [the Go guide](../implementations/go/README.md#type-conversion-and-the-callers-configuration-schema).

## Try the Go sample

With Go 1.22 or later, run these commands from the repository root:

```sh
cd implementations/go
go run ./cmd/configsample ../../examples/sample.txt
```

The default mode prints the parsed entries, their unset state and decoded strings. It does not convert annotated values. See the [Go sample guide](../implementations/go/README.md) for its conversion mode and library API, and explore the [example configuration](../examples/sample.txt).

Return to the [documentation index](README.md) or the [project introduction](../README.md).
