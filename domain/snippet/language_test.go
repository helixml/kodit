package snippet

import (
	"errors"
	"testing"
)

func TestLanguage_ExtensionsForLanguage(t *testing.T) {
	lang := Language{}

	tests := []struct {
		language   string
		wantFirst  string
		wantLength int
	}{
		{"go", "go", 1},
		{"python", "py", 4},
		{"javascript", "js", 3},
		{"typescript", "ts", 1},
		{"rust", "rs", 1},
		{"cpp", "cpp", 4},
		{"c", "c", 2},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			exts, err := lang.ExtensionsForLanguage(tt.language)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(exts) != tt.wantLength {
				t.Errorf("got %d extensions, want %d", len(exts), tt.wantLength)
			}
			if exts[0] != tt.wantFirst {
				t.Errorf("first extension = %q, want %q", exts[0], tt.wantFirst)
			}
		})
	}
}

func TestLanguage_ExtensionsForLanguage_CaseInsensitive(t *testing.T) {
	lang := Language{}
	exts, err := lang.ExtensionsForLanguage("Go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exts) == 0 {
		t.Error("expected extensions for Go")
	}
}

func TestLanguage_ExtensionsForLanguage_Unsupported(t *testing.T) {
	lang := Language{}
	_, err := lang.ExtensionsForLanguage("brainfuck")
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("error = %v, want ErrUnsupportedLanguage", err)
	}
}

func TestLanguage_ExtensionsForLanguage_ReturnsCopy(t *testing.T) {
	lang := Language{}
	exts1, _ := lang.ExtensionsForLanguage("python")
	exts1[0] = "MUTATED"

	exts2, _ := lang.ExtensionsForLanguage("python")
	if exts2[0] == "MUTATED" {
		t.Error("ExtensionsForLanguage should return a copy, not the original slice")
	}
}

func TestLanguage_LanguageForExtension(t *testing.T) {
	lang := Language{}

	tests := []struct {
		extension string
		want      string
	}{
		{"go", "go"},
		{"py", "python"},
		{"js", "javascript"},
		{"ts", "typescript"},
		{"tsx", "tsx"},
		{"rs", "rust"},
		{"java", "java"},
		{"cs", "csharp"},
		{"cpp", "cpp"},
		{"c", "c"},
		{"rb", "ruby"},
		{"sh", "bash"},
	}
	for _, tt := range tests {
		t.Run(tt.extension, func(t *testing.T) {
			got, err := lang.LanguageForExtension(tt.extension)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("LanguageForExtension(%q) = %q, want %q", tt.extension, got, tt.want)
			}
		})
	}
}

func TestLanguage_LanguageForExtension_StripsDot(t *testing.T) {
	lang := Language{}
	got, err := lang.LanguageForExtension(".py")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "python" {
		t.Errorf("got %q, want %q", got, "python")
	}
}

func TestLanguage_LanguageForExtension_CaseInsensitive(t *testing.T) {
	lang := Language{}
	got, err := lang.LanguageForExtension(".PY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "python" {
		t.Errorf("got %q, want %q", got, "python")
	}
}

func TestLanguage_LanguageForExtension_Unsupported(t *testing.T) {
	lang := Language{}
	_, err := lang.LanguageForExtension("zzz")
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
	if !errors.Is(err, ErrUnsupportedExtension) {
		t.Errorf("error = %v, want ErrUnsupportedExtension", err)
	}
}

func TestLanguage_ExtensionToLanguageMap(t *testing.T) {
	lang := Language{}
	m := lang.ExtensionToLanguageMap()

	if m["go"] != "go" {
		t.Errorf("m[\"go\"] = %q, want %q", m["go"], "go")
	}
	if m["py"] != "python" {
		t.Errorf("m[\"py\"] = %q, want %q", m["py"], "python")
	}
	if m["rs"] != "rust" {
		t.Errorf("m[\"rs\"] = %q, want %q", m["rs"], "rust")
	}
}

func TestLanguage_SupportedLanguages(t *testing.T) {
	lang := Language{}
	languages := lang.SupportedLanguages()

	if len(languages) == 0 {
		t.Fatal("expected at least one supported language")
	}

	found := false
	for _, l := range languages {
		if l == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'go' in supported languages")
	}
}

func TestLanguage_SupportedExtensions(t *testing.T) {
	lang := Language{}
	extensions := lang.SupportedExtensions()

	if len(extensions) == 0 {
		t.Fatal("expected at least one supported extension")
	}
}

func TestLanguage_IsLanguageSupported(t *testing.T) {
	lang := Language{}

	if !lang.IsLanguageSupported("go") {
		t.Error("expected go to be supported")
	}
	if !lang.IsLanguageSupported("Go") {
		t.Error("expected Go (uppercase) to be supported")
	}
	if lang.IsLanguageSupported("brainfuck") {
		t.Error("expected brainfuck to not be supported")
	}
}

func TestLanguage_IsExtensionSupported(t *testing.T) {
	lang := Language{}

	if !lang.IsExtensionSupported("go") {
		t.Error("expected go extension to be supported")
	}
	if !lang.IsExtensionSupported(".py") {
		t.Error("expected .py extension to be supported")
	}
	if lang.IsExtensionSupported("zzz") {
		t.Error("expected zzz extension to not be supported")
	}
}

func TestLanguage_ExtensionsWithFallback(t *testing.T) {
	lang := Language{}

	exts := lang.ExtensionsWithFallback("python")
	if len(exts) != 4 {
		t.Errorf("expected 4 extensions for python, got %d", len(exts))
	}

	exts = lang.ExtensionsWithFallback("unknownlang")
	if len(exts) != 1 || exts[0] != "unknownlang" {
		t.Errorf("expected fallback [\"unknownlang\"], got %v", exts)
	}
}

func TestLanguage_BidirectionalConsistency(t *testing.T) {
	lang := Language{}

	for _, language := range lang.SupportedLanguages() {
		exts, err := lang.ExtensionsForLanguage(language)
		if err != nil {
			t.Errorf("ExtensionsForLanguage(%q) failed: %v", language, err)
			continue
		}
		for _, ext := range exts {
			got, err := lang.LanguageForExtension(ext)
			if err != nil {
				t.Errorf("LanguageForExtension(%q) failed: %v", ext, err)
				continue
			}
			if got != language {
				t.Errorf("LanguageForExtension(%q) = %q, want %q", ext, got, language)
			}
		}
	}
}
