// Package comparison benchmarks decoded configurations with identical semantics.
package comparison

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	dotreg "github.com/libnumafly/dotReg/implementations/go"
	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
	ini "gopkg.in/ini.v1"
)

// Config is the final application representation produced by every operation.
type Config map[string]map[string]string

type Case struct {
	Name       string `json:"name"`
	Sections   int    `json:"sections"`
	PerSection int    `json:"values_per_section"`
	Style      string `json:"style"`
}

var Cases = []Case{
	{"small", 4, 8, "ascii"},
	{"medium", 32, 16, "ascii"},
	{"large", 128, 64, "ascii"},
	{"unicode", 32, 16, "unicode"},
	{"escaped", 32, 16, "escaped"},
	{"multiline", 32, 16, "multiline"},
}

var Formats = []string{"dotreg", "json", "toml", "yaml", "ini"}
var Parsers = []string{"dotreg-direct", "dotreg-collected", "json", "toml", "yaml", "ini"}

// Expected constructs the oracle directly from the scenario, before encoding.
// It does not use any parser's output to define expected semantics.
func Expected(c Case) Config {
	result := make(Config)
	for section := 0; section < c.Sections; section++ {
		sectionName := fmt.Sprintf("Section%04d", section)
		if c.Style == "unicode" {
			sectionName = "地域" + sectionName
		}
		values := make(map[string]string)
		for key := 0; key < c.PerSection; key++ {
			keyName := fmt.Sprintf("Key%04d", key)
			value := fmt.Sprintf("value-%04d-%04d-abcdefghijklmnop", section, key)
			switch c.Style {
			case "unicode":
				keyName = "項目" + keyName
				value = fmt.Sprintf("東京・大阪 café が が 😀 %04d-%04d", section, key)
			case "escaped":
				value = fmt.Sprintf(` path C:\config\%04d "quoted" #tag; x=y`+"\ttab %04d ", section, key)
			case "multiline":
				value = fmt.Sprintf("first line %04d\n  second line: %04d\nthird line 日本語", section, key)
			}
			values[keyName] = value
		}
		result[sectionName] = values
	}
	return result
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// quoted is the JSON/TOML/YAML/dotReg common double-quoted string subset.
// Unicode stays UTF-8; only syntax/control characters need escaping.
func quoted(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&out, `\u%04x`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

func tomlName(value string) string {
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return quoted(value)
		}
	}
	return value
}

// Restrict plain YAML to known string tokens: a letter first, no special
// syntax, no surrounding whitespace, and none of the legacy resolver words.
func yamlString(value string) string {
	first, _ := utf8.DecodeRuneInString(value)
	reserved := map[string]bool{"true": true, "false": true, "null": true, "yes": true, "no": true, "on": true, "off": true, "y": true, "n": true}
	if !unicode.IsLetter(first) || reserved[strings.ToLower(value)] || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\n\r\t:#{}[],&*?!%@`\"'\\") {
		return quoted(value)
	}
	return value
}

// Encode deliberately spells out each serialisation. Encoding is never timed.
// No format has comments, BOM, alignment padding or a final blank section.
func Encode(format string, expected Config) ([]byte, error) {
	var out strings.Builder
	sections := sortedKeys(expected)
	if format == "dotreg" {
		out.WriteString("@dotReg 1\n\n")
	} else if format == "json" {
		out.WriteString("{\n")
	}
	for si, section := range sections {
		values := expected[section]
		keys := sortedKeys(values)
		switch format {
		case "dotreg":
			out.WriteString(section + ".\n")
		case "json":
			out.WriteString("  " + quoted(section) + ": {\n")
		case "toml":
			out.WriteString("[" + tomlName(section) + "]\n")
		case "ini":
			out.WriteString("[" + section + "]\n")
		case "yaml":
			out.WriteString(yamlString(section) + ":\n")
		default:
			return nil, fmt.Errorf("unknown format %q", format)
		}
		for ki, key := range keys {
			value := values[key]
			switch format {
			case "dotreg":
				out.WriteString("  ." + key + ":")
				if strings.ContainsRune(value, '\n') {
					// Native literal pipe block; common-map output contains LF.
					out.WriteByte('\n')
					for _, line := range strings.Split(value, "\n") {
						out.WriteString("    |" + line + "\n")
					}
				} else if value == "" || strings.Trim(value, " \t") != value || strings.HasPrefix(value, "\"") || strings.ContainsRune(value, '\t') {
					out.WriteString(quoted(value) + "\n")
				} else {
					out.WriteString(value + "\n")
				}
			case "json":
				out.WriteString("    " + quoted(key) + ": " + quoted(value))
				if ki+1 < len(keys) {
					out.WriteByte(',')
				}
				out.WriteByte('\n')
			case "toml":
				out.WriteString(tomlName(key) + " = ")
				if strings.ContainsRune(value, '\n') {
					if strings.Contains(value, "'''") {
						return nil, fmt.Errorf("TOML fixture encoder excludes triple apostrophes")
					}
					out.WriteString("'''\n" + value + "'''\n")
				} else {
					out.WriteString(quoted(value) + "\n")
				}
			case "yaml":
				out.WriteString("  " + yamlString(key) + ": ")
				if strings.ContainsRune(value, '\n') {
					if strings.HasSuffix(value, "\n") {
						return nil, fmt.Errorf("YAML fixture encoder excludes trailing multiline LF")
					}
					out.WriteString("|-\n")
					for _, line := range strings.Split(value, "\n") {
						out.WriteString("    " + line + "\n")
					}
				} else {
					out.WriteString(yamlString(value) + "\n")
				}
			case "ini":
				if strings.Contains(value, `"""`) {
					return nil, fmt.Errorf("INI fixture encoder excludes triple quote sequences")
				}
				out.WriteString(key + " = ")
				if value == "" {
					out.WriteString(`""`)
				} else if strings.ContainsAny(value, "\n\"\\#;\t") || strings.Trim(value, " \t") != value {
					// go-ini's native triple quotes preserve literal text, including LF.
					out.WriteString(`"""` + value + `"""`)
				} else {
					out.WriteString(value)
				}
				out.WriteByte('\n')
			}
		}
		if format == "json" {
			out.WriteString("  }")
			if si+1 < len(sections) {
				out.WriteByte(',')
			}
			out.WriteByte('\n')
		}
	}
	if format == "json" {
		out.WriteString("}\n")
	}
	return []byte(out.String()), nil
}

