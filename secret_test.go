package cfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestNewSecret(t *testing.T) {
	got := NewSecret("SUPERSECRET")
	want := Secret{secret: "SUPERSECRET"}

	if got != want {
		t.Errorf("got secret %q, want %q", got.secret, want.secret)
	}
}

func TestSecret_Format(t *testing.T) {
	tests := []struct{ format, want string }{
		{"%q", fmt.Sprintf("%q", Mask)},
		{"%s", Mask},
		{"%v", Mask},
		{"%+v", Mask},
		{"%#v", fmt.Sprintf("%q", Mask)},
	}

	secret := NewSecret("SUPERSECRET")

	for _, test := range tests {
		t.Run("fmt verb "+test.format, func(t *testing.T) {
			got := fmt.Sprintf(test.format, secret)
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSecret_Reveal(t *testing.T) {
	want := "SUPERSECRET"
	secret := NewSecret(want)
	if got := secret.Reveal(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSecret_String(t *testing.T) {
	secret := NewSecret("SUPERSECRET")
	if got := secret.String(); got != Mask {
		t.Errorf("got %q, want %q", got, Mask)
	}
}

func TestSecret_MarshalText(t *testing.T) {
	secret := NewSecret("SUPERSECRET")

	t.Run("slog handler", func(t *testing.T) {
		type logRecord struct {
			Secret string
		}

		buf := new(bytes.Buffer) // in-memory log sink
		record := logRecord{}    // the log entry is parsed into this struct below

		logger := slog.New(slog.NewJSONHandler(buf, nil))
		logger.Info("slog logger", "secret", secret)

		if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if record.Secret != Mask {
			t.Errorf("got %q, want %q", record.Secret, Mask)
		}
	})

	t.Run("json marshal", func(t *testing.T) {
		type fileRecord struct {
			Password Secret `json:"password"`
		}

		data, err := json.Marshal(fileRecord{Password: secret})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		got := string(data)
		want := fmt.Sprintf(`{"password":%q}`, Mask)

		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}

func TestSecret_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name:    "empty value",
			data:    `{"username": "bob", "password": ""}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "regular value",
			data:    `{"username": "bob", "password": "PASSWORD"}`,
			want:    "PASSWORD",
			wantErr: false,
		},
		{
			name:    "skip field",
			data:    `{"username": "bob"}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "wrong value type",
			data:    `{"username": "bob", "password": []}`,
			want:    "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var user struct {
				Username string `json:"username"`
				Password Secret `json:"password"`
			}

			err := json.Unmarshal([]byte(test.data), &user)

			switch {
			case err == nil && test.wantErr:
				t.Error("want error")
			case err != nil && !test.wantErr:
				t.Errorf("got error: %s", err)
			case err == nil && user.Password.secret != test.want:
				t.Errorf("got %q, want %q", user.Password.secret, test.want)
			}
		})
	}
}

func TestSecret_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name:    "empty value",
			data:    `{"username": "bob", "password": ""}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "regular value",
			data:    `{"username": "bob", "password": "PASSWORD"}`,
			want:    "PASSWORD",
			wantErr: false,
		},
		{
			name:    "skip field",
			data:    `{"username": "bob"}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "wrong value type",
			data:    `{"username": "bob", "password": []}`,
			want:    "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var user struct {
				Username string `yaml:"username"`
				Password Secret `yaml:"password"`
			}

			err := yaml.Unmarshal([]byte(test.data), &user)

			switch {
			case err == nil && test.wantErr:
				t.Error("want error")
			case err != nil && !test.wantErr:
				t.Errorf("got error: %s", err)
			case err == nil && user.Password.secret != test.want:
				t.Errorf("got %q, want %q", user.Password.secret, test.want)
			}
		})
	}
}
