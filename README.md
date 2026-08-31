# vary

Simple configuration library that binds values to environment variables.

![Continuous Integration](https://img.shields.io/github/actions/workflow/status/chadweimer/vary/build-and-test.yml?branch=main)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=chadweimer_vary&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=chadweimer_vary)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=chadweimer_vary&metric=coverage)](https://sonarcloud.io/summary/new_code?id=chadweimer_vary)
[![Closed Pull Requests](https://img.shields.io/github/issues-pr-closed-raw/chadweimer/vary.svg)](https://github.com/chadweimer/vary/pulls)
[![GitHub release](https://img.shields.io/github/release/chadweimer/vary.svg)](https://github.com/chadweimer/vary/releases)
[![license](https://img.shields.io/github/license/chadweimer/vary.svg)](LICENSE)

## Quick Start

Declaring your configuration objects, optionally decorating each field with struct tags to control the variable binding:

```go
type Config struct {
    Port     int               `env:"PORT" default:"8080"`
    Debug    bool              `default:"false"` // Will use the all-caps name "DEBUG"
    Timeout  time.Duration     `env:"TIMEOUT" default:"30s"`
    Database url.URL           `env:"DATABASE_URL" required:"true"`
    Tags     map[string]string `env:"TAGS" default:"env=dev,tier=frontend"`
}
```

Then, bind the struct to set field values based on the current state of environment variables.

```go
var cfg Config
if err := vary.Bind(&cfg); err != nil {
    log.Fatal(err)
}
```

If you want to use an application specific environment variable prefix in order to avoid collisions, you can configure the prefix before binding:

```go
binder := vary.New(vary.WithPrefix("MYAPP"))
// This will look for MYAPP_PORT, MYAPP_DEBUG, MYAPP_TIMEOUT, etc.,
// falling back to the base name only when the one with the prefix is not set.
binder.Bind(&cfg)
```

Refer to the documentation for more information on prefixes, strict mode, and how to control the order of precedence between names.

## Documentation

[![Go Reference](https://pkg.go.dev/badge/github.com/chadweimer/vary.svg)](https://pkg.go.dev/github.com/chadweimer/vary)
