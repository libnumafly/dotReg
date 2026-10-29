# Integrating dotReg into an application

Use the target application's existing dotReg library where available. The
upstream [sample set](https://github.com/libnumafly/dotReg/tree/4aca4adba1cdc4c523d3e07b91a64403de4f8f68/implementations)
contains Go, Python, JavaScript, TypeScript, Java, C#, Rust and C++ implementations
with library APIs and CLIs. TypeScript shares the JavaScript engine. Read the
selected language's guide and source before depending on its API or build steps.
Do not invent a registry package, release version or cross-language API parity.

## Loader and application responsibilities

Keep these stages identifiable, even when one public function combines them:

1. Parse syntax into ordered entries with decoded key-element sequences,
   physical source line, heading/value kind, raw text, form, annotation and unset
   state. Headings do not create values. Collecting parsing fails without partial
   results; the returned raw text is not necessarily decodable as a string.
2. Decode common strings according to the entry's form. Preserve `raw` and
   `form` if keeping syntax results; guessing the form from body contents loses
   information. No second trimming or repeated escape decoding follows this step.
3. Apply the application's chosen profile. Convert annotated occurrences only
   to their declared expression. Keep unannotated occurrences as strings until
   retrieval requests conversion. A request for a different declared expression
   is a type mismatch, even if reconversion seems possible or the value is unset.
   Failure to convert an unannotated value on retrieval is also a type mismatch.
4. Validate the application schema: known keys/headings, required fields,
   allowed unset/empty states, defaults, occurrence counts, supported types,
   ranges, lengths, control characters and relationships between settings.

Use explicit supported names and conversion rules for the application's type
profile. `string`, `int` and `bool` appear in examples and Go's demo profile;
there is no complete standard type catalogue. Quoting a value does not declare
its type. Failed declared conversions cannot fall back to strings or a second
type. Unknown type names and non-string unset/default policy need application
decisions; do not silently guess them.

Keep repeated values ordered. A plain `Map<path,value>` or nested object can
lose repetitions, empty elements or a prefix's own values. Use an ordered
occurrence sequence, with an index into it if useful. In JavaScript, use a
boundary-preserving identity such as `JSON.stringify(entry.path)` for a secondary
index; `entry.path.join('.')` conflates `["A.B"]` with `["A", "B"]`.

The notation supplies no automatic includes, environment-variable expansion,
path expansion, cross-file merging, JSON array/object parsing or implicit
concatenation. Implement such application features only when requested, with
their own explicit rules.

## JavaScript and TypeScript

The bundled [JavaScript engine](../scripts/lib/dotreg.mjs) is a verbatim upstream
snapshot. It uses Node.js built-ins and requires Node.js 20+. It can also support
an application prototype; choose a maintained dependency arrangement for the
actual project and preserve [the MIT licence](../LICENSE) when copying it.

For an application-local copy at `./dotreg.mjs`:

```js
import { readFile } from 'node:fs/promises';
import { parse, decodeString } from './dotreg.mjs';

export async function loadOccurrences(filename) {
  const entries = await parse(await readFile(filename));
  return entries
    .filter(entry => entry.kind === 'value')
    .map(entry => ({ ...entry, text: decodeString(entry).value }));
}
```

Read bytes rather than a lossy pre-decoded string so the parser can reject
malformed UTF-8. This function collects syntax and decoded occurrences without
partial results. It deliberately leaves type conversion and schema validation
to the caller; `text` here is an application field, not an upstream API field.
Avoid committing externally visible application changes until the complete
configuration has passed the application's checks.

The sample API is provisional:

| API | Behaviour |
|---|---|
| `await parse(source)` | Ordered syntax entries, no partial result on failure |
| `await parseEach(source, visit)` | Awaits each recipient; `false` stops successfully |
| `decodeString(entry)` | Returns `{ value, unset }`; can fail on invalid quoted text |

Sources may be strings, byte arrays, or sync/async iterables of byte arrays.
Visitors receive headings and complete values. Their original exceptions
propagate, and already performed side effects are not rolled back. A block can
require one physical line of lookahead; byte buffering may read further ahead.
The caller owns and closes borrowed streams, including after an early stop.

For TypeScript, use the upstream typed facade with its neighbouring JavaScript
directory and declarations, or bring the matching `dotreg.d.mts` beside a copied
engine. The skill's checker does not bundle the typed facade or its declarations.

## Go consumer example

The upstream module declares
`github.com/libnumafly/dotReg/implementations/go` and Go 1.22. Check dependency
resolution for the chosen revision in the target project; this guide does not
assert that a version tag exists or that a package has been published.

The Go sample adds `Build(entries, profile)` and a provisional consumer API to
its `Parse`, `ParseEach` and `DecodeString` functions:

| Operation | Sample behaviour |
|---|---|
| `Get(path, requestedType)` | Exactly one occurrence; missing and multiple values are errors |
| `GetAll(path, requestedType)` | All occurrences of that exact path in source order; conversion failure yields no partial values |
| `GetAt(path, index, requestedType)` | Zero-based occurrence within that exact key |
| `IsUnset` / `IsUnsetAll` | Query states without converting values |
| `SelectPrefix(prefix)` | Prefix's own values and all descendant occurrences in global source order |

`SelectPrefix` follows the shared selection rules: exact decoded elements,
inclusive of the base path, without sorting, deduplication or new conversion.
Return each full path, relative path, value, annotation, unset state and source
line. The base path has relative `[]`; an empty-named child has `[""]`. An empty
prefix selects all values, while `[""]` selects that named key and descendants.
`*` is literal. No matches succeed with an empty result. Other sample languages
do not provide the Go consumer just because they share its parser contract.

See the pinned [Go guide](https://github.com/libnumafly/dotReg/blob/4aca4adba1cdc4c523d3e07b91a64403de4f8f68/implementations/go/README.md)
for ownership, errors and the illustrative `DemoProfile`. Its public API names
and profile are sample choices.

## Verification

Use [the checker](../scripts/validate.mjs) to validate syntax and strings offline.
Default success means all input syntax and common string decoding passed.
It does not mean annotated types, application schemas or a configuration's
runtime effect were validated. The checker collects a whole file, so use an
existing visitor API when bounded retention is needed.

For loader changes, exercise the meaningful application contracts: repeated
identical values in order; dotted/empty key names; own values plus descendants;
missing versus unset versus explicit empty; accepted and rejected conversions;
and safe failure without partially installing configuration. Include multiline
LF/CRLF and literal Windows paths when relevant. Ordinary file edits can use
the checker and the application's existing validation command.

Upstream's shared conformance runner can exercise the checker through `--json`.
From a dotReg checkout root with this folder in its repository location:

```sh
node implementations/conformance/run.mjs --command '["node",".agents/skills/dotreg/scripts/validate.mjs","--json"]'
```

The runner appends `--mode raw` or `--mode strings` and supplies standard input.
Those checks cover the corpus's syntax/decoding contract, not universal type or
application-schema conformance. Source revisions and hashes are recorded in
[upstream.json](upstream.json). When refreshing this skill, copy the parser,
example and licence from the same revision, update that manifest and recheck
the guidance against the matching specification and sample guides.
