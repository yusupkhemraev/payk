// Package importer defines the interface every import source implements:
// curl commands, OpenAPI specs, and FastAPI projects all produce a
// core.Collection through it.
package importer

import (
	"context"

	"github.com/yusupkhemraev/payk/internal/core"
)

// Importer turns external input (a curl command, a spec path/URL, a project
// directory) into a collection. curl yields a single-request collection;
// OpenAPI and FastAPI yield full trees.
type Importer interface {
	Name() string
	// CanHandle inspects the input cheaply, e.g. a "curl " prefix or
	// openapi/swagger keys.
	CanHandle(input string) bool
	Import(ctx context.Context, input string) (*core.Collection, error)
}

// Warner is optionally implemented by importers that collect non-fatal
// warnings (e.g. unknown curl flags) during the last Import call.
type Warner interface {
	Warnings() []string
}

// WarningsOf returns the warnings of the last Import, if the importer
// reports any.
func WarningsOf(imp Importer) []string {
	if w, ok := imp.(Warner); ok {
		return w.Warnings()
	}
	return nil
}

// EnvironmentProvider is optionally implemented by importers that derive
// environments from the source (e.g. OpenAPI server URLs).
type EnvironmentProvider interface {
	Environments() []core.Environment
}

// EnvironmentsOf returns environments produced by the last Import, if any.
func EnvironmentsOf(imp Importer) []core.Environment {
	if p, ok := imp.(EnvironmentProvider); ok {
		return p.Environments()
	}
	return nil
}

// Find returns the first registered importer that can handle the input.
func Find(importers []Importer, input string) Importer {
	for _, imp := range importers {
		if imp.CanHandle(input) {
			return imp
		}
	}
	return nil
}
