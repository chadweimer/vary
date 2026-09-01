package vary

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"time"
)

// ErrInvalidDecoder is returned when an invalid decoder function is registered.
var ErrInvalidDecoder = errors.New("decoder must be a function with signature func(string) (T, error) or func(string, T) error, where T is not a pointer type")

type decoderEntry struct {
	fn        reflect.Value
	isMutator bool
}

// RegisterDecoder registers a custom decoder function with the default binder.
func RegisterDecoder(decoder any) error {
	return DefaultBinder.RegisterDecoder(decoder)
}

// RegisterDecoder registers a custom decoder function for a specific type or interface.
// The decoder must be a function with signature:
//   - func(string) (T, error) to parse and return a new value of type T, or
//   - func(string, T) error to unmarshal into an interface of type T.
//
// T must not be a pointer type in both forms.
//
// Returns ErrInvalidDecoder if the supplied function does not match a supported signature.
func (b *Binder) RegisterDecoder(decoder any) error {
	if decoder == nil {
		return ErrInvalidDecoder
	}

	val := reflect.ValueOf(decoder)
	typ := val.Type()

	if typ.Kind() != reflect.Func {
		return ErrInvalidDecoder
	}

	// Form 1: func(string) (T, error)
	if typ.NumIn() == 1 && typ.In(0).Kind() == reflect.String && typ.NumOut() == 2 && typ.Out(0).Kind() != reflect.Pointer && typ.Out(1).AssignableTo(errorType) {
		targetType := typ.Out(0)
		return b.registerDecoder(targetType, decoderEntry{
			fn:        val,
			isMutator: false,
		})
	}

	// Form 2: func(string, T) error
	if typ.NumIn() == 2 && typ.In(0).Kind() == reflect.String && typ.In(1).Kind() == reflect.Interface && typ.NumOut() == 1 && typ.Out(0).AssignableTo(errorType) {
		targetType := typ.In(1)
		return b.registerDecoder(targetType, decoderEntry{
			fn:        val,
			isMutator: true,
		})
	}

	return ErrInvalidDecoder
}

func (b *Binder) registerDecoder(targetType reflect.Type, entry decoderEntry) error {
	if b.decoders == nil {
		b.decoders = make(map[reflect.Type]decoderEntry)
	}

	b.decoders[targetType] = entry
	return nil
}

func (b *Binder) initDefaultDecoders() {
	_ = b.RegisterDecoder(func(s string) (time.Duration, error) {
		return time.ParseDuration(s)
	})
	_ = b.RegisterDecoder(func(s string, u encoding.TextUnmarshaler) error {
		return u.UnmarshalText([]byte(s))
	})
	_ = b.RegisterDecoder(func(s string, u encoding.BinaryUnmarshaler) error {
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
