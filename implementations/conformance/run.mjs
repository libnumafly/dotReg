// Runs any sample CLI using an argv array, without shell interpolation.
import { readFile } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import assert from 'node:assert/strict';

const args = process.argv.slice(2);
if (args[0] !== '--command' || !args[1]) throw new Error('Usage: node run.mjs --command JSON_ARGV [--filter text]');
const command = JSON.parse(args[1]);
if (!Array.isArray(command) || !command.length || command.some(x => typeof x !== 'string')) throw new Error('Expected argv array');
let filter = '';
if (args.length > 2) {
  if (args.length !== 4 || args[2] !== '--filter') throw new Error('Invalid arguments');
  filter = args[3];
}
const { cases } = JSON.parse(await readFile(new URL('cases.json', import.meta.url), 'utf8'));
function run(input, mode) {
  return new Promise((resolve, reject) => {
    const child = spawn(command[0], [...command.slice(1), '--mode', mode], { stdio: ['pipe','pipe','pipe'], windowsHide: true });
    const out = [], err = [];
    const timer = setTimeout(() => { child.kill(); reject(new Error('CLI timeout (30 s)')); }, 30_000);
    child.on('error', e => { clearTimeout(timer); reject(e); });
    child.stdout.on('data', b => out.push(b)); child.stderr.on('data', b => err.push(b));
    child.stdin.on('error', () => {}); // A rejecting parser may close its input early.
    child.on('close', (code, signal) => { clearTimeout(timer); resolve({ code, signal, out: Buffer.concat(out).toString('utf8'), err: Buffer.concat(err).toString('utf8') }); });
    child.stdin.end(input);
  });
}
let checks = 0, failures = 0;
for (const c of cases.filter(c => c.name.includes(filter))) {
  const input = c.input_hex ? Buffer.from(c.input_hex, 'hex') : Buffer.from(c.input);
  for (const mode of ['raw','strings']) {
    try {
      const r = await run(input, mode);
      const expectedFailure = c.error && (c.error.stage === 'parse' || mode === 'strings');
      if (expectedFailure) {
        assert.notEqual(r.code, 0, 'expected failure'); assert.equal(r.signal, null, 'unexpected termination');
        assert.equal(r.out, '', 'failure produced partial output');
        assert.match(r.err, new RegExp(`\\bline ${c.error.line}\\b`), 'physical error line');
        assert(!r.err.includes('SECRET'), 'diagnostic exposed right-hand side');
      } else {
        assert.equal(r.code, 0, r.err);
        const expected = c.entries.map(e => {
          const { decoded, ...entry } = e;
          return mode === 'strings' && e.kind === 'value' ? {...entry, decoded} : entry;
        });
        assert.deepEqual(JSON.parse(r.out), expected);
      }
      checks++;
    } catch (e) { failures++; console.error(`${c.name} [${mode}]: ${e.message}`); }
  }
}
assert(checks + failures > 0, 'No cases selected');
console.log(`${checks} checks passed, ${failures} failed (${command.join(' ')})`);
if (failures) process.exitCode = 1;
