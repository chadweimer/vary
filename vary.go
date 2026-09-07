// Package vary provides environment variable binding to struct fields.
//
// It automatically populates struct fields from environment variables based on struct tags.
// The package supports a wide range of Go types including primitives, time.Duration, maps,
// slices, custom types with registered marshalers, and custom types that implement standard marshaling interfaces.
//
// Basic Usage:
//
//	type Config struct {
//		Port     int           `env:"PORT" default:"8080"`
//		Debug    bool          `default:"false"` // Will use the all-caps name "DEBUG"
//		Timeout  time.Duration `env:"TIMEOUT" default:"30s"`
//		Database url.URL       `env:"DATABASE_URL" required:"true"`
//		Tags     map[string]string `env:"TAGS" default:"env=dev,tier=frontend"`
//	}
//
//	var cfg Config
//	if err := vary.Bind(&cfg); err != nil {
//		log.Fatal(err)
//	}
//
// Tags:
//   - env: On a regular field, specifies the environment variable name (defaults to uppercase field name if omitted)
//     On a nested struct field, specifies a prefix to prepend to environment variable names within that struct (defaults to an empty string if omitted)
//   - default: Specifies a default value to use if the environment variable is not set
//   - required: Specifies that the field must receive a value from an environment variable or default
//
// Supported Field Types:
//   - String
//   - Integers: int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64
//   - Floating-point: float32, float64
//   - Complex: complex64, complex128
//   - Boolean
//   - time.Duration (parsed using time.ParseDuration, e.g., "5s", "10m", "1h")
//   - Maps of any supported key and value types (e.g., "key1=val1,key2=val2" or "key1:val1,key2:val2")
//   - Slices and Arrays of any supported type (comma-separated values)
//   - Any type implementing encoding.TextUnmarshaler
//   - Any type implementing encoding.BinaryUnmarshaler
//   - Any type with a registered custom marshaler
//   - Nested structs (recursively bound)
package vary

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Binder is responsible for binding environment variables to struct fields based on struct tags.
type Binder struct {
	strict            bool
	lookupEnv         LookupEnvFunc
	marshalers        map[reflect.Type]marshaler
	orderedMarshalers []reflect.Type
}

// Option is a function that configures a Binder.
type Option func(*Binder)

// WithStrict sets the strict error handling mode for the Binder.
//
// When strict mode is enabled (true), any parsing or type conversion errors encountered
// while reading environment variables (e.g., invalid integer syntax, malformed durations,
// or improper map formatting) will cause Bind to return an error.
//
// When strict mode is disabled (false, the default), environment variable conversion errors
// are logged as warnings via slog.Warn, and the field retains its existing or default value.
//
// Note that fields tagged with `required:"true"` that receive neither an environment variable
// nor a default value will always produce an ErrRequiredField error regardless of strict mode.
func WithStrict(strict bool) Option {
	return func(b *Binder) {
		b.strict = strict
	}
}

// WithLookup sets a custom function for looking up environment variables in the Binder.
//
// By default, the Binder uses os.LookupEnv to retrieve environment variable values.
func WithLookup(lookup LookupEnvFunc) Option {
	return func(b *Binder) {
		b.lookupEnv = lookup
	}
}

// DefaultBinder is the default binder used for global calls.
var DefaultBinder = New()

