# cfg

Read a configuration from JSON files and the environment.

## Usage

Define a configuration structure:
```go
type configuration struct {
	Listen  string        `json:"listen"`
	Timeout time.Duration `json:"timeout" env:"timeout"`
	Workers int           `json:"workers" env:"workers"`
}
```

After that, you can read the configuration from a file:
```go
config := configuration{}
cfg.FromFile("config.json", &config)
```

Where the contents of `config.json` could be:
```json
{
  "listen": "localhost:8080",
  "timeout": 60,
  "workers": 1
}
```

Also, if you have some environment variables:
```bash
export TIMEOUT=120 WORKERS=2
```

Then you can easily use them too:
```go
cfg.FromEnv(&config)
```
