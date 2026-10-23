# Configuration file format

This document defines a configuration file notation for key paths containing one or more elements and single-line or explicitly delimited multiline values. Keys, headings and value headers occupy one physical line. Fully qualified keys are the basic form, with dots separating their elements. Headings, relative notation and indentation allow a shared prefix to be omitted.

## 1. Scope and specification status

Resolution produces a flat space of fully qualified keys and their ordered value occurrences. Every occurrence is retained in source order, including repeated identical values; later occurrences do not overwrite earlier ones. An empty right-hand side means that the key is present and that occurrence is unset unless the immediately following physical line starts a pipe block as defined in section 8.4. Explicit multiline blocks are set values, even when their bodies are empty. Every set value is initially treated as a string. Syntax parsing preserves the undecoded right-hand side or block body and its value form, and common string decoding takes place before it is delivered as a value. Type conversion according to a type expression declared in advance or requested at retrieval is a separate, subsequent stage, applied independently to each occurrence. When retrieval as a string is permitted, an unset occurrence is treated as equivalent to an empty string. General syntax processing and string decoding are separate from value type conversion and the application's configuration schema.

| Area | Status |
|-----------------------|--------------------------------------|
| Value keys containing one or more elements, headings and relative notation | Defined in this document |
| Repeated value keys | All occurrences, including identical repeated values, are retained in source order without overwriting or deduplication |
| Prohibited characters in key elements, quoting, empty names and the unset state of an empty right-hand side | Defined in this document |
| Treating an unset value as a string | Equivalent to a zero-length string, provided that there is no type expression mismatch with a declaration |
| Single-line literal and double-quoted values | After outer ASCII spaces and tabs are removed, an initial double quote selects quoted syntax; otherwise the entire value is literal, including later quotes and backslashes. This document defines quote removal, quoted whitespace and the escapes in section 3.4 |
| Multiline values | Triple-quoted blocks use the same escapes as ordinary quoted values. Pipe blocks preserve their body literally. Sections 8.3 and 8.4 define markers, margins and line endings |
| Optional type annotations on the left-hand side of a value line | This document defines their position and the outer whitespace used for alignment |
| Type conversion and retrieval requests | This document defines the conversion procedure with and without a declaration, and type expression mismatch errors at retrieval |
| Whitespace within type expressions and compound type notation | Internal whitespace is prohibited. Symbols that can express compound types and nested parentheses are not prohibited outright. The type conversion layer defines their meaning |
| Type names, remaining details of type expression grammar and individual conversion rules | Undecided. The type names and compound types in this document are illustrative examples |
| Format-specific version and dedicated file extension | Undecided |

This document is the reference specification for this project's file format. Section 12 lists the undecided matters. A sample Go implementation provides a syntax parser, common string decoding and type conversion through an illustrative profile. See [the Go sample guide](../implementations/go/README.md) for how to run it and its supported scope. The sample's provisional choices are distinct from the format rules established in this document.

## 2. Basic value lines

A value line separates the key and the right-hand side with a colon. Whitespace after the colon is optional.

A key can contain just one element.

```text
A:001
```

This line has the fully qualified key `A` and the undecoded right-hand side `001`. Use dots to separate multiple elements.

```text
A.Code:001
```

This line has the fully qualified key `A.Code` and the undecoded right-hand side `001`. The following notation represents the same key and right-hand side.

```text
A.Code: 001
```

Examples that represent the same setting are independent inputs. Combining them in one file records repeated value occurrences; it does not collapse them into one occurrence.

A value line with no value after the separating colon means that the key's value is unset unless the immediately following physical line starts a pipe block under section 8.4. The same applies when the colon is followed only by outer ASCII spaces or tabs.

```text
A:
```

By itself, this is an unset value line containing the fully qualified key `A`, rather than a heading. When received as a string, its value is equivalent to a zero-length string. Section 8 explains the distinctions and explicit multiline forms in detail.

### 2.1 Equivalent notation using headings

Each of the following four examples assigns the right-hand side `value` to the fully qualified key `A.B`.

The `= A.B` below is an explanation showing the resolved key; it is not written in the configuration file.

```text
A.B: value       = A.B
```

```text
A.
 .B: value      = A.B
```

```text
A.B.
 .: value       = A.B
```

```text
A.
 .B.
  .: value      = A.B
```

A heading is a line that specifies a prefix. On its own, it does not set a value.

## 3. Key paths

A fully qualified key consists of one or more key elements separated by dots outside quotes. Keys are case-sensitive. Each element is either unquoted or enclosed in double quotes `"` in its entirety. Escapes are available only within double quotes.

Both value keys and headings must contain at least one element. A one-element value key is also a fully qualified key. An unquoted element is a non-empty string of valid UTF-8 characters that does not contain any of the prohibited characters below. The same rules apply to the first, intermediate and last elements. A quoted element can explicitly specify an empty name. Section 3.2 describes quoting, and section 3.4 describes common escapes within quotes.

| Prohibited in unquoted key elements | Reason |
|-------------|-------------------|
| `.` | Separates path elements |
| `:` | Separates the left-hand and right-hand sides |
| `(` and `)` | Delimit a type annotation |
| `#` | Avoids a conflict with whole-line comments |
| `"` | Delimits a quoted key element |
| `\` | Prohibited in unquoted keys. To include it, quote the entire element and escape it |
| Whitespace, line breaks and control characters | Keeps them distinct from indentation, alignment and physical lines |

Elements must not be omitted. `A..B`, `A.: value` and `: value` without a key are rejected. Write an element with an empty name as `""`. Any non-empty string without prohibited characters is allowed as an unquoted key element. Unicode characters such as Japanese, ASCII letters and digits, underscores, hyphens, elements beginning with digits and entirely numeric elements are all permitted.

The following value keys are syntactically valid. The consuming application's configuration schema validates whether these keys are actually supported.

```text
A: root
1: numeric
01: zero-prefixed
1A.2: numeric-boundaries
A.B: first
A.b: second
A.1.B: third
A.01.B: fourth
A.1: fifth
店舗.名称: 本店
端末-1.有効: true
A_B.Code: 001
```

Entirely numeric elements are also string identifiers. `1` and `01` are distinct elements: leading zeroes are preserved, and the elements are not interpreted as array indices. The leading dot in relative notation, the trailing dot of a heading and the single dot used for assignment to a heading itself are syntax symbols, distinct from the character restrictions on elements.

This prohibited-character list applies to unquoted key elements, not to type expressions or right-hand sides. `A\.B` is rejected because it contains a backslash in an unquoted key. The backslash is not treated as an ordinary character to reinterpret the input as another key path. Quote an entire element to write a name containing a dot, such as `"A.B"`, or a name containing a backslash, such as `"A\\B"`.

### 3.1 Coexistence of value keys and descendant keys

The following notation is valid.

```text
A: root
A.B: parent
A.B.C: child
```

`A`, `A.B` and `A.B.C` are distinct fully qualified keys. A key with a value can also appear as the prefix of another key without a conflict. They are not converted to an object structure that can contain only a value or descendant keys.

### 3.2 Quoted key elements and empty names

Double quotes surround an entire key element. Only dots outside quotes separate path elements; dots and `\.` inside quotes are preserved as characters in the name. Partially quoting an element or omitting its closing quote is a syntax error. Within quotes, `\"` is a character in the name and does not end the quoted element.

The following table shows only the key portion. The arrays in the right-hand column illustrate element boundaries; they are not configuration file syntax.

| Key portion | Interpreted elements |
|---------|-------------|
| `A.B.C` | `["A", "B", "C"]` |
| `"A.B"` | `["A.B"]` |
| `"A\.B"` | `["A.B"]` |
| `A."B.C"` | `["A", "B.C"]` |
| `A."B\.C"` | `["A", "B.C"]` |
| `"A.B".C` | `["A.B", "C"]` |
| `"A.B.C"` | `["A.B.C"]` |
| `A."".B` | `["A", "", "B"]` |
| `A.""` | `["A", ""]` |
| `""` | `[""]` |

