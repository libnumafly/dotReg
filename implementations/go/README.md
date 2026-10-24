# Go sample implementation

Run all commands in this document from `implementations/go`. See the [project overview](../../README.md) for an introduction to the notation and the [documentation](../../docs/README.md) for guides and the repository layout.

This is a sample Go implementation of [config-format.md](../../docs/config-format.md). It uses no external libraries. Parsing, common string decoding and type conversion are separate stages so that the notation can be tried independently of a particular application.

Standard type names and the public API remain undecided. The Go package is `dotreg`; the module path is `github.com/libnumafly/dotReg/implementations/go`. The implementation makes the provisional choices described below for parts of the syntax that remain undecided.

## Contents

- [File properties](#file-properties)
- [Running the sample](#running-the-sample)
- [Using the library](#using-the-library)
  - [Selecting values by key prefix](#selecting-values-by-key-prefix)
- [Single-line strings](#single-line-strings)
- [Multiline values](#multiline-values)
- [Type conversion and the caller's configuration schema](#type-conversion-and-the-callers-configuration-schema)
- [Supported rules and provisional choices](#supported-rules-and-provisional-choices)
- [Verification](#verification)
- [Licence](#licence)

## File properties

Every parsed file or standard-input stream must start with a declaration region and its terminating blank line. Currently the only supported declaration is `@dotReg 1`:

```text
@dotReg 1

A.Code:001
```

Declarations begin in column one. Separate the exact property name `@dotReg` and the exact version value `1` with one or more ASCII spaces or horizontal tabs; trailing ASCII spaces or tabs are allowed. No quotes or escapes are interpreted. The first blank line, containing at most ASCII spaces and tabs, ends the region. Comments, settings and indentation are not accepted before it. One initial UTF-8 BOM is allowed.

The declaration and blank separator are mandatory, and a property cannot repeat. Missing or malformed declarations and a missing separator produce `ErrSyntax`; a repeated property produces `ErrDuplicate`; unknown properties or unsupported versions produce `ErrUnsupported`. `@dotReg 1\n` alone is incomplete, while `@dotReg 1\n\n` is a valid empty-body file. A genuinely empty file is invalid.

Declarations produce no `Entry` or callback. They count towards physical line numbers, so the first setting in the complete example is line 3. After the separator, `@Name: value` is an ordinary key, a standalone `@dotReg 1` is invalid body syntax, and declaration-looking text in a multiline body remains data. This format version is separate from an application schema version.

Unless labelled as complete files, the short notation examples later in this guide are body fragments; prepend the declaration and blank separator when parsing them. See [file property declarations](../../docs/config-format.md#11-file-property-declarations).

## Running the sample

Use Go 1.22 or later. From `implementations/go`, run:

```sh
go run ./cmd/configsample ../../examples/sample.txt
```

By default, the CLI writes JSON containing each parsed occurrence's key path, undecoded right-hand side or block body, unset state and type annotation, together with the result of common string decoding. Repeated keys remain separate entries in source order, including identical repeated values. Multiline entries also include their `form`. This mode does not convert values, even when a type annotation is present.

To try type conversion, select the experimental type profile:

```sh
go run ./cmd/configsample -mode demo ../../examples/sample.txt
```

In [examples/sample.txt](../../examples/sample.txt), `A.Code(string):001` becomes the string `"001"`, `A.Count(int):42` becomes the integer `42`, and `A.Enabled(bool):true` becomes the Boolean `true`. Values without a type annotation remain strings in this mode as well.

To inspect only the parsing result:

```sh
go run ./cmd/configsample -mode raw ../../examples/sample.txt
```

Omit the filename or pass `-` to read standard input. File input reads UTF-8 bytes directly, so files containing Japanese or other non-ASCII text can be supplied in the same way.

For example, in PowerShell:

```powershell
"@dotReg 1`n`nA.Count(int):42" | go run ./cmd/configsample -mode demo -
```

For input containing non-ASCII characters, passing a UTF-8 file avoids dependence on the shell's pipe encoding.

| Mode | Processing | Value fields in the output |
|---|---|---|
| `raw` | Parsing; value quotes and escapes remain undecoded | `raw` |
| `strings` (default) | Parsing and common string decoding | `raw` and `decoded` |
| `demo` | Common string decoding, then conversion to annotated types with the experimental profile | `raw` and `value` |

`path` is an array of strings. It distinguishes `["A.B"]` from `["A", "B"]`, and `[""]` from an empty array. `unset` is `true` for an unset value and `false` for explicit empty quoted or block values. For a block, `raw` is its undecoded body after removing markers and its prescribed margin; it excludes boundary line endings and preserves LF or CRLF between body lines. The optional JSON field `form` is `"triple_quoted"` or `"pipe"` for blocks and omitted for ordinary single-line entries. A JSON `\\` or `\n` is part of JSON's representation; it does not change the configuration input syntax.

The `raw` mode can retain an unterminated ordinary quote in a single-line value or an invalid escape in a complete triple-quoted body. A missing triple-quoted closing marker or insufficient body margin is already a parsing failure. Always use `DecodeString` or `Build` before passing configuration values to a consumer. The CLI's `strings` and `demo` modes write output only after all processing succeeds, so a failure does not produce partial JSON.

## Using the library

| API | Purpose |
|---|---|
| `Parse(io.Reader) ([]Entry, error)` | Collect parsed entries; return no partial result on failure |
| `ParseEach(io.Reader, func(Entry) error) error` | Notify the callback of headings and complete values in source order, with header line numbers |
| `DecodeString(Entry) (Text, error)` | Decode according to the value form: ordinary quotes, triple-quoted escapes or a literal pipe body |
| `Build([]Entry, Profile) (*Config, error)` | Decode strings and convert annotated values only to their declared type |
| `Config.Get([]string, string) (any, error)` | Retrieve exactly one occurrence; report `ErrMultipleValues` if the key has more than one |
| `Config.GetAll([]string, string) ([]any, error)` | Retrieve every occurrence in source order as the requested type; return no partial values if any occurrence fails |
| `Config.GetAt([]string, int, string) (any, error)` | Retrieve a zero-based occurrence within a key's source-ordered values |
| `Config.SelectPrefix([]string) ([]SelectedValue, error)` | Select values at the prefix and below it in global source order, retaining full and relative paths and per-occurrence metadata without further conversion |
| `Config.IsUnset([]string) (bool, error)` | Query the unset state of exactly one occurrence; report `ErrMultipleValues` if there is more than one |
| `Config.IsUnsetAll([]string) ([]bool, error)` | Query every occurrence's unset state in source order without retrieving or converting values |

`Entry` has `Line`, `Kind`, `Path`, `Raw`, `Unset`, `Type`, `HasType` and `Form` fields. `Line` identifies the heading or value header, including for a multiline body, counted from the beginning of the file including declarations and the separator. Those declarations do not create entries. `Kind` is either `Heading` or `Value`. A heading alone does not create a value. `Form` is a `ValueForm`: its zero value denotes ordinary single-line handling, `TripleQuoted` is `"triple_quoted"` and `PipeBlock` is `"pipe"`. Preserve `Form` along with `Raw` when retaining or constructing entries; block contents alone must not be inspected to guess the value syntax. For blocks, `Raw` is already stripped of structural markers and margin, and `Unset` is always false, even for an empty body. `Text` holds the decoded `Value string` and `Unset bool`.

The following fragment demonstrates retrieval. See [example_test.go](example_test.go) for executable examples that include error handling.

```go
entries, err := dotreg.Parse(strings.NewReader(
    "@dotReg 1\n\nA.Code(string):001\nA.Count:42\nA.Missing:\nA.Empty:\"\"\n",
))
if err != nil {
    return err
}
config, err := dotreg.Build(entries, dotreg.DemoProfile())
if err != nil {
    return err
}
code, err := config.Get([]string{"A", "Code"}, "string") // "001"
// Handle code and err.
count, err := config.Get([]string{"A", "Count"}, "int") // int64(42)
// Handle count and err.
```

Repeated keys retain every occurrence. For this input:

```text
A.Tag:red
A.Tag:green
A.Tag:red
```

build the configuration as above, then use:

```go
tags, err := config.GetAll([]string{"A", "Tag"}, "string") // []any{"red", "green", "red"}
// Handle tags and err.
second, err := config.GetAt([]string{"A", "Tag"}, 1, "string") // "green"
// Handle second and err.
unset, err := config.IsUnsetAll([]string{"A", "Tag"}) // []bool{false, false, false}
// Handle unset and err.
```

`Get` and `IsUnset` require exactly one occurrence. They return `ErrMultipleValues` (`multiple_values`) for two or more, with the second occurrence's header line in the diagnostic; they do not silently select the first or last. A missing key returns `ErrMissing`. `GetAt` uses a zero-based index within that key, and returns `ErrMissing` for a missing key, a negative index or an out-of-range index.

`GetAll` applies the requested type to each occurrence independently, following the declaration rules below. If any occurrence has a different declared type or cannot be converted as requested, it returns a type mismatch error and no partial values. `IsUnsetAll` only reports states and does not perform type conversion. These API names, singular-retrieval errors and common-type aggregate-failure choices belong to this provisional Go sample; they do not establish the notation's public API or a universal policy for aggregate requests for one type. Prefix selection instead follows the established rules described below and keeps mixed declared types without requesting conversion.

Requesting the annotated `A.Code` as `int` returns `ErrTypeMismatch`, regardless of whether conversion would be possible. The unannotated `A.Count` can be retrieved as `"42"` when requesting `string`, or as `int64(42)` when requesting `int`. Retrieval conversions are not cached; the original string and unset state are retained.

`A.Missing` and `A.Empty` both return a zero-length string when requested as `string`. `IsUnset` returns `true` and `false`, respectively. To address a key with an empty name, pass `[]string{""}`. An empty path, whether `nil` or `[]string{}`, does not mean a key with an empty name.

The notation does not perform Unicode normalisation. Key elements are compared exactly after quote and escape decoding, using the same rule for grouping occurrences, retrieval and matching against a configuration schema. For example, the precomposed character `が` (U+304C) and the sequence `か` plus a combining voiced sound mark (U+304B U+3099) are different names. Type expressions and values are not implicitly normalised either.

<a id="selecting-descendants-by-key-prefix"></a>

### Selecting values by key prefix

`SelectPrefix` implements [section 9.1 of the specification](../../docs/config-format.md#91-prefix-selection). Pass decoded key elements to select the prefix's own values and all descendant values at any depth. A heading alone contributes no value. Matches retain their original global source order and repeated occurrences, with no sorting or deduplication.

For this body fragment:

```text
ip.filter:
ip.filter.
  .100099: pass * * * * *
  ."": unnamed
  .101000: reject 10.0.0.0/8 * * * *
  .100099: pass * * * * *
```

build the configuration as above, then select the group:

```go
filters, err := config.SelectPrefix([]string{"ip", "filter"})
if err != nil {
    return err
}
for _, filter := range filters {
    fmt.Printf("%q: %#v (unset=%t)\n", filter.RelativePath, filter.Value, filter.Unset)
}
// []: "" (unset=true)
// ["100099"]: "pass * * * * *" (unset=false)
// [""]: "unnamed" (unset=false)
// ["101000"]: "reject 10.0.0.0/8 * * * *" (unset=false)
// ["100099"]: "pass * * * * *" (unset=false)
```

The returned `SelectedValue` has these fields:

| Field | Meaning |
|---|---|
| `Path []string` | Full decoded key elements |
| `RelativePath []string` | Key elements after removing the selected prefix; `[]` for the prefix itself and `[""]` for an empty-named child |
| `Value any` | The declared converted value, or the decoded string when no type is declared |
| `Type string` / `HasType bool` | The occurrence's original type annotation and whether one was present |
| `Unset bool` | The occurrence's original unset state |
| `Line int` | One-based physical value-header line, counting declarations and blank lines |

Selection performs no new conversion and can return mixed declared types. It preserves unset separately from an explicit empty string. In the example, the prefix's untyped unset occurrence has `Value == ""` and `Unset == true`. `GetAll` instead addresses one exact key and requests a common type; use ordinary retrieval when a selected value needs conversion to a particular requested type.

Matching uses exact decoded elements, including case and Unicode identity. `[]string{"ip", "filter"}` excludes `ip.filters.100099` and `ip."filter.100099"`. An empty prefix (`nil` or `[]string{}`) selects all values, whereas `[]string{""}` selects the explicit empty-name key's own values and descendants. `*` is a literal element rather than a wildcard; its prefix selects that literal key's own values and descendants.

A valid `Config` with no matches returns an empty slice and no error. A nil `*Config` returns `ErrMissing` with line zero. The returned result slice and all full and relative path slices are independent of the supplied prefix, the configuration and other results; editing them cannot change later selections. A custom profile's `Value` follows the same ownership rules as values returned by existing `Config` retrieval methods: mutable objects are not deep-copied. The method name, `SelectedValue` representation and diagnostic details are provisional Go API choices; the shared selection behaviour is established by the specification.

## Single-line strings

After trimming outer ASCII spaces and tabs, the first character selects how a single-line value is read. If it is a double quote, the whole value must be properly quoted; remove the surrounding quotes and decode the specified escapes. If it is anything else, preserve the entire trimmed value literally, including later quotes and backslashes. Do not enter quoted syntax partway through a literal value.

```text
A.Saying: say "hello"
A.Unbalanced: x"unterminated
A.QuoteText: \"hello\"
A.JsonText: {"x": "\u0041"}
A.MidTriple: text """ more
```

These values retain every quote and backslash as written; the JSON-looking value is not decoded as JSON. In contrast, `"say \"hello\""` uses quoted syntax and decodes to `say "hello"`. A starting quote that is unclosed, followed by anything other than outer ASCII spaces or tabs after its close, or contains an invalid escape still produces `ErrString`; there is no literal fallback. The exact `"""` header opens a triple-quoted block, and pipe detection remains governed by the multiline rules.

See [single-line string decoding](../../docs/config-format.md#82-double-quoted-values-and-common-string-decoding) for the reference rules.

## Multiline values

Triple-quoted values use the same escapes as ordinary quoted values:

```text
A.Message: """
  Hello, \u{1F600}
    indented
  """
```

The header right-hand side is exactly `"""`; no inline content follows it. The closing marker is on its own line, with optional trailing ASCII spaces or tabs. Its leading ASCII-space count is the margin removed from every non-empty source body line; truly empty lines are exempt. Non-empty lines with insufficient margin are rejected. The closing margin is independent of structural indentation and may be odd or zero. Ordinary quote characters are body data; only an exact closing line terminates the block. Escape one of its quotes, for example `\"""`, to represent such a line as body data, or use a pipe block instead.

After structural processing, `DecodeString` decodes the body escapes exactly once and does not trim or normalise the result. With LF source endings, the example becomes `Hello, 😀`, LF and two spaces followed by `indented`. Invalid escapes still fail decoding, and a backslash never continues a physical line.

A pipe block starts only when the physical line immediately after an empty header has more leading ASCII spaces than that header, followed by `|`:

```text
A.
  .Literal:
   |C:\temp\new
   |"quotes" and \q remain literal
   |# literal text
  .Next: value
```

Collect consecutive deeper pipe lines, stripping their leading spaces and first pipe. Everything after each marker is literal: retain whitespace, additional pipes, quotes and backslashes without escape decoding. Different body-line indentation is allowed if every line remains deeper than the header. Compare raw leading-space counts: odd counts and a one-space increase are allowed for pipe bodies independently of the two-space hierarchy rule. Do not skip blank lines or comments to find or continue a pipe body. The first non-body line is processed normally. `A:|` is an ordinary single-line value. A same-depth or shallower `|:` line is not body content and must satisfy ordinary key and indentation rules: unindented `|: value` can name the key `|`, but an indented absolute key remains invalid.

Both forms preserve original LF or CRLF between body lines, exclude boundary line endings and infer no terminal newline. Add an empty body line before a triple-quoted close or an empty final pipe to retain a terminal newline:

```text
A.WithNewline:
 |text
 |
A.Empty:
 |
```

The first body is `text` followed by one source line ending; the second is an explicit empty string with `Unset == false`. An immediately closed triple-quoted block is also explicitly empty. Normal heading scope and type conversion apply to block headers; bodies do not create headings or comments. Type annotations and assignment to a heading itself support either form.

See [triple-quoted values](../../docs/config-format.md#83-triple-quoted-multiline-values) and [pipe blocks](../../docs/config-format.md#84-literal-pipe-blocks) for the reference rules.

## Type conversion and the caller's configuration schema

A `Profile` specifies `StringType`, the name of the string type, and `Types map[string]Converter` for other types. A `Converter` receives decoded `Text`. The string type always returns the decoded string, treating an unset value as an empty string. Type names in this implementation require an exact, case-sensitive match.

`DemoProfile()` provides the following experimental types. They do not establish standard type names or standard conversion rules for the notation.

| Experimental type name | Conversion in this implementation |
|---|---|
| `string` | Preserve the string; an unset value can be retrieved as an empty string |
| `int` | Convert a decimal integer with an optional sign to `int64`; allow leading zeroes; reject hexadecimal, digit separators, out-of-range values and unset values |
| `bool` | Accept only `true` and `false`; reject different capitalisation, numeric forms and unset values |

`Build` independently processes each occurrence, preserving its declaration and unset state. It allows repeated keys with different annotations, provided that each declared conversion succeeds; it does not impose one shared type on the key. If conversion to an annotated type fails, `Build` returns `ErrTypeConversion`. An unknown annotated type causes the same error. Failed retrieval conversion for an unannotated occurrence, an unknown requested type, or a requested type different from that occurrence's annotation returns `ErrTypeMismatch`. An annotated occurrence is not substituted with a different type or converted back to a string as a fallback.

Custom types can be added through a `Profile`. Known keys, required settings, defaults, ranges, single-value or other occurrence-count constraints, and consistency between settings are validated by the caller's configuration schema. This implementation has no dependency on any particular application's setting names, conversions or schema.

## Supported rules and provisional choices

The implementation covers the required declaration region and blank separator, fully qualified keys, headings, relative notation, assigning to the heading itself, quoted key elements, empty names, the position of type annotations, alignment with ASCII spaces, unset values, single-line values, both multiline forms, common string decoding and source-ordered repeated values. Structural child lines require exactly two more ASCII spaces than their parent heading; odd structural indentation, tab indentation and missing parents are rejected. Blank lines, comments, triple-quoted margins and pipe bodies follow their separate whitespace rules. It retains identical repetitions without overwriting or deduplication. Escapes are decoded in quoted key elements, single-line values selected as quoted by their first trimmed character, and triple-quoted bodies. Later quotes and backslashes in literal single-line values, and all pipe-body text, are retained; backslashes in unquoted key elements are rejected.

Quoted key elements generally allow spaces, tabs, syntax characters such as `: ( ) #`, and control characters. Key elements and ordinary quoted values cannot span physical lines; triple-quoted and pipe bodies use the explicit multiline rules above. Represent a quote or a backslash in an ordinary quoted string with `\"` or `\\`; decoded characters are not reinterpreted as syntax. A newline can also be represented as character data with `\n` or a Unicode escape in quoted text. The range of characters representable by escapes is generally unrestricted, but undefined escape forms remain errors.

| Form within quotes | Decoded character |
|---|---|
| `\.` / `\"` / `\\` / `\/` | Dot / double quote / backslash / slash, respectively |
| `\b` / `\f` / `\n` / `\r` / `\t` | Backspace / form feed / LF / CR / horizontal tab, respectively |
| `\u{H...}` (recommended) | A Unicode scalar value given by 1–6 ASCII hexadecimal digits; either case and leading zeroes are allowed; whitespace and underscores are rejected |
| `\uXXXX` | Four ASCII hexadecimal digits; represent a non-BMP character with consecutive high and low surrogate escapes |

Each value below decodes to the single emoji `😀` (U+1F600):

```text
A.Braced: "\u{1F600}"
A.Fixed: "\uD83D\uDE00"
A.Direct: "😀"
```

Unicode scalar values range from U+0000 to U+10FFFF, excluding U+D800–U+DFFF. Noncharacters and unassigned code points are accepted if they are within this range. Errors include surrogates in the braced form; isolated, reversed or incomplete surrogate pairs in the four-digit form; pairs that mix the two forms; invalid digit counts or hexadecimal digits; and out-of-range values. Invalid representations are not substituted with a replacement character. `"\u{1F600}"` and `"😀"` become the same key element, so their occurrences belong to the same key and are all retained. This is equality of decoding results, distinct from Unicode normalisation.

The basic escapes other than the dot, and the four-digit Unicode form, are shared with JSON; the notation does not adopt the complete JSON string rules. Forms used by other languages, such as `\UXXXXXXXX`, `\xXX`, octal escapes, `\a`, `\v`, `\0` and `\'`, are undefined and rejected. See [section 3.4 of config-format.md](../../docs/config-format.md#34-common-escapes-within-quotes).

A type expression cannot contain internal whitespace. Parsing is not limited to simple ASCII type names: expressions are extracted while maintaining balanced parentheses. Forms such as `list(int)`, `map(string,list(int))`, `A|B`, `名前` and `[]` can be parsed, but their meaning and validity are determined by the profile. These examples do not establish standard types or a standard grammar for compound types. Headings cannot have type annotations, and types are not inherited by children.

The implementation handles UTF-8, one initial BOM, LF, CRLF and a final line without a newline. Encoding and newline validation also applies to comments, blank lines and multiline bodies; internal source line endings are preserved in block values. Neither the notation nor this implementation imposes fixed length limits on physical lines, keys, expanded paths, nesting depth or the whole file. Resource limits in the execution environment and length constraints in the caller's schema are distinct from the notation's acceptance rules. There is no environment-variable expansion, include mechanism or additional file loading.

The following choices are specific to this implementation where the specification remains undecided. They do not replace a formal decision in [config-format.md](../../docs/config-format.md).

| Undecided area | Behaviour in this implementation |
|---|---|
| Remaining details of type-expression grammar | Extract expressions without interpreting their meaning, preserving balanced parentheses; empty expressions, quotes and escapes inside expressions are unsupported; reject internal whitespace under the already agreed rule |
| Remaining whitespace around type annotations | Alignment tabs immediately before the type expression or after the closing parenthesis are unsupported; this is distinct from normal trimming of outer SP and HT on value lines |
| Unicode whitespace and control-character detection in unquoted keys | Use Go's `unicode.IsSpace` and `unicode.IsControl`; do not apply this prohibition to quoted keys |
| Future escape extensions | Reject additional forms as undefined; implement decoding of the adopted basic and Unicode forms as agreed rules |
| API and error classification | Use this sample's singular, collection, indexed and prefix-selection APIs and `Code`; singular methods report `ErrMultipleValues`, and `GetAll` fails without partial results; concrete public API names, native result representations and the formal diagnostic format remain undecided, while prefix-selection behaviour follows section 9.1 |
| Type-expression identity and conversion failure | Match type names exactly without resolving aliases; report conversion failure for an annotated type during `Build` |

Unsupported type-expression syntax is generally reported as `ErrUnsupported`, syntax that violates agreed rules as `ErrSyntax`, and invalid quoted-value syntax or escapes as `ErrString`. Undefined escapes in quoted forms are not reinterpreted as ordinary characters. A backslash or later quote in a single-line literal value is already an ordinary character, rather than active syntax.

`Error` contains a line number and `Code`. Its diagnostic does not include the right-hand side, block body or error text returned by underlying input or conversion operations. `ParseEach` buffers a complete multiline body before notifying its value entry. Identifying a pipe block or its end can inspect one following physical line; a non-body line is retained for normal parsing only after the callback succeeds. An error returned by the callback is returned unchanged, and stops parsing and notifications for later entries. Input bytes may also have been read ahead. Changes already made by the callback are not rolled back.

## Verification

Format Go code, then run the tests, static analysis and build from `implementations/go`:

```sh
gofmt -w .
go test ./...
go vet ./...
go build ./...
```

The tests cover required declarations and their blank separator, malformed, repeated and unsupported declarations, physical line numbers, accepted and rejected body input, heading state, key-element boundaries, first-character selection of literal or quoted single-line values, literal later quotes and backslashes, basic and Unicode escapes, rejection of unknown quoted escapes and invalid surrogates, the absence of Unicode normalisation, triple-quoted margins and closing markers, literal pipe detection, preservation of source and terminal line endings, explicit empty blocks, ordered repeated values including identical values, per-occurrence unset and type state, singular and collection retrieval, type mismatches, callback-requested stops, input errors, and long physical lines and expanded paths. Fuzz seeds also run as part of the usual `go test` invocation.

The latest measurements pushed to the repository and their reproduction
instructions are in the [benchmark overview](../benchmarks/README.md) and the
[published Go multiline and allocation report](../benchmarks/multiline-allocation-2026-10-24/README.md).
The published report identifies its measured source snapshots; local parser
changes and unpushed measurements do not replace those results. Superseded
standalone results are removed when their replacement is published.

To generate arbitrary inputs during fuzzing:

```sh
go test -run '^$' -fuzz '^FuzzSampleParserNoPanic$' -fuzztime=5s -parallel=2
```

To check all three CLI modes:

```sh
go run ./cmd/configsample -mode raw ../../examples/sample.txt
go run ./cmd/configsample -mode strings ../../examples/sample.txt
go run ./cmd/configsample -mode demo ../../examples/sample.txt
```

CI is configured for Windows and Linux with Go 1.22 and the current stable Go release. It checks formatting, runs tests and static analysis, builds the packages and checks all CLI modes. The Linux stable-Go job also runs the race detector and generated-input fuzzing. Check GitHub's results after pushing changes; local checks and remote CI results are separate.

## Licence

This sample implementation is available under the [MIT License](../../LICENSE).
