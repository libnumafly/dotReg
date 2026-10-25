# Shared parser conformance checks

This directory contains explicit expected results for the sample parser set.
The [format specification](../../docs/config-format.md) remains the source of
the rules; the [sample overview](../README.md) describes the common CLI contract
and provisional grammar choices.

[`build-corpus.mjs`](build-corpus.mjs) authors cases and expected fields directly.
It does not ask Go or another parser to generate an oracle.
[`cases.json`](cases.json) is the checked-in, generated corpus.
[`run.mjs`](run.mjs) executes a supplied CLI against the current corpus in both
`raw` and `strings` modes. The corpus and runner use Node.js built-in modules and
need no package installation.

## Run the checks

Run commands from the repository root. Build compiled samples first using their
language guides. Supply the executable and its fixed arguments as a JSON array:

```sh
node implementations/conformance/run.mjs --command '["python","implementations/python/dotreg.py"]'
node implementations/conformance/run.mjs --command '["node","implementations/javascript/configsample.mjs"]'
node implementations/conformance/run.mjs --command '["go","-C","implementations/go","run","./cmd/configsample"]'
node implementations/conformance/run.mjs --command '["java","-cp","/tmp/dotreg-java","DotReg"]'
node implementations/conformance/run.mjs --command '["dotnet","implementations/csharp/bin/Release/net8.0/DotReg.Sample.dll"]'
node implementations/conformance/run.mjs --command '["implementations/rust/target/release/dotreg-sample"]'
node implementations/conformance/run.mjs --command '["./dotreg-cpp"]'
```

Use the appropriate executable names and build paths on your platform; for
example, native binaries normally have `.exe` suffixes on Windows. TypeScript
shares the JavaScript engine, so its runtime conformance is checked by the
JavaScript command; type compilation is checked separately. Compiling Go or Java
once and supplying the built executable or classes avoids a build per case.

The runner passes array elements directly to the child process without a shell,
appends `--mode raw` or `--mode strings`, and sends the case's exact bytes through
standard input. Spaces within an executable path belong inside one JSON string.
No shell redirection, pipe expression or compound command belongs in the array.
The single quotes in the examples quote that JSON argument for the invoking
shell; the runner itself performs no shell interpolation.

To select cases whose names contain a string:

```sh
node implementations/conformance/run.mjs --command '["python","implementations/python/dotreg.py"]' --filter 'pipe'
```

Regenerate the committed corpus after changing its authored cases:

```sh
node implementations/conformance/build-corpus.mjs
```

Review both the generator change and generated JSON. The runner reads the cases
stored in `cases.json` and reports its number of passing and failing checks;
it does not rely on a fixed count documented here.

## Corpus representation

The top-level object has a corpus `version` and a `cases` array. Each case has a
descriptive `name` and exactly one input representation:

- `input`: complete configuration text, encoded as UTF-8 by the runner.
- `input_hex`: exact bytes in hexadecimal, used when valid Unicode JSON text
  cannot represent the intended invalid UTF-8 input.

Successful cases specify `entries` with the complete expected CLI fields. The
runner compares every field and its value, entry order and array element order.
JSON object property order is insignificant. Expected `decoded` fields are
removed for `raw`; `strings` expects them only on values. Headings remain distinct
entries and repetitions remain separate occurrences.

A failure case contains an `error` object with `stage`, `line` and a descriptive
`code`. A `stage: "parse"` failure must fail in both modes. A `stage: "decode"`
case supplies its successful raw syntax entries, succeeds in `raw` and fails in
`strings`. The parser set's common CLI validates the full syntax first, then
decodes values in source order. The corpus includes a case in which a later
syntax error takes precedence over an earlier value-decoding error.

For expected failures, the runner checks a non-zero exit status, normal process
termination, empty standard output and the expected physical `line N` in the
diagnostic. It also rejects diagnostics containing the case marker `SECRET`.
Exact error wording and language-specific exception classes are not compared;
`error.code` records the intended cause rather than a required CLI field. Each
child process has a 30-second timeout. Unexpected exits, malformed JSON, field
differences and empty case selections fail the check.

## Coverage and limits

The corpus exercises required declarations, line numbering, BOM handling,
ordinary and quoted paths, empty element names, Unicode escapes and invalid
UTF-8, heading resolution and indentation, annotation boundaries, whitespace,
unset versus explicit empty values, repeated occurrences, single-line decoding
and both multiline forms with their margins and original line endings. It also
separates structural errors from common string-decoding failures and checks
safe failure output. New cases can extend this evidence without changing the
CLI contract.

The CLI corpus cannot observe callback timing, exception identity, source
ownership or memory retention. Those behaviours have language-native tests in
the individual implementation directories. In particular, native tests check
that invalid non-body lookahead is deferred until the current recipient returns,
while invalid qualifying block content fails before value notification.

Passing the corpus does not establish a universal type profile, standard
retrieval API, application-schema validation or conformance to future format
decisions.
