package cfg

import (
	"encoding/json"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// Secret wraps a sensitive string (a password, token, or API key) so that it stays redacted in logs and
// formatted output. The wrapped value is only accessible through Reveal.
type Secret struct {
	// The secret field holds the actual secret value; it must never be exposed directly.
	secret string
}

// NewSecret returns a Secret that wraps the given value.
func NewSecret(s string) Secret {
	return Secret{
		secret: s,
	}
}

// Format implements fmt.Formatter, so the redacted value is rendered whenever a Secret is formatted by any of
// the fmt printing functions.
func (s Secret) Format(state fmt.State, verb rune) {
	fmt.Fprintf(state, fmt.FormatString(state, verb), s.String())
}

// MarshalText implements encoding.TextMarshaler, so the redacted value is produced whenever a Secret is marshaled,
// e.g. by log/slog or encoding/json.
func (s Secret) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// Reveal returns the actual secret value. Use it with care: the result must never end up in logs, error messages,
// or other output.
func (s *Secret) Reveal() string {
	return s.secret
}

// String implements fmt.Stringer, returning the redacted value instead of the secret itself,
// which makes it safe for display purposes.
func (s *Secret) String() string {
	return Mask
}

// UnmarshalJSON implements json.Unmarshaler, storing the decoded JSON string as the secret value.
func (s *Secret) UnmarshalJSON(value []byte) error {
	if err := json.Unmarshal(value, &s.secret); err != nil {
		return fmt.Errorf("cfg: unmarshal JSON: %w", err)
	}
	return nil
}

// UnmarshalYAML implements yaml.Unmarshaler, storing the decoded YAML scalar as the secret value.
func (s *Secret) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&s.secret); err != nil {
		return fmt.Errorf("cfg: unmarshal YAML: %w", err)
	}
	return nil
}
