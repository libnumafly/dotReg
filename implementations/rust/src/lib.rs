//! Dependency-free sample syntax parser and common string decoder.
use std::convert::Infallible;
use std::fmt;
use std::io::BufRead;
use std::rc::Rc;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Entry {
    pub line: usize,
    pub kind: &'static str,
    pub path: Vec<String>,
    pub raw: String,
    pub unset: bool,
    pub type_expression: String,
    pub has_type: bool,
    pub form: &'static str,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ParseError { pub line: usize, pub cause: &'static str }
impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "dotReg line {}: {}", self.line, self.cause)
    }
}
impl std::error::Error for ParseError {}

/// Visitor errors retain their original value; source errors contain no input text.
#[derive(Debug)]
pub enum Failure<E> { Parse(ParseError), Visitor(E) }
impl<E: fmt::Display> fmt::Display for Failure<E> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self { Self::Parse(error) => error.fmt(f), Self::Visitor(error) => error.fmt(f) }
    }
}
impl<E: std::error::Error + 'static> std::error::Error for Failure<E> {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        Some(match self { Self::Parse(error) => error, Self::Visitor(error) => error })
    }
}
impl<E> From<ParseError> for Failure<E> {
    fn from(error: ParseError) -> Self { Self::Parse(error) }
}
fn error(line: usize) -> ParseError { ParseError { line, cause: "invalid syntax" } }
fn trim(s: &str) -> &str { s.trim_matches([' ', '\t']) }
fn spaces(s: &[u8]) -> usize { s.iter().take_while(|&&c| c == b' ').count() }

struct Physical { bytes: Vec<u8>, ending: &'static str, line: usize, eof: bool, failed: bool }
impl Physical {
    fn text(&self) -> Result<&str, ParseError> {
        if self.failed { return Err(ParseError { line: self.line, cause: "input failure" }); }
        let text = std::str::from_utf8(&self.bytes).map_err(|_| error(self.line))?;
        if text.contains('\r') { return Err(error(self.line)); }
        Ok(text)
    }
}
struct Lines<'a, R> { reader: &'a mut R, next_line: usize, pending: Option<Physical> }
impl<R: BufRead> Lines<'_, R> {
    fn next(&mut self) -> Physical {
        if let Some(line) = self.pending.take() { return line; }
        let mut bytes = Vec::new();
        let read = self.reader.read_until(b'\n', &mut bytes);
        let failed = read.is_err();
        let eof = !bytes.ends_with(b"\n");
        let mut ending = "";
        if !eof {
            bytes.pop(); ending = "\n";
            if bytes.ends_with(b"\r") { bytes.pop(); ending = "\r\n"; }
        }
        if self.next_line == 1 && bytes.starts_with(&[0xef, 0xbb, 0xbf]) { bytes.drain(..3); }
        let line = Physical { bytes, ending, line: self.next_line, eof, failed };
        self.next_line += 1;
        line
    }
    fn properties(&mut self) -> Result<bool, ParseError> {
        let mut declared = false;
        loop {
            let line = self.next();
            let text = line.text()?.trim_end_matches([' ', '\t']);
            if text.is_empty() {
                if !declared || (line.bytes.is_empty() && line.ending.is_empty()) { return Err(error(line.line)); }
                return Ok(line.eof);
            }
            let sep = text.find([' ', '\t']).ok_or_else(|| error(line.line))?;
            if sep == 0 || !text.starts_with('@') { return Err(error(line.line)); }
            let value = text[sep..].trim_start_matches([' ', '\t']);
            if declared || &text[..sep] != "@dotReg" || value != "1" { return Err(error(line.line)); }
            declared = true;
            if line.eof { return Err(error(line.line + 1)); }
        }
    }
    fn value_body(&mut self, e: &mut Entry, indent: usize, eof: bool) -> Result<bool, ParseError> {
        if e.raw == "\"\"\"" {
            let mut body: Vec<Physical> = Vec::new();
            loop {
                let line = self.next();
                let text = line.text()?;
                let margin = spaces(text.as_bytes());
                if text[margin..].trim_end_matches([' ', '\t']) == "\"\"\"" {
                    let mut raw = String::new();
                    for (i, part) in body.iter().enumerate() {
                        let text = part.text()?;
                        if i != 0 { raw.push_str(body[i - 1].ending); }
                        if !text.is_empty() {
                            if spaces(text.as_bytes()) < margin { return Err(error(part.line)); }
                            raw.push_str(&text[margin..]);
                        }
                    }
                    e.raw = raw; e.form = "triple_quoted"; e.unset = false;
                    return Ok(line.eof);
                }
                if line.eof { return Err(error(e.line)); }
                body.push(line);
            }
        }
        if e.unset && !eof {
            let mut raw = String::new();
            let mut found = false;
            let mut ending = "";
            loop {
                let line = self.next();
                let prefix = spaces(&line.bytes);
                if prefix <= indent || line.bytes.get(prefix) != Some(&b'|') {
                    self.pending = Some(line);
                    if found { e.raw = raw; e.form = "pipe"; e.unset = false; }
                    return Ok(false);
                }
                let text = line.text()?;
                if found { raw.push_str(ending); }
                raw.push_str(&text[prefix + 1..]);
                found = true; ending = line.ending;
                if line.eof {
                    e.raw = raw; e.form = "pipe"; e.unset = false;
                    return Ok(true);
                }
            }
        }
        Ok(eof)
    }
}

