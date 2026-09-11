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

#### Ignoring a Field

To ignore a field, set the `env` tag to "-":

```go
type Config struct {
  MyIgnoredField int `env:"-"`
}
```

> [!NOTE]
> The effects of the `default` and `required` tags still apply, if specified.
> This will only result in skipping attempting to load the value from the environment.

#### Aliases

It's supported to list multiple environment variables names on a field, with each separated by a comma.
These names are tried in the order specificed, and the first successfully found value is used.
All prefix rules documented elsewhere apply to each name.

```go
type Config struct {
  Port int `env:"PORT,HTTP_PORT,SERVER_PORT"`
}
```

Additional aliases will be tried if an error is encountered while parsing one of the values.

> [!IMPORTANT]
> This means that when strict mode is enabled, an error will NOT be returned from `Bind` if one of the aliases results in successfully setting the field even if other aliases caused errors.

#### Using a Global Prefix

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

#### Complex Lookup Composition

Putting all of the above together, the following example shows using both a prefix and an env map:

```go
baseLookup := vary.CompositeLookup(os.LookupEnv, vary.MapLookup(dotenvMap))

// This will try MYAPP_NAME first, and if not found, fall back to NAME
// using both the process's environment and the map from loading the .env file
myBinder := vary.New(vary.WithLookup(
  vary.CompositeLookup(vary.PrefixedLookup("MYAPP_", baseLookup), baseLookup)))
```

### Custom Marshalers

You can register custom marshaler functions for custom types:

```go
// Marshaling to a new value.
// This shows how to do this using the DefaultBinder.
if err := vary.AddMarshaler(func(s string) (CustomType, error) {
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
if err := myBinder.AddMutatingMarshaler(func(s string, i CustomMarshalingInterface) error {
  return i.CustomMarshalMethod(s)
}); err != nil {
  log.Fatal(err)
}
if err := myBinder.Bind(&cfg); err != nil {
  log.Fatal(err)
}
```

> [!NOTE]
> The following marshalers are registered by default:
>
> - time.Duration
> - encoding.TextUnmarshaler
> - encoding.BinaryUnmarshaler
>
> If you do not want these, use the `ClearMarshalers` option when constructing (or cloning) the `Binder`.

### Nested Structs

Nested structs are recursively bound, **except when they match a registered marshaler**.
When a type matches a marshaler, it is treated like a "normal field" even if it is a struct.

It's possible to prepend a prefix to all fields within a nested struct by adding the `env` tag.
When specified on a nested struct field, the value of this tag is prepended to the variables names of all fields within that struct; when not specified, no prefix is used.
In the example below, `SomeVal` will be populated by the value of the `NESTED1_VAL` and `NESTED2_VAL` environment variables, respectively.

```go
type NestedConfig struct {
  SomeVal string `env:"VAL"`
}
type Config struct {
  Nested1 NestedConfig `env:"NESTED1_"`
  Nested2 NestedConfig `env:"NESTED2_"`
}
```

Prefixes are inherited through additional layers of nesting.

#### Ignoring Prefixes

As a special case, it's possible to explicitly ignore prefixes for a field in a nested struct.
To do so, add a carat (`^`) before the variable name:

```go
type Config struct {
  MyInt int `env:"^MY_INT"`
}
```

This will ignore ALL inherited prefixes through all layers of nesting.

> [!IMPORTANT]
> This does NOT ignore prefixes that are the result on a `LookupEnvFunc` (e.g., `PrefixedLookup`).

There could be many use cases for this, but a common example is when you want to support a "global default" while still supporting instance-specific values.
This can be accomplished through the combination of ignoring the prefix and aliases. E.g.,

```go
type NestedConfig struct {
  // It's important that GLOBAL_VAL is listed second
  SomeVal string `env:"VAL,^GLOBAL_VAL"`
}
```

## Documentation

[![Go Reference](https://pkg.go.dev/badge/github.com/chadweimer/vary.svg)](https://pkg.go.dev/github.com/chadweimer/vary/v2)
