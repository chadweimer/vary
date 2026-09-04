package vary

import (
	"errors"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestBind_Defaults(t *testing.T) {
	type allSupportedTypes struct {
		unexportedInt int `default:"5"` // revive:disable-line:struct-tag

		TestInt            int     `default:"-1"`
		TestInt8           int8    `default:"-0b10"`
		TestInt16          int16   `default:"-0o3"`
		TestInt32          int32   `default:"-0x4"`
		TestInt64          int64   `default:"-5"`
		TestIntArray       []int   `default:"-1,-2"`
		TestIntPtrArray    []*int  `default:"-1,-2"`
		TestIntPtrPtrArray []**int `default:"-1,-2"`
		TestIntPtr         *int    `default:"-1"`
		TestIntPtrPtr      **int   `default:"-1"`

		TestUint      uint   `default:"1"`
		TestUint8     uint8  `default:"0b10"`
		TestUint16    uint16 `default:"0o3"`
		TestUint32    uint32 `default:"0x4"`
		TestUint64    uint64 `default:"5"`
		TestUintArray []uint `default:"1,2"`

		TestFloat32      float32   `default:"1.1"`
		TestFloat64      float64   `default:"2.2"`
		TestFloat64Array []float64 `default:"1.1, 2.2"` // Space after comma is intentional

		TestComplex64  complex64  `default:"1i"`
		TestComplex128 complex128 `default:"2i"`

		TestBool      bool   `default:"true"`
		TestBoolArray []bool `default:"true,false"`

		TestString           string   `default:"Hello, Tests!"`
		TestStringEmptyArray []string `default:""`

		TestDuration      time.Duration   `default:"10s"`
		TestDurationPtr   *time.Duration  `default:"500ms"`
		TestDurationArray []time.Duration `default:"1s, 2m"`

		TestTime    time.Time  `default:"2000-01-02T03:04:05Z"`
		TestTimePtr *time.Time `default:"2000-01-02T03:04:05Z"`

		TestURL      url.URL   `default:"https://example.com"`
		TestURLArray []url.URL `default:"https://example.com,https://example.org"`

		TestMapStringString map[string]string     `default:"k1=v1,k2=v2"`
		TestMapStringInt    map[string]int        `default:"a:1, b:2"`
		TestMapIntDuration  map[int]time.Duration `default:"1=10s, 2=20s"`
		TestMapEmpty        map[string]string     `default:""`
		TestMapColon        map[string]string     `default:"foo:bar, baz:qux"`
		TestMapPtrVal       map[string]*int       `default:"one=1, two=2"`
	}
	tests := []struct {
		name string
		want allSupportedTypes
	}{
		{
			name: "Defaults are set",
			want: allSupportedTypes{
				unexportedInt: 0, // Should be ignored, not set

				TestInt:            -1,
				TestInt8:           -2,
				TestInt16:          -3,
				TestInt32:          -4,
				TestInt64:          -5,
				TestIntArray:       []int{-1, -2},
				TestIntPtrArray:    []*int{new(-1), new(-2)},
				TestIntPtrPtrArray: []**int{new(new(-1)), new(new(-2))},
				TestIntPtr:         new(-1),
				TestIntPtrPtr:      new(new(-1)),

				TestUint:      1,
				TestUint8:     2,
				TestUint16:    3,
				TestUint32:    4,
				TestUint64:    5,
				TestUintArray: []uint{1, 2},

				TestFloat32:      1.1,
				TestFloat64:      2.2,
				TestFloat64Array: []float64{1.1, 2.2},

				TestComplex64:  1i,
				TestComplex128: 2i,

				TestBool:      true,
				TestBoolArray: []bool{true, false},

				TestString:           "Hello, Tests!",
				TestStringEmptyArray: []string{},

				TestDuration:      10 * time.Second,
				TestDurationPtr:   new(500 * time.Millisecond),
				TestDurationArray: []time.Duration{1 * time.Second, 2 * time.Minute},

				TestTime:    time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC),
				TestTimePtr: new(time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)),

				TestURL: url.URL{Scheme: "https", Host: "example.com"},
				TestURLArray: []url.URL{
					{Scheme: "https", Host: "example.com"},
					{Scheme: "https", Host: "example.org"},
				},

				TestMapStringString: map[string]string{"k1": "v1", "k2": "v2"},
				TestMapStringInt:    map[string]int{"a": 1, "b": 2},
				TestMapIntDuration:  map[int]time.Duration{1: 10 * time.Second, 2: 20 * time.Second},
				TestMapEmpty:        map[string]string{},
				TestMapColon:        map[string]string{"foo": "bar", "baz": "qux"},
				TestMapPtrVal:       map[string]*int{"one": new(1), "two": new(2)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got allSupportedTypes
			if err := Bind(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBind_EnvVar(t *testing.T) {
	type testStruct struct {
		TestInt      int               `env:"TEST_INT" default:"1"`
		TestString   string            `env:"TEST_STRING" default:"Default"`
		TestFloat    float32           `env:"TEST_FLOAT" default:"1.1"`
		TestDuration time.Duration     `env:"TEST_DURATION" default:"5s"`
		TestMap      map[string]string `env:"TEST_MAP" default:"a=1"`
	}
	tests := []struct {
		name   string
		env    map[string]string
		prefix string
		want   testStruct
	}{
		{
			name: "Reads envs",
			env: map[string]string{
				"TEST_INT":      "2",
				"TEST_STRING":   "Hello, Tests!",
				"TEST_FLOAT":    "2.2",
				"TEST_DURATION": "15s",
				"TEST_MAP":      "k1=v1,k2=v2",
			},
			want: testStruct{
				TestInt:      2,
				TestString:   "Hello, Tests!",
				TestFloat:    2.2,
				TestDuration: 15 * time.Second,
				TestMap:      map[string]string{"k1": "v1", "k2": "v2"},
			},
		},
		{
			name: "Handles unset env",
			env:  map[string]string{},
			want: testStruct{
				TestInt:      1,
				TestString:   "Default",
				TestFloat:    1.1,
				TestDuration: 5 * time.Second,
				TestMap:      map[string]string{"a": "1"},
			},
		},
		{
			name: "Handles invalid env",
			env: map[string]string{
				"TEST_INT":      "3a",
				"TEST_FLOAT":    "2.c",
				"TEST_DURATION": "invalid",
			},
			want: testStruct{
				TestInt:      1,
				TestString:   "Default",
				TestFloat:    1.1,
				TestDuration: 5 * time.Second,
				TestMap:      map[string]string{"a": "1"},
			},
		},
		{
			name: "App-specific Env takes precedence by default",
			env: map[string]string{
				"TEST_INT":       "2",
				"TEST_FLOAT":     "2.2",
				"APP_TEST_FLOAT": "3.3",
			},
			prefix: "APP",
			want: testStruct{
				TestInt:      2,
				TestString:   "Default",
				TestFloat:    3.3,
				TestDuration: 5 * time.Second,
				TestMap:      map[string]string{"a": "1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got testStruct
			DefaultBinder = New(WithLookupEnv(mapLookupEnv(tt.env)))
			SetPrefix(tt.prefix)
			if err := Bind(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBind_PrefixHandling(t *testing.T) {
	type testStruct struct {
		SomeVar    int `env:"SOME_VAR" default:"1"`
		AnotherVar int `env:"ANOTHER_VAR" default:"1"`
	}
	tests := []struct {
		name           string
		env            map[string]string
		prefix         string
		prefixHandling PrefixHandling
		strict         bool
		want           testStruct
		errChecker     func(error) bool
	}{
		{
			name: "Always",
			env: map[string]string{
				"SOME_VAR":     "2",
				"APP_SOME_VAR": "3",
				"ANOTHER_VAR":  "4",
			},
			prefix:         "APP",
			prefixHandling: PrefixHandlingAlways,
			strict:         false,
			want: testStruct{
				SomeVar:    3, // Prefixed value is used
				AnotherVar: 1, // Not set, should use default
			},
			errChecker: func(err error) bool {
				return err == nil
			},
		},
		{
			name: "Primary",
			env: map[string]string{
				"SOME_VAR":     "2",
				"APP_SOME_VAR": "3",
				"ANOTHER_VAR":  "4",
			},
			prefix:         "APP",
			prefixHandling: PrefixHandlingPrimary,
			strict:         false,
			want: testStruct{
				SomeVar:    3, // Prefixed value is used
				AnotherVar: 4, // Not set with prefix, should use unprefixed value
			},
			errChecker: func(err error) bool {
				return err == nil
			},
		},
		{
			name: "Secondary",
			env: map[string]string{
				"SOME_VAR":     "2",
				"APP_SOME_VAR": "3",
				"ANOTHER_VAR":  "4",
			},
			prefix:         "APP",
			prefixHandling: PrefixHandlingSecondary,
			strict:         false,
			want: testStruct{
				SomeVar:    2, // Prefixed value is ignored, should use unprefixed value
				AnotherVar: 4, // Not set with prefix, should use unprefixed value
			},
			errChecker: func(err error) bool {
				return err == nil
			},
		},
		{
			name: "Invalid with permissive mode",
			env: map[string]string{
				"SOME_VAR":     "2",
				"APP_SOME_VAR": "3",
				"ANOTHER_VAR":  "4",
			},
			prefix:         "APP",
			prefixHandling: PrefixHandling("Invalid"),
			strict:         false,
			want: testStruct{
				SomeVar:    3, // Prefixed value is used
				AnotherVar: 4, // Not set with prefix, should use unprefixed value
			},
			errChecker: func(err error) bool {
				return err == nil
			},
		},
		{
			name: "Invalid with strict mode",
			env: map[string]string{
				"SOME_VAR":     "2",
				"APP_SOME_VAR": "3",
				"ANOTHER_VAR":  "4",
			},
			prefix:         "APP",
			prefixHandling: PrefixHandling("Invalid"),
			strict:         true,
			errChecker: func(err error) bool {
				return err != nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got testStruct
			binder := NewWithPrefix(tt.prefix, tt.prefixHandling).
				With(WithStrict(tt.strict), WithLookupEnv(mapLookupEnv(tt.env)))
			err := binder.Bind(&got)
			if !tt.errChecker(err) {
				t.Fatalf("Bind() error = %v, failed errChecker", err)
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBind_Required(t *testing.T) {
	type testStruct struct {
		RequiredField   string `env:"REQ_FIELD" required:"true"`
		RequiredNum     int    `env:"REQ_NUM" required:"1"`
		RequiredWithDef string `env:"REQ_DEF" required:"true" default:"fallback"`
		OptionalField   string `env:"OPT_FIELD"`
	}

	tests := []struct {
		name       string
		env        map[string]string
		want       testStruct
		wantErr    bool
		errChecker func(error) bool
	}{
		{
			name:    "Missing all required fields",
			env:     map[string]string{},
			wantErr: true,
			errChecker: func(err error) bool {
				var reqErr *ErrRequiredField
				return errors.As(err, &reqErr)
			},
		},
		{
			name: "Missing one required field",
			env: map[string]string{
				"REQ_FIELD": "hello",
			},
			wantErr: true,
			errChecker: func(err error) bool {
				var reqErr *ErrRequiredField
				return errors.As(err, &reqErr)
			},
		},
		{
			name: "All required fields provided",
			env: map[string]string{
				"REQ_FIELD": "hello",
				"REQ_NUM":   "42",
			},
			want: testStruct{
				RequiredField:   "hello",
				RequiredNum:     42,
				RequiredWithDef: "fallback",
			},
			wantErr: false,
		},
		{
			name: "All fields provided",
			env: map[string]string{
				"REQ_FIELD": "hello",
				"REQ_NUM":   "42",
				"REQ_DEF":   "custom",
				"OPT_FIELD": "optional",
			},
			want: testStruct{
				RequiredField:   "hello",
				RequiredNum:     42,
				RequiredWithDef: "custom",
				OptionalField:   "optional",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got testStruct
			binder := New(WithLookupEnv(mapLookupEnv(tt.env)))
			err := binder.Bind(&got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Bind() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.errChecker != nil && !tt.errChecker(err) {
					t.Errorf("Bind() error = %v, failed errChecker", err)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBind_Strict(t *testing.T) {
	type testStruct struct {
		Port int `env:"PORT" default:"8080"`
	}

	tests := []struct {
		name    string
		env     map[string]string
		strict  bool
		want    testStruct
		wantErr bool
	}{
		{
			name: "Permissive mode ignores invalid env",
			env: map[string]string{
				"PORT": "not-a-port",
			},
			strict:  false,
			want:    testStruct{Port: 8080},
			wantErr: false,
		},
		{
			name: "Permissive mode parses valid env",
			env: map[string]string{
				"PORT": "9000",
			},
			strict:  false,
			want:    testStruct{Port: 9000},
			wantErr: false,
		},
		{
			name: "Strict mode returns error on invalid env",
			env: map[string]string{
				"PORT": "not-a-port",
			},
			strict:  true,
			wantErr: true,
		},
		{
			name: "SetStrict enables strict mode on binder",
			env: map[string]string{
				"PORT": "not-a-port",
			},
			strict:  true,
			wantErr: true,
		},
		{
			name: "Strict mode parses valid env",
			env: map[string]string{
				"PORT": "9000",
			},
			strict:  true,
			want:    testStruct{Port: 9000},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got testStruct
			binder := New(WithStrict(tt.strict), WithLookupEnv(mapLookupEnv(tt.env)))
			err := binder.Bind(&got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Bind() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBind_BadValuesReturnError(t *testing.T) {
	//revive:disable:struct-tag
	type goodInt struct {
		TestInt int `default:"1"`
	}
	type badInt struct {
		TestInt int `default:"a"`
	}
	type badUint struct {
		TestUint uint `default:"b"`
	}
	type badFloat struct {
		TestFloat float32 `default:"c"`
	}
	type badComplex struct {
		TestComplex complex64 `default:"d"`
	}
	type badBool struct {
		TestBool bool `default:"e"`
	}
	type badSlice struct {
		TestSlice []int `default:"1,2,f"`
	}
	type badDuration struct {
		TestDuration time.Duration `default:"not_a_duration"`
	}
	type badMapFormat struct {
		TestMap map[string]string `default:"invalid_entry_no_delim"`
	}
	type badMapKey struct {
		TestMap map[int]string `default:"invalid_key=value"`
	}
	type badMapVal struct {
		TestMap map[string]int `default:"key=invalid_val"`
	}
	type unsupportedChan struct {
		TestChan chan int `default:"chan"`
	}
	//revive:disable:enable-tag
	tests := []struct {
		name       string
		arg        any
		errChecker func(error) (error, bool)
	}{
		{
			name: "Int",
			arg:  &badInt{},
			errChecker: func(got error) (error, bool) {
				want := &strconv.NumError{
					Func: "ParseInt",
					Num:  "a",
					Err:  errors.New(""),
				}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Nested Int",
			arg: &struct {
				BadInt badInt
			}{
				BadInt: badInt{},
			},
			errChecker: func(got error) (error, bool) {
				want := &strconv.NumError{
					Func: "ParseInt",
					Num:  "a",
					Err:  errors.New(""),
				}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Uint",
			arg:  &badUint{},
			errChecker: func(got error) (error, bool) {
				want := &strconv.NumError{
					Func: "ParseUint",
					Num:  "b",
					Err:  errors.New(""),
				}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Float",
			arg:  &badFloat{},
			errChecker: func(got error) (error, bool) {
				want := &strconv.NumError{
					Func: "ParseFloat",
					Num:  "c",
					Err:  errors.New(""),
				}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Complex",
			arg:  &badComplex{},
			errChecker: func(got error) (error, bool) {
				want := &strconv.NumError{
					Func: "ParseComplex",
					Num:  "d",
					Err:  errors.New(""),
				}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Bool",
			arg:  &badBool{},
			errChecker: func(got error) (error, bool) {
				want := &strconv.NumError{
					Func: "ParseBool",
					Num:  "e",
					Err:  errors.New(""),
				}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Slice",
			arg:  &badSlice{},
			errChecker: func(got error) (error, bool) {
				want := &strconv.NumError{
					Func: "ParseInt",
					Num:  "f",
					Err:  errors.New(""),
				}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Duration",
			arg:  &badDuration{},
			errChecker: func(got error) (error, bool) {
				return errors.New("time: invalid duration"), got != nil
			},
		},
		{
			name: "Map Format",
			arg:  &badMapFormat{},
			errChecker: func(got error) (error, bool) {
				return errors.New("map: invalid format"), got != nil
			},
		},
		{
			name: "Map Key",
			arg:  &badMapKey{},
			errChecker: func(got error) (error, bool) {
				want := &strconv.NumError{
					Func: "ParseInt",
					Num:  "invalid_key",
					Err:  errors.New(""),
				}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Map Value",
			arg:  &badMapVal{},
			errChecker: func(got error) (error, bool) {
				want := &strconv.NumError{
					Func: "ParseInt",
					Num:  "invalid_val",
					Err:  errors.New(""),
				}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Unsupported type (chan)",
			arg:  &unsupportedChan{},
			errChecker: func(got error) (error, bool) {
				want := &ErrUnsupportedType{}
				return want, errors.As(got, new(want))
			},
		},
		{
			name: "Not a pointer",
			arg:  goodInt{},
			errChecker: func(got error) (error, bool) {
				want := ErrPointerRequired
				return want, errors.Is(got, want)
			},
		},
		{
			name: "Not a struct",
			arg:  new("foobar"),
			errChecker: func(got error) (error, bool) {
				want := ErrStructRequired
				return want, errors.Is(got, want)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Bind(tt.arg); err != nil {
				if tt.errChecker != nil {
					if want, ok := tt.errChecker(err); !ok {
						t.Errorf("Bind() = %v, want = %v", err, want)
					}
				}
			} else {
				t.Error("Bind() did not error")
			}
		})
	}
}
