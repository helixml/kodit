package workingcopy

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// boundaryRule restricts a set of functions to the files listed in allowed,
// each with the reason it cannot reach repository working-copy content or
// run code a repository controls. A reference counts whether it is called or
// taken as a value (f := os.ReadFile).
type boundaryRule struct {
	name    string
	funcs   map[string][]string // import path -> function names
	allowed map[string]string   // file -> reason
}

// pathConsumer is the reason for files that open a path they are handed: every
// caller obtains that path from workingcopy.File.
const pathConsumer = "opens a path its caller obtained from workingcopy.File"

var boundaryRules = []boundaryRule{
	{
		name: "file reads and directory walks",
		funcs: map[string][]string{
			"os":            {"ReadFile", "Open", "OpenFile", "OpenRoot", "OpenInRoot", "DirFS", "ReadDir"},
			"path/filepath": {"Walk", "WalkDir", "Glob"},
			"io/fs":         {"ReadFile", "ReadDir", "WalkDir", "Glob", "Sub"},
			"archive/zip":   {"OpenReader"},
			"io/ioutil":     {"ReadFile", "ReadDir"},
		},
		allowed: map[string]string{
			"internal/workingcopy/workingcopy.go":    "the helper itself",
			"infrastructure/rasterization/image.go":  pathConsumer,
			"infrastructure/rasterization/office.go": pathConsumer,
			"infrastructure/rasterization/cache.go":  "reads kodit's own page-image cache directory",
			"infrastructure/provider/hugot.go":       "reads embedding model files from the model directory",
			"infrastructure/api/docs.go":             "reads the embedded OpenAPI spec",
		},
	},
	{
		name:  "file metadata",
		funcs: map[string][]string{"os": {"Stat", "Lstat"}},
		allowed: map[string]string{
			"internal/workingcopy/workingcopy.go":               "the helper itself",
			"infrastructure/git/layout.go":                      "Lstat of .git to decide whether git may read a working copy",
			"infrastructure/git/gitea.go":                       "checks clone targets in the clone directory and whether a working copy exists",
			"infrastructure/git/cloner.go":                      "checks whether a clone in the clone directory exists",
			"infrastructure/extraction/document.go":             pathConsumer + " (size check)",
			"infrastructure/extraction/pdfium_text_renderer.go": pathConsumer + " (size check)",
			"internal/config/dotenv.go":                         "reads operator .env files",
			"infrastructure/provider/hugot.go":                  "model directory",
			"infrastructure/provider/hugot_ort.go":              "ONNX runtime library directory",
			"infrastructure/provider/local_vision.go":           "model directory",
		},
	},
	{
		name:    "processes",
		funcs:   map[string][]string{"os/exec": {"Command", "CommandContext"}},
		allowed: map[string]string{
			// git runs through Gitea's gitcmd, wrapped by hardened() and gated
			// by Layout; nothing else in kodit starts a process.
		},
	},
	{
		name: "document library openers",
		funcs: map[string][]string{
			"github.com/tsawler/tabula":      {"Open"},
			"github.com/tsawler/tabula/pptx": {"Open"},
			"github.com/tsawler/tabula/xlsx": {"Open"},
		},
		allowed: map[string]string{
			"infrastructure/extraction/document.go":                  pathConsumer,
			"infrastructure/extraction/single_page_text_renderer.go": pathConsumer,
			"infrastructure/extraction/pptx_text_renderer.go":        pathConsumer,
			"infrastructure/extraction/xlsx_text_renderer.go":        pathConsumer,
		},
	},
	{
		// PDFium opens a document by path through requests.OpenDocument's
		// FilePath field; the rule matches that field in composite literals.
		name:  "PDFium file paths",
		funcs: map[string][]string{"github.com/klippa-app/go-pdfium/requests": {"OpenDocument.FilePath"}},
		allowed: map[string]string{
			"infrastructure/extraction/pdfium_text_renderer.go": pathConsumer,
			"infrastructure/rasterization/pdfium.go":            pathConsumer,
		},
	},
}

// skippedDirs hold build tooling and binaries that never touch working copies.
var skippedDirs = map[string]bool{"cmd": true, "tools": true, "scripts": true, "test": true, ".git": true}

// TestNoRawFileAccessOutsideWorkingCopyPackage fails when production code
// reads files, inspects paths, starts processes or opens documents outside
// the files each rule allows. Working-copy content must be reached through
// this package so symbolic links committed to a repository are never
// followed, and git must run only through the hardened, Layout-gated adapter.
func TestNoRawFileAccessOutsideWorkingCopyPackage(t *testing.T) {
	root := filepath.Join("..", "..")
	var violations []string
	seen := map[string]map[string]bool{}
	for _, r := range boundaryRules {
		seen[r.name] = map[string]bool{}
	}

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
		imports := importPaths(file)

		report := func(pos token.Pos, importPath, name string) {
			for _, r := range boundaryRules {
				if !contains(r.funcs[importPath], name) {
					continue
				}
				if _, ok := r.allowed[rel]; ok {
					seen[r.name][rel] = true
					return
				}
				violations = append(violations, fset.Position(pos).String()+": "+r.name+": "+importPath+"."+name)
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SelectorExpr:
				if ident, ok := node.X.(*ast.Ident); ok {
					if importPath, ok := imports[ident.Name]; ok {
						report(node.Pos(), importPath, node.Sel.Name)
					}
				}
			case *ast.CompositeLit:
				sel, ok := node.Type.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				importPath, ok := imports[ident.Name]
				if !ok {
					return true
				}
				for _, elt := range node.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.Ident); ok {
							report(kv.Pos(), importPath, sel.Sel.Name+"."+key.Name)
						}
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("not allowed outside internal/workingcopy (use workingcopy.ReadFile/File/WalkDir/Glob, or allow the file with a reason): %s", v)
	}
	for _, r := range boundaryRules {
		for file := range r.allowed {
			if !seen[r.name][file] {
				t.Errorf("%s is allowed %s but no longer uses it; remove it from that allow list", file, r.name)
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// importPaths maps each import's local name to its import path.
func importPaths(file *ast.File) map[string]string {
	names := map[string]string{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = path
	}
	return names
}
