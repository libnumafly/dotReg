# Rust sample parser

An independent Rust 2021 implementation of the [dotReg format](../../docs/config-format.md), using only the standard library. It parses required file declarations, headings, quoted path elements, annotations, repeated occurrences and all three value forms. `decode_string` implements common string decoding; type expressions remain opaque and no type conversion profile is supplied.

## Build and check

Use a current stable Rust toolchain:

```sh
cargo test --manifest-path implementations/rust/Cargo.toml
cargo build --release --manifest-path implementations/rust/Cargo.toml
```

The binary is `implementations/rust/target/release/dotreg-sample` (`.exe` on Windows). From this directory:

```sh
cargo run --release -- --mode strings settings.reg
cargo run --release -- --mode raw settings.reg
```

Omit the file argument to read UTF-8 bytes from standard input. The default mode is `strings`. A successful run emits one JSON array; `raw` exposes syntax entries and `strings` adds `decoded` to each value. A failed parse or string decoding writes a safe line-numbered diagnostic to standard error, returns a nonzero status and emits no JSON. The CLI collects syntax entries first, then decodes values and retains its output until the whole operation succeeds; use the library visitor API when results need not be collected.

The shared [conformance runner](../conformance/README.md) accepts the executable as a command, for example:

```sh
node implementations/conformance/run.mjs --command '["implementations/rust/target/release/dotreg-sample"]'
```

## Library API and retention

```rust
use dotreg::{decode_string, parse_each, ParseError};
use std::io::{self, BufReader};

fn main() -> Result<(), dotreg::Failure<ParseError>> {
    let mut source = BufReader::new(io::stdin());
    parse_each(&mut source, |entry| {
        if entry.kind == "value" {
            let text = decode_string(&entry)?;
            // Consume text here. entry.unset distinguishes unset and explicit empty.
            println!("line {}: {} bytes", entry.line, text.len());
        }
        Ok::<_, ParseError>(())
    })
}
```

`parse_each` receives any `BufRead` and a typed closure; Rust's types require both inputs. It notifies the recipient before parsing the next logical entry. `Failure::Visitor(E)` contains the original recipient error value unchanged; `Failure::Parse(ParseError)` carries a safe source or syntax error. Side effects from earlier recipient calls are not rolled back. A non-pipe lookahead line is validated after the current notification, so a recipient failure takes precedence over that next line's error. Reader buffering may fetch bytes ahead.

`parse` returns `Result<Vec<Entry>, ParseError>`. It returns no partial collection on failure. Each read starts with fresh state. Every returned path owns its strings, so recipient mutation cannot modify active heading scopes. Paths retain element boundaries and empty names; repeated keys retain all occurrences in order.

The parser walks input once without regular expressions or a whole-file syntax tree. Active headings share immutable parent nodes containing only their written suffix. Parsing does not retain previous value occurrences. Streaming retention follows the active heading chain, current resolved path, one complete current block and one lookahead line, plus the reader's own buffering. Triple-quoted margin removal temporarily retains source body lines and the completed body together. Collecting entries necessarily retains all entries and their expanded paths. Long physical lines, block bodies and deep paths have no fixed format limit; available memory remains a practical bound. Heading path traversal and scope release are iterative.

Raw syntax and string decoding are separate operations. A complete quoted value with an invalid escape can be returned by `parse`; `decode_string` rejects it before application delivery. UTF-8, physical line endings and quoted key escapes are validated during syntax processing. The tests cover recipient stop identity, deferred lookahead, mutation isolation, source failure redaction, deep paths and multiline preservation.
