# Sample parser set

The set covers eight languages through seven parsing engines. Each engine uses
its language's standard library and processes dotReg syntax and common string
decoding without third-party runtime dependencies. TypeScript provides a typed
façade over the JavaScript engine, sharing its grammar and runtime behaviour.

| Language | Baseline | Library entry points | Guide |
|---|---|---|---|
| Go | Go 1.22+ | `Parse`, `ParseEach`, `DecodeString` | [Go](go/README.md) |
| Python | Python 3.10+ | `parse`, `parse_each`, `decode_string` | [Python](python/README.md) |
| JavaScript | Node.js 20+ | `parse`, `parseEach`, `decodeString` | [JavaScript](javascript/README.md) |
| TypeScript | Node.js 20+, TypeScript 5.7+ to compile | Typed exports of the JavaScript API | [TypeScript](typescript/README.md) |
| Java | JDK 17+ | `parse`, `parseEach`, `decodeString` | [Java](java/README.md) |
| C# | .NET 8 SDK/runtime | `Parse`, `ParseEach`, `DecodeString` | [C#](csharp/README.md) |
| Rust | Current stable Rust, edition 2021 | `parse`, `parse_each`, `decode_string` | [Rust](rust/README.md) |
| C++ | C++17 compiler | `parse`, `parse_each`, `decode_string` | [C++](cpp/README.md) |

The compiler, interpreter or SDK in this table is a development prerequisite;
the parsers need no package installation. TypeScript's compiler checks the typed
interface; its runtime implementation is the neighbouring JavaScript module.
Build commands, native tests and embedding examples are in each language guide.

## Scope and shared behaviour

The [format specification](../docs/config-format.md) defines syntax and common
string decoding. These samples handle the required `@dotReg 0.1` declaration and
blank separator, strict UTF-8, LF and CRLF, decoded quoted key elements, relative
headings, opaque type annotations, repeated occurrences and single-line,
triple-quoted and literal pipe values. They preserve source order, element
boundaries, original internal block line endings and unset state. Unicode
normalisation is not performed.

The sample ports other than Go stop at common string decoding. They retain annotations such as
`list(int)` as text and do not establish a standard type catalogue, conversion
profile or retrieval API. The Go sample additionally provides its illustrative
consumer, `demo` type profile and
[prefix-selection API](go/README.md#selecting-values-by-key-prefix).
The shared [prefix-selection rules](../docs/config-format.md#91-prefix-selection)
belong to that consumer layer; the syntax-only ports and shared syntax/decoding
corpus do not provide or test retrieval APIs.

Each engine offers a collecting syntax API, an incremental visitor API and a
separate common string decoder. A collection succeeds as a whole or reports a
failure without returning partial entries. An incremental visitor receives each
heading and complete value before the next logical entry is parsed. Recipient
failures stop processing immediately and retain the recipient's original error
value. Successful early-stop conventions vary by language; see the native API
guide. Already performed recipient side effects remain in place.

A multiline body is complete before notification. An empty header or a pipe
boundary may require one physical line of lookahead. Validation of a non-body
lookahead is deferred until the preceding recipient succeeds; a qualifying body
line is validated before that value can be delivered. Byte buffering may fetch
more input ahead. Each parse starts with fresh heading state. Exported path
containers are separate from internal scope nodes, so recipient mutation cannot
change subsequent resolution.

## Common command-line contract

Commands read a named file or UTF-8 standard input and write one JSON array on
success. `--mode raw` exposes syntax results. `--mode strings`, the default,
performs common string decoding as well. The Go command also accepts its existing
`-mode` spelling and `demo` mode. See each guide for its executable and file
argument syntax.

| JSON field | Meaning |
|---|---|
| `line` | One-based physical line number of the heading or value header, counting declarations and blank lines |
| `kind` | `"heading"` or `"value"` |
| `path` | Array of decoded string elements, retaining empty names and dots within names |
| `raw` | Undecoded right-hand side, or a block body after removing its markers and prescribed margin |
| `unset` | Whether a value has an empty right-hand side without an immediately following pipe block |
| `has_type` | Whether that occurrence has a type annotation |
| `type` | Non-empty annotation text without its enclosing parentheses; omitted when absent |
| `form` | `"triple_quoted"` or `"pipe"`; omitted for single-line entries and headings |
| `decoded` | Common decoded string, present on value entries in `strings` mode |

Every entry includes `line`, `kind`, `path`, `raw`, `unset` and `has_type`.
Headings have `raw: ""`, `unset: false` and `has_type: false`; they do not produce
value occurrences. An unset value decodes to `""`, while `unset` distinguishes it
from an explicit empty quoted value or empty block. Keys are compared as element
sequences: `["A.B"]`, `["A", "B"]` and `[""]` remain distinct paths.

Both common CLI modes validate the entire syntax before value decoding. In
`strings` mode, values are then decoded in source order. A syntax error anywhere
in the file therefore takes precedence over an earlier invalid value escape.
`raw` can retain an undecodable ordinary quoted value or structurally complete
triple-quoted body; invalid quoted key escapes, missing block terminators and
insufficient block margins are syntax failures. Input, syntax or decoding
failures produce a non-zero exit status, empty standard output and a diagnostic
on standard error. Diagnostics exclude right-hand sides and underlying input
error strings.

The CLI deliberately collects results and completes the requested processing
before writing JSON. Use the native visitor API when entries can be consumed
incrementally and full retention is unnecessary.

## Speed, work and memory

The implementations scan physical lines in source order and use immutable
parent/suffix nodes for active headings. This avoids retaining a full expanded
ancestor path at every heading level. Value paths are materialised directly in
their own exported containers, without constructing a heading node for each
value. Heading nodes remain separate from those containers, including when a
relative empty-key value copies its parent's path. Escaped strings and block
bodies use spans, builders or append buffers appropriate to the language,
avoiding repeated reconstruction of growing immutable strings. Literal text can
be reused where the language permits it.

Streaming retention follows active heading suffixes, the current resolved entry,
the reader buffer and at most one physical boundary line. An unfinished
multiline value needs its body; triple-quoted margin removal may temporarily
retain source body lines alongside the completed body. There is no fixed format
limit on line length, block size, expanded path length or heading depth. A single
large line, block or resolved path still requires memory proportional to its
content, subject to the runtime's allocation and address-space limits. A caller
which collects entries also retains every occurrence and expanded path.

## Provisional choices and verification

The [undecided matters](../docs/open-issues.md) remain open. The sample ports
apply the same implementation choice: they reject empty type expressions,
quotes or backslashes inside type expressions, and alignment tabs before a
type annotation or between its closing parenthesis and the colon. Whitespace-free
nested type expressions are retained without assigning meaning to them. These
rejections do not define the type-expression grammar.

Unicode whitespace checks follow Go's Unicode `White_Space` classification;
unquoted key elements additionally reject Unicode `Cc` control characters.
Outer trimming and structural indentation retain the specification's narrower
ASCII space/tab rules. This is a shared implementation choice for the samples,
not a broader normalisation or character-conversion policy.

The [shared conformance corpus and runner](conformance/README.md) compare syntax
entries and decoded strings against explicitly authored expectations. Native
tests separately check callback ordering, recipient errors, mutation isolation,
I/O failures and streaming behaviour. Passing those checks supports the covered
syntax and decoding cases. It does not establish universal type-profile, API,
application-schema or future-version conformance.
