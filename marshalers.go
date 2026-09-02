package vary

import (
	"encoding"
	"errors"
	"reflect"
	"time"
)

type marshaler interface {
	Decode(val reflect.Value, str string) error
}

type standardMarshaler[T any] func(string) (T, error)

func (d standardMarshaler[T]) Decode(val reflect.Value, str string) error {
	result, err := d(str)
	if err == nil {
		val.Set(reflect.ValueOf(result))
	}

	return err
}

type mutatingMarshaler[T any] func(string, T) error

func (d mutatingMarshaler[T]) Decode(val reflect.Value, str string) error {
	return d(str, val.Addr().Interface().(T))
}

// RegisterMarshaler registers a custom marshaler function for a specific type T, where T must not be an interface or pointer type.
// The marshaler function must return a new value of type T.
//
// Returns an error if an argument is nil or the supplied function does not match a supported signature.
func RegisterMarshaler[T any](b *Binder, marshaler func(string) (T, error)) error {
	if b == nil {
		return errors.New("binder must not be nil")
	}
	if marshaler == nil {
		return errors.New("marshaler must not be nil")
	}

	targetType := reflect.TypeFor[T]()
	if targetType.Kind() != reflect.Pointer && targetType.Kind() != reflect.Interface {
		b.marshalers[targetType] = standardMarshaler[T](marshaler)
		return nil
	}

	return errors.New("marshaler must be a function with signature func(string) (T, error), where T is not an interface or pointer type")
}

// RegisterMutatingMarshaler registers a custom marshaler function for an interface of type T, where T must be an interface type.
// The marshaler function must use the supplied object of type T to unmarshal and mutate in-place.
//
// Returns an error if an argument is nil or the supplied function does not match a supported signature.
func RegisterMutatingMarshaler[T any](b *Binder, marshaler func(string, T) error) error {
	if b == nil {
		return errors.New("binder must not be nil")
	}
	if marshaler == nil {
		return errors.New("marshaler must not be nil")
	}

	targetType := reflect.TypeFor[T]()
	if targetType.Kind() == reflect.Interface {
		b.marshalers[targetType] = mutatingMarshaler[T](marshaler)
		return nil
	}

	return errors.New("marshaler must be a function with signature func(string, T) error, where T is an interface")
}

func (b *Binder) initDefaultMarshalers() {
	_ = RegisterMarshaler(b, time.ParseDuration)
	_ = RegisterMutatingMarshaler(b, func(s string, u encoding.TextUnmarshaler) error {
		return u.UnmarshalText([]byte(s))
	})
	_ = RegisterMutatingMarshaler(b, func(s string, u encoding.BinaryUnmarshaler) error {
		return u.UnmarshalBinary([]byte(s))
	})
}

func (b *Binder) getMarshaler(t reflect.Type) marshaler {
	// 1. Direct type match on valType
	if marshaler, ok := b.marshalers[t]; ok {
		return marshaler
	}

	// 2. Interface match
	for targetType, marshaler := range b.marshalers {
		if targetType.Kind() == reflect.Interface && reflect.PointerTo(t).Implements(targetType) {
			return marshaler
		}
	}
	return nil
}
