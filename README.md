# cfg

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

`cfg` populates configuration structs from environment variables and JSON/YAML files, and provides
secret types that keep sensitive values redacted in logs and formatted output.

## Installation

```shell
go get github.com/bin2dec/cfg
```

The package requires Go 1.22 or later.

## Loading from the environment

Tag fields with `env` and call `FromEnv` with a pointer to your config struct.
The tag value, upper-cased, names the environment variable to read:

```go
package main

import (
	"log"
	"time"

	"github.com/bin2dec/cfg"
)

type Config struct {
	Host     string        `env:"host"`     // read from HOST
	Port     int           `env:"port"`     // read from PORT
	Debug    bool          `env:"debug"`    // read from DEBUG
	Timeout  time.Duration `env:"timeout"`  // read from TIMEOUT, e.g. "30s"
	Password cfg.Secret    `env:"password"` // read from PASSWORD
}

func main() {
	var config Config
	if err := cfg.FromEnv(&config); err != nil {
		log.Fatal(err)
	}
}
```

Supported field types:

| Field type                  | Parsing                                        |
| --------------------------- | ---------------------------------------------- |
| `bool`                      | [`strconv.ParseBool`]                          |
| `int`, `int8`…`int64`       | `strconv.ParseInt`, at the field's bit width   |
| `uint`, `uint8`…`uint64`    | `strconv.ParseUint`, at the field's bit width  |
| `float32`, `float64`        | `strconv.ParseFloat`, at the field's bit width |
| `time.Duration`             | `time.ParseDuration`, so `"30s"` works         |
| `string`                    | as is                                          |
| [`Secret`], [`SecretURL`]   | wrapped as is                                  |
| pointer to any of the above | allocated when nil, then populated             |

Rules:

- A field whose variable is absent stays untouched, so defaults set in code survive.
- A tagged field of any other type — a slice, a map, a `complex64`, a named subtype of `Secret` —
  results in an error naming the field and the type: an `env` tag means the field itself is populated
  from that variable.
- Nested structs, maps, and slices (and pointers to them) are traversed recursively, and tagged fields
  inside them are populated from their own variables:

  ```go
  type Service struct {
  	Host string `env:"host"`
  	Port int    `env:"port"`
  }

  type Config struct {
  	Default  Service            // HOST and PORT
  	Replicas map[string]Service // HOST and PORT, for every existing value
  	Backups  []Service          // HOST and PORT, for every existing element
  }
  ```

- Containers are traversed, not created: a nil map, slice, or pointer stays nil, so pre-allocate them
  (in code, or from a file) before calling `FromEnv`.
- Values must form a tree: cyclic structures are recursed into without a bound.

## Loading from files

`FromFile` picks the format from the file extension, case-insensitively: `.json` for JSON,
`.yaml` and `.yml` for YAML. Any other extension is an error.

```go
var config Config
if err := cfg.FromFile("config.yaml", &config); err != nil {
	log.Fatal(err)
}
```

`FromJSONFile` and `FromYAMLFile` load a specific format directly. A file must hold a single valid
document; content trailing the first JSON value is rejected.

To let the environment override the file, load the file first and then call `FromEnv` — fields whose
variables are absent keep the values the file set:

```go
var config Config
if err := cfg.FromFile("config.yaml", &config); err != nil {
	log.Fatal(err)
}
if err := cfg.FromEnv(&config); err != nil {
	log.Fatal(err)
}
```

## Secrets

### `Secret`

`Secret` wraps a sensitive string and keeps it redacted whenever it is rendered: by the `fmt` printing
functions, by `encoding/json`, and by `log/slog`. The wrapped value is only accessible through `Reveal`:

```go
password := cfg.NewSecret("hunter2")

fmt.Println(password) // REDACTED
password.Reveal()     // "hunter2"
```

Both loaders populate `Secret` fields directly:

```go
type Config struct {
	Password cfg.Secret `env:"password"` // or json:"password" / yaml:"password"
}
```

Marshaling a config redacts secrets by design, so writing a config back out never leaks them —
and never round-trips them either.

### `SecretURL`

`SecretURL` wraps a URL that carries credentials in its userinfo part. Only the password is masked;
the rest of the URL stays readable:

```go
u := cfg.NewSecretURL("nats://user:hunter2@nats.example.com:4222")

fmt.Println(u) // nats://user:REDACTED@nats.example.com:4222
u.Reveal()     // "nats://user:hunter2@nats.example.com:4222"
```

The username is kept visible. When the username itself is the credential — for example, a token
passed in the userinfo — use `Secret` instead, which masks the entire value.

## Errors

- Passing anything but a non-nil pointer to a struct to a loader returns an
  [`ErrInvalidConfigType`], matchable with `errors.Is`.
- `FromFile` with a path whose extension selects no supported format returns an error wrapping
  [`ErrUnknownFileExtension`] and naming the extension.
- A tagged field of an unsupported type produces an error wrapping [`ErrUnsupportedFieldType`] and naming
  the field and the type.
- A value that cannot be parsed produces an error naming the field and its variable:
  `cfg: field Port (env PORT): strconv.ParseInt: parsing "abc": invalid syntax`.

## Limitations

- Config values must form a tree; cyclic structures recurse without bound.
- Fields present in a file but not in the struct are ignored, as is standard for `encoding/json`
  and YAML decoding; there is no unknown-field detection.

## License

[MIT](LICENSE)

[`strconv.ParseBool`]: https://pkg.go.dev/strconv#ParseBool
[`Secret`]: https://pkg.go.dev/github.com/bin2dec/cfg#Secret
[`SecretURL`]: https://pkg.go.dev/github.com/bin2dec/cfg#SecretURL
[`ErrInvalidConfigType`]: https://pkg.go.dev/github.com/bin2dec/cfg#ErrInvalidConfigType
[`ErrUnknownFileExtension`]: https://pkg.go.dev/github.com/bin2dec/cfg#ErrUnknownFileExtension
[`ErrUnsupportedFieldType`]: https://pkg.go.dev/github.com/bin2dec/cfg#ErrUnsupportedFieldType
