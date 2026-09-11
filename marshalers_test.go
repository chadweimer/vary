package vary

import (
	"encoding"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type testCustomPoint struct {
	X int
	Y int
}

func parsePoint(s string) (testCustomPoint, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return testCustomPoint{}, errors.New("invalid point format, expected X:Y")
	}
	x, err := strconv.Atoi(parts[0])
	if err != nil {
		return testCustomPoint{}, err
	}
	y, err := strconv.Atoi(parts[1])
	if err != nil {
		return testCustomPoint{}, err
	}
	return testCustomPoint{X: x, Y: y}, nil
}

type sizeParser interface {
	ParseSize(s string) error
}

type testCustomSize struct {
	W int
	H int
}

func (t *testCustomSize) ParseSize(s string) error {
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return errors.New("invalid size format, expected WxH")
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}

	t.W = w
	t.H = h
	return nil
}

func TestBinder_AddMarshaler(t *testing.T) {
	type config struct {
		Point       testCustomPoint            `env:"POINT" default:"10:20"`
		Points      []testCustomPoint          `env:"POINTS" default:"1:2,3:4"`
		PointMap    map[string]testCustomPoint `env:"POINT_MAP" default:"a=5:6,b=7:8"`
		KeyPointMap map[testCustomPoint]string `env:"KEY_POINT_MAP" default:"9:10=first,11:12=second"`
	}

	binder := New()
	if err := binder.AddMarshaler(parsePoint); err != nil {
		t.Fatalf("AddMarshaler(parsePoint) unexpected error: %v", err)
	}

	var cfg config
	if err := binder.Bind(&cfg); err != nil {
		t.Fatalf("Bind() error: %v", err)
	}

	want := config{
		Point: testCustomPoint{X: 10, Y: 20},
		Points: []testCustomPoint{
			{X: 1, Y: 2},
			{X: 3, Y: 4},
		},
		PointMap: map[string]testCustomPoint{
			"a": {X: 5, Y: 6},
			"b": {X: 7, Y: 8},
		},
		KeyPointMap: map[testCustomPoint]string{
			{X: 9, Y: 10}:  "first",
			{X: 11, Y: 12}: "second",
		},
	}

	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Bind() = %+v, want %+v", cfg, want)
	}
}

func TestBinder_AddMarshaler_DefaultBinder(t *testing.T) {
	type config struct {
		Point       testCustomPoint            `env:"POINT" default:"10:20"`
		Points      []testCustomPoint          `env:"POINTS" default:"1:2,3:4"`
		PointMap    map[string]testCustomPoint `env:"POINT_MAP" default:"a=5:6,b=7:8"`
		KeyPointMap map[testCustomPoint]string `env:"KEY_POINT_MAP" default:"9:10=first,11:12=second"`
	}

	if err := AddMarshaler(parsePoint); err != nil {
		t.Fatalf("AddMarshaler unexpected error: %v", err)
	}

	var cfg config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind() error: %v", err)
	}

	want := config{
		Point: testCustomPoint{X: 10, Y: 20},
		Points: []testCustomPoint{
			{X: 1, Y: 2},
			{X: 3, Y: 4},
		},
		PointMap: map[string]testCustomPoint{
			"a": {X: 5, Y: 6},
			"b": {X: 7, Y: 8},
		},
		KeyPointMap: map[testCustomPoint]string{
			{X: 9, Y: 10}:  "first",
			{X: 11, Y: 12}: "second",
		},
	}

	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Bind() = %+v, want %+v", cfg, want)
	}
}

