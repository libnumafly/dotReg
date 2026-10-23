# Getting started

dotReg describes settings as key paths and values, one value per line. You can write each key in full or group related settings beneath a heading.

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

This is an alternative input to the first example: the resolved keys and values are the same. Try the examples separately; combining them would assign the same keys twice, which is an error.

Use one additional **ASCII space** for each written heading level. Tabs cannot replace these spaces. Indentation follows the headings you write, rather than the number of dot-separated key elements:

```text
A.B.
 .C.
  .Code:001
```

Here `A.B.` occupies one heading level, so `.C.` has one leading space. `.Code` has two and resolves to `A.B.C.Code`. A value line does not create another heading level.

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

## Distinguish unset from empty

An empty right-hand side leaves a value unset. An empty quoted string specifies a value explicitly:

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
