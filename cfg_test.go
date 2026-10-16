package cfg

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var testJSONData = []byte(`
{
  "host": "localhost",
  "port": 8080
}
`)

var testYAMLData = []byte(`
host: localhost
port: 8080
`)

var testInvalidJSONData = []byte(`{"host": "localhost",`)

// A valid JSON value followed by trailing content, which must be rejected.
var testTrailingJSONData = []byte(`{"host": "localhost", "port": 8080} trailing`)

// The tab in the second line's indentation makes this YAML invalid.
var testInvalidYAMLData = []byte("host: localhost\n\tport: 8080")

var testSecretsJSONData = []byte(`
{
  "password": "hunter2",
  "url": "nats://user:hunter2@nats.example.com:4222"
}
`)

var testSecretsYAMLData = []byte(`
password: hunter2
url: nats://user:hunter2@nats.example.com:4222
`)

type testConfig struct {
	Host string `env:"host"`
	Port int    `env:"port"`
}

// testAbsentConfig carries env tags that no test sets, so its fields stay zero-valued.
type testAbsentConfig struct {
	Host string `env:"absent_host"`
	Port int    `env:"absent_port"`
}

type testNestedConfig struct {
	Nested     testConfig
	Pointer    *testConfig
	NilPointer *testConfig
	Slice      []testConfig
	NilSlice   []testConfig
	Array      [2]testConfig
	Map        map[string]testConfig
	NilMap     map[string]testConfig
}

type testNestedParseConfig struct {
	Items []testParseConfig
	Map   map[string]testParseConfig
}

type testParseConfig struct {
	Bool     bool          `env:"parse_bool"`
	Float32  float32       `env:"parse_float32"`
	Float64  float64       `env:"parse_float64"`
	Int      int           `env:"parse_int"`
	Int32    int32         `env:"parse_int32"`
	Int64    int64         `env:"parse_int64"`
	Duration time.Duration `env:"parse_duration"`
}

type testScalarsConfig struct {
	Bool    bool    `env:"scalars_bool"`
	Float32 float32 `env:"scalars_float32"`
	Float64 float64 `env:"scalars_float64"`
	Int     int     `env:"scalars_int"`
	Int8    int8    `env:"scalars_int8"`
	Int16   int16   `env:"scalars_int16"`
	Int32   int32   `env:"scalars_int32"`
	Int64   int64   `env:"scalars_int64"`
	Uint    uint    `env:"scalars_uint"`
	Uint32  uint32  `env:"scalars_uint32"`
	Uint64  uint64  `env:"scalars_uint64"`
	String  string  `env:"scalars_string"`
}

type testDurationConfig struct {
	Timeout time.Duration `env:"timeout"`
}

// testPointerConfig has pointer fields, which must be allocated and populated when their variables are set.
type testPointerConfig struct {
	Host     *string        `env:"host"`
	Port     *int           `env:"port"`
	Timeout  *time.Duration `env:"timeout"`
	Password *Secret        `env:"password"`
	URL      *SecretURL     `env:"db_url"`
}

// testTaggedSliceConfig carries an env tag on a slice, which cannot be populated from a single variable.
type testTaggedSliceConfig struct {
	Hosts []string `env:"hosts"`
}

type testSecretsConfig struct {
	Password Secret    `env:"password"`
	URL      SecretURL `env:"db_url"`
}

type testSecretsFileConfig struct {
	Password Secret    `json:"password" yaml:"password"`
	URL      SecretURL `json:"url"      yaml:"url"`
}

// testUnexportedConfig has an unexported field with an env tag, which must be skipped.
type testUnexportedConfig struct {
	host string `env:"host"`
}

// testUnsupportedConfig has a field of a type the env engine cannot populate, which must be reported.
type testUnsupportedConfig struct {
	Timeout complex64 `env:"timeout"`
}

func TestCheckConfigType(t *testing.T) {
	config := testConfig{}
	errWant := ErrInvalidConfigType

	tests := []struct {
		config any
		err    error
	}{
		{&config, nil},
		{nil, errWant},
		{config, errWant},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if err := checkType(tt.config); !errors.Is(err, tt.err) {
				t.Errorf("got %v, want %v", err, tt.err)
			}
		})
	}
}

func TestFromEnv(t *testing.T) {
	wantConfig := initTestConfig()

	t.Setenv("HOST", wantConfig.Host)
	t.Setenv("PORT", strconv.Itoa(wantConfig.Port))

	gotConfig := testConfig{}
	if err := FromEnv(&gotConfig); err != nil {
		t.Error(err)
	}

	if wantConfig != gotConfig {
		t.Errorf("got %v, want %v", gotConfig, wantConfig)
	}
}