// Parse includes all parsing, string decoding, and construction of Config.
// No parser receives the fixture oracle's counts or map capacity hints.
func Parse(parser string, data []byte) (Config, error) {
	var result Config
	switch parser {
	case "json":
		err := json.Unmarshal(data, &result)
		return result, err
	case "toml":
		err := toml.Unmarshal(data, &result)
		return result, err
	case "yaml":
		err := yaml.Unmarshal(data, &result)
		return result, err
	case "ini":
		// Triple quotes are literal; avoid interpolation via Key.String().
		file, err := ini.LoadSources(ini.LoadOptions{
			IgnoreContinuation:  true,
			IgnoreInlineComment: true,
			KeyValueDelimiters:  "=",
		}, data)
		if err != nil {
			return nil, err
		}
		result = make(Config)
		for _, section := range file.Sections() {
			if section.Name() == ini.DefaultSection && len(section.Keys()) == 0 {
				continue
			}
			values := make(map[string]string)
			for _, key := range section.Keys() {
				values[key.Name()] = key.Value()
			}
			result[section.Name()] = values
		}
		return result, nil
	case "dotreg-direct", "dotreg-collected":
		result = make(Config)
		add := func(e dotreg.Entry) error {
			if e.Kind == dotreg.Heading {
				return nil
			}
			if len(e.Path) != 2 || e.Unset || e.HasType {
				return fmt.Errorf("fixture entry falls outside common two-level string model")
			}
			value, err := dotreg.DecodeString(e)
			if err != nil {
				return err
			}
			values := result[e.Path[0]]
			if values == nil {
				values = make(map[string]string)
				result[e.Path[0]] = values
			}
			values[e.Path[1]] = value.Value
			return nil
		}
		if parser == "dotreg-direct" {
			err := dotreg.ParseEach(bytes.NewReader(data), add)
			return result, err
		}
		entries, err := dotreg.Parse(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err := add(entry); err != nil {
				return nil, err
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown parser %q", parser)
	}
}

func FormatForParser(parser string) string {
	if strings.HasPrefix(parser, "dotreg-") {
		return "dotreg"
	}
	return parser
}

func Fingerprint(config Config) string {
	// encoding/json sorts string map keys; decoded strings retain exact bytes.
	data, err := json.Marshal(config)
	if err != nil {
		panic(err)
	}
	return hash(data)
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func LogicalBytes(config Config) int {
	total := 0
	for section, values := range config {
		for key, value := range values {
			total += len(section) + len(key) + len(value)
		}
	}
	return total
}

func Validate(parser string, data []byte, expected Config) error {
	actual, err := Parse(parser, data)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("semantic mismatch: actual %s expected %s", Fingerprint(actual), Fingerprint(expected))
	}
	return nil
}

type FileRecord struct {
	Path         string `json:"path"`
	EncodedBytes int    `json:"encoded_bytes"`
	SHA256       string `json:"sha256"`
}

type CaseRecord struct {
	Case
	Values         int                   `json:"values"`
	LogicalBytes   int                   `json:"logical_payload_bytes"`
	SemanticSHA256 string                `json:"semantic_sha256"`
	Files          map[string]FileRecord `json:"files"`
}

var Extensions = map[string]string{"dotreg": "dotreg", "json": "json", "toml": "toml", "yaml": "yaml", "ini": "ini"}

// WriteFixtures validates every adapter against the independently built oracle
// and writes bytes plus hashes to make the exact workload inspectable.
func WriteFixtures(directory string) error {
	manifest := struct {
		Version int          `json:"generator_version"`
		Cases   []CaseRecord `json:"cases"`
	}{Version: 1}
	for _, c := range Cases {
		expected := Expected(c)
		record := CaseRecord{Case: c, Values: c.Sections * c.PerSection, LogicalBytes: LogicalBytes(expected), SemanticSHA256: Fingerprint(expected), Files: make(map[string]FileRecord)}
		caseDirectory := filepath.Join(directory, c.Name)
		if err := os.MkdirAll(caseDirectory, 0755); err != nil {
			return err
		}
		for _, format := range Formats {
			data, err := Encode(format, expected)
			if err != nil {
				return err
			}
			for _, parser := range Parsers {
				if FormatForParser(parser) == format {
					if err := Validate(parser, data, expected); err != nil {
						return fmt.Errorf("%s/%s: %w", c.Name, parser, err)
					}
				}
			}
			name := "config." + Extensions[format]
			if err := os.WriteFile(filepath.Join(caseDirectory, name), data, 0644); err != nil {
				return err
			}
			record.Files[format] = FileRecord{Path: c.Name + "/" + name, EncodedBytes: len(data), SHA256: hash(data)}
		}
		manifest.Cases = append(manifest.Cases, record)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "manifest.json"), append(data, '\n'), 0644)
}

func FindCase(name string) (Case, error) {
	for _, c := range Cases {
		if c.Name == name {
			return c, nil
		}
	}
	return Case{}, fmt.Errorf("unknown case %s; available %s", strconv.Quote(name), "small, medium, large, unicode, escaped, multiline")
}
