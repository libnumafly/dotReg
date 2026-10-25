import { decodeString, parse } from './dotreg.mjs';

const isOuterSpace = c => c === ' ' || c === '\t';

function trimOuter(text) {
  let start = 0;
  let end = text.length;
  while (start < end && isOuterSpace(text[start])) start++;
  while (end > start && isOuterSpace(text[end - 1])) end--;
  return text.slice(start, end);
}

function splitLines(text) {
  const lines = [];
  let start = 0;
  while (start < text.length) {
    const newline = text.indexOf('\n', start);
    if (newline < 0) {
      lines.push({ text: text.slice(start), ending: '' });
      break;
    }
    const crlf = newline > start && text[newline - 1] === '\r';
    lines.push({
      text: text.slice(start, crlf ? newline - 1 : newline),
      ending: crlf ? '\r\n' : '\n',
    });
    start = newline + 1;
  }
  return lines;
}

function lineIndent(text) {
  let i = 0;
  while (text[i] === ' ') i++;
  return i;
}

function classifyValueHeader(body) {
  let quoted = false;
  for (let i = 0; i < body.length; i++) {
    const c = body[i];
    if (quoted) {
      if (c === '\\') i++;
      else if (c === '"') quoted = false;
      continue;
    }
    if (c === '"') {
      quoted = true;
      continue;
    }
    if (c === ':') {
      return { key: trimOuter(body.slice(0, i)), type: '', rhs: body.slice(i + 1) };
    }
    if (c !== '(') continue;

    let depth = 1;
    let end = i + 1;
    for (; end < body.length && depth > 0; end++) {
      if (body[end] === '(') depth++;
      else if (body[end] === ')') depth--;
    }
    if (depth !== 0) return null;
    const close = end - 1;
    let colon = end;
    while (body[colon] === ' ') colon++;
    if (body[colon] !== ':') return null;
    return {
      key: trimOuter(body.slice(0, i)),
      type: body.slice(i + 1, close),
      rhs: body.slice(colon + 1),
    };
  }
  return null;
}

function isCommentOrBlank(text) {
  const body = trimOuter(text);
  return body === '' || body.startsWith('#');
}

function isPipeBodyLine(text, parentIndent) {
  const indent = lineIndent(text);
  return indent > parentIndent && text[indent] === '|';
}

function isTripleQuoteClosingLine(text) {
  return /^ *"""[ \t]*$/.test(text);
}

function findBodyStart(lines) {
  for (let i = 0; i < lines.length; i++) {
    if (trimOuter(lines[i].text) === '') return i + 1;
  }
  return lines.length;
}

/**
 * Format a valid dotReg document while preserving entry order, indentation,
 * comments, line endings, and every multiline body byte-for-byte.
 */
export async function formatText(text) {
  await parse(text);
  const lines = splitLines(text);
  const bodyStart = findBodyStart(lines);
  for (let i = bodyStart; i < lines.length; i++) {
    const line = lines[i].text;
    if (isCommentOrBlank(line)) continue;

    const indent = lineIndent(line);
    const body = line.slice(indent).replace(/[ \t]+$/g, '');
    const header = classifyValueHeader(body);
    if (!header) continue;

    const rhs = trimOuter(header.rhs);
    const left = header.type
      ? `${header.key} (${header.type})`
      : header.key;
    lines[i].text = `${line.slice(0, indent)}${left}:${rhs === '' ? '' : ` ${rhs}`}`;

    if (rhs === '"""') {
      for (let j = i + 1; j < lines.length; j++) {
        if (isTripleQuoteClosingLine(lines[j].text)) {
          i = j;
          break;
        }
      }
      continue;
    }

    if (rhs === '') {
      for (let j = i + 1; j < lines.length && isPipeBodyLine(lines[j].text, indent); j++) {
        // Pipe content is data. Leave the line and its line ending untouched.
        i = j;
      }
    }
  }
  return lines.map(line => line.text + line.ending).join('');
}

/** Return parser and common string-decoding diagnostics for editor clients. */
export async function inspectText(text) {
  const diagnostics = [];
  let entries;
  try {
    entries = await parse(text);
  } catch (error) {
    diagnostics.push({
      line: Math.max(1, Number.isInteger(error?.line) ? error.line : 1),
      severity: 'error',
      code: error?.code ?? 'syntax',
      message: `Check dotReg syntax (${error?.code ?? 'syntax'}).`,
    });
    return diagnostics;
  }

  for (const entry of entries) {
    if (entry.kind !== 'value') continue;
    try {
      decodeString(entry);
    } catch (error) {
      diagnostics.push({
        line: entry.line,
        severity: 'warning',
        code: error?.code ?? 'string',
        message: `The string value cannot be decoded (${error?.code ?? 'string'}).`,
      });
    }
  }
  return diagnostics;
}
