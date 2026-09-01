package vary

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"time"
)

type decoderEntry struct {
	fn        reflect.Value
	isMutator bool
}

// RegisterDecoder registers a custom decoder function for a specific type T, where T must not be an interface or pointer type.
// The decoder function must return a new value of type T.
//
// Returns an error if an argument is nil or the supplied function does not match a supported signature.
func RegisterDecoder[T any](b *Binder, decoder func(string) (T, error)) error {
	if b == nil {
		return errors.New("binder must not be nil")
	}
	if decoder == nil {
		return errors.New("decoder must not be nil")
	}

	val := reflect.ValueOf(decoder)
	targetType := reflect.TypeFor[T]()
	if targetType.Kind() != reflect.Pointer && targetType.Kind() != reflect.Interface {
		b.registerDecoder(targetType, decoderEntry{
			fn:        val,
			isMutator: false,
		})
		return nil
	}

	return errors.New("decoder must be a function with signature func(string) (T, error), where T is not an interface or pointer type")
}

// RegisterMutatingDecoder registers a custom decoder function for an interface of type T, where T must be an interface type.
// The decoder function must use the supplied object of type T to unmarshal and mutate in-place.
//
// Returns an error if an argument is nil or the supplied function does not match a supported signature.
func RegisterMutatingDecoder[T any](b *Binder, decoder func(string, T) error) error {
	if b == nil {
		return errors.New("binder must not be nil")
	}
	if decoder == nil {
		return errors.New("decoder must not be nil")
	}

	val := reflect.ValueOf(decoder)
	targetType := reflect.TypeFor[T]()
	if targetType.Kind() == reflect.Interface {
		b.registerDecoder(targetType, decoderEntry{
			fn:        val,
			isMutator: true,
		})
		return nil
	}

	return errors.New("decoder must be a function with signature func(string, T) error, where T is an interface")
}

func (b *Binder) registerDecoder(targetType reflect.Type, entry decoderEntry) {
	if b.decoders == nil {
		b.decoders = make(map[reflect.Type]decoderEntry)
	}

	b.decoders[targetType] = entry
}

func (b *Binder) initDefaultDecoders() {
	_ = RegisterDecoder(b, time.ParseDuration)
	_ = RegisterMutatingDecoder(b, func(s string, u encoding.TextUnmarshaler) error {
		return u.UnmarshalText([]byte(s))
	})
	_ = RegisterMutatingDecoder(b, func(s string, u encoding.BinaryUnmarshaler) error {
		return u.UnmarshalBinary([]byte(s))
	})
}

func (b *Binder) hasDecoder(t reflect.Type) bool {
	if _, ok := b.decoders[t]; ok {
		return true
	}

	for targetType := range b.decoders {
		if targetType.Kind() == reflect.Interface && reflect.PointerTo(t).Implements(targetType) {
			return true
		}
	}
	return false
}

func (b *Binder) decode(val reflect.Value, str string) (bool, error) {
	valType := val.Type()

	// 1. Direct type match on valType
	if entry, ok := b.decoders[valType]; ok {
		return true, b.invokeDecoderEntry(val, entry, valType, str)
	}

	// 2. Interface match
	for targetType, entry := range b.decoders {
		if targetType.Kind() == reflect.Interface && val.CanAddr() && reflect.PointerTo(valType).Implements(targetType) {
			return true, b.invokeDecoderEntry(val, entry, targetType, str)
		}
	}

	return false, nil
}

func (b *Binder) invokeDecoderEntry(val reflect.Value, entry decoderEntry, targetType reflect.Type, str string) error {
	if entry.isMutator {
		if targetType.Kind() != reflect.Interface {
			return fmt.Errorf("mutator decoders must be for interface types, got %s", targetType)
		}
		if !val.CanAddr() || !val.Addr().Type().Implements(targetType) {
			return fmt.Errorf("value of type %s does not implement interface %s", val.Type(), targetType)
		}

		args := []reflect.Value{reflect.ValueOf(str), val.Addr()}
		results := entry.fn.Call(args)
		if !results[0].IsNil() {
			return results[0].Interface().(error)
		}
		return nil
	}

	results := entry.fn.Call([]reflect.Value{reflect.ValueOf(str)})
	if !results[1].IsNil() {
		return results[1].Interface().(error)
	}

	val.Set(results[0])
	return nil
}
