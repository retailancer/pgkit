// Package identifier provides the Generator interface and built-in ID generators
// for use with pgkit's IDGenerator option.
//
// External consumers should import this package instead of the internal one.
package identifier

import internal "github.com/retailancer/pgkit/internal/identifier"

// Generator is the interface that wraps the Generate method.
// Implement this to plug in a custom ID generation strategy.
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
