package vary

import (
	"encoding"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"slices"
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

// ErrUnsupportedType is returned when a field type is not supported for binding.
type ErrUnsupportedType struct {
	fieldType reflect.Type
}

func (e *ErrUnsupportedType) Error() string {
	return fmt.Sprintf("unsupported field type: %s", e.fieldType)
}

// --- Marshaling ---

var (
	textUnmarshalerType   = reflect.TypeFor[encoding.TextUnmarshaler]()
	binaryUnmarshalerType = reflect.TypeFor[encoding.BinaryUnmarshaler]()
	marshalerTypes        = []reflect.Type{
		textUnmarshalerType,
		binaryUnmarshalerType,
	}
)

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
}

// DefaultBinder is the default binder used for global calls.
var DefaultBinder = New()

// New creates a new Binder with default settings (no initial prefix, primary prefix handling).
func New() *Binder {
	return &Binder{
		prefix:         "",
		prefixHandling: PrefixHandlingPrimary,
	}
}

// NewWithPrefix creates a new Binder with the specified prefix and prefix handling.
func NewWithPrefix(prefix string, prefixHandling PrefixHandling) *Binder {
	return &Binder{
		prefix:         prefix,
		prefixHandling: prefixHandling,
	}
}

// SetPrefix sets the prefix for the default binder.
func SetPrefix(prefix string) {
	DefaultBinder.SetPrefix(prefix)
}

// SetPrefix sets the prefix for the binder.
func (b *Binder) SetPrefix(prefix string) {
	b.prefix = prefix
}

// Bind initializes the supplied object based on associated struct tags using the default binder.
func Bind(ptr any) error {
	return DefaultBinder.Bind(ptr)
}

// Bind initializes the supplied object based on associated struct tags
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
	for i := 0; i < objVal.NumField(); i++ {
		field := objVal.Type().Field(i)
		if !field.IsExported() {
			continue
		}

		fieldVal := resolvePointers(objVal.Field(i))

		// If this is a struct, we need to recurse unless it's a known type we handle
		if fieldVal.Kind() == reflect.Struct && !slices.ContainsFunc(marshalerTypes, fieldVal.Addr().Type().AssignableTo) {
			if err := b.bindStruct(fieldVal); err != nil {
				return err
			}
		} else {
			if err := setToDefault(field, fieldVal); err != nil {
				return err
			}
			b.setFromEnv(field, fieldVal)
		}
	}

	return nil
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

func setToDefault(field reflect.StructField, val reflect.Value) error {
	if defaultStr, ok := field.Tag.Lookup("default"); ok {
		if err := set(val, defaultStr); err != nil {
			return fmt.Errorf("improperly defined default on configuration field %s: %w", field.Name, err)
		}
	}

	return nil
}

func (b *Binder) setFromEnv(field reflect.StructField, val reflect.Value) {
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

	envStr, ok := os.LookupEnv(primaryEnvName)
	if ok {
		envName = primaryEnvName
	} else if secondaryEnvName != "" {
		envStr, ok = os.LookupEnv(secondaryEnvName)
		if ok {
			envName = secondaryEnvName
		}
	}

	if ok {
		if err := set(val, envStr); err != nil {
			slog.Warn("Failed to convert environment variable. Proceeding with existing value",
				"type", val.Type(),
				"envName", envName,
				"envVal", envStr,
				"error", err)
		}
	}
}

func set(val reflect.Value, str string) error {
	valType := val.Type()
	switch valType.Kind() {
	case reflect.Struct:
		return convertStruct(val, str)

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
		return convertSlice(str, val)

	default:
		return &ErrUnsupportedType{valType}
	}
}

func convertSlice(str string, val reflect.Value) error {
	return convertAndSet(str, func(str string) (reflect.Value, error) {
		valType := val.Type()
		newVal := reflect.MakeSlice(valType, 0, 0)
		if str != "" {
			segments := strings.Split(str, ",")
			newVal = reflect.MakeSlice(valType, 0, len(segments))
			for _, segment := range segments {
				elementPtr := reflect.New(valType.Elem())
				element := resolvePointers(elementPtr)
				if err := set(element, strings.TrimSpace(segment)); err != nil {
					return reflect.Zero(valType), err
				}
				newVal = reflect.Append(newVal, elementPtr.Elem())
			}
		}
		return newVal, nil
	}, val.Set)
}

func convertStruct(val reflect.Value, str string) error {
	addr := val.Addr()
	addrType := addr.Type()
	if addrType.AssignableTo(textUnmarshalerType) {
		unmarshaler, ok := addr.Interface().(encoding.TextUnmarshaler)
		if ok {
			return unmarshaler.UnmarshalText([]byte(str))
		}
	} else if addrType.AssignableTo(binaryUnmarshalerType) {
		marshaler, ok := addr.Interface().(encoding.BinaryUnmarshaler)
		if ok {
			return marshaler.UnmarshalBinary([]byte(str))
		}
	}
	return &ErrUnsupportedType{val.Type()}
}

func convertAndSet[T any](str string, converter func(str string) (T, error), setter func(val T)) error {
	typed, err := converter(str)
	if err != nil {
		return err
	}
	setter(typed)
	return nil
}
