# dotReg

**Readable configuration, built on dotted keys.**

dotReg is a line-oriented configuration notation. Write full keys directly, or group them beneath a shared heading. Values start as text; your application decides how to interpret them.

```text
A.
 .Code:001
 .Message: "Hello, dotReg"
 .Group.
  .Enabled:true
```

The grouped entries resolve to `A.Code`, `A.Message` and `A.Group.Enabled`. The code `001` stays text, with its leading zeroes intact.

[Get started](docs/getting-started.md) · [Documentation](docs/README.md) · [Specification](docs/config-format.md) · [Go sample](implementations/go/README.md)

**In development.** The specification distinguishes agreed rules from [open questions](docs/config-format.md#12-undecided-matters).

The specification, examples and sample implementation are available under the [MIT License](LICENSE).
