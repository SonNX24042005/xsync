package pathutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProcessResult holds valid processed relative paths and non-fatal path warnings.
type ProcessResult struct {
	ValidPaths []string
	Warnings   []string
}

// ProcessIncludePaths cleans paths: strips absolute prefixes matching either localDir or remoteDir,
// removes comments, and appends a trailing slash for directories in push mode.
func ProcessIncludePaths(rawLines []string, mode, localDir, remoteDir string) []string {
	res := ProcessIncludePathsWithValidation(rawLines, mode, localDir, remoteDir)
	return res.ValidPaths
}

// ProcessIncludePathsWithValidation cleans paths, validates existence in push mode,
// collects non-fatal warnings for invalid paths, and returns only valid relative paths.
func ProcessIncludePathsWithValidation(rawLines []string, mode, localDir, remoteDir string) ProcessResult {
	var result ProcessResult

	cleanLocal := ""
	if localDir != "" {
		cleanLocal = filepath.Clean(localDir)
	}
	cleanRemote := ""
	if remoteDir != "" {
		cleanRemote = filepath.Clean(remoteDir)
	}

	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip quotes
		if (strings.HasPrefix(line, "\"") && strings.HasSuffix(line, "\"")) ||
			(strings.HasPrefix(line, "'") && strings.HasSuffix(line, "'")) {
			line = strings.TrimSpace(line[1 : len(line)-1])
		}

		normLine := filepath.Clean(line)
		rel := line

		// Check remoteDir prefix
		if cleanRemote != "" {
			if strings.HasPrefix(normLine, cleanRemote+string(os.PathSeparator)) ||
				strings.HasPrefix(normLine, strings.ReplaceAll(cleanRemote, "\\", "/")+"/") {
				rel = normLine[len(cleanRemote)+1:]
			} else if normLine == cleanRemote {
				rel = "."
			}
		}

		// Check localDir prefix
		if cleanLocal != "" {
			if strings.HasPrefix(normLine, cleanLocal+string(os.PathSeparator)) ||
				strings.HasPrefix(normLine, strings.ReplaceAll(cleanLocal, "\\", "/")+"/") {
				rel = normLine[len(cleanLocal)+1:]
			} else if normLine == cleanLocal {
				rel = "."
			}
		}

		// Clean leading slashes
		rel = strings.TrimLeft(rel, "/\\")
		if rel == "" {
			continue
		}

		// Validate local existence in push mode
		if mode == "push" && cleanLocal != "" {
			fullPath := filepath.Join(cleanLocal, rel)
			info, err := os.Stat(fullPath)
			if os.IsNotExist(err) {
				// Path does not exist locally -> warn and skip
				result.Warnings = append(result.Warnings, fmt.Sprintf("Duong dan '%s' khong ton tai o Local, tu dong bo qua.", rel))
				continue
			}

			// If it is a directory locally and does not end with /, append /
			if err == nil && info.IsDir() {
				if !strings.HasSuffix(rel, "/") {
					rel = rel + "/"
				}
			}
		}

		result.ValidPaths = append(result.ValidPaths, rel)
	}

	return result
}

// BuildIncludeFilter constructs an rsync merge filter list from whitelist paths.
// For each path 'a/b/c':
//   - adds '+ a/'
//   - adds '+ a/b/'
//   - adds '+ a/b/c' or '+ a/b/c/**' (if directory)
// Finally appends '- *' to exclude everything else.
func BuildIncludeFilter(includeLines []string) []string {
	var filters []string
	roots := make(map[string]bool)

	for _, line := range includeLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip leading + or -
		if strings.HasPrefix(line, "+ ") || strings.HasPrefix(line, "- ") {
			line = strings.TrimSpace(line[2:])
		}

		// Strip leading ./
		if strings.HasPrefix(line, "./") {
			line = line[2:]
		}

		// Normalize double slashes
		for strings.Contains(line, "//") {
			line = strings.ReplaceAll(line, "//", "/")
		}

		if strings.HasPrefix(line, "/") {
			// Ignore absolute paths
			continue
		}

		isDir := false
		if strings.HasSuffix(line, "/") {
			isDir = true
			line = strings.TrimSuffix(line, "/")
		}

		if line == "" {
			continue
		}

		parts := strings.Split(line, "/")
		var cleanParts []string
		for _, p := range parts {
			if p != "" {
				cleanParts = append(cleanParts, p)
			}
		}
		if len(cleanParts) == 0 {
			continue
		}

		// Add parent directories
		current := ""
		for i := 0; i < len(cleanParts)-1; i++ {
			p := cleanParts[i]
			if current != "" {
				current = current + "/" + p
			} else {
				current = p
			}
			filters = append(filters, "+ "+current+"/")
		}

		// Add target path
		if isDir {
			filters = append(filters, "+ "+line+"/")
			filters = append(filters, "+ "+line+"/**")
		} else {
			filters = append(filters, "+ "+line)
		}

		roots[cleanParts[0]] = true
	}

	if len(roots) == 0 {
		return nil
	}

	// Deduplicate preserving order
	seen := make(map[string]bool)
	var deduped []string
	for _, f := range filters {
		if !seen[f] {
			seen[f] = true
			deduped = append(deduped, f)
		}
	}

	deduped = append(deduped, "- *")
	return deduped
}
