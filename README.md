# vary

Simple configuration library that binds values to environment variables.

## Quick Start

First install the package into your go project:
```bash
go get github.com/chadweimer/vary
```

When declaring your configuration objects, you can decorate each field with struct tags to control the variable binding:
```go
package main

import "github.com/chadweimer/vary"

type AppConfig struct {
    SomeVar int `env:"SOME_VAR" default:"1"`
}

func main() {
    appConfig := new(AppConfig)
    if err := vary.Bind(appConfig); err != nil {
        panic(err)
    }
}
```

If you want to use an application specific environment variable prefix in order to avoid collisions, you can configure the prefix before binding:

```go
vary.SetPrefix("MYAPP")
// This will look for MYAPP_SOME_VAR, falling back to SOME_VAR if the former is not set
vary.Bind(appConfig)
```

## Documentation

[![Go Reference](https://pkg.go.dev/badge/github.com/chadweimer/vary.svg)](https://pkg.go.dev/github.com/chadweimer/vary)