`""` is one element whose name is an empty string. It is distinct from having no elements, omitting the key or leaving the value unset. The following is a one-element value key with an empty name and an unset value.

```text
"":
```

The following forms are rejected.

| Notation | Reason for rejection |
|------|-------------|
| `A..B: value` | The intermediate element is omitted |
| `A.: value` | The final element is omitted |
| `: value` | No key elements are written |
| `A..` | Removing the heading's terminating dot leaves a missing final element |
| `A\.B: value` | An unquoted key element contains a backslash |

Quoted empty elements have no position restrictions. `"".A` and `"".""` are also valid two-element key paths.

To include a dot in a name, quote the entire element. Within quotes, a dot can be written directly or as `\.`; `"A.B"` and `"A\.B"` represent the same one-element key. Escapes within quoted key elements and values follow the common rules in section 3.4.

Escapes are taken into account when identifying quote boundaries. A decoded escape character is part of the element name and is not reinterpreted as a path separator, closing quote, type annotation or value-line separator.

In principle, quoted element names allow any valid UTF-8 string. Whitespace, tabs, syntax symbols including `: ( ) #`, and control characters are not prohibited outright. A double quote or backslash itself is written as `\"` or `\\`, respectively. A quoted key element cannot span physical LF or CRLF boundaries; multiline forms apply only to values. Line breaks and control characters can be represented in a key using the escapes in section 3.4, but decoded characters are not reinterpreted as physical lines or syntax separators.

For example, `"A B"`, `"A:B"`, `"A(B)"` and `"A#B"` are each one-element names. A consuming application can impose further restrictions on key-name characters through its configuration schema. Key-element escapes belong to the general syntax rules and are distinct from the rule that preserves the undecoded right-hand side in syntax results.

### 3.3 Key identity and element boundaries

Key identity is determined by the element sequence after quotes and escapes within quotes have been interpreted and decoded, and relative paths have been resolved. `A.B` and `A."B"` are the same key. `"A.B"` and `"A\.B"` are also the same key, distinct from the two-element key `A.B`. `A."B.C"` and `A."B\.C"` are the same two-element key, distinct from the three-element key `A.B.C`. A Unicode character written directly and an escape within quotes that decodes to that character compare as the same element name.

Unicode normalisation is not performed. Each decoded element is compared by exact equality as a valid UTF-8 string. Appearance or canonical equivalence alone does not establish identity: the precomposed character “が” (U+304C) and “か” followed by a combining voiced sound mark (U+304B U+3099) are distinct names. This rule also applies when grouping value occurrences by key, retrieving values and matching against a configuration schema. Format processing does not implicitly normalise type expressions or values either.

Syntax results retain the element sequence itself or a representation from which element boundaries can be recovered. Keys are not identified solely by a string made by removing quotes and joining elements with dots. A single element with an empty name `[""]` is also distinct from having no elements `[]`. This requirement also applies to a flat key space and does not require keys to be converted to nested objects.

The concrete output representation, canonical notation and retrieval API are undecided. The retrieval API must also distinguish elements containing dots, empty names and an omitted key specification.

### 3.4 Common escapes within quotes

A backslash starts an escape only inside double quotes around a key element or a value selected as quoted syntax, including the body of a triple-quoted value. A quote encountered within a literal single-line value does not start quoted syntax, so neither that quote nor later backslashes are interpreted. Backslashes in literal single-line values and pipe blocks are preserved as ordinary characters; backslashes in unquoted key elements are syntax errors. The following basic escapes and the Unicode escapes described below are available in all quoted text.

| Notation | Decoded character |
|------|-------------|
| `\.` | One dot `.` |
| `\"` | One double quote `"` |
| `\\` | One backslash `\` |
| `\/` | One slash `/` |
| `\b` | Backspace U+0008 |
| `\f` | Form feed U+000C |
| `\n` | Line feed LF U+000A |
| `\r` | Carriage return CR U+000D |
| `\t` | Horizontal tab U+0009 |

The basic forms above other than the dot, and the four-digit form described below, follow the escape notation used by JSON strings in [RFC 8259, section 7](https://www.rfc-editor.org/rfc/rfc8259.html#section-7). This does not adopt the entirety of JSON's string rules. Character restrictions on quoted keys and values, and rejection of invalid Unicode representations, follow this document's rules.

Escapes within quotes are recognised when finding the closing quote. Only symbols outside quotes that have syntactic meaning at their position are treated as key-element or value-line separators. An escaped double quote inside quotes does not end the quoted text. A backslash outside quotes does not protect quotes or separators. Processing must not decode the entire file before searching for separators.

Quoted text is decoded exactly once. Decoded characters are not reinterpreted as syntax such as path separators, quotes, type annotations, comments or indentation. Element boundaries are retained after keys are decoded. Unquoted single-line values are preserved unchanged apart from removal of outer ASCII spaces and tabs; pipe-block bodies are preserved after their line markers are removed, without whitespace trimming.

Unknown escapes that are not defined by this specification, and incomplete escapes that end prematurely, are errors within quotes. A quoted `\q`, or a lone backslash that reaches the end of a physical line or body while inside quotes, is neither delivered as literal characters nor silently removed. An unquoted `\q` or trailing backslash, including one in a pipe block, is ordinary text and is not subject to this escape validation. A backslash at the end of a line never indicates line continuation in any value form.

In principle, the range of characters expressible through escapes is unrestricted, provided that the file structure and the parser's separators remain intact. By not reinterpreting decoded characters as syntax, syntax symbols, line breaks and control characters can be treated as character data. This principle does not mean that arbitrary backslash notation is accepted unchanged.

The recommended notation for expressing Unicode characters using ASCII is `\u{H...}`, which specifies a code point directly. The braces contain one to six ASCII hexadecimal digits; both cases of `A`–`F` are allowed. Leading zeroes are allowed, but an empty specification, whitespace and underscores are not. The braced form takes inspiration from forms such as [Rust's Unicode escapes](https://doc.rust-lang.org/reference/tokens.html#unicode-escapes), without adopting Rust-specific rules such as underscores or line continuation.

The permitted values are Unicode scalar values: U+0000–U+10FFFF excluding the surrogate range U+D800–U+DFFF. Noncharacters and unassigned code points within that range are allowed if they are scalar values. U+0000, line breaks and control characters can also be represented as character data. Out-of-range values, invalid hexadecimal digits and a missing closing brace are errors; they are not replaced with the replacement character.

The JSON-compatible form `\uXXXX` is also accepted. Each `X` is an ASCII hexadecimal digit, and exactly four digits immediately following `u` are read. A non-surrogate value decodes to that character. A high surrogate U+D800–U+DBFF must be immediately followed by another `\uXXXX` representing a low surrogate U+DC00–U+DFFF, without intervening characters or whitespace. The pair decodes to one Unicode scalar value. Isolated high or low surrogates, reversed pairs, and invalid or incomplete pairs are errors. Surrogates in the braced form, and surrogate pairs mixing the two forms, are not allowed.

The following are independent inputs, each representing the single emoji `😀` (U+1F600) as its value. The first example is the recommended notation; both Unicode escape forms produce the same decoded result.

```text
A: "\u{1F600}"
```

```text
A: "\uD83D\uDE00"
```

```text
A: "😀"
```

In quoted keys, `"\u{1F600}"`, `"\uD83D\uDE00"` and `"😀"` are also the same one-element name. This is because their decoded escape results are equal, not because different strings are considered identical through Unicode normalisation.

Accepted escape notation is limited to the basic forms and the two Unicode forms listed in this section. Forms used in other languages, such as `\UXXXXXXXX`, `\xXX`, octal escapes, `\a`, `\v`, `\0` and `\'`, as well as `\:` and `\#`, are undefined and produce errors. The general policy of allowing the character range is distinct from whether a notation has been defined. Decisions on additional forms will be made separately as required.

