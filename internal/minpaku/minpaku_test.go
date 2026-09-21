package minpaku

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minpaku.csv")
	content := "都道府県,届出住宅数\n東京都,1234\n福岡県,567\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test csv: %v", err)
	}

	counts, err := LoadCSV(path)
	if err != nil {
		t.Fatalf("LoadCSV returned error: %v", err)
	}
	if counts["東京都"] != 1234 {
		t.Errorf("counts[東京都] = %d, want 1234", counts["東京都"])
	}
	if counts["福岡県"] != 567 {
		t.Errorf("counts[福岡県] = %d, want 567", counts["福岡県"])
	}
}

func TestLoadCSV_MissingFile(t *testing.T) {
	if _, err := LoadCSV("/nonexistent/path.csv"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestLoadCSV_InvalidCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minpaku.csv")
	content := "都道府県,届出住宅数\n東京都,not-a-number\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test csv: %v", err)
	}

	if _, err := LoadCSV(path); err == nil {
		t.Fatal("expected an error for a non-numeric count")
	}
}
