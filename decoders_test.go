package vary

import (
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

type testCustomSize struct {
	W int
	H int
}

func parseSize(s string) (testCustomSize, error) {
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return testCustomSize{}, errors.New("invalid size format, expected WxH")
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return testCustomSize{}, err
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return testCustomSize{}, err
	}
	return testCustomSize{W: w, H: h}, nil
}

func TestBinder_RegisterDecoder(t *testing.T) {
	type config struct {
		Point       testCustomPoint            `env:"POINT" default:"10:20"`
		Points      []testCustomPoint          `env:"POINTS" default:"1:2,3:4"`
		Size        testCustomSize             `env:"SIZE" default:"100x200"`
		PointMap    map[string]testCustomPoint `env:"POINT_MAP" default:"a=5:6,b=7:8"`
		KeyPointMap map[testCustomPoint]string `env:"KEY_POINT_MAP" default:"9:10=first,11:12=second"`
	}

	binder := New()
	if err := binder.RegisterDecoder(parsePoint); err != nil {
		t.Fatalf("RegisterDecoder(parsePoint) unexpected error: %v", err)
	}
	if err := binder.RegisterDecoder(parseSize); err != nil {
		t.Fatalf("RegisterDecoder(parseSize) unexpected error: %v", err)
	}

	t.Run("Defaults", func(t *testing.T) {
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
			Size: testCustomSize{W: 100, H: 200},
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
	})

	t.Run("FromEnv", func(t *testing.T) {
		t.Setenv("POINT", "50:60")
		t.Setenv("POINTS", "10:11,12:13")
		t.Setenv("SIZE", "500x600")
		t.Setenv("POINT_MAP", "k1=1:2")
		t.Setenv("KEY_POINT_MAP", "3:4=val")

		var cfg config
		if err := binder.Bind(&cfg); err != nil {
			t.Fatalf("Bind() error: %v", err)
		}

		want := config{
			Point: testCustomPoint{X: 50, Y: 60},
			Points: []testCustomPoint{
				{X: 10, Y: 11},
				{X: 12, Y: 13},
			},
			Size: testCustomSize{W: 500, H: 600},
			PointMap: map[string]testCustomPoint{
				"k1": {X: 1, Y: 2},
			},
			KeyPointMap: map[testCustomPoint]string{
				{X: 3, Y: 4}: "val",
			},
		}

		if !reflect.DeepEqual(cfg, want) {
			t.Errorf("Bind() = %+v, want %+v", cfg, want)
		}
	})
}

func TestBinder_RegisterDecoder_InvalidSignature(t *testing.T) {
	binder := New()

	tests := []struct {
		name    string
		decoder any
	}{
		{
			name:    "nil",
			decoder: nil,
		},
		{
			name:    "not a func",
			decoder: "not a function",
		},
		{
			name: "no in params",
			decoder: func() (testCustomPoint, error) {
				return testCustomPoint{}, nil
			},
		},
		{
			name: "three in params",
			decoder: func(s1, s2, s3 string) error {
				return nil
			},
		},
		{
			name: "two in params neither string",
			decoder: func(i, j int) error {
				return nil
			},
		},
		{
			name: "two in params with two out params",
			decoder: func(s1, s2 string) (testCustomPoint, error) {
				return testCustomPoint{}, nil
			},
		},
		{
			name: "int in param single",
			decoder: func(i int) (testCustomPoint, error) {
				return testCustomPoint{}, nil
			},
		},
		{
			name: "one return value not error",
			decoder: func(s string) testCustomPoint {
				return testCustomPoint{}
			},
		},
		{
			name: "three return values",
			decoder: func(s string) (testCustomPoint, int, error) {
				return testCustomPoint{}, 0, nil
			},
		},
		{
			name: "two return values second not error",
			decoder: func(s string) (testCustomPoint, int) {
				return testCustomPoint{}, 0
			},
		},
		{
			name: "two in params return not error",
			decoder: func(s string, p testCustomPoint) int {
				return 0
			},
		},
		{
			name: "pointer in params",
			decoder: func(s string, p *testCustomPoint) error {
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := binder.RegisterDecoder(tt.decoder)
			if !errors.Is(err, ErrInvalidDecoder) {
				t.Errorf("RegisterDecoder() error = %v, want %v", err, ErrInvalidDecoder)
			}
		})
	}
}

func TestRegisterDecoder_Global(t *testing.T) {
	type customScore int
	type config struct {
		Score customScore `env:"SCORE" default:"100"`
	}

	err := RegisterDecoder(func(s string) (customScore, error) {
		val, err := strconv.Atoi(s)
		if err != nil {
			return 0, err
		}
		return customScore(val * 2), nil
	})
	if err != nil {
		t.Fatalf("RegisterDecoder(nil, fn) error: %v", err)
	}

	var cfg config
	if err := Bind(&cfg); err != nil {
		t.Fatalf("Bind() error: %v", err)
	}

	if cfg.Score != 200 {
		t.Errorf("Bind() Score = %d, want 200", cfg.Score)
	}
}

func TestCustomDecoder_StrictAndPermissive(t *testing.T) {
	type config struct {
		Point testCustomPoint `env:"POINT" default:"1:2"`
	}

	t.Run("Permissive ignores invalid env", func(t *testing.T) {
		t.Setenv("POINT", "invalid_point")
		binder := New()
		_ = binder.RegisterDecoder(parsePoint)
		var cfg config
		if err := binder.Bind(&cfg); err != nil {
			t.Fatalf("Bind() unexpected error in permissive mode: %v", err)
		}
		if cfg.Point.X != 1 || cfg.Point.Y != 2 {
			t.Errorf("Bind() Point = %+v, want default {1, 2}", cfg.Point)
		}
	})

	t.Run("Strict returns error on invalid env", func(t *testing.T) {
		t.Setenv("POINT", "invalid_point")
		binder := New(WithStrict(true))
		_ = binder.RegisterDecoder(parsePoint)
		var cfg config
		if err := binder.Bind(&cfg); err == nil {
			t.Fatalf("Bind() expected error in strict mode, got nil")
		}
	})

	t.Run("Bad default returns error", func(t *testing.T) {
		type badConfig struct {
			Point testCustomPoint `default:"invalid"`
		}
		binder := New()
		_ = binder.RegisterDecoder(parsePoint)
		var cfg badConfig
		if err := binder.Bind(&cfg); err == nil {
			t.Fatalf("Bind() expected error on invalid default, got nil")
		}
	})
}
