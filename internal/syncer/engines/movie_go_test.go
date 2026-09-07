package engines

import (
	"strings"
	"testing"

	"tiramisu/internal/prowlarr"
	"tiramisu/internal/vfs"
)

func TestEstimateFileSize(t *testing.T) {
	cases := []struct {
		name string
		c    MovieStream
		want int64
	}{
		{"known size", MovieStream{SizeGB: 8.5}, int64(8.5 * 1024 * 1024 * 1024)},
		{"unknown size 4K", MovieStream{SizeGB: 0, Is4K: true}, 15 * 1024 * 1024 * 1024},
		{"unknown size 1080p", MovieStream{SizeGB: 0, Is4K: false}, 4 * 1024 * 1024 * 1024},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := estimateFileSize(tc.c); got != tc.want {
				t.Errorf("estimateFileSize(%+v) = %d, want %d", tc.c, got, tc.want)
			}
		})
	}
}

func TestWatchlistEstimateSize(t *testing.T) {
	var titleGB float64 = 8.2
	cases := []struct {
		name string
		c    prowlarr.Stream
		want int64
	}{
		{"size in title", prowlarr.Stream{Title: "Movie.2026.1080p 💾 8.2 GB 👤 50"}, int64(titleGB * 1024 * 1024 * 1024)},
		{"no size, 4K", prowlarr.Stream{Title: "Movie.2026.2160p.UHD"}, 15 * 1024 * 1024 * 1024},
		{"no size, 1080p", prowlarr.Stream{Title: "Movie.2026.1080p"}, 4 * 1024 * 1024 * 1024},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := watchlistEstimateSize(tc.c); got != tc.want {
				t.Errorf("watchlistEstimateSize(%q) = %d, want %d", tc.c.Title, got, tc.want)
			}
		})
	}
}

func TestTVEstimateSize(t *testing.T) {
	cases := []struct {
		name string
		s    TVStream
		want int64
	}{
		{"known size", TVStream{SizeGB: 2.5}, int64(2.5 * 1024 * 1024 * 1024)},
		{"unknown size 4K", TVStream{Title: "Show.S01E01.2160p"}, 4 * 1024 * 1024 * 1024},
		{"unknown size 1080p", TVStream{Title: "Show.S01E01.1080p"}, 1500 * 1024 * 1024},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tvEstimateSize(tc.s); got != tc.want {
				t.Errorf("tvEstimateSize(%+v) = %d, want %d", tc.s, got, tc.want)
			}
		})
	}
}

// A stub whose size falls outside vfs's 100MB..100GB window does not read back as a smaller
// file — ReadMetadataFromFile rejects it outright, so the title appears in the library and
// then cannot be opened at all. Eager sync wrote the torrent's real length and could never
// hit this; a lazy estimate parsed out of indexer text can.
func TestClampStubSizeStaysReadable(t *testing.T) {
	cases := []struct {
		name string
		in   int64
	}{
		{"sample-sized release", 50 * 1024 * 1024},
		{"zero", 0},
		{"negative", -1},
		{"absurd remux", 400 * 1024 * 1024 * 1024},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := clampStubSize(tc.in)
			if got < vfs.MinFileSize || got > vfs.MaxFileSize {
				t.Errorf("clampStubSize(%d) = %d, outside the readable range", tc.in, got)
			}
		})
	}

	if got := clampStubSize(8 * 1024 * 1024 * 1024); got != 8*1024*1024*1024 {
		t.Errorf("clampStubSize altered an in-range size: got %d", got)
	}
}

// Every estimator feeds a stub, so none of them may produce an unreadable size.
func TestEstimatorsStayInReadableRange(t *testing.T) {
	sizes := []int64{
		estimateFileSize(MovieStream{SizeGB: 0.05}),
		estimateFileSize(MovieStream{SizeGB: 0}),
		estimateFileSize(MovieStream{SizeGB: 0, Is4K: true}),
		tvEstimateSize(TVStream{SizeGB: 0.05}),
		tvEstimateSize(TVStream{Title: "Show.S01E01.1080p"}),
		watchlistEstimateSize(prowlarr.Stream{Title: "Movie 💾 50 MB"}),
		watchlistEstimateSize(prowlarr.Stream{Title: "Movie.2026.1080p"}),
	}
	for i, s := range sizes {
		if s < vfs.MinFileSize || s > vfs.MaxFileSize {
			t.Errorf("estimator %d returned %d, outside the readable range", i, s)
		}
	}
}

// Indexer results are raw remote input. Lazy sync writes the hash straight into a filename
// (hash[:8], hash[len-8:]) with no AddTorrent in between to reject a malformed one.
func TestValidInfoHash(t *testing.T) {
	valid := "deadbeef00112233445566778899aabbccddeeff"
	cases := []struct {
		in   string
		want bool
	}{
		{valid, true},
		{strings.ToUpper(valid), true},
		{"", false},
		{"abc", false},
		{valid[:39], false},
		{valid + "f", false},
		{"zzzzbeef00112233445566778899aabbccddeeff", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := ValidInfoHash(tc.in); got != tc.want {
				t.Errorf("ValidInfoHash(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
