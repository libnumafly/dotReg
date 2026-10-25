# dotReg editor support

The JavaScript implementation folder is also a VS Code extension package. It
uses the repository's JavaScript parser, so diagnostics follow the dotReg 0.1
syntax instead of a separate editor-only grammar.

## Features

- Format Document and format-on-save integration. The formatter standardises
  key/type spacing and the colon/value separator while preserving key order,
  indentation, comments, line endings, and multiline bodies.
- Live parser diagnostics for structural errors and warnings when common string
  decoding fails.
- Quick fixes to insert the required file declaration or add a type annotation.
  Type annotations can use an illustrative candidate, the Go sample's `int`
  type, or a custom expression.
- Type-name completion, syntax highlighting, 2-space indentation settings, and
  snippets for value and multiline forms.
- Hover information with resolved key elements, type annotation, value form,
  and unset state.
- A hierarchical Document Outline and Breadcrumbs view built from resolved key
  elements. Repeated values remain separate occurrences in source order.

Formatting is available only when the complete buffer parses. It does not
rewrite keys, reorder repeated values, infer types, or alter multiline body
text. It does not add recommended blank lines automatically.

Structural diagnostics report the first parser error; after it is fixed, the
next one is reported. Common string-decoding warnings are shown per value.
Type suggestions are provisional editor hints: the candidate list is
illustrative, and `int` is specific to the Go sample profile. Suggestions do
not validate type names or perform conversion; consumers define type meaning
and conversion rules.

Use **Quick Fix** (`Ctrl+.` on Windows) on an untyped value to add an annotation.
Type-name completion appears after `(`; press `Ctrl+Space` if it does not open
automatically. The quick fix also accepts a custom whitespace-free type
expression.

## Run from a checkout

Open `implementations/javascript` as an Extension Development Host in VS Code,
then press F5. Open or create a `.dotreg` file in the development host. The
format command is **dotReg: Format Document**; standard Format Document and
format-on-save settings also use the formatter.

The extension uses the VS Code extension API and the repository's built-in
JavaScript modules. No npm install step is required. VS Code compatible editors
that load VS Code extensions can use the same package. Other editors can reuse
`formatter.mjs` and `dotreg.mjs` from their own formatter integration.

If a `.conf` file contains dotReg syntax, associate it with the `dotreg`
language ID in the editor's file associations.
