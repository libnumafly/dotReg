# Use dotReg with coding agents

The repository includes a portable `dotreg` skill for developing applications
that read or write dotReg configuration. It gives a coding agent the notation's
rules, application-integration guidance, a complete example and an offline
syntax/string checker.

[Back to the documentation](README.md) · [Skill entry point](../.agents/skills/dotreg/SKILL.md)

## What the skill helps with

Use it when asking an agent to:

- Create or edit `.dotreg` configuration while preserving resolved keys and text.
- Add a configuration loader using an existing dotReg implementation.
- Define application-specific keys, cardinality, types and validation.
- Convert existing settings to dotReg without losing repeated values, empty
  names, leading zeroes or the distinction between unset and empty strings.

The skill separates syntax parsing, common string decoding, type conversion
and application-schema validation. It preserves repeated identical values in
source order and distinguishes the current rules from provisional type names
and sample APIs. Changes to the notation or parser internals should follow the
[specification](config-format.md) and implementation workflow in this repository.

## Files to install

Copy the complete [`.agents/skills/dotreg/` folder](../.agents/skills/dotreg/SKILL.md).
Its files use relative paths, so keep this structure:

```text
dotreg/
├── SKILL.md
├── agents/
│   └── openai.yaml
├── references/
│   ├── syntax.md
│   ├── integration.md
│   └── upstream.json
├── scripts/
│   ├── validate.mjs
│   └── lib/
│       └── dotreg.mjs
├── assets/
│   └── example.dotreg
└── LICENSE
```

`SKILL.md` is the entry point. The references are loaded when relevant, and
`agents/openai.yaml` supplies optional Codex UI metadata. The checker uses the
bundled JavaScript parser; copying only `SKILL.md` would leave its references,
example and checker unavailable.

Reading the instructions and references needs no runtime. The bundled checker
requires **Node.js 20 or later**, uses only built-in modules and needs no package
installation or network access. Your application's chosen parser may have
different language requirements; see the [sample parser set](../implementations/README.md).

## Install in a Codex project

Place the folder at `<application>/.agents/skills/dotreg/`. Codex discovers
repository skills in `.agents/skills`; see the
[official Codex skill documentation](https://learn.chatgpt.com/docs/build-skills)
for discovery and configuration details.

The following examples copy the skill from a separate dotReg checkout. Replace
the source and application paths before running them. Choose a destination that
does not already contain a `dotreg` skill; the update procedure is described below.

On Windows, use PowerShell:

```powershell
$dotregCheckout = 'C:\src\dotReg'
$applicationRoot = 'C:\src\my-app'

git clone https://github.com/libnumafly/dotReg.git $dotregCheckout
New-Item -ItemType Directory -Path (Join-Path $applicationRoot '.agents\skills') -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $dotregCheckout '.agents\skills\dotreg') -Destination (Join-Path $applicationRoot '.agents\skills') -Recurse
```

On a POSIX shell:

```sh
git clone https://github.com/libnumafly/dotReg.git /path/to/dotreg-source
mkdir -p /path/to/my-app/.agents/skills
cp -R /path/to/dotreg-source/.agents/skills/dotreg /path/to/my-app/.agents/skills/
```

If you already have a dotReg checkout, use it as the source and omit the clone.
You can commit the copied folder with your application so collaborators use the
same instructions and checker. Keep its MIT licence when copying the material.

From the application root, check the installation with the bundled example:

```sh
node .agents/skills/dotreg/scripts/validate.mjs .agents/skills/dotreg/assets/example.dotreg
```

The bundled example reports two headings and thirty value occurrences. This
checks that the copied scripts and parser work together; it does not test the
application's configuration schema or confirm agent discovery.

## Invoke the skill

In Codex, explicitly mention `$dotreg` in your request. For example:

```text
Use $dotreg to add dotReg configuration loading to this application.
Keep codes such as 001 as text, retain repeated tags in source order,
and validate the application's supported keys before applying settings.
```

For an existing configuration:

```text
Use $dotreg to edit settings.dotreg for this change.
Preserve existing resolved keys and repeated values, and run the bundled
checker plus the application's existing configuration validation.
```

The skill also allows automatic selection for matching dotReg requests. If Codex
does not show it, check the copied path and the `name: dotreg` frontmatter in
`SKILL.md`, then restart Codex if discovery has not refreshed. Other hosts that
support Agent Skills can use the same folder; follow that host's installation
location and invocation rules. `agents/openai.yaml` is Codex-specific metadata.

## Validate configuration files

Run these commands from the application root, replacing the configuration path:

```sh
node .agents/skills/dotreg/scripts/validate.mjs config/settings.dotreg
node .agents/skills/dotreg/scripts/validate.mjs --mode raw config/settings.dotreg
node .agents/skills/dotreg/scripts/validate.mjs --json config/settings.dotreg
```

| Option | Checks and output |
|---|---|
| Default, or `--mode strings` | Validates the whole syntax and common string decoding; prints heading/value counts without setting contents |
| `--mode raw` | Validates syntax only; an invalid escape in a quoted value can still pass |
| `--json` | Prints syntax entries and raw values; in `strings` mode also includes decoded strings |
| `-`, or no filename | Reads UTF-8 standard input |
| `--help` | Shows command syntax and scope |

Failures produce a non-zero exit status and no partial standard output.
Syntax/string diagnostics give the physical line and error category without
right-hand-side contents. `--json` deliberately exposes entry contents, so use
the default output when counts are sufficient.

A successful check does **not** validate supported keys, required fields, type
conversion, value ranges or other application rules. The example's `string`,
`int` and `bool` annotations illustrate the Go demo profile; the checker retains
them without converting their values. Define and test the consuming
application's profile and schema separately. The
[integration reference](../.agents/skills/dotreg/references/integration.md)
describes these responsibilities and the available sample APIs.

## Update and verify the installed copy

Record the source checkout's commit when adopting or updating the skill. Review
local changes to your installed copy, then refresh its files together from the
chosen revision. Mixing old instructions with a newer parser can give an agent
the wrong rules.

[`references/upstream.json`](../.agents/skills/dotreg/references/upstream.json)
records the revision and SHA-256 hashes used for the bundled parser, example,
licence and supporting source documents. That is the skill's source snapshot;
the commit from which you install the skill can be newer. After an update, rerun
the installation check above and your application's relevant loader/schema
checks.

The skill summarises the supported dotReg 0.1 rules. The
[format specification](config-format.md),
[type profiles](type-profile.md) and [undecided matters](open-issues.md) remain
the sources for format decisions and their limits.
