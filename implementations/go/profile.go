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

// Config contains decoded values with their original unset state and optional
// declared type. It is immutable after Build; custom converted values are owned
// by the application and should themselves be treated as immutable.
type Config struct {
	profile Profile
	values  map[string]setting
}

type setting struct {
	line     int
	text     Text
	typeName string
	hasType  bool
	value    any
}

// Build decodes all value entries, applying only their explicitly declared
// types. Untyped values stay strings until Get requests a conversion. Heading
// entries do not create settings. Failure returns no partial Config.
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
		if _, exists := cfg.values[key]; exists {
			return nil, problem(entry.Line, ErrDuplicate)
		}
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
		cfg.values[key] = item
	}
	return cfg, nil
}

// Get requests a particular profile type. A declared type must match want
// before any conversion or unset-to-empty interpretation. Undeclared values
// are converted on demand; failed conversions are type mismatches. Successful
// on-demand conversions are not cached or assigned as a declared type.
func (cfg *Config) Get(path []string, want string) (any, error) {
	item, ok := cfg.lookup(path)
	if !ok {
		return nil, problem(0, ErrMissing)
	}
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

// IsUnset distinguishes a missing value from an explicitly quoted empty string.
// It reports the state only and does not request or convert a type.
func (cfg *Config) IsUnset(path []string) (bool, error) {
	item, ok := cfg.lookup(path)
	if !ok {
		return false, problem(0, ErrMissing)
	}
	return item.text.Unset, nil
}

func (cfg *Config) lookup(path []string) (setting, bool) {
	if cfg == nil || len(path) == 0 {
		return setting{}, false
	}
	item, ok := cfg.values[pathID(path)]
	return item, ok
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
