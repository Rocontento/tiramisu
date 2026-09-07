package vfs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

// The startup cache pre-populates the whole library, so a Metadata field it forgets is a
// field the FUSE layer never sees for any existing file. ToMetadata exists so both callers
// share one conversion; this checks it carries everything, Fallbacks included.
func TestToMetadataCarriesEveryField(t *testing.T) {
	fm := &FileMetadata{
		URL:    "http://h/stream?link=abc&index=0&play",
		Size:   4 * 1024 * 1024 * 1024,
		Mtime:  time.Unix(1700000000, 0),
		Path:   "/movies/x.mkv",
		ImdbID: "tt1234567",
		Fallbacks: []FallbackCandidate{
			{Hash: "aa", Index: 0, Size: 1},
			{Hash: "bb", Index: 2, Size: 2},
		},
	}
	m := fm.ToMetadata()
	if m.URL != fm.URL || m.Size != fm.Size || !m.Mtime.Equal(fm.Mtime) ||
		m.Path != fm.Path || m.ImdbID != fm.ImdbID {
		t.Errorf("ToMetadata() = %+v, does not match %+v", m, fm)
	}
	if len(m.Fallbacks) != len(fm.Fallbacks) {
		t.Fatalf("Fallbacks dropped: got %d, want %d", len(m.Fallbacks), len(fm.Fallbacks))
	}
	for i := range fm.Fallbacks {
		if m.Fallbacks[i] != fm.Fallbacks[i] {
			t.Errorf("Fallbacks[%d] = %+v, want %+v", i, m.Fallbacks[i], fm.Fallbacks[i])
		}
	}

	if (*FileMetadata)(nil).ToMetadata() != nil {
		t.Error("ToMetadata() on nil should return nil")
	}
}
