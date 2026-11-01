# TypeScript sample

The typed entry point is [dotreg.mts](dotreg.mts). It re-exports the
[JavaScript engine](../javascript/README.md) and its explicit type declarations,
so the two languages share grammar, fixes and runtime cost. This is a typed
interface to one implementation, rather than a second copy of the parser.
Requires Node.js 20 or later and TypeScript 5.7 or later for compilation.

Run from the repository root with `tsc` available:

```sh
tsc --strict --noEmit --target ES2022 --module NodeNext implementations/typescript/example.mts
```

Import `parse`, `parseEach`, `decodeString`, `DotRegError` and types `Entry`,
`Text`, `Source`, `ValueForm` from `dotreg.mjs` after compilation. A NodeNext
build resolves that import to the `.mts` source and emits the `.mjs` facade.
Keep the neighbouring `javascript` directory when distributing it; the facade
does not bundle its engine. The JavaScript CLI and conformance checks also test
the runtime used by TypeScript.

The typed facade also exports appendValue, setText and setTextAt, together
with the WriteValue type. These functions share the JavaScript engine's
source-preserving write policy: follow a matching heading section when
appending, preserve unrelated text, and use a full key when no heading matches.
This is sample formatting behavior, not canonical notation.

See [example.mts](example.mts) for checked callback types and repeated values,
and the [parser set](../README.md) for common contracts.
