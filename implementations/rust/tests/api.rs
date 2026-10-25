use dotreg::{decode_string, parse, parse_each, Failure, ParseError};
use std::io::{self, BufRead, Cursor, Read};

#[test]
fn mutation_cannot_change_headings_and_occurrences_are_retained() {
    let mut source = Cursor::new(b"@dotReg 0.1\n\nA.B.\n  .C:one\n  .C:one\n  .:root\n");
    let mut paths = Vec::new();
    parse_each(&mut source, |mut entry| {
        paths.push(entry.path.clone());
        entry.path[0] = "recipient mutation".into();
        Ok::<_, ()>(())
    }).unwrap();
    assert_eq!(paths, vec![vec!["A", "B"], vec!["A", "B", "C"], vec!["A", "B", "C"], vec!["A", "B"]]);
}

#[test]
fn mixed_value_paths_own_quoted_empty_and_inherited_elements() {
    let mut source = Cursor::new(br#"@dotReg 0.1

Root."".
  ."A\.B"."":one
  ."A\.B"."":two
  .:self
  .Child.
    ."leaf.name":three
Other."":absolute
"#);
    let mut entries = Vec::new();
    parse_each(&mut source, |mut entry| {
        let path = entry.path.clone();
        entry.path[0] = "recipient mutation".into();
        entry.path.push("extra".into());
        entries.push((path, entry));
        Ok::<_, ()>(())
    }).unwrap();
    let paths: Vec<_> = entries.iter().map(|(path, _)| path.clone()).collect();
    assert_eq!(paths, vec![
        vec!["Root", ""], vec!["Root", "", "A.B", ""], vec!["Root", "", "A.B", ""],
        vec!["Root", ""], vec!["Root", "", "Child"], vec!["Root", "", "Child", "leaf.name"],
        vec!["Other", ""],
    ]);
    assert_eq!(entries[1].1.raw, "one");
    assert_eq!(entries[2].1.raw, "two");
    assert_eq!(entries[3].1.raw, "self");
    assert_eq!(entries[6].1.raw, "absolute");
    for (_, entry) in &entries {
        assert_eq!(entry.path[0], "recipient mutation");
        assert_eq!(entry.path.last().unwrap(), "extra");
    }
}

#[test]
fn stop_preserves_identity_before_validating_lookahead() {
    let sentinel = Box::new(17);
    let identity = &*sentinel as *const i32;
    let mut sentinel = Some(sentinel);
    let mut source = Cursor::new(b"@dotReg 0.1\n\nA:\n\xff\n");
    let mut calls = 0;
    match parse_each(&mut source, |_| { calls += 1; Err(sentinel.take().unwrap()) }) {
        Err(Failure::Visitor(returned)) => assert_eq!(&*returned as *const i32, identity),
        _ => panic!("visitor error must win"),
    }
    assert_eq!(calls, 1);
}

#[test]
fn qualifying_invalid_pipe_line_fails_before_notification() {
    let mut source = Cursor::new(b"@dotReg 0.1\n\nA:\n |\xff\n");
    let mut calls = 0;
    let result = parse_each(&mut source, |_| { calls += 1; Ok::<_, ()>(()) });
    assert!(matches!(result, Err(Failure::Parse(ParseError { line: 4, .. }))));
    assert_eq!(calls, 0);
}

struct Broken { prefix: Cursor<Vec<u8>> }
impl Read for Broken {
    fn read(&mut self, target: &mut [u8]) -> io::Result<usize> {
        let bytes = self.fill_buf()?;
        let size = target.len().min(bytes.len()); target[..size].copy_from_slice(&bytes[..size]); self.consume(size); Ok(size)
    }
}
impl BufRead for Broken {
    fn fill_buf(&mut self) -> io::Result<&[u8]> {
        if self.prefix.position() as usize == self.prefix.get_ref().len() { return Err(io::Error::other("PRIVATE SOURCE ERROR")); }
        self.prefix.fill_buf()
    }
    fn consume(&mut self, count: usize) { self.prefix.consume(count); }
}

#[test]
fn source_errors_are_safe_and_stop_needs_no_later_read() {
    let prefix = b"@dotReg 0.1\n\nA:ok\n".to_vec();
    let mut source = Broken { prefix: Cursor::new(prefix.clone()) };
    let failure = parse(&mut source).unwrap_err();
    assert_eq!(failure.line, 4);
    assert!(!failure.to_string().contains("PRIVATE"));
    let mut source = Broken { prefix: Cursor::new(prefix) };
    assert!(matches!(parse_each(&mut source, |_| Err(31)), Err(Failure::Visitor(31))));
}

#[test]
fn multiline_forms_preserve_source_endings_and_decode_once() {
    let mut source = Cursor::new(b"@dotReg 0.1\n\nA: \"\"\"\n   first\\n\r\n     second\n   \"\"\"\nB:\n |C:\\temp\\q\r\n |\n");
    let entries = parse(&mut source).unwrap();
    assert_eq!(entries[0].raw, "first\\n\r\n  second");
    assert_eq!(decode_string(&entries[0]).unwrap(), "first\n\r\n  second");
    assert_eq!(entries[1].form, "pipe");
    assert_eq!(decode_string(&entries[1]).unwrap(), "C:\\temp\\q\r\n");
}

#[test]
fn unicode_boundaries_types_and_raw_decoding_are_independent() {
    let mut source = Cursor::new(br#"@dotReg 0.1

"A\.B".""(map(str,list(int))):"\uD83D\uDE00\u{0}"
A:"\q"
"#);
    let entries = parse(&mut source).unwrap();
    assert_eq!(entries[0].path, vec!["A.B", ""]);
    assert_eq!(entries[0].type_expression, "map(str,list(int))");
    assert_eq!(decode_string(&entries[0]).unwrap(), "😀\0");
    assert!(decode_string(&entries[1]).is_err());
}

#[test]
fn declaration_encoding_and_dedent_fail_atomically() {
    for body in [b"".as_slice(), b"@dotReg 0.1\n", b"@dotReg 0.1\n\nA:\xff\n", b"@dotReg 0.1\n\nA:ok\r", b"@dotReg 0.1\n\nA.\n  .B.\n    .C:yes\n  .D:yes\n    .E:no\n"] {
        let result = parse(&mut Cursor::new(body));
        assert!(result.is_err());
    }
    assert!(parse(&mut Cursor::new(b"@dotReg 0.1\n\n")).unwrap().is_empty());
}

#[test]
fn deep_streaming_paths_have_no_recursive_walk_or_scope_release() {
    let mut text = String::from("@dotReg 0.1\n\nRoot.\n");
    for depth in 1..1500 { text.push_str(&" ".repeat(depth * 2)); text.push_str(".Child.\n"); }
    let mut count = 0;
    parse_each(&mut Cursor::new(text), |entry| { count += 1; assert_eq!(entry.path.len(), count); Ok::<_, ParseError>(()) }).unwrap();
    assert_eq!(count, 1500);
}
