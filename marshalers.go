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
	result, err := d(str)
	if err == nil {
		val.Set(reflect.ValueOf(result))
	}

	return err
}

type mutatingMarshaler[T any] func(string, T) error

func (d mutatingMarshaler[T]) Marshal(val reflect.Value, str string) error {
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
		b.registerMarshaler(targetType, standardMarshaler[T](marshaler))
		return nil
	}

	return errors.New("marshaler must be a function with signature func(string) (T, error), where T is not an interface or pointer type")
}

// RegisterMutatingMarshaler registers a custom marshaler function for an interface of type T, where T must be an interface type.
// The marshaler function must use the supplied object of type T to unmarshal and mutate in-place.
//
// During the binding processes, the marshalers are evaluated in the order they were registered.
// The first registered interface that matches the type of the field being bound will be used.
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
		b.registerMarshaler(targetType, mutatingMarshaler[T](marshaler))
		return nil
	}

	return errors.New("marshaler must be a function with signature func(string, T) error, where T is an interface")
}

func (b *Binder) registerMarshaler(targetType reflect.Type, marshaler marshaler) {
	b.marshalers[targetType] = marshaler
	b.orderedMarshalers = append(b.orderedMarshalers, targetType)
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
	// It's important to check these in the order of registration, as the first registered interface that matches will be used.
	for _, targetType := range b.orderedMarshalers {
		marshaler := b.marshalers[targetType]
		if targetType.Kind() == reflect.Interface && reflect.PointerTo(t).Implements(targetType) {
			return marshaler
		}
	}
	return nil
}
