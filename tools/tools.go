//go:build tools

// Package tools tracks tool dependencies for go mod.
// See https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

// Tool dependencies managed via `go install` commands in GNUmakefile.
// These imports ensure `go mod tidy` keeps the tools in go.sum.
import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
)
