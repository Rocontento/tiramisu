package engines

import "testing"

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
