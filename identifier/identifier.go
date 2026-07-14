package identifier

import internal "github.com/retailancer/pgkit/internal/identifier"

type Generator = internal.Generator

// NewCUID2Generator returns a Generator that produces CUID2 identifiers.
// CUID2 IDs are collision-resistant, URL-safe, and well-suited for database primary keys.
func NewCUID2Generator() *internal.CUID2Generator {
	return internal.NewCUID2Generator()
}

// NewIgnoreGenerator returns a Generator that always returns an empty string,
// effectively disabling client-side ID generation (the default behaviour).
func NewIgnoreGenerator() *internal.IgnoreGenerator {
	return internal.NewIgnoreGenerator()
}
