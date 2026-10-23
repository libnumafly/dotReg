# JavaScript sample

Requires Node.js 20 or later. Uses only built-in modules; no installation step.
Run from the repository root:

```sh
node implementations/javascript/configsample.mjs examples/sample.txt
node implementations/javascript/configsample.mjs --mode raw examples/sample.txt
node --test implementations/javascript/test.mjs
```

Omit the filename or use `-` to read UTF-8 standard input. The CLI collects and
decodes the whole file before writing JSON; errors leave standard output empty.
`strings` is the default mode. Type annotations are retained without conversion.

```js
import { createReadStream } from 'node:fs';
import { parseEach, decodeString } from './dotreg.mjs';

const source = createReadStream('settings.txt');
try {
  await parseEach(source, entry => {
    if (entry.kind === 'value') {
      const text = decodeString(entry);
      // Consume text.value here; text.unset distinguishes explicit empty values.
    }
  });
} finally { source.destroy(); }
```

`parse(source)` returns a promise of syntax entries without partial results on
failure. `parseEach(source, visit)` awaits each callback before parsing another
entry; returning `false` stops successfully and exceptions propagate unchanged.
Both accept a string, a byte array, or an iterable/async iterable of byte arrays.
Streams are borrowed; callers close their own resources, including after a stop.
The library retains a physical line, one lookahead line and the heading stack;
an unfinished multiline value requires its body. Persistent path nodes avoid
copying all ancestors into every stored scope. Each emitted path is independent.
Literal block decoding returns the existing string where possible; escaped
text is assembled from spans. It does not split the whole source into lines.

See the [parser set](../README.md) for the common contract, conformance checks,
measurement method and provisional grammar choices. [TypeScript](../typescript/README.md)
uses this same engine with static types. The public APIs remain provisional.
