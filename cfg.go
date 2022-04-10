package cfg

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

const envTagName = "env"

func FromEnv(config interface{}) error {
	if err := checkType(config); err != nil {
		return err
	}
	return populateStructFromEnv(reflect.ValueOf(config))
}

func populateStructFromEnv(s reflect.Value) error {
	if s.Kind() == reflect.Ptr {
		s = s.Elem()
	}
	t := s.Type()

	for i := s.NumField() - 1; i >= 0; i-- {
		tagValue := t.Field(i).Tag.Get(envTagName)
		if tagValue == "" {
			if f := s.Field(i); f.Kind() == reflect.Struct {
				if err := populateStructFromEnv(f); err != nil {
					return err
				}
			}
			continue
		}
		envValue, ok := os.LookupEnv(strings.ToUpper(tagValue))
		if !ok {
			continue
		}
		if err := setStructFieldValue(s.Field(i), envValue); err != nil {
			return err
		}
	}

	return nil
}

func setStructFieldValue(f reflect.Value, v string) error {
	if !f.CanSet() {
		return nil
	}

	switch f.Kind() {
	case reflect.Bool:
		s, err := strconv.ParseBool(v)
		if err != nil {
			return err
		}
		f.SetBool(s)
	case reflect.Float32:
		s, err := strconv.ParseFloat(v, 32)
		if err != nil {
			return err
		}
		f.SetFloat(s)
	case reflect.Float64:
		s, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		f.SetFloat(s)
	case reflect.Int:
		s, err := strconv.ParseInt(v, 10, 0)
		if err != nil {
			return err
		}
		f.SetInt(s)
	case reflect.Int32:
		s, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return err
		}
		f.SetInt(s)
	case reflect.Int64:
		s, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		f.SetInt(s)
	case reflect.String:
		f.SetString(v)
	}

	return nil
}

func FromFile(path string, config interface{}) error {
	if err := checkType(config); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	return json.NewDecoder(file).Decode(config)
}

type ConfigTypeError struct {
	Msg string
}

func (e ConfigTypeError) Error() string {
	return fmt.Sprintf("cfg: invalid config type (%s)", e.Msg)
}

func checkType(config interface{}) error {
	if v := reflect.ValueOf(config); v.Kind() == reflect.Ptr && v.Elem().Kind() == reflect.Struct {
		return nil
	}
	return ConfigTypeError{"should be a pointer to a struct"}
}
