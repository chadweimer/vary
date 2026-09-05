package vary

import (
	"errors"
	"fmt"
	"reflect"
)

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
	FieldType reflect.Type
}

func (e *ErrUnsupportedType) Error() string {
	return fmt.Sprintf("unsupported field type: %s", e.FieldType)
}
