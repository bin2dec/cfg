package cfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestNewSecretURL(t *testing.T) {
	want := SecretURL{NewSecret("https://user:PASSWORD@example.com")}
	if got := NewSecretURL("https://user:PASSWORD@example.com"); got != want {
		t.Errorf("got secret %q, want %q", got.secret, want.secret)
	}
}

func TestSecretURL_Format(t *testing.T) {
	wantURL := fmt.Sprintf("https://user:%s@example.com", Mask)
	tests := []struct{ format, want string }{
		{"%q", fmt.Sprintf("%q", wantURL)},
		{"%s", wantURL},
		{"%v", wantURL},
		{"%+v", wantURL},
		{"%#v", fmt.Sprintf("%q", wantURL)},
	}

	secretURL := NewSecretURL("https://user:PASSWORD@example.com")

	for _, test := range tests {
		t.Run("fmt verb "+test.format, func(t *testing.T) {
			if got := fmt.Sprintf(test.format, secretURL); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSecretURL_Reveal(t *testing.T) {
	want := "https://user:PASSWORD@example.com"
	secretURL := NewSecretURL(want)
	if got := secretURL.Reveal(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSecretURL_String(t *testing.T) {
	tests := []struct {
		name      string
		secretURL SecretURL
		want      string
	}{
		{
			name:      "empty password",
			secretURL: NewSecretURL("https://user:@example.com"),
			want:      fmt.Sprintf("https://user:%s@example.com", Mask),
		},
		{
			name:      "invalid url",
			secretURL: NewSecretURL("https://example.com#"),
			want:      Mask,
		},
		{
			name:      "username and password",
			secretURL: NewSecretURL("https://user:PASSWORD@example.com"),
			want:      fmt.Sprintf("https://user:%s@example.com", Mask),
		},
		{
			name:      "username only",
			secretURL: NewSecretURL("https://user@example.com"),
			want:      "https://user@example.com",
		},
		{
			name:      "without username and password",
			secretURL: NewSecretURL("https://example.com"),
			want:      "https://example.com",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.secretURL.String(); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSecretURL_MarshalText(t *testing.T) {
	secretURL := NewSecretURL("https://user:PASSWORD@example.com")

	t.Run("slog handler", func(t *testing.T) {
		type logRecord struct {
			URL string
		}

		buf := new(bytes.Buffer)
		record := &logRecord{} // the log entry is parsed into this struct below

		logger := slog.New(slog.NewJSONHandler(buf, nil))
		logger.Info("slog logger", "url", secretURL)

		if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		want := fmt.Sprintf("https://user:%s@example.com", Mask)

		if record.URL != want {
			t.Errorf("got %q, want %q", record.URL, want)
		}
	})

	t.Run("json marshal", func(t *testing.T) {
		type fileRecord struct {
			URL SecretURL `json:"url"`
		}

		data, err := json.Marshal(fileRecord{URL: secretURL})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		got := string(data)
		want := fmt.Sprintf(`{"url":"https://user:%s@example.com"}`, Mask)

		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}

func TestSecretURL_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name:    "empty value",
			data:    `{"url": ""}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "regular url",
			data:    `{"url": "https://user:PASSWORD@example.com"}`,
			want:    "https://user:PASSWORD@example.com",
			wantErr: false,
		},
		{
			name:    "skip field",
			data:    `{}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "wrong value type",
			data:    `{"url": []}`,
			want:    "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var host struct {
				URL SecretURL `json:"url"`
			}

			err := json.Unmarshal([]byte(test.data), &host)

			switch {
			case err == nil && test.wantErr:
				t.Error("want error")
			case err != nil && !test.wantErr:
				t.Errorf("got error: %s", err)
			case err == nil && host.URL.secret != test.want:
				t.Errorf("got %q, want %q", host.URL.secret, test.want)
			}
		})
	}
}

func TestSecretURL_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name:    "empty value",
			data:    `{"url": ""}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "regular url",
			data:    `{"url": "https://user:PASSWORD@example.com"}`,
			want:    "https://user:PASSWORD@example.com",
			wantErr: false,
		},
		{
			name:    "skip field",
			data:    `{}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "wrong value type",
			data:    `{"url": []}`,
			want:    "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var host struct {
				URL SecretURL `yaml:"url"`
			}

			err := yaml.Unmarshal([]byte(test.data), &host)

			switch {
			case err == nil && test.wantErr:
				t.Error("want error")
			case err != nil && !test.wantErr:
				t.Errorf("got error: %s", err)
			case err == nil && host.URL.secret != test.want:
				t.Errorf("got %q, want %q", host.URL.secret, test.want)
			}
		})
	}
}