struct Node { parent: Option<Rc<Node>>, suffix: Vec<String>, count: usize }
impl Node {
    fn elements(&self) -> Vec<String> {
        let mut parts = Vec::new();
        let mut current = Some(self);
        while let Some(node) = current { parts.push(&node.suffix); current = node.parent.as_deref(); }
        let mut result = Vec::with_capacity(self.count);
        for suffix in parts.into_iter().rev() { result.extend(suffix.iter().cloned()); }
        result
    }
}
struct Scopes(Vec<Rc<Node>>);
impl Scopes {
    fn truncate(&mut self, size: usize) { while self.0.len() > size { self.0.pop(); } }
}
impl Drop for Scopes { fn drop(&mut self) { self.truncate(0); } }

/// Stream source-order entries. Each entry owns its path, so mutation cannot alter scopes.
/// Only the active heading chain, current value and one physical lookahead are retained.
pub fn parse_each<R: BufRead, E, F: FnMut(Entry) -> Result<(), E>>(
    reader: &mut R, mut visit: F,
) -> Result<(), Failure<E>> {
    let mut lines = Lines { reader, next_line: 1, pending: None };
    if lines.properties()? { return Ok(()); }
    let mut scopes = Scopes(Vec::new());
    loop {
        let physical = lines.next();
        let source = physical.text()?;
        let mut eof = physical.eof;
        if !trim(source).is_empty() && !trim(source).starts_with('#') {
            let indent = spaces(source.as_bytes());
            let body = source[indent..].trim_end_matches([' ', '\t']);
            if body.starts_with('\t') || indent % 2 != 0 { return Err(error(physical.line).into()); }
            let relative = body.starts_with('.');
            if relative != (indent > 0) { return Err(error(physical.line).into()); }
            let (mut entry, mut key) = split_line(body, physical.line)?;
            let parent = if relative {
                let depth = indent / 2;
                if depth > scopes.0.len() { return Err(error(physical.line).into()); }
                scopes.truncate(depth);
                key = &key[1..];
                Some(Rc::clone(&scopes.0[depth - 1]))
            } else { scopes.truncate(0); None };
            let self_assignment = relative && key.is_empty() && entry.kind == "value";
            let node = if self_assignment { parent.unwrap() } else {
                let suffix = parse_path(key, physical.line)?;
                let count = suffix.len() + parent.as_ref().map_or(0, |p| p.count);
                Rc::new(Node { parent, suffix, count })
            };
            entry.path = node.elements();
            if entry.kind == "heading" { scopes.0.push(node); }
            else { eof = lines.value_body(&mut entry, indent, eof)?; }
            visit(entry).map_err(Failure::Visitor)?;
        }
        if eof { return Ok(()); }
    }
}

