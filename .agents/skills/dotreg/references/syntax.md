# dotReg 0.1 syntax for application configuration

This is a task-focused guide to the revision recorded in
[upstream.json](upstream.json). The upstream
[format specification](https://github.com/libnumafly/dotReg/blob/4aca4adba1cdc4c523d3e07b91a64403de4f8f68/docs/config-format.md)
defines the rules. Fragments below omit file properties unless marked complete.

## Complete files and physical lines

Every complete file begins with the case-sensitive declaration and a physical
blank separator:

```text
@dotReg 0.1

A.Code:001
```

The declaration must start in column one. Its name and version are literal;
ASCII spaces or horizontal tabs may separate them. Leading comments, leading
blank lines, repeated declarations, unknown properties and unsupported versions
are rejected. The separator may contain ASCII spaces or tabs. A declaration
followed by one LF and EOF is missing the separator; two LFs supply it.

Use UTF-8; one initial BOM is allowed. LF and CRLF are allowed, and the final
line may omit a line ending. Lone CR and invalid UTF-8 are rejected everywhere,
including comments and block bodies. Line numbers count declarations, comments
and blank lines. `.dotreg` is the default extension; `.conf` is also suitable.
Validity depends on contents. The notation imposes no fixed length or depth
limit; application resource limits are separate.

## Keys and headings

A colon outside quoted key elements and the type expression separates a value.
A heading instead ends in an unquoted dot and creates no value occurrence.
One-element value keys are allowed. Dots outside quotes separate key elements.

Unquoted elements are non-empty and cannot contain whitespace, control
characters, `.`, `:`, `(`, `)`, `#`, `"` or `\`. Quote an entire element to use
these characters or an empty name. Partial quoting and unquoted backslash
escapes in keys are invalid.

| Written key | Decoded elements |
|---|---|
| `A.B` | `["A", "B"]` |
| `"A.B"` or `"A\.B"` | `["A.B"]` |
| `A."B.C"` | `["A", "B.C"]` |
| `A.""` | `["A", ""]` |
| `""` | `[""]` |
| `A.01` | `["A", "01"]` |

Elements compare exactly after quote/escape decoding. Names are case-sensitive;
no Unicode normalisation occurs. `1` and `01` are distinct string names, not
array indices. `A`, `A.B` and `A.B.C` can each have values simultaneously.

Absolute headings and values start in column one. Relative lines start with
`.` and use exactly two additional ASCII spaces per written heading level.
A compound heading such as `A.B.` still occupies just one level.

```text
A.B.
  .: parent
  .C.
    .D: child
  .E: sibling
```

The value paths, in source order, are `["A", "B"]`, `["A", "B", "C", "D"]` and
`["A", "B", "E"]`. Assignment `.:` addresses the active parent itself; `."":`
instead addresses an empty-named child, and `.".":` a child named one dot.

For structural indentation `d`, a relative line's parent is the active heading
at `d - 2` spaces. Dedenting closes deeper headings; a value line creates no
heading. An absolute line closes all active headings. Closed headings are not
revived when later lines indent again. Odd structural indentation, indentation
tabs, unindented relative lines, indented absolute lines and skipped parent
levels are invalid. Multiline bodies have different indentation rules below.

Blank lines and whole-line comments beginning with `#` after outer ASCII
space/tab removal preserve the active headings. These rules apply only in the
body outside multiline values. There are no inline comments: `A: x # y` has
the literal value `x # y`. Do not append a comment to a heading.

## Single-line values and escapes

Remove outer ASCII spaces and tabs from a single-line right-hand side. Other
Unicode whitespace is not trimmed. The first remaining character chooses the
form; later quotes never change it.

| Right-hand side | Common decoded string or state |
|---|---|
| `001`, `true`, `null`, `[1,2]` | The corresponding literal text; no inference |
| `C:\temp\new` | Literal backslashes remain |
| `say "hello"` | Later quotes remain literal |
| `x"unterminated` | Literal; later quotes need no balance |
| `"  hello  "` | Leading and trailing spaces inside quotes remain |
| `"C:\\temp\\new"` | `C:\temp\new` after escape decoding |
| Empty, with no immediately following pipe body | Unset occurrence |
| `""` | Explicit empty string; unset is false |

If the trimmed value begins with `"`, it must be quoted in its entirety. A
missing closing quote, trailing non-whitespace after it, or an invalid escape
fails common string decoding. Do not fall back to literal text. Unset values
can be retained separately even when string retrieval returns an empty string.
An absent key or heading alone has no value occurrence.

Within quoted keys, quoted single-line values and triple-quoted bodies, the
basic escapes are `\.`, `\"`, `\\`, `\/`, `\b`, `\f`, `\n`, `\r` and `\t`.
Unicode escapes accept `\u{H...}` with one to six hexadecimal digits or
`\uXXXX` with exactly four. Non-BMP characters in the four-digit form require
consecutive high/low surrogate escapes, such as `\uD83D\uDE00`.

Reject unknown/incomplete escapes, isolated or reversed surrogates, surrogates
in braces, mixed surrogate notation and values above U+10FFFF. Do not substitute
a replacement character. Decode once and never reinterpret decoded characters
as separators, comments or escapes. Unquoted values and pipe bodies do not
decode escapes. A trailing backslash never continues a physical line.

## Multiline values

An entire trimmed header right-hand side of `"""` opens a triple-quoted block.
Its closing marker occupies a line containing only leading ASCII spaces,
`"""` and optional trailing ASCII spaces/tabs.

```text
A.Message: """
  first line
    second line: \u{1F600}
  """
```

The closing marker's leading-space count defines a margin. Remove exactly that
many ASCII spaces from every non-empty source body line; a line containing no
characters is exempt. Non-empty lines shorter than the margin fail. Additional
spaces remain. The margin can be odd or zero and is independent of heading
depth. Ordinary body quotes are data; only an exact closing-marker line ends
the block. Escape at least one quote to include a body line shaped like the
closing marker. Decode the same escapes as quoted single-line values.

An empty header opens a literal pipe block only if the immediately next physical
line has more leading ASCII spaces than the header and then `|`. Do not skip
blank lines or comments when checking this condition.

```text
A.Literal:
 |C:\temp\new
 |"quotes" and \q stay literal
 ||starts with a pipe
A.Next: value
```

Collect consecutive qualifying lines. Each may use a different indentation
greater than the header's; odd counts are valid. Remove all leading ASCII
spaces and the first `|`. Preserve everything after the marker literally,
including further pipes, quotes, backslashes, tabs and spaces. The first
non-qualifying line ends the block and is processed normally. `A:|` is instead
the ordinary single-line value `|`.

Both block forms preserve original LF/CRLF *between* body lines and exclude
boundary line endings. Neither adds a final newline automatically. Add an
empty body line before triple-quote closure or an empty final pipe line to
retain one. Blocks always have unset false, including an immediately closed
triple block or a single empty pipe line. Block bodies do not change headings.

## Type annotations and repeated values

An annotation belongs between the key and colon: `A.Code(string):001`.
Zero or more ASCII alignment spaces can surround the annotation outside its
parentheses. There is no type inheritance, and headings cannot be annotated.
Types on repeated values belong to each occurrence separately.

Type expressions have no internal whitespace. Balanced nested expressions and
symbols can be preserved, without assigning meaning: `list(int)` and `A|B` are
examples, not standard types. `A.Code: (string) 001` has no annotation; its
right-hand side is literal text. Simple type names follow a lowercase-English
naming convention, but the syntax is not restricted to ASCII names.

All occurrences remain in source order, including identical values and mixed
annotations. Writing equivalent notation twice records two occurrences. Do not
overwrite, sort or deduplicate them. Cardinality is an application rule. Every
read begins with fresh heading state and a separate occurrence sequence.

## Specification and sample implementation boundaries

The standard type catalogue, concrete conversions, parts of type-expression
grammar, annotation alignment tabs, exact Unicode key classification and API
names remain open. The checker follows the bundled sample parser's choices:
it rejects empty type expressions, quotes/backslashes inside them and alignment
tabs before an annotation or between its closing parenthesis and colon. It uses
Unicode White_Space and Cc classification for unquoted keys. These choices do
not settle the open specification questions.

The `base64` text representation is defined as padded standard Base64 with no
wrapping or non-alphabet characters, including whitespace. This does not make
`base64` a standard type name or a supported Go demo type. Read the upstream
[type profile](https://github.com/libnumafly/dotReg/blob/4aca4adba1cdc4c523d3e07b91a64403de4f8f68/docs/type-profile.md)
and [open matters](https://github.com/libnumafly/dotReg/blob/4aca4adba1cdc4c523d3e07b91a64403de4f8f68/docs/open-issues.md)
when those distinctions affect the application.
