import { open } from 'node:fs/promises';
import { parse, parseEach, decodeString } from './dotreg.mjs';
const [filename, repetitions = '10', mode = 'stream'] = process.argv.slice(2);
const iterations = Number(repetitions);
if (!filename || !Number.isSafeInteger(iterations) || iterations < 1 || !['stream','collect'].includes(mode)) throw new Error('Expected fixture iterations stream|collect');
const file = await open(filename, 'r');
try {
  const input_bytes = (await file.stat()).size;
  // Positional reads borrow the same descriptor without stream auto-close.
  const once = async () => {
    let entries = 0, decoded_units = 0;
    async function* chunks() {
      let position = 0;
      while (true) {
        const buffer = Buffer.allocUnsafe(64 * 1024);
        const {bytesRead} = await file.read(buffer, 0, buffer.length, position);
        if (!bytesRead) return;
        position += bytesRead;
        yield buffer.subarray(0, bytesRead);
      }
    }
    const source = chunks();
    const visit = e => { if (e.kind === 'value') decoded_units += decodeString(e).value.length; entries++; };
    try {
      if (mode === 'stream') await parseEach(source, visit);
      else for (const e of await parse(source)) visit(e);
    } finally { await source.return(); }
    return {entries, decoded_units};
  };
  await once(); await once();
  const start = process.hrtime.bigint();
  let result;
  for (let i = 0; i < iterations; i++) {
    const next = await once();
    if (result && (next.entries !== result.entries || next.decoded_units !== result.decoded_units)) throw new Error('inconsistent pass');
    result = next;
  }
  const elapsed_seconds = Number(process.hrtime.bigint() - start) / 1e9;
  console.log(JSON.stringify({language:'javascript',mode,iterations,input_bytes,...result,elapsed_seconds}));
} finally { await file.close(); }