// New creates a new Binder with default settings (strict mode disabled, os.LookupEnv),
// and applies any provided options.
func New(opts ...Option) *Binder {
	b := &Binder{
		strict:            false,
		lookupEnv:         os.LookupEnv,
		marshalers:        make(map[reflect.Type]marshaler),
		orderedMarshalers: make([]reflect.Type, 0),
	}

	b.initDefaultMarshalers()

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// With returns a new Binder that inherits the settings of the current binder and applies any provided options.
func (b *Binder) With(opts ...Option) *Binder {
	newBinder := &Binder{
		strict:            b.strict,
		lookupEnv:         b.lookupEnv,
		marshalers:        maps.Clone(b.marshalers),
		orderedMarshalers: slices.Clone(b.orderedMarshalers),
	}

	for _, opt := range opts {
		opt(newBinder)
	}

	return newBinder
}

// Bind initializes the supplied object based on associated struct tags using the default binder.
func Bind(ptr any) error {
	return DefaultBinder.Bind(ptr)
}

// Bind initializes the supplied object based on associated struct tags.
//
// Supported Types:
// Bind supports the following field types:
//   - String
//   - Integer types: int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64
//   - Floating-point types: float32, float64
//   - Complex types: complex64, complex128
//   - Boolean
//   - time.Duration (parsed using time.ParseDuration)
//   - Maps of any supported key and value types (comma-separated key=value or key:value pairs)
//   - Slices and Arrays of any supported type (values are comma-separated in the environment variable)
//   - Nested structs (recursively bound)
//   - Types implementing encoding.TextUnmarshaler
//   - Types implementing encoding.BinaryUnmarshaler
//   - Types with registered custom marshalers
//
// Marshaler Types:
// Bind recognizes and uses the following standard Go marshaling interfaces:
//   - encoding.TextUnmarshaler: used for converting string values to custom types
//   - encoding.BinaryUnmarshaler: used for binary-encoded environment values
//
// Tags:
// Bind looks for the following struct tags on exported fields:
//   - env: On a regular field, specifies the environment variable name (defaults to uppercase field name if omitted)
//     On a nested struct field, specifies a prefix to prepend to environment variable names within that struct (defaults to an empty string if omitted)
//   - default: Specifies a default value to use if the environment variable is not set
//   - required: Specifies that the field must receive a value from an environment variable or default
//
// Returns:
//   - ErrPointerRequired if ptr is not a pointer
//   - ErrStructRequired if the pointer does not point to a struct
//   - ErrUnsupportedType if a field type is not supported
//   - ErrRequiredField if a required field is not set
//   - Other errors from type conversions or unmarshaling
func (b *Binder) Bind(ptr any) error {
	if val := reflect.ValueOf(ptr); val.Kind() != reflect.Pointer {
		return ErrPointerRequired
	} else if val = val.Elem(); val.Kind() == reflect.Struct {
		return b.bindStruct(val, "")
	}

	return ErrStructRequired
}

func (b *Binder) bindStruct(objVal reflect.Value, prefix string) error {
	var errs []error

	for i := 0; i < objVal.NumField(); i++ {
		if field := objVal.Type().Field(i); field.IsExported() {
			errs = append(errs, b.bindField(field, objVal.Field(i), prefix)...)
		}
	}

	return errors.Join(errs...)
}

func (b *Binder) bindField(field reflect.StructField, fieldVal reflect.Value, prefix string) []error {
	var errs []error
	fieldVal = resolvePointers(fieldVal)

	// If this is a struct, we need to recurse unless it has a registered marshaler
	if fieldVal.Kind() == reflect.Struct && b.getMarshaler(fieldVal.Type()) == nil {
		if fieldPrefix, ok := field.Tag.Lookup("env"); ok {
			prefix += fieldPrefix
		}

		if err := b.bindStruct(fieldVal, prefix); err != nil {
			errs = append(errs, err)
		}
	} else if hasDefault, err := b.setToDefault(field, fieldVal); err != nil {
		errs = append(errs, err)
	} else {
		if envSet, err := b.setFromEnv(field, fieldVal, prefix); err != nil {
			errs = append(errs, err)
		} else if isRequired(field) && !hasDefault && !envSet {
			errs = append(errs, &ErrRequiredField{
				FieldName: field.Name,
			})
		}
	}
	return errs
}

func isRequired(field reflect.StructField) bool {
	reqStr, ok := field.Tag.Lookup("required")
	return ok && (reqStr == "" || strings.EqualFold(reqStr, "true") || reqStr == "1")
}

func resolvePointers(val reflect.Value) reflect.Value {
	for val.Type().Kind() == reflect.Pointer {
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		val = val.Elem()
	}
	return val
}

func (b *Binder) setToDefault(field reflect.StructField, val reflect.Value) (bool, error) {
	if defaultStr, ok := field.Tag.Lookup("default"); ok {
		if err := b.set(val, defaultStr); err != nil {
			return true, fmt.Errorf("improperly defined default on configuration field %s: %w", field.Name, err)
		}

		return true, nil
	}

	return false, nil
}

func (b *Binder) setFromEnv(field reflect.StructField, val reflect.Value, prefix string) (bool, error) {
	envName, ok := field.Tag.Lookup("env")
	if !ok {
		envName = strings.ToUpper(field.Name)
	}

	envStr, ok := b.lookupEnv(prefix + envName)
	if !ok {
		return false, nil
	}

	if err := b.set(val, envStr); err != nil {
		if b.strict {
			return false, fmt.Errorf("failed to parse environment variable %s=%q for field %s: %w", envName, envStr, field.Name, err)
		}
		slog.Warn("Failed to convert environment variable. Proceeding with existing value",
			"type", val.Type(),
			"envName", envName,
			"envVal", envStr,
			"error", err)
		return false, nil
	}
	return true, nil
}

func (b *Binder) set(val reflect.Value, str string) error {
	if marshaler := b.getMarshaler(val.Type()); marshaler != nil {
		return marshaler.Marshal(val, str)
	}

	valType := val.Type()
	switch valType.Kind() {
	case reflect.String:
		val.SetString(str)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return convertAndSet(str, func(str string) (int64, error) {
			return strconv.ParseInt(str, 0, valType.Bits())
		}, val.SetInt)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return convertAndSet(str, func(str string) (uint64, error) {
			return strconv.ParseUint(str, 0, valType.Bits())
		}, val.SetUint)

	case reflect.Float32, reflect.Float64:
		return convertAndSet(str, func(str string) (float64, error) {
			return strconv.ParseFloat(str, valType.Bits())
		}, val.SetFloat)

	case reflect.Complex64, reflect.Complex128:
		return convertAndSet(str, func(str string) (complex128, error) {
			return strconv.ParseComplex(str, valType.Bits())
		}, val.SetComplex)

	case reflect.Bool:
		return convertAndSet(str, strconv.ParseBool, val.SetBool)

	case reflect.Array, reflect.Slice:
		return b.convertSlice(str, val)

	case reflect.Map:
		return b.convertMap(str, val)

	default:
		return &ErrUnsupportedType{valType}
	}
}

func (b *Binder) convertSlice(str string, val reflect.Value) error {
	return convertAndSet(str, func(str string) (reflect.Value, error) {
		valType := val.Type()
		if str == "" {
			return reflect.MakeSlice(valType, 0, 0), nil
		}

		segments := strings.Split(str, ",")
		newVal := reflect.MakeSlice(valType, 0, len(segments))
		for _, segment := range segments {
			elementPtr := reflect.New(valType.Elem())
			element := resolvePointers(elementPtr)
			if err := b.set(element, strings.TrimSpace(segment)); err != nil {
				return reflect.Zero(valType), err
			}
			newVal = reflect.Append(newVal, elementPtr.Elem())
		}
		return newVal, nil
	}, val.Set)
}

func (b *Binder) convertMap(str string, val reflect.Value) error {
	return convertAndSet(str, func(str string) (reflect.Value, error) {
		valType := val.Type()
		if str == "" {
			return reflect.MakeMap(valType), nil
		}

		pairs := strings.Split(str, ",")
		newMap := reflect.MakeMapWithSize(valType, len(pairs))
		for _, pair := range pairs {
			if pair = strings.TrimSpace(pair); pair != "" {
				if key, elem, err := b.getMapElement(pair, valType.Key(), valType.Elem()); err != nil {
					return reflect.Zero(valType), err
				} else {
					newMap.SetMapIndex(key, elem)
				}
			}
		}
		return newMap, nil
	}, val.Set)
}

func (b *Binder) getMapElement(pair string, keyType reflect.Type, elemType reflect.Type) (reflect.Value, reflect.Value, error) {
	keyStr, valStr, found := strings.Cut(pair, "=")
	if !found {
		keyStr, valStr, found = strings.Cut(pair, ":")
	}
	if !found {
		return reflect.Zero(keyType), reflect.Zero(elemType), fmt.Errorf("invalid map entry: must be key=value or key:value: %q", pair)
	}

	keyPtr := reflect.New(keyType)
	keyTarget := resolvePointers(keyPtr)
	if err := b.set(keyTarget, strings.TrimSpace(keyStr)); err != nil {
		return reflect.Zero(keyType), reflect.Zero(elemType), fmt.Errorf("invalid map key %q: %w", keyStr, err)
	}

	elemPtr := reflect.New(elemType)
	elemTarget := resolvePointers(elemPtr)
	if err := b.set(elemTarget, strings.TrimSpace(valStr)); err != nil {
		return reflect.Zero(keyType), reflect.Zero(elemType), fmt.Errorf("invalid map value for key %q: %w", keyStr, err)
	}
	return keyPtr.Elem(), elemPtr.Elem(), nil
}

func convertAndSet[T any](str string, converter func(str string) (T, error), setter func(val T)) error {
	typed, err := converter(str)
	if err == nil {
		setter(typed)
	}
	return err
}
