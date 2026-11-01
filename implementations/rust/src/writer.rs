use std::io::Cursor;

use crate::{Entry, ParseError, Text, WriteValue};

fn failure(cause: &'static str) -> ParseError { ParseError { line: 0, cause } }

/// Append one occurrence under the deepest matching heading, or as a full key.
pub fn append_value(source: &str, path: &[String], value: &WriteValue) -> Result<String, ParseError> {
    let entries = parse_source(source)?;
    append_parsed(source, &entries, path, value)
}

/// Replace a unique occurrence, or append an untyped value when absent.
pub fn set_text(source: &str, path: &[String], text: &Text) -> Result<String, ParseError> {
    let entries = parse_source(source)?;
    let matches = matching_values(&entries, path)?;
    if matches.is_empty() { return append_parsed(source, &entries, path, &WriteValue { text: text.clone(), type_expression: None }); }
    if matches.len() > 1 {
        return Err(ParseError { line: matches[1].line, cause: "multiple values" });
    }
    replace_text(source, matches[0], text)
}

/// Replace one zero-based occurrence of a repeated key.
pub fn set_text_at(source: &str, path: &[String], occurrence: usize,
                   text: &Text) -> Result<String, ParseError> {
    let entries = parse_source(source)?;
    let matches = matching_values(&entries, path)?;
    if occurrence >= matches.len() { return Err(failure("missing")); }
    replace_text(source, matches[occurrence], text)
}

fn parse_source(source: &str) -> Result<Vec<Entry>, ParseError> {
    let mut cursor = Cursor::new(source.as_bytes());
    crate::parse(&mut cursor)
}

fn matching_values<'a>(entries: &'a [Entry], path: &[String]) -> Result<Vec<&'a Entry>, ParseError> {
    if path.is_empty() { return Err(failure("input")); }
    Ok(entries.iter().filter(|entry| entry.kind == "value" && entry.path.as_slice() == path).collect())
}

fn append_parsed(source: &str, entries: &[Entry], path: &[String],
                 value: &WriteValue) -> Result<String, ParseError> {
    if path.is_empty() || value.text.unset && !value.text.value.is_empty() ||
        value.type_expression.as_ref().is_some_and(String::is_empty) {
        return Err(failure("input"));
    }
    let key = encode_path(path)?;
    let mut line = key.clone();
    if let Some(type_expression) = &value.type_expression {
        line.push_str(" (");
        line.push_str(type_expression);
        line.push(')');
    }
    line.push(':');
    if !value.text.unset {
        line.push(' ');
        line.push_str(&encode_quoted(&value.text.value));
    }
    let mut insert_at = source.len();
    let mut indent = 0;
    let mut heading: Option<&Entry> = None;
    for entry in entries {
        if entry.kind == "heading" && has_prefix(path, &entry.path) &&
            heading.map_or(true, |current| entry.path.len() >= current.path.len()) {
            heading = Some(entry);
        }
    }
    if let Some(heading) = heading {
        let lines = writer_lines(source);
        indent = leading_spaces(lines[heading.line - 1].text) + 2;
        let suffix = if path.len() == heading.path.len() {
            String::new()
        } else {
            encode_path(&path[heading.path.len()..])?
        };
        line = format!(".{suffix}{}", &line[key.len()..]);
        insert_at = heading_section_end(entries, &lines, heading)?;
    }
    line.insert_str(0, &" ".repeat(indent));
    let updated = insert_source_line(source, insert_at, &line, line_ending_at(source, insert_at));
    parse_source(&updated)?;
    Ok(updated)
}

fn replace_text(source: &str, entry: &Entry, text: &Text) -> Result<String, ParseError> {
    if text.unset && !text.value.is_empty() { return Err(failure("input")); }
    let lines = writer_lines(source);
    let start = entry.line - 1;
    if start >= lines.len() { return Err(ParseError { line: entry.line, cause: "invalid syntax" }); }
    let end = entry_end_line(&lines, entry)?;
    let first = lines[start];
    let separator = value_separator(first.text).ok_or(ParseError { line: entry.line, cause: "invalid syntax" })?;
    let rhs = if text.unset { String::new() } else { format!(" {}", encode_quoted(&text.value)) };
    let last = lines[end];
    let replacement = format!("{}{}{}", &source[first.start..first.start + separator + 1],
        rhs, &source[last.content_end..last.end]);
    let updated = format!("{}{}{}", &source[..first.start], replacement, &source[last.end..]);
    parse_source(&updated)?;
    Ok(updated)
}

fn has_prefix(path: &[String], prefix: &[String]) -> bool {
    path.starts_with(prefix)
}

fn encode_path(path: &[String]) -> Result<String, ParseError> {
    if path.is_empty() { return Err(failure("input")); }
    let mut result = String::new();
    for (index, element) in path.iter().enumerate() {
        if index > 0 { result.push('.'); }
        result.push_str(&encode_key_element(element));
    }
    Ok(result)
}

