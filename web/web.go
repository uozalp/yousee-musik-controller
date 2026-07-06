// Package web embeds the frontend templates and static assets.
package web

import "embed"

// Templates holds the HTML templates.
//
//go:embed templates
var Templates embed.FS

// Static holds the static assets (css, js, img).
//
//go:embed static
var Static embed.FS
