import { createReadStream } from 'node:fs';
import { decodeString, DotRegError, parse } from './lib/dotreg.mjs';

const usage = 'Usage: node validate.mjs [--mode strings|raw] [--json] [file|-]';

function options(args) {
  let mode = 'strings', json = false, filename = '-', hasFilename = false;
  let hasMode = false;
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === '--mode') {
      if (hasMode || !['raw', 'strings'].includes(args[i + 1])) throw new Error();
      mode = args[++i];
      hasMode = true;
    } else if (arg === '--json') {
      if (json) throw new Error();
      json = true;
    } else if ((!arg.startsWith('-') || arg === '-') && !hasFilename) {
      filename = arg;
      hasFilename = true;
    } else {
      throw new Error();
    }
  }
  return { mode, json, filename };
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length === 1 && ['--help', '-h'].includes(args[0])) {
    process.stdout.write(`${usage}\nDefault: validate syntax and strings; print counts only.\n`
      + '--mode raw checks syntax only. --json prints entry contents.\n'
      + 'Neither mode checks type conversions or application schemas.\n');
    return;
  }

  let config;
  try {
    config = options(args);
  } catch {
    process.stderr.write(`${usage}\n`);
    process.exitCode = 1;
    return;
  }

  const source = config.filename === '-' ? process.stdin : createReadStream(config.filename);
  try {
    const entries = await parse(source);
    if (config.mode === 'strings') {
      for (const entry of entries) {
        if (entry.kind === 'value') entry.decoded = decodeString(entry).value;
      }
    }
    if (config.json) {
      process.stdout.write(`${JSON.stringify(entries)}\n`);
    } else {
      const values = entries.filter(entry => entry.kind === 'value').length;
      const scope = config.mode === 'raw' ? 'syntax only' : 'syntax and strings';
      process.stdout.write(`OK: ${entries.length - values} headings, ${values} values (${scope}; no type or schema validation)\n`);
    }
  } finally {
    if (source !== process.stdin) source.destroy();
  }
}

try {
  await main();
} catch (error) {
  process.stderr.write(error instanceof DotRegError ? `${error.message}\n` : 'input or validation error\n');
  process.exitCode = 1;
}
