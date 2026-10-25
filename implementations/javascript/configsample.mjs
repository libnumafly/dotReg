import { createReadStream } from 'node:fs';
import { parse, decodeString } from './dotreg.mjs';

let mode = 'strings', filename = '-';
try {
  const args = process.argv.slice(2);
  if (args[0] === '--mode') { args.shift(); mode = args.shift(); }
  if (args.length > 1 || !['raw', 'strings'].includes(mode)) throw new Error();
  if (args.length) filename = args[0];
  const source = filename === '-' ? process.stdin : createReadStream(filename);
  try {
    const entries = await parse(source);
    if (mode === 'strings') for (const e of entries) {
      if (e.kind === 'value') e.decoded = decodeString(e).value;
    }
    process.stdout.write(JSON.stringify(entries) + '\n');
  } finally { if (filename !== '-') source.destroy(); }
} catch (error) {
  process.stderr.write(error?.name === 'DotRegError' ? `${error.message}\n` : 'input or command error\n');
  process.exitCode = 1;
}
