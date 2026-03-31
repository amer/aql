package diff

import (
	"strconv"
	"strings"
)

// ParseNumstat parses the output of `git diff HEAD --numstat` into
// a slice of DiffFile (with path and line counts) and aggregate DiffStats.
// Binary files show as "-\t-\tpath" and get IsBinary=true with zero counts.
func ParseNumstat(output string) ([]DiffFile, DiffStats) {
	if strings.TrimSpace(output) == "" {
		return nil, DiffStats{}
	}

	var files []DiffFile
	var stats DiffStats

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		path := parts[2]

		// Binary files have "-" for both added and removed counts.
		if parts[0] == "-" && parts[1] == "-" {
			files = append(files, DiffFile{Path: path, IsBinary: true})
			stats.FilesChanged++
			continue
		}

		added, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		removed, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		files = append(files, DiffFile{
			Path:         path,
			LinesAdded:   added,
			LinesRemoved: removed,
		})
		stats.FilesChanged++
		stats.Additions += added
		stats.Deletions += removed
	}

	return files, stats
}
