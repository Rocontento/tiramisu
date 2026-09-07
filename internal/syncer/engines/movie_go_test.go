package engines

import (
	"testing"

	"tiramisu/internal/prowlarr"
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
