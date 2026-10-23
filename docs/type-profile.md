# Type profiles and candidate types

The [format specification](config-format.md) defines [type annotation syntax](config-format.md#6-type-annotations) and the [conversion and retrieval procedure](config-format.md#81-type-conversion-and-retrieval). Value decoders and profiles assign meanings and conversion rules to type expressions. This page records one established representation decision and an illustrative candidate catalogue; it does not define a complete standard type profile.

[Back to the documentation](README.md) · [Undecided matters](open-issues.md)

## Established base64 representation

For `base64`, the text form uses the standard Base64 alphabet and required padding in [RFC 4648 section 4](https://www.rfc-editor.org/rfc/rfc4648#section-4), rather than the URL-safe alphabet. Encoded text has no line wrapping, and decoding rejects non-alphabet characters, including whitespace, following the default rules in RFC 4648 sections 3.1–3.3. `SGVsbG8=` decodes to the five octets spelling `Hello` in ASCII.

This representation decision is established. It does not by itself make `base64` a standard permitted type name or a supported Go sample type. The remaining decisions about type names and profiles are listed in [Undecided matters](open-issues.md).

## Candidate type names and sample values

These names and right-hand-side values are an illustrative candidate set, not a declaration that the core format or the Go sample accepts these types. Except for the `base64` text representation described above, the samples do not define the accepted spelling or conversion rules for a type.

| Candidate type | Intended value | Sample right-hand side |
|----------------|---------------|-----------------------|
| `string` | Text | `hello` |
| `bool` | Boolean | `true` |
| `int32` | Signed 32-bit integer | `-42` |
| `int64` | Signed 64-bit integer | `-9000000000` |
| `uint32` | Unsigned 32-bit integer | `42` |
| `uint64` | Unsigned 64-bit integer | `9000000000` |
| `decimal` | Decimal number | `1234.50` |
| `float64` | 64-bit floating-point number | `0.125` |
| `base64` | Octets encoded as standard Base64 | `SGVsbG8=` |
| `duration` | Duration | `PT1H30M` |
| `date` | Calendar date | `2026-10-24` |
| `time` | Time of day | `09:30:00` |
| `datetime` | Date and time | `2026-10-24T09:30:00+09:00` |
| `uuid` | UUID | `550e8400-e29b-41d4-a716-446655440000` |
| `ip4` | IPv4 address | `192.0.2.10` |
| `ip6` | IPv6 address | `2001:db8::10` |
| `cidr4` | IPv4 network prefix | `192.0.2.0/24` |
| `cidr6` | IPv6 network prefix | `2001:db8::/32` |
| `uri` | URI | `https://example.com/items/42` |

The type names and examples for IP addresses and network prefixes distinguish IPv4 from IPv6; their complete accepted textual forms and conversion behaviour remain to be specified.