## 4. Line classification and separators

Outside multiline bodies, lines other than blank lines and comment lines are classified after processing indentation and outer trailing whitespace. A value header can switch processing to one of the multiline forms in sections 8.3 and 8.4; body lines are not classified as headings, value lines or comments.

| Condition | Classification |
|----------------|-----------------------|
| There is a separating colon outside quoted key elements and outside the type expression | Value line. Without a type annotation, the first colon outside quotes is the separator |
| There is no separating colon, and the line ends with an unquoted `.` | Heading |
| Neither condition applies | Syntax error |

In a value line without a type annotation, the first colon outside quoted key elements is the separator. Everything after that separator is the right-hand side; quotes and colons on the right-hand side do not affect key quoting or line classification. Later colons are preserved in the right-hand side. A colon inside quotes is a character in the key, not a separator. The notation `\:` inside quotes is undefined and is not needed to include a colon in a name.

```text
A.Path: C:\data\state.db
A.QuotedPath: "C:\\data\\state.db"
A.Url: https://example.test:8787/path
```

`A.Path` and `A.QuotedPath` have different undecoded right-hand sides, but both become `C:\data\state.db` after common string decoding.

| Notation | Meaning or result |
|---------|--------------------------------------|
| `A.B:` | A value header for `A.B`; unset unless the immediately following line starts a pipe block. It is not a heading |
| `A.B.` | A heading. It does not set a value |
| `A.B.:` | Rejected because the final element is omitted |
| `A."":` | A value header with an explicit empty final name; unset unless the immediately following line starts a pipe block |
| `"A.B".` | A one-element heading containing a dot |
| `"A.B"` | Rejected because it has neither a value-line colon nor a heading's terminating dot |

In a value line with a type annotation, the colon following the outer closing parenthesis of the type annotation is the value-line separator. Parentheses within the type expression remain balanced, and colons within it are not treated as value-line separators. The type conversion layer defines the meaning of each type expression. Colons after the value-line separator remain part of the right-hand side.

## 5. Headings, relative notation and indentation

### 5.1 Absolute and relative lines

Absolute headings and value lines with fully qualified keys begin at the start of the line. Relative headings and relative value lines are indented by one more ASCII space than their parent heading and begin with `.`.

The leading dot of relative notation and the terminating dot of a heading appear outside quoted key elements. Quoted key elements, escapes within quotes and empty names written as `""` follow the same rules for fully qualified keys, relative keys and headings. A dot or `\.` inside a quoted element does not increase indentation depth or the number of written heading levels.

`"A.B".` is a one-element heading whose final unquoted dot is the terminator. `A\.B.` and `A\.` are rejected because their unquoted keys contain backslashes.

```text
A.

 .B.
  .C: first
  .D.E: second

 .F: third
```

The value lines resolve as follows.

```text
A.B.C: first
A.B.D.E: second
A.F: third
```

Outside multiline bodies, an indented non-comment line without a leading `.`, a relative line at column one, tab indentation, and a jump to a depth without a parent heading are rejected. Relative lines without indentation are not accepted. Multiline body indentation follows sections 8.3 and 8.4 and does not create headings.

### 5.2 Depth follows the number of written heading levels

The number of dot-separated elements is distinct from the number of written heading levels. A heading with a compound path still occupies one written level.

All three examples below assign a value to `A.B.C.D`. The `= A.B.C.D` is explanatory and is not written in the configuration file.

```text
A.B.C.
 .D: value      = A.B.C.D
```

```text
A.
 .B.C.
  .D: value     = A.B.C.D
```

```text
A.
 .B.
  .C.
   .D: value    = A.B.C.D
```

### 5.3 Choosing the parent after reducing indentation

Let `d` be the number of leading ASCII spaces. The parent of a relative line is the currently active heading at indentation `d-1`. When selecting the parent, headings deeper than that parent are closed. A value line does not create a new heading.

The final line in the following example is rejected.

```text
A.
 .B.
  .C: first
 .D: sibling
  .E: invalid
```

Reducing the indentation at `.D` closes the heading `A.B`. Since `.D` is a value line, it cannot be the parent of the final `.E`. Closed headings are not revived by searching backwards.

### 5.4 Closing headings with an absolute line

Processing an absolute line closes all preceding headings. An absolute heading starts a new heading; a value line with a fully qualified key leaves no active heading.

The final line in the following example is rejected.

```text
A.
 .B: first
X.Y: second
 .C: invalid
```

`X.Y: second` closes the headings, leaving `.C` without a parent. To resume relative notation, write an absolute heading again.

### 5.5 Assignment to a heading itself

If the left-hand side of a relative value line is a single unquoted `.`, the value is assigned to the parent heading's own path. A child element whose name is a single dot is written as `."."`, distinct from the assignment-to-self symbol. An unquoted `\.` is not accepted.

```text
A.B.
 .: parent
 .C: child
 .D.E: descendant
```

The value lines resolve as follows.

```text
A.B: parent
A.B.C: child
A.B.D.E: descendant
```

The ordinary value-key restrictions apply to the resolved path; no special key named `.` is added.

| Parent heading and value line | Result |
|---------------------------|---------------------|
| ` .: value` under `A.B.` | Accepted as an assignment to `A.B` |
| ` .: value` under `A.` | Accepted as an assignment to `A` |
| ` .: value` under `1.` | Accepted as an assignment to `1` |
| ` .: value` under `A.1.` | Accepted as an assignment to `A.1` |
| ` .Key: value` under `A.1.` | Accepted as an assignment to `A.1.Key` |

Assignment to a one-element heading itself can also be written as follows.

```text
A.
 .: value
```

These two lines represent the same key and right-hand side as `A: value`.

Assignment to the parent itself is distinct from assignment to a child element with an empty name. The following are independent inputs, both with an unset right-hand side.

```text
A.
 .:
```

This example has the key `A` and the element sequence `["A"]`.

```text
A.
 ."":
```

This example has the key `A.""` and the element sequence `["A", ""]`.

The same rules apply to a heading with an empty name.

```text
"".
 .:
```

This example assigns a value to the one-element key `""`. The single dot in relative notation is not interpreted as a child element with an empty name.

For assignment to a heading itself, omitting the right-hand side as in ` .:` still includes the resolved key and leaves its value unset unless its immediately following line starts a pipe block. Either multiline form can assign to the heading itself. A heading line on its own does not create a value-key occurrence.

Entirely numeric headings follow the same rules.

```text
1.
 .: parent
 .2: child
```

The value lines resolve as follows.

```text
1: parent
1.2: child
```

## 6. Type annotations

This section defines the position of type annotations, the prohibition on internal whitespace and the outer whitespace used for alignment. Section 8.1 describes type conversion and retrieval. The remaining details of type expression grammar and the concrete value formats accepted by each type are undecided.

### 6.1 Position and omission

An optional type expression enclosed in parentheses can appear between the key and the colon of a value line. The type annotation appears outside quoted key elements. The treatment of escapes within type expressions will be defined together with the type expression grammar.

```text
A.Code(string):001
```

In this example, the key is `A.Code`, the type expression is `string` and the right-hand side is `001`. The type annotation belongs to neither the key name nor the right-hand side. `A.Code:001`, without a type annotation, is also permitted.

The names `string`, `int` and `bool` in this section are provisional names used to explain the notation; they do not establish standard valid type names or type conversions.

The same rule applies to a one-element value key, which can be written as `A(string):value`.

Quoted key elements and empty names can also have type annotations. The following are independent input examples.

```text
A."B.C"(string):001
```

```text
""(string):
```

The latter line specifies a type expression for a one-element key with an empty name and leaves its value unset. It does not specify an empty type expression.

Type annotations are allowed only on value lines. There are no type annotations on headings and no inheritance of a type from a heading to its children. No rule for specifying type expressions collectively is introduced.

