package enricher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helixml/kodit/domain/enrichment"
	"github.com/helixml/kodit/domain/repository"
)

const outsideMarker = "OUTSIDE-WORKING-COPY"

// symlinkedRepo builds a working copy in which every file and directory an
// enricher looks for is a symbolic link: to another clone's files (which hold
// outsideMarker), to /etc/hostname, or to /proc/self/environ. It returns the
// working copy and the strings that must never appear in enricher output.
func symlinkedRepo(t *testing.T) (string, []string) {
	t.Helper()

	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	other := filepath.Join(base, "other-clone")
	for _, dir := range []string{repo, filepath.Join(other, "examples"), filepath.Join(other, "migrations"), filepath.Join(repo, "app")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	secretFiles := map[string]string{
		"secret.md":              "# " + outsideMarker,
		"secret.go":              "package secret\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\nfunc D() {} // " + outsideMarker,
		"compose.yml":            "services:\n  " + strings.ToLower(outsideMarker) + ":\n    image: x\n",
		"examples/example.py":    "print('" + outsideMarker + "')",
		"migrations/001.sql":     "CREATE TABLE " + outsideMarker + " (id int);",
		"schema.sql":             "CREATE TABLE " + outsideMarker + " (id int);",
		"models.py":              "class " + outsideMarker + ": pass",
		"pyproject.toml":         "[project]\nname = \"" + outsideMarker + "\"",
		"package.json":           "{\"name\": \"" + outsideMarker + "\"}",
		"go.mod":                 "module " + outsideMarker,
		"Cargo.toml":             "[package]\nname = \"" + outsideMarker + "\"",
		"setup.py":               "name='" + outsideMarker + "'",
		"requirements.txt":       outsideMarker,
		"docker-compose.yml":     "services:\n  " + strings.ToLower(outsideMarker) + ":\n    image: x\n",
		"docker-compose.dev.yml": "services:\n  " + strings.ToLower(outsideMarker) + ":\n    image: x\n",
	}
	for rel, content := range secretFiles {
		if err := os.WriteFile(filepath.Join(other, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	links := map[string]string{
		"README.md":              "/proc/self/environ",
		"README.rst":             "/etc/hostname",
		"README.txt":             filepath.Join(other, "secret.md"),
		"README":                 filepath.Join(other, "secret.md"),
		"pyproject.toml":         filepath.Join(other, "pyproject.toml"),
		"setup.py":               filepath.Join(other, "setup.py"),
		"package.json":           filepath.Join(other, "package.json"),
		"go.mod":                 "/proc/self/environ",
		"Cargo.toml":             filepath.Join(other, "Cargo.toml"),
		"requirements.txt":       filepath.Join(other, "requirements.txt"),
		"examples":               filepath.Join(other, "examples"),
		"samples":                filepath.Join(other, "examples"),
		"migrations":             filepath.Join(other, "migrations"),
		"schema.sql":             filepath.Join(other, "schema.sql"),
		"app/models.py":          filepath.Join(other, "models.py"),
		"docker-compose.yml":     filepath.Join(other, "docker-compose.yml"),
		"docker-compose.dev.yml": "../other-clone/docker-compose.dev.yml",
		"api.go":                 filepath.Join(other, "secret.go"),
		"environ.go":             "/proc/self/environ",
		"hostname.go":            "/etc/hostname",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(repo, name)); err != nil {
			t.Fatal(err)
		}
	}

	// /proc/self/environ holds the environment the process started with, so
	// its PATH entry is the marker for links to it.
	forbidden := []string{outsideMarker, strings.ToLower(outsideMarker), "PATH=" + os.Getenv("PATH")}
	if hostname, err := os.ReadFile("/etc/hostname"); err == nil && strings.TrimSpace(string(hostname)) != "" {
		forbidden = append(forbidden, strings.TrimSpace(string(hostname)))
	}
	return repo, forbidden
}

func assertNoneLeaked(t *testing.T, what, output string, forbidden []string) {
	t.Helper()
	for _, f := range forbidden {
		if strings.Contains(output, f) {
			t.Errorf("%s followed a symbolic link out of the working copy: output contains %q:\n%s", what, f, output)
		}
	}
}

func TestEnrichers_DoNotFollowSymlinks(t *testing.T) {
	repo, forbidden := symlinkedRepo(t)
	ctx := context.Background()

	cookbook, err := NewCookbookContextService().Gather(ctx, repo, "python")
	if err != nil {
		t.Fatal(err)
	}
	assertNoneLeaked(t, "cookbook context", cookbook, forbidden)

	wiki := NewWikiContextService()
	readme, _, _, err := wiki.Gather(ctx, repo, nil, []enrichment.Enrichment{})
	if err != nil {
		t.Fatal(err)
	}
	assertNoneLeaked(t, "wiki README", readme, forbidden)
	for _, rel := range []string{"README.md", "api.go", "environ.go", "hostname.go", "examples/example.py"} {
		assertNoneLeaked(t, "wiki file content "+rel, wiki.FileContent(repo, rel, 3000), forbidden)
	}

	schema, err := NewDatabaseSchemaService().Discover(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	assertNoneLeaked(t, "database schema", schema, forbidden)

	architecture, err := NewPhysicalArchitectureService().Discover(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	assertNoneLeaked(t, "physical architecture", architecture, forbidden)
	if strings.Contains(architecture, "Node.js") || strings.Contains(architecture, "Python components") || strings.Contains(architecture, "Docker Compose orchestration") {
		t.Errorf("physical architecture treated symbolic links as present files:\n%s", architecture)
	}

	var files []repository.File
	for _, rel := range []string{"api.go", "environ.go", "hostname.go"} {
		files = append(files, repository.NewFile("sha", rel, "go", 1))
	}
	docs, err := NewAPIDocService().Extract(ctx, repo, files, "go", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		assertNoneLeaked(t, "API docs", d.Content(), forbidden)
	}
}

// TestEnrichers_ReadRegularFiles is the control for the test above: the same
// enrichers read regular files in the working copy.
func TestEnrichers_ReadRegularFiles(t *testing.T) {
	repo := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "own readme")
	write("go.mod", "module own")
	write("examples/demo.go", "package main // own example")
	write("migrations/001.sql", "CREATE TABLE own (id int);")
	write("api.go", "package own\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\n")
	ctx := context.Background()

	cookbook, err := NewCookbookContextService().Gather(ctx, repo, "go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"own readme", "module own", "own example"} {
		if !strings.Contains(cookbook, want) {
			t.Errorf("cookbook context missing %q:\n%s", want, cookbook)
		}
	}
	schema, err := NewDatabaseSchemaService().Discover(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(schema, "CREATE TABLE own") {
		t.Errorf("schema missing migration:\n%s", schema)
	}
	docs, err := NewAPIDocService().Extract(ctx, repo, []repository.File{repository.NewFile("sha", "api.go", "go", 1)}, "go", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || !strings.Contains(docs[0].Content(), "func A()") {
		t.Errorf("API docs did not read api.go: %+v", docs)
	}
}
