// Package web holds the browser UI, embedded into the binary.
package web

import "embed"

//go:embed index.html book.html app.js app.css i18n.js vendor
var FS embed.FS
