//go:build !embed_model

package provider

import "embed"

var embeddedModelFS embed.FS

const hasEmbeddedModel = false
