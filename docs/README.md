# dotReg documentation

dotReg is a line-oriented configuration notation built around dotted key paths, with explicit triple-quoted and literal pipe forms for multiline values. Start with the short guide, then use the specification when you need the precise rules.

[Back to the project](../README.md)

## Find your way

| Document | Read it to… |
|---|---|
| [Getting started](getting-started.md) | Learn the required file declaration, value lines, repeated values, headings, strings and unset values through small examples |
| [Configuration file format](config-format.md) | Consult the reference specification for syntax, processing stages and application responsibilities |
| [Type profiles and candidate types](type-profile.md) | Look up established type-specific representations and distinguish them from candidate type names and examples |
| [Conformance checklist](conformance.md) | Check an implementation's parser against the established format rules |
| [Undecided matters](open-issues.md) | Check which parts of the format remain open |
| [Example configuration](../examples/sample.txt) | Explore repeated values, quoted names, escapes, both multiline forms and the Go sample's experimental type annotations |
| [Go sample guide](../implementations/go/README.md) | Run the optional parser and CLI, use its API, or inspect its supported scope and provisional choices |
| [Sample parser set](../implementations/README.md) | Choose Go, Python, JavaScript, TypeScript, Java, C#, Rust or C++; compare the shared syntax contract and streaming APIs |
| [Time and memory measurements](../implementations/benchmarks/README.md) | Reproduce native API measurements and distinguish allocation volume from process memory |

## Project status

**The notation is still being developed.** The core specification defines the established format rules, the type-profile document records type-specific representations and candidates, and the open-issues document lists undecided matters. The getting started guide introduces the core rules without replacing them.

The current format version is declared by the mandatory `@dotReg 1` property region and blank separator. Future version policy, additional properties, a dedicated file extension and standard type names remain undecided. See [file properties](config-format.md#11-file-property-declarations) and [undecided matters](open-issues.md).

The sample implementations are for trying the notation. Their public APIs are provisional. The new ports cover syntax and common string decoding, retaining type expressions without conversion. The Go sample additionally provides an experimental type profile and retrieval API. In particular, the `string`, `int` and `bool` annotations in the example configuration do not establish standard type names or conversion rules.

## How the documents fit together

The [core specification](config-format.md) defines the notation and separates syntax processing, common string decoding, type conversion and application-specific validation. [Type profiles and candidate types](type-profile.md) supplements it with type-specific representations and clearly labelled candidates. [Undecided matters](open-issues.md) records the remaining questions; it does not add format rules. The [Go guide](../implementations/go/README.md) describes one implementation of those stages and the temporary choices it makes where the format remains open.

For a first read, follow [Getting started](getting-started.md). For a particular rule, use the specification sections below. Implementers can also check their parser against the [conformance checklist](conformance.md):

- [File properties and format version](config-format.md#11-file-property-declarations).
- [Key paths and quoting](config-format.md#3-key-paths).
- [Headings, relative notation and indentation](config-format.md#5-headings-relative-notation-and-indentation).
- [Type annotations](config-format.md#6-type-annotations).
- [String decoding and unset values](config-format.md#8-preserving-right-hand-sides-and-interpreting-values).
- [Repeated values and read state](config-format.md#9-repeated-values-and-read-state).
- [Triple-quoted multiline values](config-format.md#83-triple-quoted-multiline-values) and [literal pipe blocks](config-format.md#84-literal-pipe-blocks).
- [Processing and application responsibilities](config-format.md#11-responsibilities-of-syntax-processing-and-consuming-applications).

## Repository layout

```text
.
├── README.md              # Project introduction
├── docs/                  # Guides and reference specification
├── examples/              # Configuration inputs
├── implementations/
│   ├── README.md          # Parser set and common contract
│   ├── go/                # Go sample and experimental type consumer
│   ├── python/            # Python sample
│   ├── javascript/        # JavaScript engine
│   ├── typescript/        # Typed interface to the JavaScript engine
│   ├── java/              # Java sample
│   ├── csharp/            # C# sample
│   ├── rust/              # Rust sample
│   ├── cpp/               # C++ sample
│   ├── conformance/       # Shared syntax/decoding fixtures and runner
│   └── benchmarks/        # Common fixture, measurement tools and observations
├── .github/               # CI and GitHub configuration
└── LICENSE                # MIT licence
```

## Licence

The specification, examples and sample implementations are available under the [MIT License](../LICENSE).

You may choose your own licence for configuration files you author with this notation and for independently written implementations. Copies of material from this repository remain subject to the MIT terms.