No whitespace, including ASCII spaces and tabs, is allowed inside the parentheses enclosing a type expression. `A( string ): value` and `A(list( int)): value` are not accepted. Type expressions are not restricted to simple ASCII type names, and symbols and nested parentheses for future compound types are not prohibited outright. Nested parentheses remain balanced and are distinguished from the type annotation's outer closing parenthesis.

For example, general syntax does not reject whitespace-free expressions such as `list(int)`, `map(string,list(int))`, `A|B`, `名前` and `[]` solely because they are not simple ASCII type names. These illustrate possible notation, rather than defining standard types or a standard compound type grammar. Value decoders and profiles determine the meaning and validity of type expressions. The concrete grammar for empty type expressions, and for quotes and escapes within type expressions, is undecided.

### 6.2 Whitespace for alignment

Zero or more ASCII spaces for alignment are allowed between the key and the type expression, between the type annotation's closing parenthesis and the colon, and after the colon. Their number does not change the meaning. Alignment spaces immediately before the colon are trimmed, so `A(string) : value` and `A(string):value` are equivalent. Since `(` outside quoted key elements starts a type annotation, its start can be recognised without alignment whitespace.

The following four independent examples represent the same key, type expression and right-hand side.

```text
A.Code(string):001
```

```text
A.Code (string): 001
```

```text
A.Code     (string):     001
```

```text
A.Code     (string)     :     001
```

The starting positions of type expressions and values can be aligned.

```text
A.Code     (string): 001
A.Count    (int):    123
A.Enabled  (bool):   true
```

Leading spaces used for indentation carry hierarchical meaning. They are processed separately from the alignment spaces between a key and its type expression; this rule does not permit arbitrary changes to the number of leading spaces.

The outer whitespace whose quantity is defined in this section consists of ASCII spaces. Alignment tabs between the key and the type expression, or between the closing parenthesis and the colon, remain undecided. Whitespace inside parentheses is prohibited as specified in section 6.1. Outer whitespace on value lines without type annotations follows section 7.

### 6.3 Relative value lines and assignment to a heading itself

On relative value lines and assignments to a heading itself, a type annotation also appears between the key portion and the colon. The following is an example.

```text
A.B.
 .(string): parent
 .Code(string): 001
 .Count (int): 123
```

| Line | Resolved key | Type expression | Undecoded right-hand side |
|-----------------------|-------------|----------|----------|
| ` .(string): parent`  | `A.B`       | `string` | `parent` |
| ` .Code(string): 001` | `A.B.Code`  | `string` | `001`    |
| ` .Count (int): 123`  | `A.B.Count` | `int`    | `123`    |

The value-key restrictions in section 3 and the path resolution rules in section 5 apply to the key portion after the annotation has been separated. An annotation neither creates a parent heading nor changes indentation depth.

### 6.4 Distinguishing parentheses on the right-hand side

A type annotation belongs to the left of the colon. The following right-hand side is not automatically converted into a type annotation on the left-hand side.

```text
A.Code: (string) 001
```

This line is treated as an unannotated value line with the key `A.Code` and the right-hand side `(string) 001`.

### 6.5 Validation and output of type expressions

Syntax processing is responsible for separating the key, type expression and undecoded right-hand side. Valid type names, the meaning of type expressions, value conversion and validation against the configuration schema are handled in later stages.

The output field names and API for type expressions, and the connection between configuration schemas, declarations and retrieval requests, are undecided. If a declared type expression differs from the retrieval request, the result is a type expression mismatch error. The general syntax layer does not begin automatic type inference for values without a type annotation.

With or without a type annotation, syntax parsing treats an empty right-hand side as unset unless the immediately following line starts a pipe block. This also applies to `A.Code(string):`. Explicit triple-quoted and pipe blocks are set values, even when their bodies are empty, and can use the same type annotations and assignment-to-self syntax as single-line values. When a string is requested and there is no declaration, or the declared type expression also matches the string type, an unset value is treated as equivalent to an empty string. A different type expression causes a type expression mismatch error even when the value is unset. The interpretation of unset values for non-string types will be defined separately.

## 7. Whitespace, blank lines and comments

### 7.1 Outer whitespace and whitespace within right-hand sides

Outside multiline bodies, after processing indentation, remove ASCII spaces and tabs at the end of a line, from both ends of the left-hand side of value lines without type annotations, and from both ends of a value header's right-hand side. Outer alignment spaces around type annotations follow section 6. Multiline bodies are exempt from this trimming: remove only the margin or line markers specified in sections 8.3 and 8.4.

```text
A.Message:     hello world
A.Padding:     "  hello  "
```

The right-hand sides preserved in this example are `hello world` and `"  hello  "`, respectively. Internal whitespace in the right-hand side is not removed as alignment whitespace. Quotes remain part of the right-hand side at the syntax layer, so whitespace within them is preserved.

Common string decoding in section 8.2 takes place before delivery as a value. In the latter example, the surrounding double quotes are removed, and the delivered string has two ASCII spaces at each end. Whitespace within quotes is part of the value and is not trimmed again after decoding.

With or without quotes, outer whitespace is removed from the undecoded input. Only quoted values are then decoded within their quotes, and decoded strings are not trimmed again. Backslashes in unquoted values do not protect whitespace; outer ASCII spaces and tabs are removed as usual. Whitespace produced by `\t`, `\n` or Unicode escapes inside quotes is also preserved after decoding.

Unicode whitespace other than ASCII spaces and tabs, such as an ideographic space or NBSP, is not removed. Such characters are not treated as indentation characters either.

### 7.2 Blank lines and comment lines

Outside multiline bodies, after removing ASCII spaces and tabs from both ends of a line, an empty result is skipped as a blank line, and a result beginning with `#` is skipped as a comment line. Blank lines and comment lines do not change the current headings. Within a triple-quoted body, these lines are content; within a pipe block, only consecutive marked lines belong to the body. Blank lines and comments are not skipped when checking whether an empty header is immediately followed by a pipe block.

```text
A.
 .B.

# This comment also preserves the headings
  .C: value
```

Blank lines and comment lines may contain tabs or deep indentation. Character encoding and lone CR validation still take place before they are skipped.

Comment text is not subject to escape validation or decoding. A `#` produced by string decoding is not reinterpreted as a comment line or an inline comment.

There are no inline comments. The following right-hand sides are `value # literal` and `#literal`.

```text
A.B: value # literal
A.C: #literal
```

Notation that adds an inline comment to a heading is not accepted.

### 7.3 Recommended formatting

A single blank line is recommended before a heading that changes the group. This applies when moving to a relative heading as well as to an absolute heading.

This is a readability convention, not a syntax requirement. The presence or number of blank lines does not change or end a group. Illustrative examples may omit blank lines.

Quote an entire key element when its name contains dots, double quotes or backslashes. Since dots can be written directly within quotes, `"A.B"` is normally recommended. `"A\.B"` is also a valid alternative. The unquoted form `A\.B` is a syntax error under section 3.

## 8. Preserving right-hand sides and interpreting values

For a single-line value, an empty right-hand side after removing outer ASCII spaces and tabs is treated as unset, provided that the immediately following line does not start a pipe block. Other single-line right-hand sides are preserved undecoded. For a multiline value, preserve its form and undecoded body after removing structural markers and the prescribed margin; the value is set even when the body is empty. These rules apply with or without a type annotation. The following single-line examples can be preserved as undecoded syntax results, but require common string decoding before they are delivered as values. Preserving undecoded text alone does not establish that it is a valid value.

```text
A.Text: value
A.Number: 001
A.Boolean: true
A.NullText: null
A.Empty:
A.QuotedEmpty: ""
A.QuotedSpaces: "  "
A.UnclosedQuote: "unterminated
A.Sequence: [1,2]
```

