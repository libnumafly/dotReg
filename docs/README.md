# dotReg documentation

dotReg is a line-oriented configuration notation built around dotted key paths. Start with the short guide, then use the specification when you need the precise rules.

[Back to the project](../README.md)

## Find your way

| Document | Read it to… |
|---|---|
| [Getting started](getting-started.md) | Learn value lines, headings, strings and unset values through small examples |
| [Configuration file format](config-format.md) | Consult the reference specification, including syntax, processing responsibilities and conformance checks |
| [Undecided matters](config-format.md#12-undecided-matters) | Check which parts of the format remain open |
| [Example configuration](../examples/sample.txt) | Explore a larger input with quoted names, escapes and the Go sample's experimental type annotations |
| [Go sample guide](../implementations/go/README.md) | Run the optional parser and CLI, use its API, or inspect its supported scope and provisional choices |

## Project status

**The notation is still being developed.** The specification distinguishes established rules from undecided matters; it is the reference for the format. The getting started guide introduces those rules without replacing them.

The format version, dedicated file extension and standard type names remain undecided. See [section 12 of the specification](config-format.md#12-undecided-matters) for the full list.

The Go implementation is a sample for trying the notation. Its public API and experimental type profile are provisional. In particular, the `string`, `int` and `bool` annotations in the example configuration do not establish standard type names or conversion rules.

## How the documents fit together

The [specification](config-format.md) defines the notation and separates syntax processing, common string decoding, type conversion and application-specific validation. The [Go guide](../implementations/go/README.md) describes one implementation of those stages and the temporary choices it makes where the format remains open.

For a first read, follow [Getting started](getting-started.md). For a particular rule, the specification covers:

- [Key paths and quoting](config-format.md#3-key-paths).
- [Headings, relative notation and indentation](config-format.md#5-headings-relative-notation-and-indentation).
- [Type annotations](config-format.md#6-type-annotations).
- [String decoding and unset values](config-format.md#8-preserving-right-hand-sides-and-interpreting-values).
- [Processing and application responsibilities](config-format.md#11-responsibilities-of-syntax-processing-and-consuming-applications).

## Repository layout

```text
.
├── README.md              # Project introduction
├── docs/                  # Guides and reference specification
├── examples/              # Configuration inputs
├── implementations/
│   └── go/                # Optional Go sample, CLI, tests and guide
├── .github/               # CI and GitHub configuration
└── LICENSE                # MIT licence
```

## Licence

The specification, examples and sample implementation are available under the [MIT License](../LICENSE).

You may choose your own licence for configuration files you author with this notation and for independently written implementations. Copies of material from this repository remain subject to the MIT terms.
