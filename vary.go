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
//   - env: specifies the environment variable name (defaults to uppercase field name)
//   - default: specifies a default value if the environment variable is not set
//   - required: indicates the field must be set by an environment variable or default value
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
	"os"
	"reflect"
	"strconv"
	"strings"
)

// --- Error types ---

var (
	// ErrPointerRequired is returned when a non-pointer type is passed to Bind.
	ErrPointerRequired = errors.New("bind requires pointer types")

	// ErrStructRequired is returned when a non-struct type is passed to Bind.
	ErrStructRequired = errors.New("bind requires struct types")
)

// ErrRequiredField is returned when a required configuration field has no value set.
type ErrRequiredField struct {
	FieldName string
}

func (e *ErrRequiredField) Error() string {
	return fmt.Sprintf("required configuration field %s is not set", e.FieldName)
}

// ErrUnsupportedType is returned when a field type is not supported for binding.
type ErrUnsupportedType struct {
	fieldType reflect.Type
}

func (e *ErrUnsupportedType) Error() string {
	return fmt.Sprintf("unsupported field type: %s", e.fieldType)
}

// --- Bind ---

// PrefixHandling defines how the binder should handle prefixes when binding environment variables to struct fields.
type PrefixHandling string

const (
	// PrefixHandlingAlways indicates that the prefix should always be applied to the environment variable names.
	// For example, if the prefix is "APP" and the field is "PORT", the environment variable will be "APP_PORT".
	PrefixHandlingAlways PrefixHandling = "Always"

	// PrefixHandlingPrimary indicates that the prefix should be applied to the primary environment variable name,
	// but the unprefixed name should also be checked as a fallback.
	// For example, if the prefix is "APP" and the field is "PORT", the binder will first check "APP_PORT",
	// and if that is not set, it will check "PORT".
	PrefixHandlingPrimary PrefixHandling = "Primary"

	// PrefixHandlingSecondary indicates that the unprefixed name should be checked first,
	// and the prefixed name should be used as a fallback.
	// For example, if the prefix is "APP" and the field is "PORT", the binder will first check "PORT",
	// and if that is not set, it will check "APP_PORT".
	PrefixHandlingSecondary PrefixHandling = "Secondary"
)

// Binder is responsible for binding environment variables to struct fields based on struct tags.
type Binder struct {
	prefix         string
	prefixHandling PrefixHandling
	strict         bool
	marshalers     map[reflect.Type]marshaler
}

// Option is a function that configures a Binder.
type Option func(*Binder)

// WithPrefix sets the prefix for the Binder.
func WithPrefix(prefix string) Option {
	return func(b *Binder) {
		b.prefix = prefix
	}
}

// WithPrefixHandling sets the prefix handling for the Binder.
func WithPrefixHandling(prefixHandling PrefixHandling) Option {
	return func(b *Binder) {
		b.prefixHandling = prefixHandling
	}
}

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

// DefaultBinder is the default binder used for global calls.
var DefaultBinder = New()

