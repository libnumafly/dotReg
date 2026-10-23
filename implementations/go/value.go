package dotreg

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Text is the common string-decoding result, before any profile conversion.
// Unset distinguishes an empty right-hand side without a block from an
// explicitly empty string, including an empty block.
type Text struct {
	Value string
	Unset bool
}

// DecodeString removes value quotes and decodes the specified escapes inside
// them. Triple-quoted blocks use the same escapes; pipe blocks and bare values
// preserve backslashes. Block bodies preserve all whitespace. Entry must be a
// value entry with a consistent form and Unset flag.
func DecodeString(entry Entry) (Text, error) {
	if entry.Kind != Value {
		return Text{}, problem(entry.Line, ErrString)
	}
	if entry.Form != "" {
		if entry.Unset || !validBlockText(entry.Raw) {
			return Text{}, problem(entry.Line, ErrString)
		}
		switch entry.Form {
		case PipeBlock:
			return Text{Value: entry.Raw}, nil
		case TripleQuoted:
			value, err := decodeBlockEscapes(entry.Raw)
			if err != nil {
				return Text{}, problem(entry.Line, ErrString)
			}
			return Text{Value: value}, nil
		default:
			return Text{}, problem(entry.Line, ErrString)
		}
	}
	raw := strings.Trim(entry.Raw, " \t")
	if !utf8.ValidString(raw) || strings.ContainsAny(raw, "\r\n") || entry.Unset != (raw == "") {
		return Text{}, problem(entry.Line, ErrString)
	}
	if entry.Unset {
		return Text{Unset: true}, nil
	}
	if raw[0] != '"' {
		if strings.ContainsRune(raw, '"') {
			return Text{}, problem(entry.Line, ErrString)
		}
		return Text{Value: raw}, nil
	}
	value, end, err := decodeQuoted(raw)
	if err != nil || end != len(raw) {
		return Text{}, problem(entry.Line, ErrString)
	}
	return Text{Value: value}, nil
}

var errQuoted = errors.New("invalid quoted string")

// Block Raw has already had its structural markers and margin removed by the
// parser. Source CR is valid only as part of CRLF; decoded \r remains data.
func validBlockText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' && (i+1 == len(s) || s[i+1] != '\n') {
			return false
		}
	}
	return true
}

// Quotes and physical line breaks in a triple-quoted body are ordinary data.
// Escapes are decoded once, using exactly the ordinary quoted-string repertoire.
func decodeBlockEscapes(s string) (string, error) {
	escape := strings.IndexByte(s, '\\')
	if escape < 0 {
		return s, nil
	}
	var decoded strings.Builder
	for escape >= 0 {
		decoded.WriteString(s[:escape])
		value, end, err := decodeEscape(s, escape)
		if err != nil {
			return "", err
		}
		decoded.WriteRune(value)
		s = s[end:]
		escape = strings.IndexByte(s, '\\')
	}
	decoded.WriteString(s)
	return decoded.String(), nil
}

// decodeQuoted decodes a quoted element beginning at s[0], returning the byte
// index immediately after its closing quote. Its errors never contain input.
// Callers validate UTF-8 before calling and validate the remaining characters
// for their own key/value grammar.
func decodeQuoted(s string) (string, int, error) {
	var decoded strings.Builder
	end, err := scanQuoted(s, &decoded)
	if err != nil {
		return "", 0, err
	}
	return decoded.String(), end, nil
}

// scanQuoted validates the same quoted grammar with or without output.
// A nil builder locates the closing quote without allocating decoded text.
func scanQuoted(s string, decoded *strings.Builder) (int, error) {
	if len(s) == 0 || s[0] != '"' {
		return 0, errQuoted
	}
	literal := 1
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '"':
			if decoded != nil {
				decoded.WriteString(s[literal:i])
			}
			return i + 1, nil
		case '\r', '\n':
			return 0, errQuoted
		case '\\':
			value, end, err := decodeEscape(s, i)
			if err != nil {
				return 0, err
			}
			if decoded != nil {
				decoded.WriteString(s[literal:i])
				decoded.WriteRune(value)
			}
			literal = end
			i = end - 1
		}
	}
	return 0, errQuoted
}

// decodeEscape starts at a backslash and returns the scalar and the byte index
// after the escape. It never treats a decoded character as source syntax.
func decodeEscape(s string, start int) (rune, int, error) {
	if start+1 >= len(s) || s[start] != '\\' {
		return 0, 0, errQuoted
	}
	end := start + 2
	switch s[start+1] {
	case '.', '"', '\\', '/':
		return rune(s[start+1]), end, nil
	case 'b':
		return '\b', end, nil
	case 'f':
		return '\f', end, nil
	case 'n':
		return '\n', end, nil
	case 'r':
		return '\r', end, nil
	case 't':
		return '\t', end, nil
	case 'u':
		return decodeUnicodeEscape(s, end)
	default:
		return 0, 0, errQuoted
	}
}

// decodeUnicodeEscape starts immediately after the u and returns one scalar
// and the index after its encoding. Fixed-width high surrogates consume the
// immediately following fixed-width low surrogate as part of the same scalar.
func decodeUnicodeEscape(s string, start int) (rune, int, error) {
	if start >= len(s) {
		return 0, 0, errQuoted
	}
	if s[start] == '{' {
		var value rune
		digits := 0
		for i := start + 1; i < len(s); i++ {
			if s[i] == '}' {
				if digits == 0 || !utf8.ValidRune(value) {
					return 0, 0, errQuoted
				}
				return value, i + 1, nil
			}
			hex, ok := hexValue(s[i])
			if !ok || digits == 6 {
				return 0, 0, errQuoted
			}
			value = value*16 + hex
			digits++
		}
		return 0, 0, errQuoted
	}
	value, err := fourHex(s, start)
	if err != nil {
		return 0, 0, errQuoted
	}
	end := start + 4
	if value >= 0xD800 && value <= 0xDBFF {
		if len(s)-end < 6 || s[end] != '\\' || s[end+1] != 'u' {
			return 0, 0, errQuoted
		}
		low, err := fourHex(s, end+2)
		if err != nil || low < 0xDC00 || low > 0xDFFF {
			return 0, 0, errQuoted
		}
		return 0x10000 + (value-0xD800)*0x400 + (low - 0xDC00), end + 6, nil
	}
	if !utf8.ValidRune(value) {
		return 0, 0, errQuoted
	}
	return value, end, nil
}

func fourHex(s string, start int) (rune, error) {
	if len(s)-start < 4 {
		return 0, errQuoted
	}
	var value rune
	for i := start; i < start+4; i++ {
		hex, ok := hexValue(s[i])
		if !ok {
			return 0, errQuoted
		}
		value = value*16 + hex
	}
	return value, nil
}

func hexValue(c byte) (rune, bool) {
	switch {
	case c >= '0' && c <= '9':
		return rune(c - '0'), true
	case c >= 'a' && c <= 'f':
		return rune(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return rune(c-'A') + 10, true
	default:
		return 0, false
	}
}
