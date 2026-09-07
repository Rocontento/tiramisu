package vfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMetadataFromFile_Fallbacks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Movie_2026_2160p_abcd1234.mkv")
	stub := `{"url":"http://127.0.0.1:8090/stream?link=` +
		`1111111111111111111111111111111111111111&index=0&play","size":16106127360,` +
		`"magnet":"magnet:?xt=urn:btih:1111111111111111111111111111111111111111",` +
		`"imdb":"tt1234567","fallbacks":[{"hash":"2222222222222222222222222222222222222222","index":0,"size":9663676416}]}`
	if err := os.WriteFile(path, []byte(stub), 0644); err != nil {
		t.Fatal(err)
	}

	meta, err := ReadMetadataFromFile(path)
	if err != nil {
		t.Fatalf("ReadMetadataFromFile: %v", err)
	}
	if len(meta.Fallbacks) != 1 {
		t.Fatalf("expected 1 fallback, got %d", len(meta.Fallbacks))
	}
	if meta.Fallbacks[0].Hash != "2222222222222222222222222222222222222222" {
		t.Errorf("unexpected fallback hash: %s", meta.Fallbacks[0].Hash)
	}
	if meta.Fallbacks[0].Size != 9663676416 {
		t.Errorf("unexpected fallback size: %d", meta.Fallbacks[0].Size)
	}
}

func TestReadMetadataFromFile_NoFallbacks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Movie_2026_1080p_abcd1234.mkv")
	stub := `{"url":"http://127.0.0.1:8090/stream?link=` +
		`1111111111111111111111111111111111111111&index=0&play","size":4294967296,` +
		`"magnet":"magnet:?xt=urn:btih:1111111111111111111111111111111111111111","imdb":"tt1234567"}`
	if err := os.WriteFile(path, []byte(stub), 0644); err != nil {
		t.Fatal(err)
	}

	meta, err := ReadMetadataFromFile(path)
	if err != nil {
		t.Fatalf("ReadMetadataFromFile: %v", err)
	}
	if len(meta.Fallbacks) != 0 {
		t.Errorf("expected no fallbacks, got %d", len(meta.Fallbacks))
	}
}