// New creates a new Binder with default settings (no initial prefix, primary prefix handling, strict mode disabled),
// and applies any provided options.
func New(opts ...Option) *Binder {
	b := &Binder{
		prefix:         "",
		prefixHandling: PrefixHandlingPrimary,
		strict:         false,
		marshalers:     make(map[reflect.Type]marshaler),
	}

	b.initDefaultMarshalers()

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// NewWithPrefix creates a new Binder with the specified prefix and prefix handling.
//
// Deprecated: use New with the WithPrefix and WithPrefixHandling options instead.
func NewWithPrefix(prefix string, prefixHandling PrefixHandling) *Binder {
	return New(WithPrefix(prefix), WithPrefixHandling(prefixHandling))
}

// SetPrefix sets the prefix for the default binder.
//
// Deprecated: use New with the WithPrefix option and assign to DefaultBinder instead.
func SetPrefix(prefix string) {
	DefaultBinder.SetPrefix(prefix)
}

// SetPrefix sets the prefix for the binder.
//
// Deprecated: configure the binder using the WithPrefix option during creation instead.
func (b *Binder) SetPrefix(prefix string) {
	b.prefix = prefix
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
//   - Nested structs (recursively bound with optional prefix handling)
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
//   - env: specifies the environment variable name (defaults to uppercase field name if omitted)
//   - default: specifies a default value to use if the environment variable is not set
//   - required: specifies that the field must receive a value from an environment variable or default
//
// Prefix Handling:
// The prefix behavior is controlled by the PrefixHandling parameter passed to WithPrefixHandling.
// See PrefixHandling for more details on how prefixes are applied.
//
// Returns:
//   - ErrPointerRequired if ptr is not a pointer
//   - ErrStructRequired if the pointer does not point to a struct
//   - ErrUnsupportedType if a field type is not supported
//   - ErrRequiredField if a required field is not set
//   - Other errors from type conversions or unmarshaling
func (b *Binder) Bind(ptr any) error {
	val := reflect.ValueOf(ptr)
	if val.Kind() != reflect.Pointer {
		return ErrPointerRequired
	}

	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return ErrStructRequired
	}

	return b.bindStruct(val)
}

func (b *Binder) bindStruct(objVal reflect.Value) error {
	var errs []error

	for i := 0; i < objVal.NumField(); i++ {
		field := objVal.Type().Field(i)
		if field.IsExported() {
			errs = append(errs, b.bindField(field, objVal.Field(i))...)
		}
	}

	return errors.Join(errs...)
}

func (b *Binder) bindField(field reflect.StructField, fieldVal reflect.Value) []error {
	var errs []error
	fieldVal = resolvePointers(fieldVal)

	// If this is a struct, we need to recurse unless it has a registered marshaler
	if fieldVal.Kind() == reflect.Struct && b.getMarshaler(fieldVal.Type()) == nil {
		if err := b.bindStruct(fieldVal); err != nil {
			errs = append(errs, err)
		}
	} else {
		hasDefault, err := b.setToDefault(field, fieldVal)
		if err != nil {
			errs = append(errs, err)
		} else {
			envSet, err := b.setFromEnv(field, fieldVal)
			if err != nil {
				errs = append(errs, err)
			} else if isRequired(field) && !hasDefault && !envSet {
				errs = append(errs, &ErrRequiredField{
					FieldName: field.Name,
				})
			}
		}
	}
	return errs
}

func isRequired(field reflect.StructField) bool {
	reqStr, ok := field.Tag.Lookup("required")
	if !ok {
		return false
	}
	return reqStr == "" || strings.EqualFold(reqStr, "true") || reqStr == "1"
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

func (b *Binder) setFromEnv(field reflect.StructField, val reflect.Value) (bool, error) {
	envName, ok := field.Tag.Lookup("env")
	if !ok {
		envName = strings.ToUpper(field.Name)
	}

	primaryEnvName := envName
	secondaryEnvName := ""
	if b.prefix != "" {
		prefixedEnvName := b.prefix + "_" + envName
		switch b.prefixHandling {
		case PrefixHandlingAlways:
			primaryEnvName = prefixedEnvName
		case PrefixHandlingPrimary:
			primaryEnvName = prefixedEnvName
			secondaryEnvName = envName
		case PrefixHandlingSecondary:
			primaryEnvName = envName
			secondaryEnvName = prefixedEnvName
		default:
			slog.Warn("Unknown prefix handling type. Proceeding with default behavior",
				"prefixHandling", b.prefixHandling,
				"envName", envName)
		}
	}

	resolvedEnvName := primaryEnvName
	envStr, ok := os.LookupEnv(primaryEnvName)
	if !ok && secondaryEnvName != "" {
		envStr, ok = os.LookupEnv(secondaryEnvName)
		if ok {
			resolvedEnvName = secondaryEnvName
		}
	}

	if ok {
		if err := b.set(val, envStr); err != nil {
			if b.strict {
				return false, fmt.Errorf("failed to parse environment variable %s=%q for field %s: %w", resolvedEnvName, envStr, field.Name, err)
			}
			slog.Warn("Failed to convert environment variable. Proceeding with existing value",
				"type", val.Type(),
				"envName", resolvedEnvName,
				"envVal", envStr,
				"error", err)
			return false, nil
		}
		return true, nil
	}

	return false, nil
}

func (b *Binder) set(val reflect.Value, str string) error {
	marshaler := b.getMarshaler(val.Type())
	if marshaler != nil {
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
		newVal := reflect.MakeSlice(valType, 0, 0)
		if str != "" {
			segments := strings.Split(str, ",")
			newVal = reflect.MakeSlice(valType, 0, len(segments))
			for _, segment := range segments {
				elementPtr := reflect.New(valType.Elem())
				element := resolvePointers(elementPtr)
				if err := b.set(element, strings.TrimSpace(segment)); err != nil {
					return reflect.Zero(valType), err
				}
				newVal = reflect.Append(newVal, elementPtr.Elem())
			}
		}
		return newVal, nil
	}, val.Set)
}

func (b *Binder) convertMap(str string, val reflect.Value) error {
	return convertAndSet(str, func(str string) (reflect.Value, error) {
		valType := val.Type()
		keyType := valType.Key()
		elemType := valType.Elem()

		newMap := reflect.MakeMap(valType)
		if str != "" {
			pairs := strings.Split(str, ",")
			newMap = reflect.MakeMapWithSize(valType, len(pairs))
			for _, pair := range pairs {
				pair = strings.TrimSpace(pair)
				if pair != "" {
					key, elem, err := b.getMapElement(pair, keyType, elemType)
					if err != nil {
						return reflect.Zero(valType), err
					}

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
	if err != nil {
		return err
	}
	setter(typed)
	return nil
}