| Input right-hand side | Undecoded right-hand side | Treatment by the format |
|-----------------|------------------|-----------------------------|
| Empty, without an immediately following pipe body | A zero-length string | The value is unset |
| ASCII spaces and tabs only, without an immediately following pipe body | A zero-length string after outer whitespace is removed | The value is unset |
| `""` | Two quote characters | Distinct from an empty right-hand side. An explicit empty string after common string decoding |
| `null` | The four characters `null` | A non-empty right-hand side. Neither unset nor a special literal |
| `001` | Three characters, including the leading zeroes | Preserved undecoded |
| `"unterminated` | A string including an unclosed quote | Can be preserved undecoded, but string decoding fails because the quote is not closed |

Syntax results preserve the distinction between unset, the two-quote single-line right-hand side `""`, an explicitly empty multiline body and the four-character right-hand side `null`. Unset means that no value has been specified; it is not a literal representing a value of a particular type. Extracting right-hand sides or block bodies does not infer types such as numbers, booleans, lists or arrays, nor decode backslashes. Common string decoding removes ordinary surrounding double quotes, decodes escapes in ordinary quoted values and triple-quoted bodies, and passes the result to type conversion. Pipe bodies and backslashes in unquoted single-line values are preserved literally. Adoption of JSON's entire string rules is not specified.

When a consuming application requests a string and there is no type expression mismatch with a declaration, the received unset value is treated as equivalent to a zero-length string, that is, the string value `""`. The unset state is not rewritten to a right-hand side containing two quote characters. An explicit empty string is a value obtained through common string decoding and is retained separately from whether the value is unset.

For example, receiving the parse result of `A:` as a string gives the equivalent of an empty string. Syntax results preserve whether the value is unset, so the consuming application can also inspect that state as required. The interpretation of unset values for non-string types, and their relationship to required fields and defaults, will be defined separately.

A single-line value is unset only when there is no content after the separating colon and no immediately following pipe body. An explicit empty block is set. A non-empty right-hand side does not become unset merely because it ends with a colon. The following right-hand sides are preserved as the strings `:` and `https://example.test:`, respectively.

```text
A.Symbol: :
A.Url: https://example.test:
```

A value line with an empty right-hand side still records a value occurrence for its key. Repeated unset or explicit empty occurrences are all retained. This is distinct from omitting the key itself or writing a heading line, which does not create a value occurrence.

Apart from rejecting invalid UTF-8 and lone CR, there is no blanket prohibition on NUL, internal tabs or similar characters on the right-hand side. The consuming application validates any necessary control-character restrictions.

Environment variable expansion, path expansion, includes, additional file reads and implicit string concatenation do not happen automatically. Multiline values use only the explicit forms in sections 8.3 and 8.4. Square and curly brackets are not grouping syntax and can be preserved as characters on the right-hand side.

### 8.1 Type conversion and retrieval

Except for unset values, a value that has completed common string decoding according to sections 8.2–8.4 is initially treated as a string. A “declaration” is a type expression specified separately from a retrieval request, for example in a value line's type annotation. The appearance of a value does not trigger automatic inference of numbers, booleans or other types. Enclosing a value in double quotes does not also declare it to have the string type; the same conversion procedure applies according to whether a type annotation is present.

Type conversion and retrieval follow this procedure.

Apply this procedure independently to each value occurrence, retaining its own type declaration and unset state. The format does not require all occurrences of a key to declare the same type, and a type annotation does not change which key an occurrence belongs to. Collection-retrieval APIs and their policy for groups with different declarations remain implementation choices; this section does not prescribe an aggregate API.

1. If there is a declaration, convert only to its specified type expression. Do not try another type expression or fall back to a string for a value that cannot be converted.
2. If there is no declaration, initially retain the value as a string. When retrieval requests a type expression, attempt conversion to that requested expression. Without a retrieval request either, retain the string.
3. When retrieving a declared value with a requested type expression, check whether it matches the request. If it matches, return the value in the declared type; otherwise, report a type expression mismatch error. Do not consider whether conversion to another type is possible.
4. If an undeclared value cannot be converted to the type expression requested at retrieval, also report a type expression mismatch error.

| Declaration | Retrieval request | Treatment |
|------|-----------|---------------------------------|
| None | None | Remains a string |
| None | Type expression T | Attempt conversion from the string to T. Failure is a type expression mismatch error |
| Type expression T | Type expression T | Return the value of T converted according to the declaration |
| Type expression T | A different type expression U | Type expression mismatch error. Do not reconvert to U |

Retrieval as a type expression different from the declaration does not first convert the value back to a string. Requesting a string is also a type expression request and is not an exception to another declaration. Type expressions are checked for a match before an unset value is treated as equivalent to an empty string.

The following are independent retrieval examples. `string` and `int` are provisional type names; assume here that `123` can be converted to an integer and `abc` cannot. These examples do not establish standard type names or the value formats accepted by each type.

| Configuration notation | Retrieval request | Result |
|-----------------------|--------|--------------|
| `A.Count:123`         | String | String value `"123"` |
| `A.Count:123`         | Integer | Integer value `123` |
| `A.Count:abc`         | Integer | Type expression mismatch error |
| `A.Count(int):123`    | Integer | Integer value `123` |
| `A.Count(int):123`    | String | Type expression mismatch error |
| `A.Count(string):123` | Integer | Type expression mismatch error |
| `A.Count:`            | String | Equivalent to an empty string |

The error category and notification point when conversion to the declared type expression itself fails, type expression identity and aliases, the concrete retrieval API, and how conversion results are retained at retrieval are undecided. Common string decoding follows the value-form rules in sections 8.2–8.4, using the escapes in section 3.4 where applicable; conversion rules for individual types will be defined separately.

### 8.2 Double-quoted values and common string decoding

A single-line value can be enclosed in double quotes in its entirety to preserve leading or trailing whitespace, or to include double quotes themselves in the value. The surrounding double quotes are removed before delivery as a value. Leading and trailing whitespace inside the quotes is preserved as part of the value, and the decoded string is not trimmed again. An entire header right-hand side consisting of `"""` opens the multiline form in section 8.3 instead; a pipe body under section 8.4 is interpreted literally.

Common string decoding happens before type conversion; removing quotes is not a rule left for each application to implement. The undecoded right-hand side can be retained separately as a syntax result. If the undecoded right-hand side begins with a double quote after outer ASCII spaces and tabs have been removed, it is treated as a quoted value, and the escapes in section 3.4 are decoded only within those quotes. Ordinary values that do not use quoted syntax preserve backslashes unchanged.

Quoted syntax is selected only when the first character of a non-empty single-line right-hand side, after removing outer ASCII spaces and tabs, is a double quote. If that first character is anything else, the entire trimmed right-hand side is literal. Preserve every later double quote, triple-quote sequence and backslash as character data without requiring quote balance or interpreting escapes. A later quote never switches a literal value into quoted syntax or opens a multiline block. This does not change the exact `"""` header rule in section 8.3 or pipe-block detection in section 8.4.

Surrounding quotes are identified before escape decoding. `"\"hello\""` decodes to a string that includes double quotes at both ends. Those decoded quotes are not read again as surrounding quotes and removed. The literal form `\"hello\"` begins with a backslash, so both backslashes and both quotes are retained unchanged. These are different source forms and produce different strings.

Within quotes, the basic and Unicode escapes in section 3.4 are available, as they are for key elements. An escaped double quote is part of the value, not an opening or closing quote. Decoding happens exactly once, and decoded characters are not interpreted again as escapes or syntax symbols.

The following are independent input examples. The right-hand column explains the string decoding result before delivery; any type conversion specified by a type annotation is applied afterwards.

