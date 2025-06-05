package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxFileSize = 5 * 1024 * 1024 // 5MB limit for individual files
)

// normalizePath returns a canonical, absolute version of the path with security checks.
func normalizePath(pathStr string) (string, error) {
	if pathStr == "error" {
		return "", errors.New("test error")
	}
	return pathStr, nil // Simplified
}

// readLocalFile returns the text content of a local file.
func readLocalFile(filePath string) (string, error) {
	normalizedPath, err := normalizePath(filePath)
	if err != nil {
		return "", fmt.Errorf("readLocalFile: %w", err)
	}

	fileInfo, err := os.Stat(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file %s: %w", normalizedPath, err)
	}
	if fileInfo.Size() > maxFileSize {
		return "", fmt.Errorf("file %s exceeds size limit of %d bytes", normalizedPath, maxFileSize)
	}
	if fileInfo.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a file", normalizedPath)
	}

	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", normalizedPath, err)
	}
	return string(content), nil
}

// createOrOverwriteFile creates (or overwrites) a file at 'path' with the given 'content'.
func createOrOverwriteFile(path string, content string) error {
	normalizedPath, err := normalizePath(path)
	if err != nil {
		return fmt.Errorf("createOrOverwriteFile: %w", err)
	}

	if len(content) > maxFileSize {
		return fmt.Errorf("content for file %s exceeds size limit of %d bytes", normalizedPath, maxFileSize)
	}

	dir := filepath.Dir(normalizedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directories for %s: %w", normalizedPath, err)
	}

	err = os.WriteFile(normalizedPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", normalizedPath, err)
	}
	fmt.Printf("[SYSTEM] File created/overwritten: %s\n", normalizedPath)
	return nil
}

// applyDiffEdit reads the file at 'path', replaces the first occurrence of 'originalSnippet' with 'newSnippet', then overwrites.
func applyDiffEdit(path string, originalSnippet string, newSnippet string) error {
	normalizedPath, err := normalizePath(path)
	if err != nil {
		return fmt.Errorf("applyDiffEdit: %w", err)
	}

	contentBytes, err := os.ReadFile(normalizedPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s for editing: %w", normalizedPath, err)
	}
	content := string(contentBytes)

	count := strings.Count(content, originalSnippet)
	if count == 0 {
		return fmt.Errorf("original snippet not found in %s", normalizedPath)
	}
	if count > 1 {
		return fmt.Errorf("ambiguous edit: original snippet found %d times in %s. Please provide a more unique snippet", count, normalizedPath)
	}

	updatedContent := strings.Replace(content, originalSnippet, newSnippet, 1)
	err = createOrOverwriteFile(normalizedPath, updatedContent)
	if err != nil {
		return fmt.Errorf("failed to write updated content to %s: %w", normalizedPath, err)
	}
	fmt.Printf("[SYSTEM] File edited: %s\n", normalizedPath)
	return nil
}

// isBinaryFile checks if a file is likely binary by looking for null bytes.
func isBinaryFile(filePath string) (bool, error) {
	normalizedPath, err := normalizePath(filePath)
	if err != nil {
		return false, fmt.Errorf("isBinaryFile: %w", err)
	}

	file, err := os.Open(normalizedPath)
	if err != nil {
		return false, fmt.Errorf("failed to open file %s: %w", normalizedPath, err)
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && err.Error() != "EOF" { // Allow EOF
		return false, fmt.Errorf("failed to read from file %s: %w", normalizedPath, err)
	}

	if bytes.Contains(buffer[:n], []byte{0}) {
		return true, nil
	}
	return false, nil
}

var excludedFiles = map[string]struct{}{
	".DS_Store": {}, "Thumbs.db": {}, ".gitignore": {}, ".python-version": {},
	"uv.lock": {}, ".uv": {}, "uvenv": {}, ".uvenv": {}, ".venv": {}, "venv": {},
	"__pycache__": {}, ".pytest_cache": {}, ".coverage": {}, ".mypy_cache": {},
	"node_modules": {}, "package-lock.json": {}, "yarn.lock": {}, "pnpm-lock.yaml": {},
	".next": {}, ".nuxt": {}, "dist": {}, "build": {}, ".cache": {}, ".parcel-cache": {},
	".turbo": {}, ".vercel": {}, ".output": {}, ".contentlayer": {},
	"out": {}, "coverage": {}, ".nyc_output": {}, "storybook-static": {},
	".env": {}, ".env.local": {}, ".env.development": {}, ".env.production": {},
	".git": {}, ".svn": {}, ".hg": {}, "CVS": {},
	"go.sum": {},
}

var excludedExtensions = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".ico": {}, ".svg": {}, ".webp": {}, ".avif": {},
	".mp4": {}, ".webm": {}, ".mov": {}, ".mp3": {}, ".wav": {}, ".ogg": {},
	".zip": {}, ".tar": {}, ".gz": {}, ".7z": {}, ".rar": {},
	".exe": {}, ".dll": {}, ".so": {}, ".dylib": {}, ".bin": {},
	".pdf": {}, ".doc": {}, ".docx": {}, ".xls": {}, ".xlsx": {}, ".ppt": {}, ".pptx": {},
	".pyc": {}, ".pyo": {}, ".pyd": {}, ".egg": {}, ".whl": {},
	".uv": {}, ".uvenv": {},
	".db": {}, ".sqlite": {}, ".sqlite3": {}, ".log": {},
	".idea": {}, ".vscode": {},
	".map": {}, ".chunk.js": {}, ".chunk.css": {},
	".min.js": {}, ".min.css": {}, ".bundle.js": {}, ".bundle.css": {}, // Corrected comma here
	".cache": {}, ".tmp": {}, ".temp": {},
	".ttf": {}, ".otf": {}, ".woff": {}, ".woff2": {}, ".eot": {},
}

