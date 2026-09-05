package vary

// LookupEnvFunc defines a function type for looking up environment variables.
type LookupEnvFunc func(key string) (string, bool)

// MapLookup returns a LookupEnvFunc that looks up environment variables from the provided map.
func MapLookup(m map[string]string) LookupEnvFunc {
	return func(key string) (string, bool) {
		val, ok := m[key]
		return val, ok
	}
}

// PrefixedLookup returns a LookupEnvFunc that looks up environment variables with the specified prefix.
// The prefix is prepended directly to the key without any separator.
func PrefixedLookup(prefix string, lookup LookupEnvFunc) LookupEnvFunc {
	return func(key string) (string, bool) {
		return lookup(prefix + key)
	}
}

// CompositeLookup combines multiple LookupEnvFunc functions into a single LookupEnvFunc.
// It queries each provided function in order and returns the first successful result.
// If none of the functions return a value, it returns an empty string and false.
func CompositeLookup(lookups ...LookupEnvFunc) LookupEnvFunc {
	return func(key string) (string, bool) {
		for _, lookup := range lookups {
			if val, ok := lookup(key); ok {
				return val, ok
			}
		}
		return "", false
	}
}
