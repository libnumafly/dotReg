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
      if (value !== '0.1') fail(p.number, 'unsupported');
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

function elements(node, suffix = null) {
  const result = new Array(node.count + (suffix?.length ?? 0));
  if (suffix) {
    for (let i = 0; i < suffix.length; i++) result[node.count + i] = suffix[i];
  }
  let end = node.count;
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
      if (e.kind === 'heading') {
        const suffix = parsePath(key, p.number);
        const node = { parent, suffix, count: (parent?.count ?? 0) + suffix.length };
        e.path = elements(node);
        scopes.push(node);
      } else {
        const suffix = relative && key === '' ? null : parsePath(key, p.number);
        e.path = parent ? elements(parent, suffix) : suffix;
        eof = await lines.valueBody(e, indent, eof);
      }
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

/** Append one value occurrence, following the deepest matching heading when present. */
export async function appendValue(source, path, value) {
  if (typeof source !== 'string') fail(0, 'input');
  const entries = await parse(source);
  return appendParsed(source, entries, path, value);
}

/** Replace a unique key occurrence, or append an untyped value when it is absent. */
export async function setText(source, path, text) {
  if (typeof source !== 'string') fail(0, 'input');
  const entries = await parse(source);
  const matches = matchingValues(entries, path);
  if (matches.length === 0) return appendParsed(source, entries, path, text);
  if (matches.length > 1) fail(matches[1].line, 'multiple_values');
  return replaceParsed(source, matches[0], text);
}

/** Replace one zero-based occurrence of a repeated key. */
export async function setTextAt(source, path, occurrence, text) {
  if (typeof source !== 'string') fail(0, 'input');
  const entries = await parse(source);
  if (!Array.isArray(path) || path.length === 0 || !Number.isInteger(occurrence) || occurrence < 0)
    fail(0, 'input');
  const matches = matchingValues(entries, path);
  if (occurrence >= matches.length) fail(0, 'missing');
  return replaceParsed(source, matches[occurrence], text);
}

async function appendParsed(source, entries, path, value) {
  if (typeof source !== 'string' || !Array.isArray(path) || path.length === 0 ||
      !value || typeof value.value !== 'string' || typeof value.unset !== 'boolean' ||
      (value.unset && value.value !== '') ||
      (value.type !== undefined && (typeof value.type !== 'string' || value.type === ''))) fail(0, 'input');
  const key = encodeKeyPath(path);
  let line = encodeWriteLine(key, value);
  let insertAt = source.length;
  const heading = deepestHeadingAncestor(entries, path);
  let indent = 0;
  if (heading) {
    const lines = writerLines(source);
    const headingLine = lines[heading.line - 1];
    if (!headingLine) fail(heading.line, 'syntax');
    indent = leadingSpaces(headingLine.text) + 2;
    const remainder = line.slice(key.length);
    const suffix = path.length === heading.path.length ? '' : encodeKeyPath(path.slice(heading.path.length));
    line = `.${suffix}${remainder}`;
    insertAt = headingSectionEnd(entries, lines, heading);
  }
  if (indent) line = ' '.repeat(indent) + line;
  const updated = insertSourceLine(source, insertAt, line, lineEndingAt(source, insertAt));
  await parse(updated);
  return updated;
}

async function replaceParsed(source, entry, text) {
  if (typeof source !== 'string' || !text || typeof text.value !== 'string' ||
      typeof text.unset !== 'boolean' || (text.unset && text.value !== '')) fail(0, 'input');
  const lines = writerLines(source);
  const firstIndex = entry.line - 1;
  const first = lines[firstIndex];
  if (!first) fail(entry.line, 'syntax');
  const lastIndex = entryEndLine(lines, entry);
  const firstLine = first.text;
  const separator = valueSeparator(firstLine);
  if (separator < 0) fail(entry.line, 'syntax');
  const rhs = text.unset ? '' : ` ${encodeQuotedValue(text.value)}`;
  const last = lines[lastIndex];
  const replacement = source.slice(first.start, first.start + separator + 1) + rhs +
    source.slice(last.contentEnd, last.end);
  const updated = source.slice(0, first.start) + replacement + source.slice(last.end);
  await parse(updated);
  return updated;
}

function matchingValues(entries, path) {
  if (!Array.isArray(path) || path.length === 0 || path.some(element => typeof element !== 'string')) fail(0, 'input');
  return entries.filter(entry => entry.kind === 'value' && samePath(entry.path, path));
}

function samePath(left, right) {
  return left.length === right.length && left.every((element, index) => element === right[index]);
}

function deepestHeadingAncestor(entries, path) {
  let best = null;
  for (const entry of entries) {
    if (entry.kind !== 'heading' || entry.path.length > path.length ||
        !entry.path.every((element, index) => element === path[index])) continue;
    if (!best || entry.path.length >= best.path.length) best = entry;
  }
  return best;
}

function encodeWriteLine(key, value) {
  const annotation = value.type === undefined ? '' : ` (${value.type})`;
  if (value.unset) return `${key}${annotation}:`;
  return `${key}${annotation}: ${encodeQuotedValue(value.value)}`;
}

function encodeKeyPath(path) {
  if (!Array.isArray(path) || path.length === 0 || path.some(element => typeof element !== 'string')) fail(0, 'input');
  return path.map(encodeKeyElement).join('.');
}

function encodeKeyElement(element) {
  if (!validText(element)) fail(0, 'input');
  if (element !== '' && !/[.\p{White_Space}\p{Cc}:()#"\\]/u.test(element)) return element;
  return encodeQuotedValue(element);
}

function encodeQuotedValue(value) {
  if (!validText(value)) fail(0, 'input');
  let result = '"';
  for (const character of value) {
    const scalar = character.codePointAt(0);
    switch (character) {
      case '"': result += '\\"'; break;
      case '\\': result += '\\\\'; break;
      case '\b': result += '\\b'; break;
      case '\f': result += '\\f'; break;
      case '\n': result += '\\n'; break;
      case '\r': result += '\\r'; break;
      case '\t': result += '\\t'; break;
      default:
        result += /\p{Cc}/u.test(character) ? `\\u{${scalar.toString(16)}}` : character;
    }
  }
  return result + '"';
}

function writerLines(source) {
  const result = [];
  for (let start = 0; start < source.length;) {
    const newline = source.indexOf('\n', start);
    const end = newline < 0 ? source.length : newline + 1;
    let contentEnd = newline < 0 ? end : newline;
    if (contentEnd > start && source[contentEnd - 1] === '\r') contentEnd--;
    result.push({ text: source.slice(start, contentEnd), start, contentEnd, end });
    start = end;
  }
  return result;
}

function entryEndLine(lines, entry) {
  const start = entry.line - 1;
  if (!lines[start]) fail(entry.line, 'syntax');
  if (!entry.form) return start;
  if (entry.form === 'triple_quoted') {
    for (let i = start + 1; i < lines.length; i++) {
      const indent = leadingSpaces(lines[i].text);
      if (lines[i].text.slice(indent).replace(/[ \t]+$/g, '') === '\"\"\"') return i;
    }
    fail(entry.line, 'syntax');
  }
  if (entry.form === 'pipe') {
    const parentIndent = leadingSpaces(lines[start].text);
    let end = start;
    for (let i = start + 1; i < lines.length; i++) {
      const indent = leadingSpaces(lines[i].text);
      if (indent <= parentIndent || indent === lines[i].text.length || lines[i].text[indent] !== '|') break;
      end = i;
    }
    if (end === start) fail(entry.line, 'syntax');
    return end;
  }
  fail(entry.line, 'syntax');
}

function headingSectionEnd(entries, lines, heading) {
  const headingIndex = heading.line - 1;
  const indent = leadingSpaces(lines[headingIndex].text);
  let insertAt = lines[headingIndex].end;
  let found = false;
  for (const entry of entries) {
    if (!found) {
      if (entry.kind === 'heading' && entry.line === heading.line) found = true;
      continue;
    }
    const index = entry.line - 1;
    if (!lines[index] || leadingSpaces(lines[index].text) <= indent) break;
    const lastIndex = entryEndLine(lines, entry);
    insertAt = lines[lastIndex].end;
  }
  return insertAt;
}

function valueSeparator(line) {
  let quotedKey = false, depth = 0;
  for (let i = 0; i < line.length; i++) {
    const character = line[i];
    if (character === '\\' && quotedKey) { i++; continue; }
    if (character === '"') { quotedKey = !quotedKey; continue; }
    if (quotedKey) continue;
    if (character === '(') depth++;
    else if (character === ')' && depth > 0) depth--;
    else if (character === ':' && depth === 0) return i;
  }
  return -1;
}

function leadingSpaces(line) {
  let i = 0;
  while (line[i] === ' ') i++;
  return i;
}

function lineEndingAt(source, offset) {
  for (let i = Math.max(0, offset); i < source.length; i++) {
    if (source[i] === '\n') return i > 0 && source[i - 1] === '\r' ? '\r\n' : '\n';
  }
  for (let i = Math.min(source.length, offset) - 1; i >= 0; i--) {
    if (source[i] === '\n') return i > 0 && source[i - 1] === '\r' ? '\r\n' : '\n';
  }
  return '\n';
}

function insertSourceLine(source, offset, line, ending) {
  const hasPreviousBreak = offset === 0 || source[offset - 1] === '\n';
  const following = offset < source.length;
  const endsWithBreak = source.endsWith('\n');
  const needsFollowingBreak = following || (offset === source.length && endsWithBreak);
  return source.slice(0, offset) + (hasPreviousBreak ? '' : ending) + line +
    (needsFollowingBreak ? ending : '') + source.slice(offset);
}