/// Collect syntax entries atomically: failures return no partial collection.
pub fn parse<R: BufRead>(reader: &mut R) -> Result<Vec<Entry>, ParseError> {
    let mut entries = Vec::new();
    match parse_each(reader, |entry| { entries.push(entry); Ok::<_, Infallible>(()) }) {
        Ok(()) => Ok(entries),
        Err(Failure::Parse(e)) => Err(e),
        Err(Failure::Visitor(never)) => match never {},
    }
}

fn split_line(body: &str, line: usize) -> Result<(Entry, &str), ParseError> {
    let mut e = Entry { line, kind: "heading", path: Vec::new(), raw: String::new(), unset: false,
        type_expression: String::new(), has_type: false, form: "" };
    let mut i = 0;
    let bytes = body.as_bytes();
    while i < bytes.len() {
        let (key, rhs) = match bytes[i] {
            b'"' => { i = quoted(body, i, None).map_err(|_| error(line))?; continue; }
            b':' => (trim(&body[..i]), &body[i + 1..]),
            b'(' => {
                let key = body[..i].trim_end_matches(' ');
                if key.ends_with('\t') { return Err(error(line)); }
                let mut depth = 1;
                let mut close = None;
                for (offset, c) in body[i + 1..].char_indices() {
                    if c.is_whitespace() || c == '"' || c == '\\' { return Err(error(line)); }
                    if c == '(' { depth += 1; }
                    if c == ')' { depth -= 1; if depth == 0 { close = Some(i + 1 + offset); break; } }
                }
                let close = close.ok_or_else(|| error(line))?;
                if close == i + 1 { return Err(error(line)); }
                let tail = body[close + 1..].trim_start_matches(' ');
                if !tail.starts_with(':') { return Err(error(line)); }
                e.type_expression = body[i + 1..close].to_owned(); e.has_type = true;
                (key, &tail[1..])
            }
            _ => { i += 1; continue; }
        };
        e.kind = "value"; e.raw = trim(rhs).to_owned(); e.unset = e.raw.is_empty();
        return Ok((e, key));
    }
    if let Some(key) = body.strip_suffix('.') { Ok((e, key)) } else { Err(error(line)) }
}

fn parse_path(mut key: &str, line: usize) -> Result<Vec<String>, ParseError> {
    let mut path = Vec::new();
    loop {
        if key.is_empty() { return Err(error(line)); }
        let element;
        if key.starts_with('"') {
            let mut decoded = String::new();
            let end = quoted(key, 0, Some(&mut decoded)).map_err(|_| error(line))?;
            element = decoded; key = &key[end..];
            if !key.is_empty() && !key.starts_with('.') { return Err(error(line)); }
        } else {
            let end = key.find('.').unwrap_or(key.len());
            let raw = &key[..end];
            if raw.is_empty() || raw.chars().any(|c| c.is_whitespace() || c.is_control() || ":()#\"\\".contains(c)) {
                return Err(error(line));
            }
            element = raw.to_owned(); key = &key[end..];
        }
        path.push(element);
        if key.is_empty() { return Ok(path); }
        key = &key[1..];
    }
}

