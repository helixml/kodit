//go:build embed_model

package provider

import "embed"

//go:embed all:models
var embeddedModelFS embed.FS

const hasEmbeddedModel = true