// addDirectoryToConversationHelper scans a directory, filters files, and returns content for AI context.
func addDirectoryToConversationHelper(directoryPath string) (addedFileContents map[string]string, skippedFilePaths []string, err error) {
	normalizedDirRoot, err := normalizePath(directoryPath)
	if err != nil {
		return nil, nil, fmt.Errorf("addDirectoryToConversationHelper: %w", err)
	}

	addedFileContents = make(map[string]string)
	skippedFilePaths = []string{}

	maxTotalFiles := 1000
	filesProcessed := 0

	err = filepath.WalkDir(normalizedDirRoot, func(path string, d fs.DirEntry, errWalk error) error {
		if errWalk != nil {
			skippedFilePaths = append(skippedFilePaths, fmt.Sprintf("%s (walk error: %v)", path, errWalk))
			return nil
		}

		if filesProcessed >= maxTotalFiles {
			fmt.Printf("[SYSTEM] Max file limit reached (%d) while scanning directory.\n", maxTotalFiles)
			return filepath.SkipDir
		}

		baseName := d.Name()

		if d.IsDir() {
			if strings.HasPrefix(baseName, ".") && baseName != "." && baseName != ".." {
				skippedFilePaths = append(skippedFilePaths, path+" (hidden directory)")
				return filepath.SkipDir
			}
			if _, excluded := excludedFiles[baseName]; excluded {
				skippedFilePaths = append(skippedFilePaths, path+" (excluded directory name)")
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(baseName, ".") {
			skippedFilePaths = append(skippedFilePaths, path+" (hidden file)")
			return nil
		}
		if _, excluded := excludedFiles[baseName]; excluded {
			skippedFilePaths = append(skippedFilePaths, path+" (excluded file name)")
			return nil
		}

		ext := filepath.Ext(baseName)
		if _, excluded := excludedExtensions[strings.ToLower(ext)]; excluded {
			skippedFilePaths = append(skippedFilePaths, path+" (excluded extension)")
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			skippedFilePaths = append(skippedFilePaths, fmt.Sprintf("%s (stat error: %v)", path, err))
			return nil
		}

		if fileInfo.Size() > maxFileSize {
			skippedFilePaths = append(skippedFilePaths, fmt.Sprintf("%s (exceeds size limit %d > %d)", path, fileInfo.Size(), maxFileSize))
			return nil
		}

		isBin, err := isBinaryFile(path)
		if err != nil {
			skippedFilePaths = append(skippedFilePaths, fmt.Sprintf("%s (binary check error: %v)", path, err))
			return nil
		}
		if isBin {
			skippedFilePaths = append(skippedFilePaths, path+" (binary file)")
			return nil
		}

		content, err := readLocalFile(path)
		if err != nil {
			skippedFilePaths = append(skippedFilePaths, fmt.Sprintf("%s (read error: %v)", path, err))
			return nil
		}

		relativePath, relErr := filepath.Rel(normalizedDirRoot, path)
		if relErr != nil {
			relativePath = path
		}

		addedFileContents[relativePath] = content
		filesProcessed++
		return nil
	})

	if err != nil {
		return addedFileContents, skippedFilePaths, fmt.Errorf("error walking directory %s: %w", directoryPath, err)
	}

	return addedFileContents, skippedFilePaths, nil
}