func TestFromEnv_AbsentEnvVars(t *testing.T) {
	got := testAbsentConfig{}
	if err := FromEnv(&got); err != nil {
		t.Fatal(err)
	}

	if want := (testAbsentConfig{}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFromEnv_NestedStructures(t *testing.T) {
	wantField := initTestConfig()

	t.Setenv("HOST", wantField.Host)
	t.Setenv("PORT", strconv.Itoa(wantField.Port))

	got := testNestedConfig{
		Pointer: &testConfig{},
		Slice:   []testConfig{{}, {}},
		Map:     map[string]testConfig{"db": {}},
	}
	if err := FromEnv(&got); err != nil {
		t.Fatal(err)
	}

	checkTestConfig(t, got.Nested, wantField)
	checkTestConfig(t, *got.Pointer, wantField)

	if got.NilPointer != nil {
		t.Errorf("got %v, want the nil pointer to stay nil", got.NilPointer)
	}

	if got.NilSlice != nil {
		t.Errorf("got %v, want the nil slice to stay nil", got.NilSlice)
	}

	if got.NilMap != nil {
		t.Errorf("got %v, want the nil map to stay nil", got.NilMap)
	}

	for _, item := range got.Slice {
		checkTestConfig(t, item, wantField)
	}

	for _, item := range got.Array {
		checkTestConfig(t, item, wantField)
	}

	checkTestConfig(t, got.Map["db"], wantField)
}

func TestFromEnv_ParseErrors(t *testing.T) {
	tests := []struct {
		name   string
		config any
		env    string
	}{
		{"invalid bool value", &testParseConfig{}, "PARSE_BOOL"},
		{"invalid float32 value", &testParseConfig{}, "PARSE_FLOAT32"},
		{"invalid float64 value", &testParseConfig{}, "PARSE_FLOAT64"},
		{"invalid int value", &testParseConfig{}, "PARSE_INT"},
		{"invalid int32 value", &testParseConfig{}, "PARSE_INT32"},
		{"invalid int64 value", &testParseConfig{}, "PARSE_INT64"},
		{"invalid duration value", &testParseConfig{}, "PARSE_DURATION"},
		{"invalid int value in a slice", &testNestedParseConfig{Items: []testParseConfig{{}}}, "PARSE_INT"},
		{"invalid int value in a map", &testNestedParseConfig{Map: map[string]testParseConfig{"a": {}}}, "PARSE_INT"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.env, "abc")

			if err := FromEnv(test.config); err == nil {
				t.Error("expect error")
			}
		})
	}
}

