# Go sample implementation

Run all commands in this document from `implementations/go`. See the [project overview](../../README.md) for an introduction to the notation and the [documentation](../../docs/README.md) for guides and the repository layout.

This is a sample Go implementation of [config-format.md](../../docs/config-format.md). It uses no external libraries. Parsing, common string decoding and type conversion are separate stages so that the notation can be tried independently of a particular application.

Standard type names and the public API remain undecided. The Go package is `dotreg`; the module path is `github.com/libnumafly/dotReg/implementations/go`. The implementation makes the provisional choices described below for parts of the syntax that remain undecided.

## Running the sample

Use Go 1.22 or later. From `implementations/go`, run:

```sh
go run ./cmd/configsample ../../examples/sample.txt
```

By default, the CLI writes JSON containing the parsed key path, undecoded right-hand side, unset state and type annotation, together with the result of common string decoding. This mode does not convert values, even when a type annotation is present.

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
'A.Count(int):42' | go run ./cmd/configsample -mode demo -
```

For input containing non-ASCII characters, passing a UTF-8 file avoids dependence on the shell's pipe encoding.

| Mode | Processing | Value fields in the output |
|---|---|---|
| `raw` | Parsing; value quotes and escapes remain undecoded | `raw` |
| `strings` (default) | Parsing and common string decoding | `raw` and `decoded` |
| `demo` | Common string decoding, then conversion to annotated types with the experimental profile | `raw` and `value` |

`path` is an array of strings. It distinguishes `["A.B"]` from `["A", "B"]`, and `[""]` from an empty array. `unset` is `true` for an unset value and `false` for the explicit empty string written as `""`. A JSON `\\` is part of JSON's representation; it does not change the configuration input syntax.

The `raw` mode can retain an unterminated quote in a value. Always use `DecodeString` or `Build` before passing configuration values to a consumer. The CLI's `strings` and `demo` modes write output only after all processing succeeds, so a failure does not produce partial JSON.

## Using the library

| API | Purpose |
|---|---|
| `Parse(io.Reader) ([]Entry, error)` | Collect parsed entries; return no partial result on failure |
| `ParseEach(io.Reader, func(Entry) error) error` | Notify the callback of headings and value lines in source order, with physical line numbers |
| `DecodeString(Entry) (Text, error)` | Remove the surrounding value quotes and decode escapes only within quotes |
| `Build([]Entry, Profile) (*Config, error)` | Decode strings and convert annotated values only to their declared type |
| `Config.Get([]string, string) (any, error)` | Retrieve a value with an explicit key path and requested type name |
| `Config.IsUnset([]string) (bool, error)` | Query the unset state without retrieving or converting the value |

`Entry` has `Line`, `Kind`, `Path`, `Raw`, `Unset`, `Type` and `HasType` fields. `Kind` is either `Heading` or `Value`. A heading alone does not create a value. `Text` holds the decoded `Value string` and `Unset bool`.

The following fragment demonstrates retrieval. See [example_test.go](example_test.go) for executable examples that include error handling.

```go
entries, err := dotreg.Parse(strings.NewReader(
    "A.Code(string):001\nA.Count:42\nA.Missing:\nA.Empty:\"\"\n",
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

Requesting the annotated `A.Code` as `int` returns `ErrTypeMismatch`, regardless of whether conversion would be possible. The unannotated `A.Count` can be retrieved as `"42"` when requesting `string`, or as `int64(42)` when requesting `int`. Retrieval conversions are not cached; the original string and unset state are retained.

`A.Missing` and `A.Empty` both return a zero-length string when requested as `string`. `IsUnset` returns `true` and `false`, respectively. To address a key with an empty name, pass `[]string{""}`. An empty path, whether `nil` or `[]string{}`, does not mean a key with an empty name.

The notation does not perform Unicode normalisation. Key elements are compared exactly after quote and escape decoding, using the same rule for duplicate detection, retrieval and matching against a configuration schema. For example, the precomposed character `が` (U+304C) and the sequence `か` plus a combining voiced sound mark (U+304B U+3099) are different names. Type expressions and values are not implicitly normalised either.

## Type conversion and the caller's configuration schema

A `Profile` specifies `StringType`, the name of the string type, and `Types map[string]Converter` for other types. A `Converter` receives decoded `Text`. The string type always returns the decoded string, treating an unset value as an empty string. Type names in this implementation require an exact, case-sensitive match.

`DemoProfile()` provides the following experimental types. They do not establish standard type names or standard conversion rules for the notation.

| Experimental type name | Conversion in this implementation |
|---|---|
| `string` | Preserve the string; an unset value can be retrieved as an empty string |
| `int` | Convert a decimal integer with an optional sign to `int64`; allow leading zeroes; reject hexadecimal, digit separators, out-of-range values and unset values |
| `bool` | Accept only `true` and `false`; reject different capitalisation, numeric forms and unset values |

If conversion to an annotated type fails, `Build` returns `ErrTypeConversion`. An unknown annotated type causes the same error. Failed retrieval conversion for an unannotated value, an unknown requested type, or a requested type different from the annotation returns `ErrTypeMismatch`. An annotated value is not substituted with a different type or converted back to a string as a fallback.

Custom types can be added through a `Profile`. Known keys, required settings, defaults, ranges and consistency between settings are validated by the caller's configuration schema. This implementation has no dependency on any particular application's setting names, conversions or schema.

## Supported rules and provisional choices

The implementation covers the agreed rules for fully qualified keys, headings, relative notation, assigning to the heading itself, quoted key elements, empty names, the position of type annotations, alignment with ASCII spaces, unset values, common string decoding and duplicate rejection. Escapes are decoded only within quotes. Backslashes in unquoted values are retained; backslashes in unquoted key elements are rejected.

Quoted key elements generally allow spaces, tabs, syntax characters such as `: ( ) #`, and control characters. Physical newlines delimit lines, so quotes cannot span physical lines. Represent a quote or a backslash with `\"` or `\\`; decoded characters are not reinterpreted as syntax. A newline can be represented as character data with `\n` or a Unicode escape. The range of characters representable by escapes is generally unrestricted, but undefined escape forms remain errors.

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

Unicode scalar values range from U+0000 to U+10FFFF, excluding U+D800–U+DFFF. Noncharacters and unassigned code points are accepted if they are within this range. Errors include surrogates in the braced form; isolated, reversed or incomplete surrogate pairs in the four-digit form; pairs that mix the two forms; invalid digit counts or hexadecimal digits; and out-of-range values. Invalid representations are not substituted with a replacement character. `"\u{1F600}"` and `"😀"` become the same key element and are therefore duplicates. This is equality of decoding results, distinct from Unicode normalisation.

The basic escapes other than the dot, and the four-digit Unicode form, are shared with JSON; the notation does not adopt the complete JSON string rules. Forms used by other languages, such as `\UXXXXXXXX`, `\xXX`, octal escapes, `\a`, `\v`, `\0` and `\'`, are undefined and rejected. See [section 3.4 of config-format.md](../../docs/config-format.md#34-common-escapes-within-quotes).

A type expression cannot contain internal whitespace. Parsing is not limited to simple ASCII type names: expressions are extracted while maintaining balanced parentheses. Forms such as `list(int)`, `map(string,list(int))`, `A|B`, `名前` and `[]` can be parsed, but their meaning and validity are determined by the profile. These examples do not establish standard types or a standard grammar for compound types. Headings cannot have type annotations, and types are not inherited by children.

The implementation handles UTF-8, one initial BOM, LF, CRLF and a final line without a newline. Encoding and newline validation also applies to comments and blank lines. Neither the notation nor this implementation imposes fixed length limits on physical lines, keys, expanded paths, nesting depth or the whole file. Resource limits in the execution environment and length constraints in the caller's schema are distinct from the notation's acceptance rules. There is no environment-variable expansion, include mechanism, additional file loading or multiline value support.

The following choices are specific to this implementation where the specification remains undecided. They do not replace a formal decision in [config-format.md](../../docs/config-format.md).

| Undecided area | Behaviour in this implementation |
|---|---|
| Remaining details of type-expression grammar | Extract expressions without interpreting their meaning, preserving balanced parentheses; empty expressions, quotes and escapes inside expressions are unsupported; reject internal whitespace under the already agreed rule |
| Remaining whitespace around type annotations | Alignment tabs immediately before the type expression or after the closing parenthesis are unsupported; this is distinct from normal trimming of outer SP and HT on value lines |
| Unicode whitespace and control-character detection in unquoted keys | Use Go's `unicode.IsSpace` and `unicode.IsControl`; do not apply this prohibition to quoted keys |
| Future escape extensions | Reject additional forms as undefined; implement decoding of the adopted basic and Unicode forms as agreed rules |
| API and error classification | Use this sample's API and `Code`; the public API and formal diagnostic format remain undecided |
| Type-expression identity and conversion failure | Match type names exactly without resolving aliases; report conversion failure for an annotated type during `Build` |

Unsupported type-expression syntax is generally reported as `ErrUnsupported`, syntax that violates agreed rules as `ErrSyntax`, and invalid value quoting or escapes as `ErrString`. Undefined escapes are not reinterpreted as ordinary characters. A backslash in an unquoted value is already an ordinary character, rather than an escape.

`Error` contains a line number and `Code`. Its diagnostic does not include the right-hand side or the error text returned by underlying input or conversion operations. An error returned by the `ParseEach` callback is returned unchanged, and stops further parsing and notifications. Input bytes may have been read ahead. Changes already made by the callback are not rolled back.

## Verification

Format Go code, then run the tests, static analysis and build from `implementations/go`:

```sh
gofmt -w .
go test ./...
go vet ./...
go build ./...
```

The tests cover accepted and rejected input, heading state, key-element boundaries, backslashes within and outside quotes, basic and Unicode escapes, rejection of unknown forms and invalid surrogates, the absence of Unicode normalisation, unset values, type mismatches, callback-requested stops, input errors, and long physical lines and expanded paths. Fuzz seeds also run as part of the usual `go test` invocation.

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