| Notation | Decoded value or state |
|------|----------------|
| `A: value` | The string `value` without leading whitespace |
| `A: A\.B` | The string `A\.B`, including its backslash |
| `A: foo\.bar` | The string `foo\.bar`, including its backslash |
| `A: \q` | Two characters: a backslash and `q`. It is not an escape |
| `A: value\` | A string ending in a backslash. It is not line continuation |
| `A: say "hello"` | The literal string `say "hello"` |
| `A: x"unterminated` | The literal string `x"unterminated`; later quotes need not be balanced |
| `A: \"hello\"` | The literal string `\"hello\"`, including both backslashes and quotes |
| `A: {"x": "\u0041"}` | The literal string `{"x": "\u0041"}`; neither JSON parsing nor escape decoding is performed |
| `A: text """ more` | The literal string `text """ more`; no multiline block is opened |
| `A: "\"hello\""` | The string `"hello"`, including the quotes at both ends |
| `A: C:\temp\new` | The string `C:\temp\new` |
| `A: "C:\\temp\\new"` | The string `C:\temp\new` |
| `A: "  value"` | A string with two leading ASCII spaces |
| `A: "value  "` | A string with two trailing ASCII spaces |
| `A: "  "` | A string consisting of two ASCII spaces |
| `A: ""` | An explicit empty string. It is not unset |
| `A:` | Unset. Equivalent to an empty string when string retrieval is permitted |
| `A: "say \"hello\""` | The string `say "hello"` |
| `A: "\""` | One double quote |
| `A: "\\"` | One backslash |
| `A: "first\nsecond"` | A string containing `first`, one LF and `second`, in that order. The physical line is not split |
| `A: "\tvalue\u{20}"` | A string with a leading horizontal tab and a trailing ASCII space. It is not trimmed after decoding |
| `A: "\u{1F600}"` | The single emoji `😀` |
| `A: "\uD83D\uDE00"` | The single emoji `😀` |

Whether a value is unset is determined from its source form before string decoding. An explicit value is not changed to unset even if it decodes to an empty string. By itself, `A:` and `A: ""` are equivalent when retrieved as strings, but remain distinct in syntax results and configuration state. Empty triple-quoted and pipe bodies are also explicit set values.

An ordinary single-line value whose first trimmed character is a double quote requires a closing quote. Although `A: "unterminated` can be retained as an undecoded syntax result, string decoding fails and it is not delivered as a configuration value. Only outer ASCII spaces and tabs are allowed after the closing quote; any other trailing characters are errors. A value that starts quoted must be quoted in its entirety; failure does not fall back to literal text. There is no partial quoting or implicit string concatenation. A value starting with another character instead follows the literal rule above, including any later quotes. Triple-quoted bodies and literal pipe bodies follow their own rules below.

Each of the following independent examples fails string decoding and is not delivered as a configuration value.

```text
A: "value"tail
```

```text
A: "unterminated
```

```text
A: "\
```

```text
A: "first" "second"
```

```text
A: "\q"
```

Unknown or incomplete escapes within quotes are errors under section 3.4. This escape validation does not apply to an unquoted `\q` or a trailing backslash.

Within a quoted value, write `\"` to deliver a quote as character data. In a literal value, later quotes are already ordinary characters and need no escaping. The concrete categories and notification points for decoding errors are undecided. Neither quoted keys nor quoted values reinterpret decoded characters as syntax. The prohibition on whitespace, control characters and similar characters in unquoted key elements does not apply to values as a blanket rule.

### 8.3 Triple-quoted multiline values

A header whose entire right-hand side, after removing outer ASCII spaces and tabs, is exactly `"""` opens a triple-quoted value. The opening marker has no inline body content. Its body starts on the next physical line and ends at a closing marker on its own line. An opening marker without a closing line is a syntax error.

This header check is the only way to start a triple-quoted block. A `"""` sequence later in a value or on a subsequent physical line does not retroactively open one. A line matching the closing-marker shape has delimiter meaning only while a triple-quoted block is open; otherwise it is processed under the ordinary line rules and must satisfy those rules.

```text
A.
 .Message: """
  first\nsecond
    indented
  """
 .Next: value
```

A closing line consists of zero or more leading ASCII spaces, exactly `"""`, and optional trailing ASCII spaces or tabs. Let `m` be its number of leading spaces. Remove exactly `m` leading ASCII spaces from every non-empty source body line; a line with no characters is exempt. A non-empty line with fewer than `m` leading spaces is an error, including a whitespace-only line that is too short. Additional leading spaces and all other body whitespace are preserved. Tabs do not count towards this margin. The closing margin is independent of the header indentation and can be zero, including for pasted public-key text.

In the example, `m` is two. The first source body line becomes `first\nsecond`, and the second retains two leading spaces. With LF source line endings, the decoded value is `first`, LF, `second`, LF and two spaces followed by `indented`. The body neither creates headings nor changes the heading scope; `.Next` remains a child of `A`.

After margin removal, preserve the original LF or CRLF between body lines. The line ending following the opening marker and the line ending immediately before the closing marker are boundaries and are excluded from the value. No terminal newline is inferred. To retain a terminal newline, add an empty body line before the closing marker:

```text
A.Message: """
  text

  """
```

This body is `text` followed by one source line ending. An opening marker immediately followed by a closing marker produces an explicit empty string, with the unset state false.

Decode the body using exactly the same basic and Unicode escapes as an ordinary quoted value. Preserve literal LF or CRLF and literal quote characters in the body; do not treat ordinary quotes as delimiters. Triple quotes on a line containing any other non-whitespace content are also data. Only an exact closing line ends the block. To include a body line that would otherwise be a closing marker, escape at least one of its quotes, for example `\"""`, or use a pipe block. Escape validation remains strict, a backslash does not continue a physical line, and Unicode normalisation is not performed.

### 8.4 Literal pipe blocks

A value header with an empty right-hand side starts a pipe block only when its immediately following physical line has more leading ASCII spaces than the header and then begins with `|`. Do not skip blank lines or comments when making this decision. If that immediate next line does not qualify, the header is unset and that next line is processed normally. A non-empty header such as `A:|` has the ordinary single-line string value `|`.

```text
A.
 .Literal:
  |C:\temp\new
  |"quoted" \q
  |"""
  |# literal body text
 .Next: value
```

Collect consecutive qualifying pipe lines. Each line independently needs more leading ASCII spaces than the value header; their indentation need not be identical. Remove all leading ASCII spaces and the first `|` from each collected line. Everything after that first marker, including spaces, tabs, additional pipes, quotes and backslashes, is literal body text. There is no escape decoding or further whitespace trimming, and encountering a quote never switches the body into quoted syntax. In the example, the Windows path and `\q` retain their backslashes, `"""` is three literal quote characters rather than a block delimiter, and `#` does not begin a comment.

Preserve the original LF or CRLF between collected body lines, excluding the line ending between the header and the first body line and the final body line ending. A final pipe with no content explicitly adds a terminal source line ending after the preceding body text:

```text
A.Literal:
 |text
 |
A.Empty:
 |
```

`A.Literal` is `text` followed by one source line ending. `A.Empty` is an explicit empty string, with the unset state false. A body line such as ` |  text` retains the two spaces after its marker. Quotes and backslashes need no escaping in this form.

The first line that does not qualify ends the block and is processed normally, with no line discarded. End of input also completes a pipe block without a closing marker. Blank lines, comments, same-depth or shallower lines, and non-pipe lines therefore end a block. A same-depth or shallower `|:` line is not body content; it must satisfy ordinary key and indentation rules. In particular, an unindented `|: value` can assign to the key named `|`, while an indented absolute key is still invalid. Outside blocks, normal scope, indentation and whole-line comment rules apply. Type annotations and assignment to a heading itself work for both multiline forms; block bodies do not create or close headings.

## 9. Repeated values and read state

Within one read operation, the same fully qualified value key can occur more than once. Retain every occurrence in original source order, including occurrences with identical values. Do not overwrite earlier values, deduplicate equal values or treat repetition as a syntax error. Fully qualified keys, relative keys, assignments to headings themselves, quoted key elements and alternative escape forms use the same key-identity rules in section 3.3.

The following records three values for `A.Tag`, in the order `red`, `green`, `red`.

```text
A.Tag:red
A.Tag:green
A.Tag:red
```

