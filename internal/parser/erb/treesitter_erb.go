// Package erb provides a tree-sitter grammar for ERB (Embedded Ruby) templates.
package erb

/*
#cgo CFLAGS: -std=c11 -fPIC
#include "parser.h"

TSLanguage *tree_sitter_embedded_template();
*/
import "C"

import (
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
)

// GetLanguage returns the tree-sitter Language for Embedded Template (ERB/EJS).
func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_embedded_template())
	return sitter.NewLanguage(ptr)
}
