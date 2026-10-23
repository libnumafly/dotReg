// dotReg syntax and common string decoding. No type conversion or dependencies.
import { TextDecoder } from 'node:util';

export class DotRegError extends Error {
  constructor(line, code = 'syntax') {
    super(`${line > 0 ? `line ${line}: ` : ''}configuration error (${code})`);
    this.name = 'DotRegError';
    this.line = line;
    this.code = code;
  }
}
const fail = (line, code) => { throw new DotRegError(line, code); };
const outerSpace = c => c === ' ' || c === '\t';
const trim = s => {
  let start = 0, end = s.length;
  while (start < end && outerSpace(s[start])) start++;
  while (end > start && outerSpace(s[end - 1])) end--;
  return start === 0 && end === s.length ? s : s.slice(start, end);
};
const right = s => {
  let end = s.length;
  while (end > 0 && outerSpace(s[end - 1])) end--;
  return end === s.length ? s : s.slice(0, end);
};
const spaces = s => { let i = 0; while (s[i] === ' ') i++; return i; };
const whitespace = /\p{White_Space}/u;
const forbidden = /[\p{White_Space}\p{Cc}:()#"\\]/u;
const basicEscapes = { '.': '.', '"': '"', '\\': '\\', '/': '/', b: '\b', f: '\f', n: '\n', r: '\r', t: '\t' };

function validText(s) {
  for (let i = 0; i < s.length; i++) {
    const n = s.charCodeAt(i);
    if (n >= 0xd800 && n <= 0xdbff) {
      const low = s.charCodeAt(++i);
      if (!(low >= 0xdc00 && low <= 0xdfff)) return false;
    } else if (n >= 0xdc00 && n <= 0xdfff) return false;
  }
  return true;
}

function escapeAt(s, i, line, code) {
  const c = s[i + 1];
  if (Object.hasOwn(basicEscapes, c)) return [basicEscapes[c], i + 2];
  if (c !== 'u') fail(line, code);
  let end, n;
  if (s[i + 2] === '{') {
    end = s.indexOf('}', i + 3);
    const digits = end < 0 ? '' : s.slice(i + 3, end);
    if (!/^[0-9a-fA-F]{1,6}$/.test(digits)) fail(line, code);
    n = Number.parseInt(digits, 16);
    end++;
  } else {
    const digits = s.slice(i + 2, i + 6);
    if (!/^[0-9a-fA-F]{4}$/.test(digits)) fail(line, code);
    n = Number.parseInt(digits, 16);
    end = i + 6;
    if (n >= 0xd800 && n <= 0xdbff) {
      const lowDigits = s.slice(end + 2, end + 6);
      if (s.slice(end, end + 2) !== '\\u' || !/^[0-9a-fA-F]{4}$/.test(lowDigits)) fail(line, code);
      const low = Number.parseInt(lowDigits, 16);
      if (low < 0xdc00 || low > 0xdfff) fail(line, code);
      n = 0x10000 + (n - 0xd800) * 0x400 + low - 0xdc00;
      end += 6;
    }
  }
  if (n > 0x10ffff || (n >= 0xd800 && n <= 0xdfff)) fail(line, code);
  return [String.fromCodePoint(n), end];
}

// The same scanner can locate a quote without constructing discarded text.
function quoted(s, start, line, code = 'syntax', decode = true) {
  const parts = decode ? [] : null;
  let literal = start + 1;
  for (let i = literal; i < s.length; i++) {
    if (s[i] === '"') {
      if (decode) parts.push(s.slice(literal, i));
      return [decode ? parts.join('') : '', i + 1];
    }
    if (s[i] === '\r' || s[i] === '\n') fail(line, code);
    if (s[i] === '\\') {
      const [value, end] = escapeAt(s, i, line, code);
      if (decode) parts.push(s.slice(literal, i), value);
      literal = end;
      i = end - 1;
    }
  }
  fail(line, code);
}

function parsePath(key, line) {
  const path = [];
  let i = 0;
  while (true) {
    if (i === key.length) fail(line);
    if (key[i] === '"') {
      const [element, end] = quoted(key, i, line);
      path.push(element);
      i = end;
      if (i < key.length && key[i] !== '.') fail(line);
    } else {
      let end = key.indexOf('.', i);
      if (end < 0) end = key.length;
      const element = key.slice(i, end);
      if (!element || forbidden.test(element)) fail(line);
      path.push(element);
      i = end;
    }
    if (i === key.length) return path;
    i++;
  }
}

function splitLine(body, line) {
  const e = { line, kind: 'heading', path: [], raw: '', unset: false, has_type: false };
  for (let i = 0; i < body.length; i++) {
    if (body[i] === '"') { i = quoted(body, i, line, 'syntax', false)[1] - 1; continue; }
    if (body[i] === ':') {
      e.kind = 'value'; e.raw = trim(body.slice(i + 1)); e.unset = e.raw === '';
      return [e, trim(body.slice(0, i))];
    }
    if (body[i] !== '(') continue;
    const key = body.slice(0, i).replace(/ +$/g, '');
    if (key.endsWith('\t')) fail(line, 'unsupported');
    let depth = 1, j = i + 1;
    for (; j < body.length; j++) {
      const c = body[j];
      if (whitespace.test(c)) fail(line);
      if (c === '"' || c === '\\') fail(line, 'unsupported');
      if (c === '(') depth++;
      if (c === ')' && --depth === 0) break;
    }
    if (depth !== 0) fail(line);
    const type = body.slice(i + 1, j);
    if (!type) fail(line, 'unsupported');
    const tail = body.slice(j + 1).replace(/^ +/g, '');
    if (tail.startsWith('\t')) fail(line, 'unsupported');
    if (!tail.startsWith(':')) fail(line);
    e.kind = 'value'; e.type = type; e.has_type = true;
    e.raw = trim(tail.slice(1)); e.unset = e.raw === '';
    return [e, key];
  }
  if (!body.endsWith('.')) fail(line);
  return [e, body.slice(0, -1)];
}

// Byte chunks are consumed on demand. Only a physical line and one lookahead
// are retained, apart from an unfinished block and the heading stack.
class Lines {
  constructor(source) {
    if (typeof source === 'string') {
      if (!validText(source)) fail(0, 'input');
      source = [Buffer.from(source)];
    } else if (source instanceof Uint8Array) source = [source];
    const iterator = source?.[Symbol.asyncIterator]?.() ?? source?.[Symbol.iterator]?.();
    if (!iterator) fail(0, 'input');
    this.iterator = iterator;
    this.chunk = Buffer.alloc(0); this.offset = 0; this.done = false;
    this.number = 0; this.pending = null;
    this.decoder = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true });
  }
  async next() {
    if (this.pending) { const p = this.pending; this.pending = null; return p; }
    const number = ++this.number;
    const parts = [];
    let size = 0, ending = '', readError = false;
    while (true) {
      if (this.offset < this.chunk.length) {
        const end = this.chunk.indexOf(10, this.offset);
        const part = this.chunk.subarray(this.offset, end < 0 ? this.chunk.length : end);
        // A producer may reuse its chunk on next(). Own unfinished tails before
        // advancing it; complete lines can be decoded directly from the chunk.
        parts.push(end < 0 ? Buffer.from(part) : part); size += part.length;
        this.offset = end < 0 ? this.chunk.length : end + 1;
        if (end >= 0) { ending = '\n'; break; }
      }
      if (this.done) break;
      try {
        const item = await this.iterator.next();
        this.done = Boolean(item.done);
        if (!this.done) {
          if (!(item.value instanceof Uint8Array)) throw new Error();
          this.chunk = Buffer.from(item.value.buffer, item.value.byteOffset, item.value.byteLength);
          this.offset = 0;
        } else { this.chunk = Buffer.alloc(0); this.offset = 0; }
      } catch { this.done = true; readError = true; break; }
    }
    let bytes = parts.length === 1 ? parts[0] : Buffer.concat(parts, size);
    if (ending && bytes.at(-1) === 13) { bytes = bytes.subarray(0, -1); ending = '\r\n'; }
    // Pipe qualification precedes UTF-8 validation: an invalid body line still
    // belongs to the block and must fail before the value is delivered.
    let rawIndent = 0;
    while (bytes[rawIndent] === 32) rawIndent++;
    const rawPipe = bytes[rawIndent] === 124;
    let text = '', invalid = false;
    try { text = this.decoder.decode(bytes); } catch { invalid = true; }
    if (number === 1 && text.startsWith('\ufeff')) text = text.slice(1);
    return { text, ending, number, eof: this.done && !ending, readError, invalid, rawIndent, rawPipe };
  }
  validate(p) {
    if (p.readError) fail(p.number, 'input');
    if (p.invalid || p.text.includes('\r')) fail(p.number);
  }
  async properties() {
    let declared = false;
    while (true) {
      const p = await this.next(); this.validate(p);
      const text = right(p.text);
      if (!text) {
        if (!declared || (!p.text && !p.ending)) fail(p.number);
        return p.eof;
      }
      const separator = text.search(/[ \t]/);
      if (separator <= 0 || text[0] !== '@') fail(p.number);
      const name = text.slice(0, separator), value = text.slice(separator).replace(/^[ \t]+/g, '');
      if (!value) fail(p.number);
      if (name !== '@dotReg') fail(p.number, 'unsupported');
      if (declared) fail(p.number, 'duplicate');
      if (value !== '1') fail(p.number, 'unsupported');
      declared = true;
      if (p.eof) fail(p.number + 1);
    }
  }
  async valueBody(e, indent, eof) {
    if (e.raw === '"""') {
      const rows = [];
      while (true) {
        const p = await this.next(); this.validate(p);
        const margin = spaces(p.text);
        if (right(p.text.slice(margin)) === '"""') {
          const parts = [];
          for (let i = 0; i < rows.length; i++) {
            const row = rows[i];
            if (row.text && spaces(row.text) < margin) fail(row.number);
            if (i) parts.push(rows[i - 1].ending);
            parts.push(row.text ? row.text.slice(margin) : '');
          }
          e.raw = parts.join(''); e.form = 'triple_quoted'; e.unset = false;
          return p.eof;
        }
        if (p.eof) fail(e.line);
        rows.push(p);
      }
    }
    if (!e.unset || eof) return eof;
    const parts = [];
    let previousEnding = '', found = false;
    while (true) {
      const p = await this.next(), prefix = p.rawIndent;
      if (prefix <= indent || !p.rawPipe) {
        this.pending = p;
        if (found) { e.raw = parts.join(''); e.form = 'pipe'; e.unset = false; }
        return false;
      }
      this.validate(p);
      if (found) parts.push(previousEnding);
      parts.push(p.text.slice(prefix + 1));
      found = true; previousEnding = p.ending;
      if (p.eof) { e.raw = parts.join(''); e.form = 'pipe'; e.unset = false; return true; }
    }
  }
}

