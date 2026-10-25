use std::io::{self, BufRead, BufReader, Write};
use dotreg::{decode_string, json_string, ParseError};

fn run() -> Result<(), ParseError> {
    let invalid = || ParseError { line: 0, cause: "invalid arguments or input" };
    let mut mode = "strings".to_owned();
    let mut file = None;
    let mut args = std::env::args().skip(1);
    while let Some(arg) = args.next() {
        if arg == "--mode" { mode = args.next().ok_or_else(invalid)?; }
        else if arg.starts_with('-') || file.is_some() { return Err(invalid()); }
        else { file = Some(arg); }
    }
    if mode != "raw" && mode != "strings" { return Err(invalid()); }
    let mut reader: Box<dyn BufRead> = if let Some(path) = file {
        Box::new(BufReader::new(std::fs::File::open(path).map_err(|_| invalid())?))
    } else { Box::new(BufReader::new(io::stdin())) };
    let mut output = String::from("[");
    let mut first = true;
    let entries = dotreg::parse(&mut reader)?;
    for entry in entries {
        let decoded = if mode == "strings" && entry.kind == "value" { Some(decode_string(&entry)?) } else { None };
        if !first { output.push(','); } first = false;
        output.push_str(&format!("{{\"line\":{},\"kind\":", entry.line)); json_string(entry.kind, &mut output);
        output.push_str(",\"path\":[");
        for (i, part) in entry.path.iter().enumerate() { if i != 0 { output.push(','); } json_string(part, &mut output); }
        output.push_str("],\"raw\":"); json_string(&entry.raw, &mut output);
        output.push_str(&format!(",\"unset\":{},\"has_type\":{}", entry.unset, entry.has_type));
        if !entry.type_expression.is_empty() { output.push_str(",\"type\":"); json_string(&entry.type_expression, &mut output); }
        if !entry.form.is_empty() { output.push_str(",\"form\":"); json_string(entry.form, &mut output); }
        if let Some(decoded) = decoded { output.push_str(",\"decoded\":"); json_string(&decoded, &mut output); }
        output.push('}');
    }
    output.push_str("]\n");
    io::stdout().lock().write_all(output.as_bytes()).map_err(|_| invalid())
}
fn main() {
    if let Err(error) = run() { eprintln!("{error}"); std::process::exit(1); }
}