fn encode_key_element(element: &str) -> String {
    let plain = !element.is_empty() && !element.contains('.') &&
        !element.chars().any(|c| c.is_whitespace() || c.is_control() || ":()#\"\\\\".contains(c));
    if plain { element.to_owned() } else { encode_quoted(element) }
}

fn encode_quoted(value: &str) -> String {
    let mut result = String::from("\"");
    for c in value.chars() {
        match c {
            '"' => result.push_str("\\\""),
            '\\' => result.push_str("\\\\"),
            '\u{8}' => result.push_str("\\b"),
            '\u{c}' => result.push_str("\\f"),
            '\n' => result.push_str("\\n"),
            '\r' => result.push_str("\\r"),
            '\t' => result.push_str("\\t"),
            c if c.is_control() => result.push_str(&format!("\\u{{{:x}}}", c as u32)),
            c => result.push(c),
        }
    }
    result.push('"');
    result
}

#[derive(Clone, Copy)]
struct SourceLine<'a> { text: &'a str, start: usize, content_end: usize, end: usize }

fn writer_lines(source: &str) -> Vec<SourceLine<'_>> {
    let mut lines = Vec::new();
    let mut start = 0;
    while start < source.len() {
        let newline = source[start..].find('\n').map(|i| start + i);
        let end = newline.map_or(source.len(), |i| i + 1);
        let mut content_end = newline.unwrap_or(end);
        if content_end > start && source.as_bytes()[content_end - 1] == b'\r' { content_end -= 1; }
        lines.push(SourceLine { text: &source[start..content_end], start, content_end, end });
        start = end;
    }
    lines
}

fn leading_spaces(text: &str) -> usize { text.bytes().take_while(|&c| c == b' ').count() }

fn entry_end_line(lines: &[SourceLine<'_>], entry: &Entry) -> Result<usize, ParseError> {
    let start = entry.line - 1;
    if start >= lines.len() { return Err(ParseError { line: entry.line, cause: "invalid syntax" }); }
    if entry.form.is_empty() { return Ok(start); }
    if entry.form == "triple_quoted" {
        for (index, line) in lines.iter().enumerate().skip(start + 1) {
            let indent = leading_spaces(line.text);
            if line.text[indent..].trim_end_matches([' ', '\t']) == "\"\"\"" { return Ok(index); }
        }
        return Err(ParseError { line: entry.line, cause: "invalid syntax" });
    }
    if entry.form == "pipe" {
        let parent = leading_spaces(lines[start].text);
        let mut end = start;
        for (index, line) in lines.iter().enumerate().skip(start + 1) {
            let indent = leading_spaces(line.text);
            if indent <= parent || indent == line.text.len() || line.text.as_bytes()[indent] != b'|' { break; }
            end = index;
        }
        if end == start { return Err(ParseError { line: entry.line, cause: "invalid syntax" }); }
        return Ok(end);
    }
    Err(ParseError { line: entry.line, cause: "invalid syntax" })
}

fn heading_section_end(entries: &[Entry], lines: &[SourceLine<'_>], heading: &Entry) -> Result<usize, ParseError> {
    let heading_index = heading.line - 1;
    let indent = leading_spaces(lines[heading_index].text);
    let mut insert_at = lines[heading_index].end;
    let mut found = false;
    for entry in entries {
        if !found {
            if entry.kind == "heading" && entry.line == heading.line { found = true; }
            continue;
        }
        let index = entry.line - 1;
        if leading_spaces(lines[index].text) <= indent { break; }
        insert_at = lines[entry_end_line(lines, entry)?].end;
    }
    Ok(insert_at)
}

fn value_separator(line: &str) -> Option<usize> {
    let mut quoted = false;
    let mut depth = 0;
    let bytes = line.as_bytes();
    let mut i = 0;
    while i < bytes.len() {
        match bytes[i] {
            b'\\' if quoted => i += 1,
            b'"' => quoted = !quoted,
            b'(' if !quoted => depth += 1,
            b')' if !quoted && depth > 0 => depth -= 1,
            b':' if !quoted && depth == 0 => return Some(i),
            _ => {}
        }
        i += 1;
    }
    None
}

fn line_ending_at(source: &str, offset: usize) -> &'static str {
    let bytes = source.as_bytes();
    let mut found = bytes.get(offset..).unwrap_or_default().iter().position(|&c| c == b'\n')
        .map(|i| offset + i);
    if found.is_none() {
        found = bytes[..offset.min(bytes.len())].iter().rposition(|&c| c == b'\n');
    }
    found.filter(|&i| i > 0 && bytes[i - 1] == b'\r').map_or("\n", |_| "\r\n")
}

fn insert_source_line(source: &str, offset: usize, line: &str, ending: &str) -> String {
    let bytes = source.as_bytes();
    let previous_break = offset == 0 || bytes[offset - 1] == b'\n';
    let following = offset < source.len();
    let final_break = source.ends_with('\n');
    let needs_following_break = following || offset == source.len() && final_break;
    format!("{}{}{}{}{}", &source[..offset], if previous_break { "" } else { ending },
        line, if needs_following_break { ending } else { "" }, &source[offset..])
}