func TestFromEnv_ScalarTypes(t *testing.T) {
	t.Setenv("SCALARS_BOOL", "true")
	t.Setenv("SCALARS_FLOAT32", "1.5")
	t.Setenv("SCALARS_FLOAT64", "2.25")
	t.Setenv("SCALARS_INT", "42")
	t.Setenv("SCALARS_INT8", "-8")
	t.Setenv("SCALARS_INT16", "-16")
	t.Setenv("SCALARS_INT32", "-42")
	t.Setenv("SCALARS_INT64", "9223372036854775807")
	t.Setenv("SCALARS_UINT", "42")
	t.Setenv("SCALARS_UINT32", "4294967295")
	t.Setenv("SCALARS_UINT64", "18446744073709551615")
	t.Setenv("SCALARS_STRING", "localhost")

	want := testScalarsConfig{
		Bool:    true,
		Float32: 1.5,
		Float64: 2.25,
		Int:     42,
		Int8:    -8,
		Int16:   -16,
		Int32:   -42,
		Int64:   9223372036854775807,
		Uint:    42,
		Uint32:  4294967295,
		Uint64:  18446744073709551615,
		String:  "localhost",
	}

	got := testScalarsConfig{}
	if err := FromEnv(&got); err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFromEnv_SecretFields(t *testing.T) {
	const (
		password    = "hunter2"
		databaseURL = "nats://user:hunter2@nats.example.com:4222"
	)

	t.Setenv("PASSWORD", password)
	t.Setenv("DB_URL", databaseURL)

	got := testSecretsConfig{}
	if err := FromEnv(&got); err != nil {
		t.Fatal(err)
	}

	if got.Password.Reveal() != password {
		t.Errorf("got %q, want %q", got.Password.Reveal(), password)
	}

	if got.URL.Reveal() != databaseURL {
		t.Errorf("got %q, want %q", got.URL.Reveal(), databaseURL)
	}

	if got.Password.String() != Mask {
		t.Errorf("got %q, want %q", got.Password.String(), Mask)
	}

	if want := "nats://user:" + Mask + "@nats.example.com:4222"; got.URL.String() != want {
		t.Errorf("got %q, want %q", got.URL.String(), want)
	}
}

func TestFromEnv_UnexportedFields(t *testing.T) {
	t.Setenv("HOST", "localhost")

	config := testUnexportedConfig{}
	if err := FromEnv(&config); err != nil {
		t.Fatal(err)
	}

	if config.host != "" {
		t.Errorf("got %q, want the unexported field to stay untouched", config.host)
	}
}

func TestFromEnv_UnsupportedFieldType(t *testing.T) {
	t.Setenv("TIMEOUT", "1+2i")

	config := testUnsupportedConfig{}
	err := FromEnv(&config)
	if err == nil {
		t.Fatal("expect error")
	}
	if !errors.Is(err, ErrUnsupportedFieldType) {
		t.Errorf("got %v, want ErrUnsupportedFieldType", err)
	}

	// The error must name the env variable, the field, and the offending type.
	for _, want := range []string{"TIMEOUT", "Timeout", "complex64"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestFromEnv_TaggedUnsupportedContainer(t *testing.T) {
	t.Setenv("HOSTS", "a,b")

	config := testTaggedSliceConfig{}
	if err := FromEnv(&config); !errors.Is(err, ErrUnsupportedFieldType) {
		t.Errorf("got %v, want ErrUnsupportedFieldType", err)
	}
}

// TestFromEnv_ErrorContext checks that parse errors name the env variable and the field, so failures are debuggable.
func TestFromEnv_ErrorContext(t *testing.T) {
	t.Setenv("PARSE_INT", "abc")

	err := FromEnv(&testParseConfig{})
	if err == nil {
		t.Fatal("expect error")
	}

	for _, want := range []string{"PARSE_INT", "Int"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestFromEnv_Duration(t *testing.T) {
	t.Setenv("TIMEOUT", "90s")

	got := testDurationConfig{}
	if err := FromEnv(&got); err != nil {
		t.Fatal(err)
	}

	if want := 90 * time.Second; got.Timeout != want {
		t.Errorf("got %v, want %v", got.Timeout, want)
	}
}

func TestFromEnv_IntOverflow(t *testing.T) {
	t.Setenv("SCALARS_INT8", "300")

	config := testScalarsConfig{}
	if err := FromEnv(&config); err == nil {
		t.Error("expect error")
	}
}

func TestFromEnv_PointerFields(t *testing.T) {
	const (
		password    = "hunter2"
		databaseURL = "nats://user:hunter2@nats.example.com:4222"
	)

	t.Setenv("HOST", "localhost")
	t.Setenv("PORT", "8080")
	t.Setenv("TIMEOUT", "30s")
	t.Setenv("PASSWORD", password)
	t.Setenv("DB_URL", databaseURL)

	got := testPointerConfig{}
	if err := FromEnv(&got); err != nil {
		t.Fatal(err)
	}

	if got.Host == nil || *got.Host != "localhost" {
		t.Errorf("got %v, want a pointer to %q", got.Host, "localhost")
	}

	if got.Port == nil || *got.Port != 8080 {
		t.Errorf("got %v, want a pointer to %d", got.Port, 8080)
	}

	if got.Timeout == nil || *got.Timeout != 30*time.Second {
		t.Errorf("got %v, want a pointer to %v", got.Timeout, 30*time.Second)
	}

	if got.Password == nil || got.Password.Reveal() != password {
		t.Errorf("got %v, want a pointer to a secret holding %q", got.Password, password)
	}

	if got.URL == nil || got.URL.Reveal() != databaseURL {
		t.Errorf("got %v, want a pointer to a secret URL holding %q", got.URL, databaseURL)
	}
}

func TestFromEnv_PointerFieldsAbsentEnvVars(t *testing.T) {
	got := testPointerConfig{}
	if err := FromEnv(&got); err != nil {
		t.Fatal(err)
	}

	if got.Host != nil || got.Port != nil || got.Timeout != nil || got.Password != nil || got.URL != nil {
		t.Errorf("got %+v, want the pointer fields to stay nil", got)
	}
}

func TestNonPointerConfig(t *testing.T) {
	config := testConfig{}
	errWant := ErrInvalidConfigType

	loaders := []struct {
		name   string
		config any
		load   func(any) error
	}{
		{"FromEnv", config, FromEnv},
		{"FromEnv with nil", nil, FromEnv},
		{"FromEnv with a typed nil pointer", (*testConfig)(nil), FromEnv},
		{"FromFile", config, func(c any) error { return FromFile("config.json", c) }},
		{"FromJSONFile", config, func(c any) error { return FromJSONFile("config.json", c) }},
		{"FromYAMLFile", config, func(c any) error { return FromYAMLFile("config.yaml", c) }},
	}

	for _, loader := range loaders {
		t.Run(loader.name, func(t *testing.T) {
			if err := loader.load(loader.config); !errors.Is(err, errWant) {
				t.Errorf("got %v, want %v", err, errWant)
			}
		})
	}
}

func TestFromFile(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		data []byte
	}{
		{"json file", ".json", testJSONData},
		{"json file with uppercase extension", ".JSON", testJSONData},
		{"yaml file", ".yaml", testYAMLData},
		{"yml file", ".yml", testYAMLData},
	}

	wantConfig := initTestConfig()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config"+test.ext)
			if err := os.WriteFile(path, test.data, 0600); err != nil {
				t.Fatal(err)
			}

			gotConfig := testConfig{}
			if err := FromFile(path, &gotConfig); err != nil {
				t.Error(err)
			}

			if gotConfig != wantConfig {
				t.Errorf("got %v, want %v", gotConfig, wantConfig)
			}
		})
	}
}

func TestFromFile_UnknownExtension(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name string
		path string
		want string
	}{
		{"unsupported extension", filepath.Join(dir, "config.toml"), `".toml"`},
		{"missing extension", filepath.Join(dir, "config"), `""`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testConfig{}
			err := FromFile(test.path, &config)
			if !errors.Is(err, ErrUnknownFileExtension) {
				t.Errorf("got %v, want ErrUnknownFileExtension", err)
			}
			// The extension must stay in the message, so the failure names the offender.
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %v does not mention the extension %s", err, test.want)
			}
		})
	}
}

func TestFromJSONFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, testJSONData, 0600); err != nil {
		t.Fatal(err)
	}

	gotConfig := testConfig{}
	if err := FromJSONFile(path, &gotConfig); err != nil {
		t.Error(err)
	}

	if wantConfig := initTestConfig(); wantConfig != gotConfig {
		t.Errorf("got %v, want %v", gotConfig, wantConfig)
	}
}

func TestFromJSONFile_InvalidData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, testInvalidJSONData, 0600); err != nil {
		t.Fatal(err)
	}

	config := testConfig{}
	if err := FromJSONFile(path, &config); err == nil {
		t.Error("expect error")
	}
}

func TestFromJSONFile_TrailingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, testTrailingJSONData, 0600); err != nil {
		t.Fatal(err)
	}

	config := testConfig{}
	if err := FromJSONFile(path, &config); err == nil {
		t.Error("expect error")
	}
}

func TestFromJSONFile_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	config := testConfig{}
	if err := FromJSONFile(path, &config); err == nil {
		t.Error("expect error")
	}
}

func TestFromJSONFile_SecretFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.json")
	if err := os.WriteFile(path, testSecretsJSONData, 0600); err != nil {
		t.Fatal(err)
	}

	const (
		wantPassword = "hunter2"
		wantURL      = "nats://user:hunter2@nats.example.com:4222"
	)

	got := testSecretsFileConfig{}
	if err := FromJSONFile(path, &got); err != nil {
		t.Fatal(err)
	}

	if got.Password.Reveal() != wantPassword {
		t.Errorf("got %q, want %q", got.Password.Reveal(), wantPassword)
	}

	if got.URL.Reveal() != wantURL {
		t.Errorf("got %q, want %q", got.URL.Reveal(), wantURL)
	}
}

func TestFromYAMLFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, testYAMLData, 0600); err != nil {
		t.Fatal(err)
	}

	gotConfig := testConfig{}
	if err := FromYAMLFile(path, &gotConfig); err != nil {
		t.Error(err)
	}

	if wantConfig := initTestConfig(); wantConfig != gotConfig {
		t.Errorf("got %v, want %v", gotConfig, wantConfig)
	}
}

func TestFromYAMLFile_InvalidData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, testInvalidYAMLData, 0600); err != nil {
		t.Fatal(err)
	}

	config := testConfig{}
	if err := FromYAMLFile(path, &config); err == nil {
		t.Error("expect error")
	}
}

func TestFromYAMLFile_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	config := testConfig{}
	if err := FromYAMLFile(path, &config); err == nil {
		t.Error("expect error")
	}
}

func TestFromYAMLFile_SecretFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, testSecretsYAMLData, 0600); err != nil {
		t.Fatal(err)
	}

	const (
		wantPassword = "hunter2"
		wantURL      = "nats://user:hunter2@nats.example.com:4222"
	)

	got := testSecretsFileConfig{}
	if err := FromYAMLFile(path, &got); err != nil {
		t.Fatal(err)
	}

	if got.Password.Reveal() != wantPassword {
		t.Errorf("got %q, want %q", got.Password.Reveal(), wantPassword)
	}

	if got.URL.Reveal() != wantURL {
		t.Errorf("got %q, want %q", got.URL.Reveal(), wantURL)
	}
}

func checkTestConfig(t *testing.T, got, want testConfig) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func initTestConfig() testConfig {
	return testConfig{
		Host: "localhost",
		Port: 8080,
	}
}