function elements(node) {
  const result = new Array(node.count);
  let end = result.length;
  for (let n = node; n; n = n.parent) {
    end -= n.suffix.length;
    for (let i = 0; i < n.suffix.length; i++) result[end + i] = n.suffix[i];
  }
  return result;
}

/** Visit complete entries in order. Await each visitor; false stops normally.
 * A thrown/rejected visitor error is propagated unchanged. Source is borrowed.
 */
export async function parseEach(source, visit) {
  if (typeof visit !== 'function') fail(0, 'input');
  const lines = new Lines(source);
  if (await lines.properties()) return;
  const scopes = [];
  while (true) {
    const p = await lines.next(); lines.validate(p);
    let eof = p.eof;
    const trimmed = trim(p.text);
    if (trimmed && !trimmed.startsWith('#')) {
      const indent = spaces(p.text), body = right(p.text.slice(indent));
      if (body.startsWith('\t') || indent % 2 !== 0) fail(p.number);
      const relative = body.startsWith('.');
      if (relative !== (indent > 0)) fail(p.number);
      let [e, key] = splitLine(body, p.number);
      let parent = null;
      if (relative) {
        const depth = indent / 2;
        if (depth > scopes.length || !key.startsWith('.')) fail(p.number);
        scopes.length = depth; parent = scopes[depth - 1]; key = key.slice(1);
      } else scopes.length = 0;
      let node = parent;
      if (!(relative && key === '' && e.kind === 'value')) {
        const suffix = parsePath(key, p.number);
        node = { parent, suffix, count: (parent?.count ?? 0) + suffix.length };
      }
      e.path = elements(node);
      if (e.kind === 'heading') scopes.push(node);
      else eof = await lines.valueBody(e, indent, eof);
      if (await visit(e) === false) return;
    }
    if (eof) return;
  }
}

