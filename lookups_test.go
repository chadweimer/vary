package vary

import "testing"

func TestMapLookup(t *testing.T) {
	t.Run("MapLookup returns correct values", func(t *testing.T) {
		env := MapLookup(map[string]string{"KEY": "value"})

		if val, ok := env("KEY"); !ok || val != "value" {
			t.Errorf("MapLookup(KEY) = %v, %v; want value, true", val, ok)
		}
		if val, ok := env("OTHER"); ok || val != "" {
			t.Errorf("MapLookup(OTHER) = %v, %v; want \"\", false", val, ok)
		}
	})
}

func TestPrefixedLookup(t *testing.T) {
	t.Run("PrefixedLookup returns value with prefix", func(t *testing.T) {
		env := MapLookup(map[string]string{"PREFIX_KEY": "value"})
		prefixed := PrefixedLookup("PREFIX_", env)

		if val, ok := prefixed("KEY"); !ok || val != "value" {
			t.Errorf("PrefixedLookup(KEY) = %v, %v; want value, true", val, ok)
		}
		if val, ok := prefixed("OTHER"); ok || val != "" {
			t.Errorf("PrefixedLookup(OTHER) = %v, %v; want \"\", false", val, ok)
		}
	})
}

func TestCompositeLookup(t *testing.T) {
	t.Run("CompositeLookup returns first successful result", func(t *testing.T) {
		env1 := MapLookup(map[string]string{"KEY1": "value1"})
		env2 := MapLookup(map[string]string{"KEY1": "value2", "KEY2": "value2"})
		composite := CompositeLookup(env1, env2)

		if val, ok := composite("KEY1"); !ok || val != "value1" {
			t.Errorf("CompositeLookup(KEY1) = %v, %v; want value1, true", val, ok)
		}
		if val, ok := composite("KEY2"); !ok || val != "value2" {
			t.Errorf("CompositeLookup(KEY2) = %v, %v; want value2, true", val, ok)
		}
		if val, ok := composite("KEY3"); ok || val != "" {
			t.Errorf("CompositeLookup(KEY3) = %v, %v; want \"\", false", val, ok)
		}
	})
}