func TestBinder_AddMarshaler_Invalid(t *testing.T) {
	binder := New()

	tests := []struct {
		name        string
		registrator func(b *Binder) error
	}{
		{
			name: "nil marshaler",
			registrator: func(b *Binder) error {
				return b.AddMarshaler[testCustomPoint](nil)
			},
		},
		{
			name: "pointer",
			registrator: func(b *Binder) error {
				return b.AddMarshaler(func(string) (*testCustomPoint, error) {
					return &testCustomPoint{}, nil
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.registrator(binder); err == nil {
				t.Fatalf("AddMarshaler() expected error on invalid marshaler, got nil")
			}
		})
	}
}

func TestBinder_RegisterMarshaler(t *testing.T) {
	binder := New()

	tests := []struct {
		name        string
		registrator func(b *Binder) error
		wantErr     bool
	}{
		{
			name: "Nominal",
			registrator: func(b *Binder) error {
				return RegisterMarshaler(b, parsePoint)
			},
			wantErr: false,
		},
		{
			name: "nil binder",
			registrator: func(b *Binder) error {
				return RegisterMarshaler(nil, parsePoint)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.registrator(binder)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RegisterMarshaler() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBinder_AddMutatingMarshaler(t *testing.T) {
	type config struct {
		Size       testCustomSize            `env:"SIZE" default:"10x20"`
		Sizes      []testCustomSize          `env:"SIZES" default:"1x2,3x4"`
		SizeMap    map[string]testCustomSize `env:"SIZE_MAP" default:"a=5x6,b=7x8"`
		KeySizeMap map[testCustomSize]string `env:"KEY_SIZE_MAP" default:"9x10=first,11x12=second"`
	}

	binder := New()
	if err := binder.AddMutatingMarshaler(func(s string, t sizeParser) error {
		return t.ParseSize(s)
	}); err != nil {
		t.Fatalf("AddMutatingMarshaler unexpected error: %v", err)
	}

	var cfg config
	if err := binder.Bind(&cfg); err != nil {
		t.Fatalf("Bind() error: %v", err)
	}

	want := config{
		Size: testCustomSize{W: 10, H: 20},
		Sizes: []testCustomSize{
			{W: 1, H: 2},
			{W: 3, H: 4},
		},
		SizeMap: map[string]testCustomSize{
			"a": {W: 5, H: 6},
			"b": {W: 7, H: 8},
		},
		KeySizeMap: map[testCustomSize]string{
			{W: 9, H: 10}:  "first",
			{W: 11, H: 12}: "second",
		},
	}

	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Bind() = %+v, want %+v", cfg, want)
	}
}

func TestBinder_AddMutatingMarshaler_DefaultBinder(t *testing.T) {
	type config struct {
		Size       testCustomSize            `env:"SIZE" default:"10x20"`
		Sizes      []testCustomSize          `env:"SIZES" default:"1x2,3x4"`
		SizeMap    map[string]testCustomSize `env:"SIZE_MAP" default:"a=5x6,b=7x8"`
		KeySizeMap map[testCustomSize]string `env:"KEY_SIZE_MAP" default:"9x10=first,11x12=second"`
	}

	if err := AddMutatingMarshaler(func(s string, t sizeParser) error {
		return t.ParseSize(s)
	}); err != nil {
		t.Fatalf("AddMutatingMarshaler unexpected error: %v", err)
	}

	var cfg config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind() error: %v", err)
	}

	want := config{
		Size: testCustomSize{W: 10, H: 20},
		Sizes: []testCustomSize{
			{W: 1, H: 2},
			{W: 3, H: 4},
		},
		SizeMap: map[string]testCustomSize{
			"a": {W: 5, H: 6},
			"b": {W: 7, H: 8},
		},
		KeySizeMap: map[testCustomSize]string{
			{W: 9, H: 10}:  "first",
			{W: 11, H: 12}: "second",
		},
	}

	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Bind() = %+v, want %+v", cfg, want)
	}
}

func TestBinder_RegisterMutatingMarshaler(t *testing.T) {
	binder := New()

	tests := []struct {
		name        string
		registrator func(b *Binder) error
		wantErr     bool
	}{
		{
			name: "Nominal",
			registrator: func(b *Binder) error {
				return RegisterMutatingMarshaler(b, func(string, encoding.TextUnmarshaler) error {
					return nil
				})
			},
			wantErr: false,
		},
		{
			name: "nil binder",
			registrator: func(b *Binder) error {
				return RegisterMutatingMarshaler(nil, func(string, encoding.TextUnmarshaler) error {
					return nil
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.registrator(binder)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RegisterMutatingMarshaler() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBinder_AddMutatingMarshaler_Invalid(t *testing.T) {
	binder := New()

	tests := []struct {
		name        string
		registrator func(b *Binder) error
	}{
		{
			name: "nil marshaler",
			registrator: func(b *Binder) error {
				return b.AddMutatingMarshaler[testCustomPoint](nil)
			},
		},
		{
			name: "non-interface",
			registrator: func(b *Binder) error {
				return b.AddMutatingMarshaler(func(string, testCustomPoint) error {
					return nil
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.registrator(binder); err == nil {
				t.Fatalf("AddMutatingMarshaler() expected error on invalid marshaler, got nil")
			}
		})
	}
}

func TestCustomMarshaler_StrictAndPermissive(t *testing.T) {
	type config struct {
		Point testCustomPoint `env:"POINT" default:"1:2"`
	}
	binder := New()
	_ = binder.AddMarshaler(parsePoint)

	t.Run("Permissive ignores invalid env", func(t *testing.T) {
		binder = binder.With(WithLookup(MapLookup(map[string]string{"POINT": "invalid_point"})))
		var cfg config
		if err := binder.Bind(&cfg); err != nil {
			t.Fatalf("Bind() unexpected error in permissive mode: %v", err)
		}
		if cfg.Point.X != 1 || cfg.Point.Y != 2 {
			t.Errorf("Bind() Point = %+v, want default {1, 2}", cfg.Point)
		}
	})

	t.Run("Strict returns error on invalid env", func(t *testing.T) {
		binder = binder.With(
			WithStrict(true),
			WithLookup(MapLookup(map[string]string{"POINT": "invalid_point"})))
		var cfg config
		if err := binder.Bind(&cfg); err == nil {
			t.Fatalf("Bind() expected error in strict mode, got nil")
		}
	})

	t.Run("Bad default returns error", func(t *testing.T) {
		type badConfig struct {
			Point testCustomPoint `default:"invalid"`
		}
		var cfg badConfig
		if err := binder.Bind(&cfg); err == nil {
			t.Fatalf("Bind() expected error on invalid default, got nil")
		}
	})
}

func TestClearMarshalers(t *testing.T) {
	tests := []struct {
		name    string
		creator func() *Binder
	}{
		{
			name: "New",
			creator: func() *Binder {
				return New(ClearMarshalers())
			},
		},
		{
			name: "With (Defaults)",
			creator: func() *Binder {
				return New().With(ClearMarshalers())
			},
		},
		{
			name: "With (Custom)",
			creator: func() *Binder {
				binder := New()
				_ = binder.AddMarshaler(parsePoint)
				return binder.With(ClearMarshalers())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binder := tt.creator()

			// Ensure the marshalers are cleared
			if len(binder.marshalers) != 0 {
				t.Errorf("Expected all marshalers to be cleared, but found %d", len(binder.marshalers))
			}
			if len(binder.orderedMarshalers) != 0 {
				t.Errorf("Expected all ordered marshalers to be cleared, but found %d", len(binder.orderedMarshalers))
			}
		})
	}
}
