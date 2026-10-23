package dotreg

import (
	"errors"
	"strconv"
)

// Converter interprets the already decoded string and its unset state. Its
// errors are replaced with a diagnostic that does not include the source value.
type Converter func(Text) (any, error)

// Profile supplies application-defined type names and conversions. StringType
// names the string type, whose conversion is always the decoded string itself.
// Types contains only other types. Type identity is exact string equality in
// this sample; aliases and compound expressions are not inferred.
type Profile struct {
	StringType string
	Types      map[string]Converter
}

// DemoProfile is an illustrative profile, not the notation's standard type set.
// int accepts signed decimal digits and returns int64. bool accepts exactly
// true or false. Both reject unset values. No environment expansion occurs.
func DemoProfile() Profile {
	return Profile{
		StringType: "string",
		Types: map[string]Converter{
			"int": func(text Text) (any, error) {
				if text.Unset || !decimal(text.Value) {
					return nil, errConversion
				}
				return strconv.ParseInt(text.Value, 10, 64)
			},
			"bool": func(text Text) (any, error) {
				if !text.Unset {
					switch text.Value {
					case "true":
						return true, nil
					case "false":
						return false, nil
					}
				}
				return nil, errConversion
			},
		},
	}
}

// Config retains every occurrence in source order for each key, including equal
// values, with its unset state and optional declared type. It is immutable after
// Build; custom converted values are owned by the application and should
// themselves be treated as immutable.
type Config struct {
	profile Profile
	values  map[string]setting
	more    map[string][]setting
}

type setting struct {
	line     int
	text     Text
	typeName string
	hasType  bool
	value    any
}

// Build decodes all value entries, applying only their explicitly declared
// types. Untyped values stay strings until retrieval requests a conversion.
// Repeated keys append independent occurrences without deduplication or
// overwriting. Heading entries do not create settings. Failure returns no
// partial Config.
func Build(entries []Entry, profile Profile) (*Config, error) {
	if _, exists := profile.Types[profile.StringType]; profile.StringType == "" || exists {
		return nil, problem(0, ErrInput)
	}
	copyProfile := Profile{StringType: profile.StringType, Types: make(map[string]Converter, len(profile.Types))}
	for name, converter := range profile.Types {
		if name == "" || converter == nil {
			return nil, problem(0, ErrInput)
		}
		copyProfile.Types[name] = converter
	}
	cfg := &Config{profile: copyProfile, values: make(map[string]setting)}
	for _, entry := range entries {
		if entry.Kind == Heading {
			continue
		}
		if entry.Kind != Value || len(entry.Path) == 0 {
			return nil, problem(entry.Line, ErrInput)
		}
		key := pathID(entry.Path)
		text, err := DecodeString(entry)
		if err != nil {
			return nil, err
		}
		item := setting{line: entry.Line, text: text, typeName: entry.Type, hasType: entry.HasType}
		if entry.HasType {
			item.value, err = cfg.convert(text, entry.Type)
			if err != nil {
				return nil, problem(entry.Line, ErrTypeConversion)
			}
		}
		if _, exists := cfg.values[key]; exists {
			if cfg.more == nil {
				cfg.more = make(map[string][]setting)
			}
			cfg.more[key] = append(cfg.more[key], item)
		} else {
			cfg.values[key] = item
		}
	}
	return cfg, nil
}

// Get requests a particular profile type for a key with exactly one occurrence.
// Multiple occurrences return ErrMultipleValues at the second header, before
// any retrieval conversion. Use GetAll to retrieve repeated values.
func (cfg *Config) Get(path []string, want string) (any, error) {
	first, more, ok := cfg.lookup(path)
	if !ok {
		return nil, problem(0, ErrMissing)
	}
	if len(more) > 0 {
		return nil, problem(more[0].line, ErrMultipleValues)
	}
	return cfg.get(first, want)
}

// GetAll retrieves every occurrence in source order. Each declaration must
// match want; untyped occurrences convert on demand without caching. A failure
// reports that occurrence's header and returns no partial values.
func (cfg *Config) GetAll(path []string, want string) ([]any, error) {
	first, more, ok := cfg.lookup(path)
	if !ok {
		return nil, problem(0, ErrMissing)
	}
	values := make([]any, 1+len(more))
	for i := range values {
		item := first
		if i > 0 {
			item = more[i-1]
		}
		value, err := cfg.get(item, want)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

// GetAt retrieves one zero-based occurrence using that occurrence's declaration
// and conversion rules. Missing keys and out-of-range indices return ErrMissing.
func (cfg *Config) GetAt(path []string, index int, want string) (any, error) {
	first, more, ok := cfg.lookup(path)
	if !ok || index < 0 || index > len(more) {
		return nil, problem(0, ErrMissing)
	}
	if index == 0 {
		return cfg.get(first, want)
	}
	return cfg.get(more[index-1], want)
}

// A declaration is checked before conversion or unset-to-empty interpretation.
// Retrieval conversion never changes the stored string or declaration.
func (cfg *Config) get(item setting, want string) (any, error) {
	if item.hasType {
		if item.typeName != want {
			return nil, problem(item.line, ErrTypeMismatch)
		}
		return item.value, nil
	}
	value, err := cfg.convert(item.text, want)
	if err != nil {
		return nil, problem(item.line, ErrTypeMismatch)
	}
	return value, nil
}

// IsUnset reports the state for a key with exactly one occurrence. It does not
// request or convert a type. Repeated keys return ErrMultipleValues.
func (cfg *Config) IsUnset(path []string) (bool, error) {
	first, more, ok := cfg.lookup(path)
	if !ok {
		return false, problem(0, ErrMissing)
	}
	if len(more) > 0 {
		return false, problem(more[0].line, ErrMultipleValues)
	}
	return first.text.Unset, nil
}

// IsUnsetAll returns each occurrence's original unset state in source order.
// The returned slice is independent of the Config and of other calls.
func (cfg *Config) IsUnsetAll(path []string) ([]bool, error) {
	first, more, ok := cfg.lookup(path)
	if !ok {
		return nil, problem(0, ErrMissing)
	}
	states := make([]bool, 1+len(more))
	states[0] = first.text.Unset
	for i, item := range more {
		states[i+1] = item.text.Unset
	}
	return states, nil
}

func (cfg *Config) lookup(path []string) (setting, []setting, bool) {
	if cfg == nil || len(path) == 0 {
		return setting{}, nil, false
	}
	key := pathID(path)
	first, ok := cfg.values[key]
	return first, cfg.more[key], ok
}

func (cfg *Config) convert(text Text, want string) (any, error) {
	if want == cfg.profile.StringType {
		return text.Value, nil
	}
	converter, exists := cfg.profile.Types[want]
	if !exists {
		return nil, errConversion
	}
	return converter(text)
}

// decimal deliberately excludes hexadecimal, underscores, and other Go syntax.
func decimal(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '+' || value[0] == '-' {
		value = value[1:]
	}
	if value == "" {
		return false
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

// The underlying converter error is never included in public diagnostics.
var errConversion = errors.New("value cannot be converted")
