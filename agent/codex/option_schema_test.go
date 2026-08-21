package codex

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Drift guard: every opts["..."] read anywhere in this package must be
// declared in KnownOptionKeys, or the feedback channel will wrongly flag a
// working option as unsupported. Parses the package source so a new option
// read cannot land without updating the schema.
func TestKnownOptionKeys_CoverEveryOptsReadInPackage(t *testing.T) {
	known := (&Agent{}).KnownOptionKeys()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var reads []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			idx, ok := n.(*ast.IndexExpr)
			if !ok {
				return true
			}
			ident, ok := idx.X.(*ast.Ident)
			if !ok || ident.Name != "opts" {
				return true
			}
			lit, ok := idx.Index.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			key, err := strconv.Unquote(lit.Value)
			if err == nil && key != "" && !slices.Contains(reads, key) {
				reads = append(reads, key)
			}
			return true
		})
	}
	if len(reads) == 0 {
		t.Fatal("no opts reads found — the scanner is broken")
	}
	for _, key := range reads {
		if !slices.Contains(known, key) {
			t.Errorf("opts[%q] is read in this package but missing from KnownOptionKeys — add it or the feedback channel will flag a working option as unsupported", key)
		}
	}
}
