import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parse, parseEach, decodeString, DotRegError } from './dotreg.mjs';

const file = body => '@dotReg 1\n\n' + body;
test('fresh owned paths, repeats, Unicode escapes and no normalisation', async () => {
  const seen = [];
  await parseEach(file('A.\n  .B:x\n  .B:y\n'), e => {
    seen.push([...e.path]); e.path[0] = 'changed';
  });
  assert.deepEqual(seen, [['A'], ['A', 'B'], ['A', 'B']]);
  const entries = await parse(file('"\\u{1F600}":001\n"":\n'));
  assert.equal(entries[0].path[0], '😀');
  assert.deepEqual(decodeString(entries[1]), { value: '', unset: true });
  assert.deepEqual(await parse(file('')), []);
});
test('callback is awaited, stops before next syntax and preserves error identity', async () => {
  const sentinel = new Error('recipient'); let calls = 0;
  await assert.rejects(parseEach(file('A:\ninvalid\n'), async () => {
    calls++; await Promise.resolve(); throw sentinel;
  }), e => e === sentinel);
  assert.equal(calls, 1);
  await parseEach(file('A:\ninvalid\n'), () => false);
  await assert.rejects(parse(file('A:x\ninvalid\n')), DotRegError);
});
test('byte chunk boundaries and CRLF are preserved', async () => {
  const bytes = Buffer.from('\ufeff@dotReg 1\r\n\r\n日本語:\r\n |一\r\n |二\r\nA:"\\uD83D\\uDE00"');
  async function* chunks() { for (const b of bytes) yield Uint8Array.of(b); }
  const e = await parse(chunks());
  assert.equal(e[0].raw, '一\r\n二');
  assert.equal(e[0].form, 'pipe');
  assert.equal(decodeString(e[1]).value, '😀');
});
test('input failures and malformed UTF-8 stay private', async () => {
  async function* broken() { yield Buffer.from(file('A:x\n')); throw new Error('SECRET'); }
  let calls = 0;
  await assert.rejects(parseEach(broken(), () => { calls++; }), e => e.code === 'input' && !e.message.includes('SECRET'));
  assert.equal(calls, 1);
  await assert.rejects(parse(Buffer.from([0xc0, 0x80])), DotRegError);
  await assert.rejects(parse(file('A:\ud800')), DotRegError);
  await assert.rejects(parseEach(null, null), DotRegError);
});
test('invalid UTF-8 in a qualifying pipe body fails before its visitor', async () => {
  let calls = 0;
  const input = Buffer.concat([Buffer.from(file('A:\n |')), Buffer.from([0xff])]);
  await assert.rejects(parseEach(input, () => { calls++; return false; }), e => e.code === 'syntax' && e.line === 4);
  assert.equal(calls, 0);
});
test('a producer may reuse its byte buffer between yielded chunks', async () => {
  const buffer = Buffer.alloc(5);
  const bytes = Buffer.from(file('A:' + 'x'.repeat(27)));
  async function* reused() {
    for (let i = 0; i < bytes.length; i += buffer.length) {
      const count = Math.min(buffer.length, bytes.length - i);
      bytes.copy(buffer, 0, i, i + count);
      yield buffer.subarray(0, count);
    }
  }
  assert.equal((await parse(reused()))[0].raw, 'x'.repeat(27));
});
test('decoding is separate, form driven and validates constructed entries', async () => {
  const e = await parse(file('A:"\\q"\nA:say "hello\\q\nA:"""\n \\n\n """\n'));
  assert.throws(() => decodeString(e[0]), e => e.code === 'string');
  assert.equal(decodeString(e[1]).value, 'say "hello\\q');
  assert.equal(decodeString(e[2]).value, '\n');
  assert.throws(() => decodeString({ ...e[2], form: 'other' }), DotRegError);
  assert.throws(() => decodeString({ ...e[1], unset: true }), DotRegError);
});
