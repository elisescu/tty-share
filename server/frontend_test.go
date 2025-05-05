package server

import (
	"io/fs"
	"path/filepath"
	"testing"
)

func TestFrontendEmbedFS(t *testing.T) {
	expectedFiles := []string{
		"404.css",
		"404.in.html",
		"bootstrap.min.css",
		"tty-share.in.html",
		"tty-share.js",
	}

	publicPrefix := "frontend/public"

	t.Run("CheckExpectedFilesExist", func(t *testing.T) {
		for _, filename := range expectedFiles {
			t.Run(filename, func(t *testing.T) {
				// Use the full path with the public prefix
				fullPath := filepath.Join(publicPrefix, filename)
				f, err := Frontend.Open(fullPath)
				if err != nil {
					t.Fatalf("Failed to open embedded file '%s': %v", fullPath, err)
				}
				defer f.Close()

				stat, err := f.Stat()
				if err != nil {
					t.Fatalf("Failed to get stat for embedded file '%s': %v", fullPath, err)
				}
				if stat.IsDir() {
					t.Errorf("Expected '%s' to be a file, but it's a directory", fullPath)
				}
				if stat.Size() <= 0 {
					t.Logf("Warning: Embedded file '%s' has size 0", fullPath)
				}
			})
		}
	})

	t.Run("TestAssetFunction", func(t *testing.T) {
		for _, filename := range expectedFiles {
			t.Run(filename, func(t *testing.T) {
				// Test the Asset function which should handle the public prefix internally
				content, err := Asset(filename)
				if err != nil {
					t.Fatalf("Asset() failed to retrieve '%s': %v", filename, err)
				}

				// Verify content is not empty
				if len(content) == 0 {
					t.Errorf("Asset('%s') returned empty content", filename)
				}

				// Compare with direct FS access to ensure they match
				fullPath := filepath.Join(publicPrefix, filename)
				expected, err := fs.ReadFile(Frontend, fullPath)
				if err != nil {
					t.Fatalf("Failed to read file directly for comparison: %v", err)
				}

				if string(content) != string(expected) {
					t.Errorf("Asset('%s') content doesn't match direct file read", filename)
				}
			})
		}
	})

	t.Run("TestAssetWithNonExistentFile", func(t *testing.T) {
		nonExistentFile := "this_file_should_not_exist.txt"
		_, err := Asset(nonExistentFile)
		if err == nil {
			t.Fatalf("Expected Asset() to return an error for non-existent file '%s', but got nil", nonExistentFile)
		}
	})

	t.Run("CheckDirectoryStructure", func(t *testing.T) {
		entries, err := fs.ReadDir(Frontend, publicPrefix)
		if err != nil {
			t.Fatalf("Failed to read directory '%s': %v", publicPrefix, err)
		}

		if len(entries) != len(expectedFiles) {
			t.Errorf("Expected %d entries in '%s' directory, but found %d",
				len(expectedFiles), publicPrefix, len(entries))
		}

		foundFiles := make(map[string]bool)
		for _, entry := range entries {
			if entry.IsDir() {
				t.Errorf("Found unexpected directory '%s' in embedded filesystem", entry.Name())
			}
			foundFiles[entry.Name()] = true
		}

		for _, expected := range expectedFiles {
			if !foundFiles[expected] {
				t.Errorf("Expected file '%s' not found in directory listing", expected)
			}
		}
	})

	t.Run("CheckFilesAreNonEmpty", func(t *testing.T) {
		for _, fileName := range expectedFiles {
			t.Run(fileName, func(t *testing.T) {
				content, err := Asset(fileName)
				if err != nil {
					t.Fatalf("Failed to read content of '%s' using Asset(): %v", fileName, err)
				}

				if len(content) == 0 {
					t.Errorf("Content of '%s' is empty, expected non-empty file", fileName)
				}
			})
		}
	})
}
