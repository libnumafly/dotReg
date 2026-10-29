---
name: dotreg
description: "Create, edit, migrate and integrate dotReg (.dotreg) configuration files in applications. Use for dotReg syntax, configuration loading, schema validation and repeated-value handling; format-specification and parser-internal changes require the upstream repository workflow."
---

# dotReg application development

Use dotReg 0.1 for application configuration while preserving its text, paths,
occurrences and value states. This folder is portable: its references and checker
work without a dotReg checkout or network access. The checker needs Node.js 20+.

## Choose the relevant reference

- Before writing, editing or converting configuration, read
  [references/syntax.md](references/syntax.md). It covers heading resolution,
  literal versus quoted text, multiline bodies and unset values.
- Before implementing a loader or application schema, read
  [references/integration.md](references/integration.md). It covers existing
  language implementations, stage boundaries, retrieval and validation.
- Use [assets/example.dotreg](assets/example.dotreg) for complete-file examples.
  Its annotated types illustrate a demo profile; adopt only the application's
  supported types and keys.

## Preserve the configuration contract

1. Identify the target application's existing keys, accepted type profile,
   cardinality, defaults and loader. Preserve those choices when editing files;
   make any new application rules explicit when adding support.
2. Start complete files with `@dotReg 0.1` and a blank line. Use two ASCII spaces
   per written heading level. Resolve relative paths before moving entries or
   changing headings; an absolute line closes the active headings.
3. Keep decoded key elements as a sequence. A quoted dot is part of one name,
   numeric names remain strings, and an empty name is an explicit element.
   A key can have both its own values and descendants.
4. Retain every occurrence in source order, including identical repeated values.
   Keep each occurrence's type annotation, unset flag, value form and source
   line. Enforce single-value requirements in the application's schema.
5. Keep syntax parsing, common string decoding, type conversion and schema
   validation distinct. Preserve leading zeroes and literal backslashes; do not
   infer numbers, booleans, arrays or null from the text's appearance.
6. Use the application's declared type profile. A quoted value does not declare
   a string type. Never fall back to another type after a declared conversion
   fails. Candidate type names and sample APIs are provisional, not standards.
7. Preserve block contents and their internal LF/CRLF when editing. There are no
   inline comments, implicit environment expansion or automatic includes.

## Check the result

Resolve script paths relative to this `SKILL.md`, not the target application's
working directory. From the skill folder:

```sh
node scripts/validate.mjs path/to/settings.dotreg
node scripts/validate.mjs --mode raw path/to/settings.dotreg
```

The default validates the entire syntax and common string decoding, then prints
counts without values. `raw` checks syntax only: a bad quoted value escape can
pass that mode. Neither mode checks supported keys, type conversions, required
fields or other application rules. Run the target application's relevant loader
and schema checks as well. If Node is unavailable, use its existing dotReg
implementation and report exactly which validation was possible.

Use `--json` only when entry contents are needed; it prints raw values and, in
the default `strings` mode, decoded strings. The checker also accepts standard
input as `-`. It exits nonzero without partial standard output on failure.

When reporting a change, identify the affected configuration or loader,
application decisions and checks performed. Distinguish syntax success from
successful conversion and application validation.

## Installation and source

For Codex project discovery, copy this whole `dotreg` folder into the target
project's `.agents/skills/`, then invoke `$dotreg` or request dotReg work. Other
Agent Skills hosts can use the same `SKILL.md`; follow their discovery rules.
`agents/openai.yaml` supplies optional Codex UI metadata.

[references/upstream.json](references/upstream.json) records the source revision
and hashes. [LICENSE](LICENSE) accompanies the bundled parser and example.
Read the matching upstream specification when changing the format or evaluating
an undecided case. Update the bundled source, hashes and guidance together.
