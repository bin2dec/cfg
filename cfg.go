// Package cfg populates configuration structures from environment variables and JSON/YAML files.
// It also provides secret types (Secret and SecretURL) that keep sensitive data redacted in logs and formatted output.
package cfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const envTagName = "env"

// FromEnv populates the given config struct from environment variables.
// The config must be a pointer to a struct; otherwise, ErrInvalidConfigType is returned.
//
// A field is populated only if it carries an `env` tag (see envTagName) and its variable is set;
// the tag value, upper-cased, names the environment variable to read, so `env:"host"` is looked up as HOST,
// and a field whose variable is absent stays untouched. Nested structs, maps, and slices are traversed
// recursively, so tagged fields inside them are populated as well; the config values must form a tree,
// as cyclic structures are recursed into without a bound.
//
// A tagged field must be of a supported type:
//
//	bool, any width of signed or unsigned integer, float32, float64, string, time.Duration, Secret,
//	SecretURL, or a pointer to any of these, allocated when nil. Durations are parsed by time.ParseDuration,
//	so unit suffixes like "30s" are accepted, and numbers are parsed at the field's bit width.
//
// A tagged field of any other type results in an error wrapping ErrUnsupportedFieldType and naming
// the field and the type.
func FromEnv(config any) error {
	if err := checkType(config); err != nil {
		return err
	}
	return populateStructFromEnv(reflect.ValueOf(config))
}

// FromFile populates the given config struct from the contents of a JSON or YAML file.
// The format is selected by the file extension, case-insensitively: .json for JSON, .yaml/.yml for YAML;
// any other extension results in an error wrapping ErrUnknownFileExtension.
// Like FromEnv, it requires config to be a pointer to a struct.
func FromFile(p string, config any) error {
	switch ext := strings.ToLower(filepath.Ext(p)); ext {
	case ".json":
		return FromJSONFile(p, config)
	case ".yaml", ".yml":
		return FromYAMLFile(p, config)
	default:
		return fmt.Errorf("%w %q", ErrUnknownFileExtension, ext)
	}
}

// FromJSONFile populates the given config struct from the contents of the JSON file at path p,
// which must hold a single valid JSON document: content trailing the first value is an error.
// Like FromEnv, it requires config to be a pointer to a struct.
func FromJSONFile(p string, config any) error {
	if err := checkType(config); err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, config); err != nil {
		return fmt.Errorf("cfg: unmarshal JSON from %s: %w", p, err)
	}
	return nil
}

// FromYAMLFile populates the given config struct from the contents of the YAML file at path p,
// which must hold a single valid YAML document.
// Like FromEnv, it requires config to be a pointer to a struct.
func FromYAMLFile(p string, config any) error {
	if err := checkType(config); err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("cfg: unmarshal YAML from %s: %w", p, err)
	}
	return nil
}

// checkType verifies that config is a pointer to a struct, returning ErrInvalidConfigType otherwise.
func checkType(config any) error {
	if v := reflect.ValueOf(config); v.Kind() == reflect.Pointer && v.Elem().Kind() == reflect.Struct {
		return nil
	}
	return ErrInvalidConfigType
}

// populateStructFromEnv populates the given value with data from the environment.
// Nil pointers and non-struct values are ignored.
// Each exported field is visited: a field with an `env` tag whose variable is set is populated from it
// and traversed no further, while every other field is traversed recursively so nested structs, maps,
// and slices are populated too.
func populateStructFromEnv(s reflect.Value) error {
	if s.Kind() == reflect.Pointer {
		if s.IsNil() {
			return nil
		}
		s = s.Elem()
	}

	if s.Kind() != reflect.Struct {
		return nil
	}

	t := s.Type()

	n := s.NumField()
	for i := range n {
		fieldType := t.Field(i)

		if !fieldType.IsExported() {
			continue
		}

		field := s.Field(i)

		tagValue := fieldType.Tag.Get(envTagName)
		if tagValue != "" {
			envName := strings.ToUpper(tagValue)
			if envValue, ok := os.LookupEnv(envName); ok {
				if err := setFieldValue(field, envValue); err != nil {
					return fmt.Errorf("cfg: field %s (env %s): %w", fieldType.Name, envName, err)
				}
				// The field is fully populated from its variable; nested traversal would find nothing new.
				continue
			}
		}

		// Recursively populate nested structs, maps, and slices.
		if err := traverseValue(field); err != nil {
			return err
		}
	}

	return nil
}

