package pipeline

import (
	jsonv2 "encoding/json/v2"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenNextCommandsContainNoPlaceholdersOrStalePaths(t *testing.T) {
	root := repoRootForTest(t)
	err := filepath.WalkDir(filepath.Join(root, "testdata"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read golden %s: %v", path, err)
			return nil
		}
		var golden struct {
			NextCommands []string `json:"nextCommands"`
		}
		if err := jsonv2.Unmarshal(data, &golden); err != nil {
			t.Errorf("decode golden %s: %v", path, err)
			return nil
		}
		for _, command := range golden.NextCommands {
			if strings.Contains(command, "<") || strings.HasPrefix(strings.TrimSpace(command), "#") || strings.Contains(command, "work/after") {
				t.Errorf("golden %s has non-executable nextCommand %q", path, command)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk testdata goldens: %v", err)
	}
}