/** Collect syntax results. A rejected promise never exposes a partial list. */
export async function parse(source) {
  const entries = [];
  await parseEach(source, e => { entries.push(e); });
  return entries;
}

/** Decode common strings; annotations stay opaque and values are never inferred. */
export function decodeString(entry) {
  const { raw, unset, line, form = '' } = entry;
  if (entry.kind !== 'value' || typeof raw !== 'string' || !validText(raw)) fail(line, 'string');
  if (form) {
    if (unset || /\r(?!\n)/.test(raw)) fail(line, 'string');
    if (form === 'pipe') return { value: raw, unset: false };
    if (form !== 'triple_quoted') fail(line, 'string');
    if (!raw.includes('\\')) return { value: raw, unset: false };
    const parts = [];
    let literal = 0, i = raw.indexOf('\\');
    while (i >= 0) {
      const [value, end] = escapeAt(raw, i, line, 'string');
      parts.push(raw.slice(literal, i), value); literal = end;
      i = raw.indexOf('\\', end);
    }
    parts.push(raw.slice(literal));
    return { value: parts.join(''), unset: false };
  }
  const text = trim(raw);
  if (/[\r\n]/.test(text) || unset !== (text === '')) fail(line, 'string');
  if (!text.startsWith('"')) return { value: text, unset };
  const [value, end] = quoted(text, 0, line, 'string');
  if (end !== text.length) fail(line, 'string');
  return { value, unset: false };
}