// setFieldValue parses the string value according to the type of the field and assigns the result.
// Pointer fields are allocated when `nil` and their element is populated instead.
// Field types that cannot be populated from a single string result in an error wrapping
// ErrUnsupportedFieldType and naming the type.
func setFieldValue(field reflect.Value, value string) error {
	if !field.CanSet() {
		return nil
	}

	if field.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return setFieldValue(field.Elem(), value)
	}

	switch field.Kind() {
	case reflect.Bool:
		s, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(s)
	case reflect.Float32, reflect.Float64:
		s, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Durations are the one named integer type parsed from a unit-suffixed string rather than a plain number.
		if field.Type() == reflect.TypeFor[time.Duration]() {
			s, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.SetInt(int64(s))
			return nil
		}
		s, err := strconv.ParseInt(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(s)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s, err := strconv.ParseUint(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(s)
	case reflect.String:
		field.SetString(value)
	default:
		// Handle the package's own types, which wrap the raw string instead of parsing it.
		switch field.Type() {
		case reflect.TypeFor[Secret]():
			field.Set(reflect.ValueOf(NewSecret(value)))
		case reflect.TypeFor[SecretURL]():
			field.Set(reflect.ValueOf(NewSecretURL(value)))
		default:
			return fmt.Errorf("%w %s", ErrUnsupportedFieldType, field.Type())
		}
	}

	return nil
}

// traverseValue recurses into the given value, populating any structs found along the way:
// maps are walked pair by pair, slices and arrays element by element, and pointers are followed.
// Values of other kinds are ignored.
func traverseValue(value reflect.Value) error {
	switch value.Kind() {
	case reflect.Array, reflect.Slice:
		// Only slices are nil-checked, because reflect.Value.IsNil panics on arrays, which are never nil.
		if value.Kind() == reflect.Slice && value.IsNil() {
			return nil
		}
		return traverseSlice(value)
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		return traverseMap(value)
	case reflect.Struct:
		return populateStructFromEnv(value)
	case reflect.Pointer:
		if !value.IsNil() {
			return traverseValue(value.Elem())
		}
	default:
		// Ignore other kinds.
	}

	return nil
}

// traverseMap visits every value of the map and recurses into it, populating any structs found within.
func traverseMap(m reflect.Value) error {
	iter := m.MapRange()
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Map values are not settable, so work on a writable copy...
		newValue := reflect.New(value.Type()).Elem()
		newValue.Set(value)

		if err := traverseValue(newValue); err != nil {
			return err
		}

		// ...and write the result back into the map.
		m.SetMapIndex(key, newValue)
	}

	return nil
}

// traverseSlice visits every element of the slice or array and recurses into it, populating any structs found within.
func traverseSlice(slice reflect.Value) error {
	n := slice.Len()
	for i := range n {
		elem := slice.Index(i)

		if err := traverseValue(elem); err != nil {
			return err
		}
	}

	return nil
}

// ErrInvalidConfigType is returned when a value passed to one of the package's loaders
// is not a non-nil pointer to a struct. Match it with errors.Is.
var ErrInvalidConfigType = errors.New("cfg: config must be a non-nil pointer to a struct")

// ErrUnknownFileExtension is returned by FromFile when the extension of the path selects no supported format.
// The returned error wraps it and names the extension; match it with errors.Is.
var ErrUnknownFileExtension = errors.New("cfg: unknown file extension")

// ErrUnsupportedFieldType is returned by FromEnv when a field carrying an env tag has a type that cannot
// be populated from a single string. The returned error wraps it and names the field, its variable, and
// the type; match it with errors.Is.
var ErrUnsupportedFieldType = errors.New("cfg: unsupported field type")
