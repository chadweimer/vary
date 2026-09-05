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

You can also create specific instances of a `Binder` rather than using the package global `DefaultBinder`:

```go
myBinder := vary.New() // Customize the instance as needed via passing options to New...
if err := myBinder.Bind(&cfg); err != nil {
    log.Fatal(err)
}
```

Or even create a new customized instance starting from the same settings as an existing one:

```go
myOtherBinder := myBinder.With(vary.WithStrict(true))
if err := myOtherBinder.Bind(&cfg); err != nil {
    log.Fatal(err)
}
```

### Environment Variable Lookup

This library defaults to loading environment variables from the process's environment via `os.LookupEnv()`.
This behavior can be overridden via the `WithLookup` method:

```go
func myCustomLookupFunc(key string) (string, bool) {
  // Do custom lookup logic
}

myBinder := vary.New(vary.WithLookup(myCustomLookupFunc))
```

A common scenario is to use an in-memory map, which can be useful for utilizing a dotenv (`.env`) file:

```go
// Use your favorite library to load the contents of the .env file
// Assumes this returns map[string]string. Real examples likely require handling errors.
dotenvMap := myFavoriteDotenvLib.Load(".env")
myBinder := vary.New(vary.WithLookup(vary.MapLookup(dotenvMap)))
```

You can also combine multiple sources together:

```go
// The lookups are evaluated in the order provided to CompositeLookup,
// and the first successfully found value is used.
myBinder := vary.New(vary.WithLookup(
  vary.CompositeLookup(os.LookupEnv, vary.MapLookup(dotenvMap))))
```

#### Using a Name Prefix

If you want to use an application specific environment variable prefix in order to avoid collisions,
you can use `PrefixedLookup` to prepend a prefix to all variable lookups:

```go
myBinder := vary.New(vary.WithLookup(vary.PrefixedLookup("MYAPP_", os.LookupEnv))
// If you wanted to apply this to the DefaultBinder
// vary.DefaultBinder = vary.DefaultBinder.With(vary.WithLookup(vary.PrefixedLookup("MYAPP", os.LookupEnv)))

// This will look for MYAPP_PORT, MYAPP_DEBUG, MYAPP_TIMEOUT, etc.
if err := myBinder.Bind(&cfg); err != nil {
    log.Fatal(err)
}
```

Combining this with `CompositeLookup` allows trying both the prefixed and non-prefixed name:

```go
// This will try MYAPP_NAME first, and if not found, fall back to NAME
prefixedPreferredBinder := vary.New(vary.WithLookup(
  vary.CompositeLookup(vary.PrefixedLookup("MYAPP_", os.LookupEnv), os.LookupEnv)))

// This will try NAME first, and if not found, fall back to MYAPP_NAME
unprefixedPreferredBinder := vary.New(vary.WithLookup(
  vary.CompositeLookup(os.LookupEnv, vary.PrefixedLookup("MYAPP_", os.LookupEnv))))
```

### Custom Marshalers

You can register custom marshaler functions for custom types:

```go
// Marshaling to a new value.
// This shows how to do this using the DefaultBinder.
if err := vary.RegisterMarshaler(vary.DefaultBinder, func(s string) (CustomType, error) {
    return parseCustomType(s)
}); err != nil {
    log.Fatal(err)
}
if err := vary.Bind(&cfg); err != nil {
    log.Fatal(err)
}

// Mutating the value of any type that implements a custom interface.
// This shows using a specific Binder instance
myBinder := vary.New()
if err := vary.RegisterMutatingMarshaler(myBinder, func(s string, i CustomMarshalingInterface) error {
    return i.CustomMarshalMethod(s)
}); err != nil {
    log.Fatal(err)
}
if err := myBinder.Bind(&cfg); err != nil {
    log.Fatal(err)
}
```

Refer to the documentation for more information on prefixes, custom marshalers, strict mode, and how to control the order of precedence between names.

## Documentation

[![Go Reference](https://pkg.go.dev/badge/github.com/chadweimer/vary.svg)](https://pkg.go.dev/github.com/chadweimer/vary)
