package main

import (
	"embed"
	"io/fs"
)

// Embed the React UI build files into the Go binary
//
//go:embed ui-build/*
var embeddedUI embed.FS

// GetEmbeddedUI returns the embedded UI filesystem
func GetEmbeddedUI() (fs.FS, error) {
	// Return the ui-build subdirectory from the embedded filesystem
	return fs.Sub(embeddedUI, "ui-build")
}