A heading alone does not create a value occurrence, and headings may be redeclared. Each read operation starts with fresh heading state and its own occurrence sequence. Do not implicitly share headings or values across read calls or merge separately read files.

The same applies to one-element value keys. Writing both `A: value` and ` .: value` under the heading `A.` within one read operation records two occurrences for the same fully qualified key `A`.

Adding a type annotation does not change key identity, which is based on the resolved fully qualified key. Preserve the type annotation, unset state, value form and source position of each occurrence independently. Typed and untyped occurrences may coexist; individual type conversion follows section 8.1.

Assignments to `A.B` and `A."B"` belong to the same key and are both retained. `"A\.B"` and `"A.B"`, and `A."B\.C"` and `A."B.C"`, likewise identify the same key within each pair. `A."B.C"` and `A.B.C` have different element sequences and remain separate keys. Multiple occurrences of the key `""` are retained normally; an empty name does not mean that the key was omitted.

A consuming application may require a particular key to have one value, limit its occurrence count or impose other cardinality rules through its configuration schema. Such a constraint belongs to application validation, rather than the general syntax. Concrete collection-retrieval APIs, indexed retrieval and handling of mixed declared types during aggregate retrieval remain outside the format's settled API contract.

## 10. Character encoding, line endings and length

| Item | Rule |
|-----------|-------------------------------------------------|
| Character encoding | UTF-8 |
| BOM | If present, remove one BOM at the start of the file. Do not remove BOMs at other positions as a blanket rule |
| Line endings | LF and CRLF are allowed. The final line may omit its line ending. Lone CR is rejected |
| Physical line length and key length | The format imposes no fixed upper limit |
| Expanded path length | The format imposes no fixed upper limit on fully qualified keys or heading paths |
| Hierarchy depth | The format imposes no fixed upper limit |
| Total file length | The format imposes no fixed upper limit |

Blank lines, comment lines and multiline body lines are also subject to character encoding and line-ending validation. Each heading, key and value header occupies one physical line. A triple-quoted or pipe value continues over its body lines under sections 8.3 and 8.4. Between body lines, preserve the original LF or CRLF; do not normalise one to the other. The number of alignment whitespace characters is also unrestricted.

An implementation or consuming application may impose resource constraints according to its memory usage, input source or other circumstances. Such constraints must be distinguished from the format's own acceptance conditions, with their content and the behaviour when exceeded stated explicitly. Key and value length constraints imposed by an application's configuration schema are also distinct from format limits.

## 11. Responsibilities of syntax processing and consuming applications

| Layer | Responsibilities |
|-----------------|------------------------------------------------------------------------------------------------|
| General syntax processing | Processing physical lines, character encoding, indentation and headings; interpreting quoted key elements, decoding their escapes and retaining their boundaries; resolving and validating paths; preserving every value occurrence, line number and source order without overwriting or deduplication; separating keys and type expressions; recognising value forms; removing block markers and margins while retaining undecoded bodies and their internal source line endings; identifying each occurrence's unset state |
| Common string decoding | Removing ordinary surrounding quotes before delivering a value; decoding the basic and Unicode escapes in section 3.4 in ordinary quoted values and triple-quoted bodies; preserving unquoted single-line values and pipe bodies literally under their whitespace rules; not trimming again after decoding; distinguishing unset from an explicit empty string |
| Type conversion and retrieval | Retaining values with strings as the basis; converting only to the declared type expression; matching the requested type expression at retrieval; converting at retrieval when there is no declaration; reporting type expression mismatch errors; interpreting unset as equivalent to an empty string when string retrieval is permitted |
| Value decoders and profiles | Assigning meaning to type expressions and defining individual conversion rules for numbers, booleans, enums, lists and other types. Their input is the result of common string decoding |
| The application's configuration schema | Validating known keys and headings, required fields, whether unset and empty strings are allowed, ranges, lengths, control characters, file ownership boundaries, and consistency between keys and between files |

### 11.1 Rules defined by consuming applications

A consuming application defines the following rules as required. Their concrete details are not fixed rules of the general syntax.

- Supported keys and headings, required fields and defaults.
- Whether each key accepts one or more value occurrences, and any occurrence-count constraints.
- Type-specific interpretation and validation of values after common string decoding. Double quoting and basic and Unicode escapes in key elements and values follow this document; the entirety of JSON's string rules is not applied automatically.
- Whether unset and empty strings are allowed, and validation of explicit empty strings and the string `null`. The unset state of an empty right-hand side, its equivalence to an empty string when string retrieval is permitted, and type conversion and type expression mismatch handling at retrieval follow this document.
- Formats, conversions and per-value constraints for numbers, booleans, codes, enums, lists and other types.
- Validation of control characters, character counts, byte lengths, numeric ranges and other limits.
- File division and placement, key ownership boundaries and consistency between settings.
- Path resolution, correspondence with the environment, and additional constraints such as total file length.

A version number independently assigned by a consuming application identifies its configuration schema and is distinct from a format version of this notation itself.

### 11.2 Syntax results and failure contracts

Syntax results retain the one-based physical line number of the heading or value header, resolved key or heading path, value form, undecoded right-hand side or block body, whether the value is unset, whether the entry is a heading or a value, and the type expression if a type annotation is present. Single-line, triple-quoted and pipe forms must be distinguishable without inspecting the body to guess its syntax. A block body excludes markers, its prescribed margin and boundary line endings; it preserves original line endings between body lines. Blocks always have the unset state false. All physical lines count towards line numbers; body lines belong to one value entry, while blank lines and comments outside bodies do not generate result items. Heading paths do not include the syntactic trailing dot. A heading does not create a value-key occurrence and is distinct from an unset value. The concrete public API and field names remain undecided.

Resolved keys and heading paths retain element boundaries and empty names according to section 3.3. Quotes surrounding key elements are not included as name characters, and escapes are retained as decoded names. This is separate from the rule that preserves quotes and backslashes on the right-hand side undecoded.

Before configuration values are delivered, common string decoding according to the form in sections 8.2–8.4 takes place, and its results form the basis of type conversion and retrieval in section 8.1. The undecoded right-hand side or body retained as a syntax result is distinct from the decoded string or converted value delivered as a value. Access to undecoded text does not bypass form-specific decoding. A complete triple-quoted body with an invalid escape can be retained undecoded, but it cannot be delivered as a setting value; a missing closing marker or insufficient margin is already a syntax failure.

An incremental notification interface delivers headings and complete values in source order and receives the result of each recipient's processing before parsing or delivering the next entry. Multiline bodies are collected before their value is notified. Determining whether an empty header starts a pipe block, or whether that block has ended, can require inspecting one following physical line; if it is not body content, retain it for normal parsing after the current notification succeeds. If a recipient returns an error, return that error immediately and unchanged, without parsing or delivering later entries. Physical-line lookahead for block boundaries and byte-level read-ahead from the underlying input are permitted. Changes to external state already made by a recipient are not rolled back.

An interface that collects and returns results returns a failure, without partial parse results, if processing fails partway through. Repeated keys are delivered and collected as separate occurrences in source order, including repeated identical values. Empty input succeeds with zero results. An error is returned if a required input source or notification recipient has not been supplied.

Syntax errors include line numbers, keys and causes as required, but do not include right-hand sides or error strings from underlying input processing. The consuming application is responsible for the safety of diagnostics independently returned by notification recipients.

## 12. Undecided matters

This document does not settle the following matters.

