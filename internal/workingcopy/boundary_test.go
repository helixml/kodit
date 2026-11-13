package workingcopy

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// rawFileAccess lists the standard-library calls that read files or walk
// directories while following symbolic links. Outside the files allowed
// below, code must use this package instead.
var rawFileAccess = map[string]map[string]bool{
	"os":       {"ReadFile": true, "Open": true, "OpenFile": true, "OpenRoot": true, "OpenInRoot": true, "DirFS": true, "ReadDir": true},
	"filepath": {"Walk": true, "WalkDir": true, "Glob": true},
	"fs":       {"ReadFile": true, "ReadDir": true, "WalkDir": true, "Glob": true, "Sub": true},
	"zip":      {"OpenReader": true},
	"ioutil":   {"ReadFile": true, "ReadDir": true},
}

// allowedRawFileAccess are the files that may call rawFileAccess functions,
// with the reason each one cannot reach repository working-copy content.
var allowedRawFileAccess = map[string]string{
	"internal/workingcopy/workingcopy.go":    "the helper itself",
	"infrastructure/rasterization/image.go":  "renders a path its caller obtained from workingcopy.File",
	"infrastructure/rasterization/office.go": "renders a path its caller obtained from workingcopy.File",
	"infrastructure/rasterization/cache.go":  "reads kodit's own page-image cache directory",
	"infrastructure/provider/hugot.go":       "reads embedding model files from the model directory",
	"infrastructure/api/docs.go":             "reads the embedded OpenAPI spec",
}

// skippedDirs hold build tooling and binaries that never touch working copies.
var skippedDirs = map[string]bool{"cmd": true, "tools": true, "scripts": true, "test": true, ".git": true}

// TestNoRawFileAccessOutsideWorkingCopyPackage fails when production code
// reads files or walks directories with the standard library outside the
// allowed files. Working-copy reads must go through this package so symbolic
// links committed to a repository are never followed.
func TestNoRawFileAccessOutsideWorkingCopyPackage(t *testing.T) {
	root := filepath.Join("..", "..")
	var violations []string
	seenAllowed := map[string]bool{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skippedDirs[d.Name()] && rel == d.Name() {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		imports := importNames(file)

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			pkg, imported := imports[ident.Name]
			if !imported || !rawFileAccess[pkg][sel.Sel.Name] {
				return true
			}
			if _, allowed := allowedRawFileAccess[rel]; allowed {
				seenAllowed[rel] = true
				return true
			}
			violations = append(violations, fset.Position(call.Pos()).String()+": "+pkg+"."+sel.Sel.Name)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("raw file access outside internal/workingcopy (use workingcopy.ReadFile/File/WalkDir/Glob, or allow it with a reason): %s", v)
	}
	for file := range allowedRawFileAccess {
		if !seenAllowed[file] {
			t.Errorf("%s is allowed raw file access but no longer uses it; remove it from allowedRawFileAccess", file)
		}
	}
}

// importNames maps each import's local name to the last element of its path
// for the standard-library packages in rawFileAccess.
func importNames(file *ast.File) map[string]string {
	paths := map[string]string{
		"os": "os", "path/filepath": "filepath", "io/fs": "fs",
		"archive/zip": "zip", "io/ioutil": "ioutil",
	}
	names := map[string]string{}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		pkg, ok := paths[path]
		if !ok {
			continue
		}
		name := pkg
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = pkg
	}
	return names
}
