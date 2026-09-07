package main

import (
	"os"
	"testing"

	"tiramisu/internal/vfs"
)

// A lazy stub carries index=0, which is not a GoStorm file id at all (ids are 1-based).
// Once the real index is resolved it has to be written back into the URL verbatim, so the
// next open finds it and warmup lands under the same key the reads use.
func TestReplaceURLIndex(t *testing.T) {
	cases := []struct {
		name string
		url  string
		idx  int
		want string
	}{
		{
			"lazy stub, index in the middle",
			"http://127.0.0.1:8090/stream?link=abc&index=0&play",
			2,
			"http://127.0.0.1:8090/stream?link=abc&index=2&play",
		},
		{
			"index is the last parameter",
			"http://127.0.0.1:8090/stream?link=abc&index=0",
			11,
			"http://127.0.0.1:8090/stream?link=abc&index=11",
		},
		{
			"no index parameter at all",
			"http://127.0.0.1:8090/stream?link=abc",
			3,
			"http://127.0.0.1:8090/stream?link=abc&index=3",
		},
		{"empty url", "", 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := replaceURLIndex(tc.url, tc.idx); got != tc.want {
				t.Errorf("replaceURLIndex(%q, %d) = %q, want %q", tc.url, tc.idx, got, tc.want)
			}
		})
	}
}

// replaceURLIndex has to round-trip through the parser the rest of the code reads with,
// otherwise a written-back index is invisible on the next open.
func TestReplaceURLIndexRoundTrip(t *testing.T) {
	url := "http://127.0.0.1:8090/stream?link=abc&index=0&play"
	for _, idx := range []int{1, 7, 42} {
		rewritten := replaceURLIndex(url, idx)
		if got := urlFileIndex(rewritten); got != idx {
			t.Errorf("urlFileIndex(%q) = %d, want %d", rewritten, got, idx)
		}
		_, got := vfs.ExtractHashAndIndex(rewritten)
		if got != idx {
			t.Errorf("vfs.ExtractHashAndIndex(%q) index = %d, want %d", rewritten, got, idx)
		}
	}
}

func TestURLFileIndex(t *testing.T) {
	cases := []struct {
		url  string
		want int
	}{
		{"http://h/stream?link=abc&index=0&play", 0},
		{"http://h/stream?link=abc&index=4&play", 4},
		{"http://h/stream?link=abc&play", 0},
		{"http://h/stream?link=abc&index=notanumber&play", 0},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			if got := urlFileIndex(tc.url); got != tc.want {
				t.Errorf("urlFileIndex(%q) = %d, want %d", tc.url, got, tc.want)
			}
		})
	}
}

// A stub rewritten by persistResolvedIndex must keep every other field sync wrote, or the
// next read loses the fallbacks, the magnet or the imdb id.
func TestPersistResolvedIndexKeepsOtherFields(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/movie_deadbeef.mkv"
	original := `{"url":"http://127.0.0.1:8090/stream?link=` +
		"deadbeef00000000000000000000000000000000" +
		`&index=0&play","size":4294967296,"magnet":"magnet:?xt=urn:btih:deadbeef","imdb":"tt1234567",` +
		`"fallbacks":[{"hash":"aa","index":0,"size":123}]}`
	if err := writeFile(path, original); err != nil {
		t.Fatal(err)
	}

	newURL, ok := persistResolvedIndex(path, 3)
	if !ok {
		t.Fatal("persistResolvedIndex reported no rewrite")
	}
	if got := urlFileIndex(newURL); got != 3 {
		t.Errorf("returned url has index %d, want 3", got)
	}

	meta, err := vfs.ReadMetadataFromFile(path)
	if err != nil {
		t.Fatalf("rewritten stub no longer parses: %v", err)
	}
	if got := urlFileIndex(meta.URL); got != 3 {
		t.Errorf("stub on disk has index %d, want 3", got)
	}
	if meta.Size != 4294967296 {
		t.Errorf("size = %d, want 4294967296", meta.Size)
	}
	if meta.ImdbID != "tt1234567" {
		t.Errorf("imdb = %q, want tt1234567", meta.ImdbID)
	}
	if len(meta.Fallbacks) != 1 || meta.Fallbacks[0].Hash != "aa" {
		t.Errorf("fallbacks = %+v, want the one written", meta.Fallbacks)
	}
}

// A stub that already carries a real index is left untouched: rewriting it would churn the
// file (and its mtime) on every single open.
func TestPersistResolvedIndexNoOpWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/movie.mkv"
	if err := writeFile(path, `{"url":"http://h/stream?link=abc&index=2&play","size":4294967296}`); err != nil {
		t.Fatal(err)
	}
	if _, ok := persistResolvedIndex(path, 2); ok {
		t.Error("persistResolvedIndex rewrote a stub whose index already matched")
	}
}

// A legacy line-format stub has no url field to patch; it must be left alone rather than
// replaced with JSON that drops its other lines.
func TestPersistResolvedIndexIgnoresLineFormat(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/legacy.mkv"
	if err := writeFile(path, "http://h/stream?link=abc&index=0&play\n4294967296\nmagnet:?xt=urn:btih:abc\n"); err != nil {
		t.Fatal(err)
	}
	if _, ok := persistResolvedIndex(path, 2); ok {
		t.Error("persistResolvedIndex rewrote a line-format stub")
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
