package vary

import (
	"encoding"
	"errors"
	"reflect"
	"time"
)

type marshaler interface {
	Marshal(val reflect.Value, str string) error
}

type standardMarshaler[T any] func(string) (T, error)

func (d standardMarshaler[T]) Marshal(val reflect.Value, str string) error {
	return convertAndSet(str, d, func(result T) {
		val.Set(reflect.ValueOf(result))
	})
}

type mutatingMarshaler[T any] func(string, T) error

func (d mutatingMarshaler[T]) Marshal(val reflect.Value, str string) error {
	return d(str, val.Addr().Interface().(T))
}

// RegisterMarshaler calls AddMarshaler on the provided Binder.
//
// Deprecated: Use AddMarshaler instead.
func RegisterMarshaler[T any](b *Binder, marshaler func(string) (T, error)) error {
	if b == nil {
		return errors.New("binder must not be nil")
	}

	return b.AddMarshaler(marshaler)
}

// AddMarshaler calls DefaultBinder.AddMarshaler() to register a custom marshaler function globally.
func AddMarshaler[T any](marshaler func(string) (T, error)) error {
	return DefaultBinder.AddMarshaler(marshaler)
}

// AddMarshaler registers a custom marshaler function for a specific type T, where T must not be an interface or pointer type.
// The marshaler function must return a new value of type T.
//
// Returns an error if an argument is nil or the supplied function does not match a supported signature.
func (b *Binder) AddMarshaler[T any](marshaler func(string) (T, error)) error {
	if marshaler == nil {
		return errors.New("marshaler must not be nil")
	}

	if targetType := reflect.TypeFor[T](); targetType.Kind() != reflect.Pointer && targetType.Kind() != reflect.Interface {
		b.addMarshaler(targetType, standardMarshaler[T](marshaler))
		return nil
	}

	return errors.New("marshaler must be a function with signature func(string) (T, error), where T is not an interface or pointer type")
}

// RegisterMutatingMarshaler calls AddMutatingMarshaler on the provided Binder.
//
// Deprecated: Use AddMutatingMarshaler instead.
func RegisterMutatingMarshaler[T any](b *Binder, marshaler func(string, T) error) error {
	if b == nil {
		return errors.New("binder must not be nil")
	}

	return b.AddMutatingMarshaler(marshaler)
}

// AddMutatingMarshaler calls DefaultBinder.AddMutatingMarshaler() to register a custom mutating marshaler function globally.
func AddMutatingMarshaler[T any](marshaler func(string, T) error) error {
	return DefaultBinder.AddMutatingMarshaler(marshaler)
}

// AddMutatingMarshaler registers a custom marshaler function for an interface of type T, where T must be an interface type.
// The marshaler function must use the supplied object of type T to unmarshal and mutate in-place.
//
// During the binding processes, the marshalers are evaluated in the order they were registered.
// The first registered interface that matches the type of the field being bound will be used.
//
// Returns an error if an argument is nil or the supplied function does not match a supported signature.
func (b *Binder) AddMutatingMarshaler[T any](marshaler func(string, T) error) error {
	if marshaler == nil {
		return errors.New("marshaler must not be nil")
	}

	if targetType := reflect.TypeFor[T](); targetType.Kind() == reflect.Interface {
		b.addMarshaler(targetType, mutatingMarshaler[T](marshaler))
		return nil
	}

	return errors.New("marshaler must be a function with signature func(string, T) error, where T is an interface")
}

// ClearMarshalers returns an Option that clears all marshalers from the Binder.
// When used, this option will remove all previously registered marshalers, including both default and custom ones.
func ClearMarshalers() Option {
	return func(b *Binder) {
		clear(b.marshalers)
		b.orderedMarshalers = b.orderedMarshalers[:0]
	}
}

func (b *Binder) addMarshaler(targetType reflect.Type, marshaler marshaler) {
	b.marshalers[targetType], b.orderedMarshalers = marshaler, append(b.orderedMarshalers, targetType)
}

func (b *Binder) initDefaultMarshalers() {
	_ = b.AddMarshaler(time.ParseDuration)
	_ = b.AddMutatingMarshaler(func(s string, u encoding.TextUnmarshaler) error {
		return u.UnmarshalText([]byte(s))
	})
	_ = b.AddMutatingMarshaler(func(s string, u encoding.BinaryUnmarshaler) error {
		return u.UnmarshalBinary([]byte(s))
	})
}

func (b *Binder) getMarshaler(targetType reflect.Type) marshaler {
	// 1. Direct type match on valType
	if m, ok := b.marshalers[targetType]; ok {
		return m
	}

	// 2. Interface match
	// It's important to check these in the order of registration, as the first registered interface that matches will be used.
	for _, iface := range b.orderedMarshalers {
		if m := b.marshalers[iface]; iface.Kind() == reflect.Interface && reflect.PointerTo(targetType).Implements(iface) {
			return m
		}
	}
	return nil
}
