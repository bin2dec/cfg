package cfg

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var testFileData = []byte(`{"host": "localhost", "port": 8080}`)

type testConfig struct {
	Host string
	Port int
}

func TestCheckConfigType(t *testing.T) {
	config := testConfig{}
	errWant := ConfigTypeError{"should be a pointer to a struct"}

	tests := []struct {
		config interface{}
		err    error
	}{
		{&config, nil},
		{nil, errWant},
		{config, errWant},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if err := checkType(tt.config); err != tt.err {
				t.Errorf("got %T, want %T", err, tt.err)
			}
		})
	}
}

func TestFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, testFileData, 0755); err != nil {
		t.Fatal(err)
	}

	config := testConfig{}
	if err := FromFile(path, &config); err != nil {
		t.Fatal(err)
	}

	if wantConfig := (testConfig{"localhost", 8080}); wantConfig != config {
		t.Errorf("got %v, want %v", config, wantConfig)
	}
}
