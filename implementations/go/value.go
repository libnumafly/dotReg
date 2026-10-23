package dotreg

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Text is the common string-decoding result, before any profile conversion.
// Unset distinguishes an empty right-hand side from an explicitly quoted "".
type Text struct {
	Value string
	Unset bool
}

// DecodeString removes value quotes and decodes the specified escapes inside
// them. Bare values preserve backslashes. Entry must be a value entry, and its
// Unset flag must agree with the trimmed raw right-hand side.
func DecodeString(entry Entry) (Text, error) {
	if entry.Kind != Value {
		return Text{}, problem(entry.Line, ErrString)
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

// decodeQuoted decodes a quoted element beginning at s[0], returning the byte
// index immediately after its closing quote. Its errors never contain input.
// Callers validate UTF-8 before calling and validate the remaining characters
// for their own key/value grammar.
func decodeQuoted(s string) (string, int, error) {
	if len(s) == 0 || s[0] != '"' {
		return "", 0, errQuoted
	}
	var decoded strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '"':
			return decoded.String(), i + 1, nil
		case '\r', '\n':
			return "", 0, errQuoted
		case '\\':
			i++
			if i >= len(s) {
				return "", 0, errQuoted
			}
			switch s[i] {
			case '.', '"', '\\', '/':
				decoded.WriteByte(s[i])
			case 'b':
				decoded.WriteByte('\b')
			case 'f':
				decoded.WriteByte('\f')
			case 'n':
				decoded.WriteByte('\n')
			case 'r':
				decoded.WriteByte('\r')
			case 't':
				decoded.WriteByte('\t')
			case 'u':
				value, end, err := decodeUnicodeEscape(s, i+1)
				if err != nil {
					return "", 0, errQuoted
				}
				decoded.WriteRune(value)
				i = end - 1
			default:
				return "", 0, errQuoted
			}
		default:
			decoded.WriteByte(s[i])
		}
	}
	return "", 0, errQuoted
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
