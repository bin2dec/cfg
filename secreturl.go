package cfg

import (
	"fmt"
	"net/url"
)

// SecretURL wraps a URL that may carry credentials in its userinfo part, [<user>[:<password>]@,
// located between the scheme and the host. When rendered, only the password is masked, and the username
// stays visible; use Secret instead when the entire string must stay hidden, for example, when the username
// itself is the credential, as with schemes that pass a token in the userinfo.
type SecretURL struct {
	Secret
}

// NewSecretURL returns a SecretURL that wraps the given URL string.
func NewSecretURL(url string) SecretURL {
	return SecretURL{Secret: NewSecret(url)}
}

// Format implements fmt.Formatter, so the redacted URL is rendered whenever a SecretURL is formatted by any of
// the fmt printing functions.
func (s SecretURL) Format(state fmt.State, verb rune) {
	fmt.Fprintf(state, fmt.FormatString(state, verb), s.String())
}

// MarshalText implements encoding.TextMarshaler, so the redacted URL is produced whenever a SecretURL is marshaled,
// e.g. by log/slog or encoding/json.
func (s SecretURL) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// String implements fmt.Stringer, returning the URL with its password, if any, replaced by Mask.
// The username is kept visible: when the username itself is the credential, use Secret instead,
// which masks the whole value.
// A URL that cannot be parsed is replaced with Mask entirely.
func (s *SecretURL) String() string {
	u, err := url.ParseRequestURI(s.secret)
	// An invalid URL cannot be redacted selectively, so mask it entirely.
	if err != nil {
		return Mask
	}

	// Keep the username visible and replace only the password.
	if _, hasPassword := u.User.Password(); hasPassword {
		u.User = url.UserPassword(u.User.Username(), Mask)
	}

	return u.String()
}