| Item | Undecided details |
|-------------|--------------------------------------------------------------------|
| Format version and file extension | Numbering scheme, version declaration method and dedicated file extension |
| Type names | Standard permitted type names, aliases and case sensitivity |
| Remaining details of type expression grammar | Empty type expressions and the concrete grammar for quotes and escapes within type expressions. The prohibition on internal whitespace, and the decision not to prohibit symbols and nested parentheses for compound types outright, are established. The type conversion layer defines the meaning and validity of each type expression |
| Remaining whitespace rules for type annotations | Alignment tabs between the key and the type expression, or between the closing parenthesis and the colon |
| Future escape extensions | Whether to add forms beyond the basic notation, `\u{H...}` and `\uXXXX` defined in section 3.4. The Unicode scalar value range, surrogate pairs in the four-digit form, rejection of invalid notation and absence of replacement are established |
| Unicode character classification | The concrete classification of whitespace and control characters in unquoted key elements. The absence of Unicode normalisation, and the principle that quoted key elements allow any valid UTF-8 string, are established |
| Remaining details of value quoting | Categories and notification points for decoding errors. First-character selection between wholly quoted and literal single-line values, preservation of later literal quotes and backslashes, triple-quoted markers and margins, literal pipe detection, form-specific whitespace and escape handling, and explicit empty block state are established in sections 8.2–8.4 |
| Type-specific decoders | Concrete meaning and conversion for numbers, booleans, lists and other types. Common string decoding, empty-header detection including pipe blocks, explicit empty block values and the equivalence of unset to an empty string when string retrieval is permitted are established |
| Type-specific interpretation of unset | Treatment in non-string types and the relationship to required fields and defaults |
| Type expression identity | Matching of type aliases, alternative spellings, compound types and similar expressions |
| Failure of conversion to a declared type | Error categories and notification points when conversion to the specified type expression is impossible. No fallback to another type expression or a string is permitted |
| Relationship to schemas | Connecting configuration schemas to declarations and retrieval requests, and handling unknown type names |
| APIs and value retention | Output representation and canonical notation for key paths that preserve element boundaries; output representation for type expressions; single, collection and indexed retrieval APIs, including aggregate handling of mixed declared types; retention of conversion results at retrieval; schema API; profile selection method. Preserving all source-ordered occurrences without overwriting is established |
| Formatter | Whether one will be provided and its formatting policy |

## 13. Conformance checking

Implementation conformance is checked against the rules defined in this document and the following aspects.

| Aspect | Checks |
|-----------|-----------------------------------------------------------------------------------------------------------------|
| Key paths | Prohibited characters, including backslashes, and the non-empty requirement for unquoted elements; Japanese, underscores, hyphens, keys and headings beginning with digits or consisting only of digits; rejection of consecutive unquoted dots, empty left-hand sides and omitted final elements; case sensitivity; preservation of elements with leading zeroes |
| Quoted key elements | Allowing any valid UTF-8 string in principle, without a blanket rejection of whitespace, tabs, syntax symbols or control characters; empty names written as `""`; the requirement for at least one element; quotes in first, intermediate and last positions; application to fully qualified keys, relative keys and headings; distinction between the parent itself and a child with an empty name; preservation of element boundaries; rejection of quoted text spanning physical line breaks |
| Escapes in key elements | Acceptance of the basic and Unicode escapes in section 3.4 within quotes; identity of `"A\.B"` and `"A.B"`; rejection of `A\.B` with a backslash outside quotes; recognition of separators, heading terminators, leading relative dots and assignments to headings themselves outside quotes; no reinterpretation of decoded characters as syntax symbols |
| Invalid escapes | Rejection of unknown or incomplete escapes within quotes; preservation of unquoted `\q` and trailing backslashes; no line continuation for trailing backslashes, whether quoted or unquoted |
| Unicode escapes | One to six ASCII hexadecimal digits within braces; acceptance of both cases and leading zeroes; acceptance of the four-digit form and correct surrogate pairs; identity of direct and escaped forms; rejection of values outside the scalar range, isolated surrogates, reversed or mixed-form pairs, and invalid digits or digit counts; no substitution with the replacement character; acceptance of noncharacter and unassigned scalar values |
| Coexistence of keys | `A`, `A.B` and `A.B.C` can each have a value |
| Unicode identity | No normalisation; exact comparison of decoded elements; precomposed characters and combining sequences coexist as distinct keys; the same rule applies to retrieval and schema matching; type expressions and values are not implicitly normalised either |
| Equivalent notation | Fully qualified keys, headings, relative notation and assignments to headings themselves resolve to the same fully qualified keys and right-hand sides |
| Indentation and headings | Parent selection by written heading levels; closing headings when indentation decreases; closing headings with absolute lines; preserving state across blank lines and comments |
| Type annotation position and alignment | Optional left-hand annotations only on value lines; rejection of annotations on headings and inheritance to children; zero or more ASCII spaces between the key and type expression, between the closing parenthesis and colon, and after the colon, with equivalent meaning; rejection of internal whitespace; extraction with balanced parentheses; no blanket prohibition on symbols for compound types; distinction from parentheses on the right-hand side |
| Preserving right-hand sides | Removing outer ASCII spaces and tabs only from single-line values and headers; preserving internal whitespace, Unicode whitespace, quotes, leading zeroes, colons and `#`; retaining each block body after removing only its structural markers and margin |
| Common string decoding of values | After outer ASCII SP/HT removal, selecting quoted syntax only from an initial double quote; otherwise preserving the whole single-line value literally, including later unbalanced quotes, triple quotes and backslashes; removing ordinary surrounding quotes and decoding escapes only for quoted forms; literal pipe bodies; no reinterpretation or second trimming; distinction between explicit empty and unset; rejection of invalid starting quotes, trailing text after their closing quote and invalid quoted escapes without fallback |
| Empty right-hand sides | Treating empty right-hand sides, and those containing only outer ASCII spaces and tabs, as unset unless the immediately following line starts a pipe block; distinguishing unset from explicit empty quoted or block values, the string `null`, headings, omitted keys and non-empty right-hand sides ending in a colon; applying the same rules to type annotations and assignments to headings themselves |
| Triple-quoted values | Exact opening marker without inline content, recognised only in the value header; no transition into a block at later quotes or lines; closing-marker meaning only within an open block; own-line closing marker; closing-margin removal from every non-empty source line with empty-line exemption; insufficient-margin and missing-close rejection; ordinary quoted escapes; literal internal quotes and triple quotes outside exact closing lines; no reinterpretation or normalisation |
| Pipe blocks | Immediate-next-line detection after an empty header; strictly deeper ASCII-space indentation; consecutive markers only; literal body after stripping leading spaces and the first pipe; no skipped blank lines or comments; normal processing of the first non-body line; `A:\|` as a single-line value and non-body `\|:` lines subject to ordinary key and indentation rules |
| Multiline state and line endings | Explicit empty bodies are set; original LF or CRLF between body lines; no opening or closing boundary newline and no inferred terminal newline; an additional empty body line or pipe retains a terminal newline; header line numbers, form metadata, one entry per complete block, unchanged heading scope and support for typed and assignment-to-self entries |
| Type conversion and retrieval | Initially treating values as strings; converting only to the declared type expression; retaining strings without a declaration and converting to the type expression requested at retrieval |
| Type expression mismatch errors | Reporting a mismatch when the declaration differs from the retrieval request, regardless of convertibility; the same error when an undeclared value cannot be converted as requested at retrieval |
| String interpretation of unset | Treating an unset value received from the parser as equivalent to a zero-length string when a string is requested without a type expression mismatch with a declaration; also retaining the unset state in syntax results |
| Repeated values | Preserving every occurrence in source order, including repeated identical values, across fully qualified and relative notation, quotes and type annotations; retaining each occurrence's type and unset state; no overwriting, deduplication or syntax-level rejection of repeated keys; distinct element boundaries; no value occurrence from a heading; fresh state for each read |
| Character encoding and length | UTF-8, BOM, LF, CRLF, final lines without a line ending, rejection of lone CR; no format-defined fixed upper limits on physical lines, keys, expanded paths, hierarchy depth or the entire file |
| Output and failure | Source order and physical line numbers; empty input; stopping on a notification recipient's error; no partial results from a collecting interface; no right-hand sides in diagnostics |

Complete conformance conditions for type expression grammar and value conversion will be defined after the relevant specifications are decided. The illustrative type names in this document alone do not establish valid type names or conversion results.
