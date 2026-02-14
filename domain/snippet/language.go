package snippet

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// languageExtensions maps language names to their file extensions.
var languageExtensions = map[string][]string{
	"python":     {"py", "pyw", "pyx", "pxd"},
	"go":         {"go"},
	"javascript": {"js", "jsx", "mjs"},
	"typescript": {"ts"},
	"tsx":        {"tsx"},
	"java":       {"java"},
	"csharp":     {"cs"},
	"cpp":        {"cpp", "cc", "cxx", "hpp"},
	"c":          {"c", "h"},
	"rust":       {"rs"},
	"php":        {"php"},
	"ruby":       {"rb"},
	"swift":      {"swift"},
	"kotlin":     {"kt", "kts"},
	"scala":      {"scala"},
	"r":          {"r", "R"},
	"matlab":     {"m"},
	"perl":       {"pl", "pm"},
	"bash":       {"sh", "bash"},
	"powershell": {"ps1"},
	"sql":        {"sql"},
	"yaml":       {"yml", "yaml"},
	"json":       {"json"},
	"xml":        {"xml"},
	"markdown":   {"md", "markdown"},
}

// ErrUnsupportedLanguage indicates an unsupported programming language.
var ErrUnsupportedLanguage = errors.New("unsupported language")

// ErrUnsupportedExtension indicates an unsupported file extension.
var ErrUnsupportedExtension = errors.New("unsupported file extension")

// Language provides bidirectional mapping between programming languages
// and their file extensions.
type Language struct{}

// ExtensionsForLanguage returns the file extensions for a language.
func (Language) ExtensionsForLanguage(language string) ([]string, error) {
	lang := strings.ToLower(language)
	extensions, ok := languageExtensions[lang]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, language)
	}
	result := make([]string, len(extensions))
	copy(result, extensions)
	return result, nil
}

// LanguageForExtension returns the language for a file extension.
func (Language) LanguageForExtension(extension string) (string, error) {
	ext := strings.TrimPrefix(strings.ToLower(extension), ".")
	for language, extensions := range languageExtensions {
		if slices.Contains(extensions, ext) {
			return language, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrUnsupportedExtension, extension)
}

// ExtensionToLanguageMap returns a map of extensions to languages.
func (Language) ExtensionToLanguageMap() map[string]string {
	result := make(map[string]string)
	for language, extensions := range languageExtensions {
		for _, ext := range extensions {
			result[ext] = language
		}
	}
	return result
}

// SupportedLanguages returns all supported programming languages.
func (Language) SupportedLanguages() []string {
	result := make([]string, 0, len(languageExtensions))
	for lang := range languageExtensions {
		result = append(result, lang)
	}
	return result
}

// SupportedExtensions returns all supported file extensions.
func (Language) SupportedExtensions() []string {
	var result []string
	for _, extensions := range languageExtensions {
		result = append(result, extensions...)
	}
	return result
}

// IsLanguageSupported checks if a language is supported.
func (Language) IsLanguageSupported(language string) bool {
	_, ok := languageExtensions[strings.ToLower(language)]
	return ok
}

// IsExtensionSupported checks if a file extension is supported.
func (l Language) IsExtensionSupported(extension string) bool {
	_, err := l.LanguageForExtension(extension)
	return err == nil
}

// ExtensionsWithFallback returns extensions for a language,
// or the language name itself if not found.
func (l Language) ExtensionsWithFallback(language string) []string {
	lang := strings.ToLower(language)
	if l.IsLanguageSupported(lang) {
		// Error can be ignored since we just checked it's supported
		extensions, _ := l.ExtensionsForLanguage(lang)
		return extensions
	}
	return []string{lang}
}