fn hex(c: u8) -> Result<u32, ()> {
    match c { b'0'..=b'9' => Ok((c - b'0') as u32), b'a'..=b'f' => Ok((c - b'a' + 10) as u32),
        b'A'..=b'F' => Ok((c - b'A' + 10) as u32), _ => Err(()) }
}
fn four(bytes: &[u8], start: usize) -> Result<u32, ()> {
    let slice = bytes.get(start..start + 4).ok_or(())?;
    slice.iter().try_fold(0, |v, &c| Ok(v * 16 + hex(c)?))
}
fn escape(s: &str, start: usize) -> Result<(char, usize), ()> {
    let b = s.as_bytes();
    let end = start + 2;
    let c = *b.get(start + 1).ok_or(())?;
    let basic = match c { b'.' | b'"' | b'\\' | b'/' => Some(c as char), b'b' => Some('\u{8}'),
        b'f' => Some('\u{c}'), b'n' => Some('\n'), b'r' => Some('\r'), b't' => Some('\t'), _ => None };
    if let Some(c) = basic { return Ok((c, end)); }
    if c != b'u' { return Err(()); }
    if b.get(end) == Some(&b'{') {
        let mut value = 0;
        for digits in 0..=6 {
            let c = *b.get(end + 1 + digits).ok_or(())?;
            if c == b'}' && digits > 0 { return Ok((char::from_u32(value).ok_or(())?, end + 2 + digits)); }
            if digits == 6 { return Err(()); }
            value = value * 16 + hex(c)?;
        }
        return Err(());
    }
    let mut value = four(b, end)?;
    let mut next = end + 4;
    if (0xd800..=0xdbff).contains(&value) {
        if b.get(next..next + 2) != Some(b"\\u") { return Err(()); }
        let low = four(b, next + 2)?;
        if !(0xdc00..=0xdfff).contains(&low) { return Err(()); }
        value = 0x10000 + (value - 0xd800) * 0x400 + low - 0xdc00;
        next += 6;
    }
    Ok((char::from_u32(value).ok_or(())?, next))
}
fn quoted(s: &str, start: usize, mut out: Option<&mut String>) -> Result<usize, ()> {
    if s.as_bytes().get(start) != Some(&b'"') { return Err(()); }
    let mut i = start + 1;
    let mut literal = i;
    while i < s.len() {
        match s.as_bytes()[i] {
            b'"' => { if let Some(o) = out.as_deref_mut() { o.push_str(&s[literal..i]); } return Ok(i + 1); }
            b'\r' | b'\n' => return Err(()),
            b'\\' => {
                let (c, end) = escape(s, i)?;
                if let Some(o) = out.as_deref_mut() { o.push_str(&s[literal..i]); o.push(c); }
                i = end; literal = end;
            }
            _ => i += 1,
        }
    }
    Err(())
}

/// Apply the shared string rules. Type annotations are deliberately opaque.
pub fn decode_string(entry: &Entry) -> Result<String, ParseError> {
    let invalid = || ParseError { line: entry.line, cause: "invalid string" };
    if entry.kind != "value" { return Err(invalid()); }
    if !entry.form.is_empty() {
        if entry.unset || entry.raw.as_bytes().iter().enumerate().any(|(i, &c)| c == b'\r' && entry.raw.as_bytes().get(i + 1) != Some(&b'\n')) {
            return Err(invalid());
        }
        if entry.form == "pipe" { return Ok(entry.raw.clone()); }
        if entry.form != "triple_quoted" { return Err(invalid()); }
        let mut out = String::new();
        let mut start = 0;
        while let Some(offset) = entry.raw[start..].find('\\') {
            let i = start + offset;
            out.push_str(&entry.raw[start..i]);
            let (c, end) = escape(&entry.raw, i).map_err(|_| invalid())?;
            out.push(c); start = end;
        }
        out.push_str(&entry.raw[start..]); return Ok(out);
    }
    let raw = trim(&entry.raw);
    if raw.contains(['\r', '\n']) || entry.unset != raw.is_empty() { return Err(invalid()); }
    if !raw.starts_with('"') { return Ok(raw.to_owned()); }
    let mut out = String::new();
    if quoted(raw, 0, Some(&mut out)).map_err(|_| invalid())? != raw.len() { return Err(invalid()); }
    Ok(out)
}

/// Minimal JSON string writer for the sample command; no deserialiser is involved.
pub fn json_string(s: &str, out: &mut String) {
    out.push('"');
    for c in s.chars() {
        match c { '"' => out.push_str("\\\""), '\\' => out.push_str("\\\\"), '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"), '\t' => out.push_str("\\t"), c if c < ' ' => {
                use std::fmt::Write; write!(out, "\\u{:04x}", c as u32).unwrap();
            }, c => out.push(c) }
    }
    out.push('"');
}
