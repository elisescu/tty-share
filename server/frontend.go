package server

import (
	"embed"
	"fmt"
	"io"
	"path/filepath"
)

//go:embed frontend/public
var Frontend embed.FS

// Asset returns the content of a file from the embedded frontend filesystem
func Asset(filename string) ([]byte, error) {
	// Ensure the path starts with frontend/public
	path := filepath.Join("frontend/public", filename)

	// Open the file
	file, err := Frontend.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	// Read all content
	return io.ReadAll(file)
}
