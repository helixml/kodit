package analyzers

import (
	"github.com/helixml/kodit/internal/indexing/slicer"
)

// Factory creates language-specific analyzers.
type Factory struct {
	config slicer.LanguageConfig
}

// NewFactory creates a new Factory.
func NewFactory(config slicer.LanguageConfig) *Factory {
	return &Factory{
		config: config,
	}
}

// ByName returns an analyzer for the specified language name.
func (f *Factory) ByName(name string) (slicer.Analyzer, bool) {
	lang, ok := f.config.ByName(name)
	if !ok {
		return nil, false
	}
	return f.createAnalyzer(lang), true
}

// ByExtension returns an analyzer for the specified file extension.
func (f *Factory) ByExtension(ext string) (slicer.Analyzer, bool) {
	lang, ok := f.config.ByExtension(ext)
	if !ok {
		return nil, false
	}
	return f.createAnalyzer(lang), true
}

func (f *Factory) createAnalyzer(lang slicer.Language) slicer.Analyzer {
	switch lang.Name() {
	case "python":
		return NewPython(lang)
	case "go":
		return NewGo(lang)
	case "java":
		return NewJava(lang)
	case "c":
		return NewC(lang)
	case "cpp":
		return NewCPP(lang)
	case "rust":
		return NewRust(lang)
	case "javascript":
		return NewJavaScript(lang)
	case "typescript":
		return NewTypeScript(lang)
	case "tsx":
		return NewTSX(lang)
	case "csharp":
		return NewCSharp(lang)
	default:
		return NewPython(lang)
	}
}
