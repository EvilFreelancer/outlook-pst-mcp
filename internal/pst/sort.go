package pst

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func sortExtractedMessages(messages []ExtractedMessage) {
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].FolderPath != messages[j].FolderPath {
			return messages[i].FolderPath < messages[j].FolderPath
		}
		return emlNumericOrder(messages[i].Path) < emlNumericOrder(messages[j].Path)
	})
}

func emlNumericOrder(path string) int {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	n, err := strconv.Atoi(base)
	if err != nil {
		return 0
	}
	return n
}
