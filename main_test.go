package main

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fuse"

	"tiramisu/internal/config"
	"tiramisu/internal/vfs"
)

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

// A stub corrected by persistResolvedSize must keep every other field sync wrote, or the
// next read loses the fallbacks, the magnet or the imdb id.
func TestPersistResolvedSizeKeepsOtherFields(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/movie_deadbeef.mkv"
	original := `{"url":"http://127.0.0.1:8090/stream?link=` +
		"deadbeef00000000000000000000000000000000" +
		`&index=0&play","size":4294967296,"magnet":"magnet:?xt=urn:btih:deadbeef","imdb":"tt1234567",` +
		`"fallbacks":[{"hash":"aa","index":0,"size":123}]}`
	if err := writeFile(path, original); err != nil {
		t.Fatal(err)
	}

	const realSize = int64(12) * 1024 * 1024 * 1024
	if !persistResolvedSize(path, realSize) {
		t.Fatal("persistResolvedSize reported no rewrite")
	}

	meta, err := vfs.ReadMetadataFromFile(path)
	if err != nil {
		t.Fatalf("rewritten stub no longer parses: %v", err)
	}
	if meta.Size != realSize {
		t.Errorf("size = %d, want %d", meta.Size, realSize)
	}
	// The URL must survive untouched: the file's inode is derived from (hash, index), so a
	// changed index would change the identity of a file the kernel already has cached.
	if got := urlFileIndex(meta.URL); got != 0 {
		t.Errorf("url index changed to %d, want it left at 0", got)
	}
	if meta.ImdbID != "tt1234567" {
		t.Errorf("imdb = %q, want tt1234567", meta.ImdbID)
	}
	if len(meta.Fallbacks) != 1 || meta.Fallbacks[0].Hash != "aa" {
		t.Errorf("fallbacks = %+v, want the one written", meta.Fallbacks)
	}
}

// A size already correct is left untouched: rewriting would churn the file on every open.
func TestPersistResolvedSizeNoOpWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/movie.mkv"
	if err := writeFile(path, `{"url":"http://h/stream?link=abc&index=2&play","size":4294967296}`); err != nil {
		t.Fatal(err)
	}
	if persistResolvedSize(path, 4294967296) {
		t.Error("persistResolvedSize rewrote a stub whose size already matched")
	}
}

// A size outside the range ReadMetadataFromFile accepts must not be written: the stub would
// stop parsing entirely, which loses the title rather than just mis-sizing it.
func TestPersistResolvedSizeRejectsUnreadable(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/movie.mkv"
	const original = `{"url":"http://h/stream?link=abc&index=2&play","size":4294967296}`
	for _, bad := range []int64{0, 50 * 1024 * 1024, 200 * 1024 * 1024 * 1024} {
		if err := writeFile(path, original); err != nil {
			t.Fatal(err)
		}
		if persistResolvedSize(path, bad) {
			t.Errorf("persistResolvedSize accepted an unreadable size %d", bad)
		}
		if _, err := vfs.ReadMetadataFromFile(path); err != nil {
			t.Fatalf("stub stopped parsing after size %d: %v", bad, err)
		}
	}
}

// A legacy line-format stub has no size field to patch; it must be left alone rather than
// replaced with JSON that drops its other lines.
func TestPersistResolvedSizeIgnoresLineFormat(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/legacy.mkv"
	if err := writeFile(path, "http://h/stream?link=abc&index=0&play\n4294967296\nmagnet:?xt=urn:btih:abc\n"); err != nil {
		t.Fatal(err)
	}
	if persistResolvedSize(path, 12*1024*1024*1024) {
		t.Error("persistResolvedSize rewrote a line-format stub")
	}
}

// The resolved-target cache is what lets a later open probe warmup under the key the writes
// use. An unresolved id must never be stored: it would pin the handle to a bad file id.
func TestResolvedTargetCache(t *testing.T) {
	const path = "/movies/TestResolvedTargetCache.mkv"
	resolvedTargets.Delete(path)

	if _, ok := lookupResolvedTarget(path); ok {
		t.Fatal("empty cache reported a hit")
	}
	rememberResolvedTarget(path, 0, 123)
	if _, ok := lookupResolvedTarget(path); ok {
		t.Error("stored an unresolved file id of 0")
	}
	rememberResolvedTarget(path, 3, 456)
	rt, ok := lookupResolvedTarget(path)
	if !ok || rt.fileID != 3 || rt.size != 456 {
		t.Errorf("lookupResolvedTarget = %+v, %v; want {3 456}, true", rt, ok)
	}
	resolvedTargets.Delete(path)
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// The data-slot helpers are the only thing standing between a live MasterConcurrencyLimit and
// an unbounded pump count, so they get exercised directly: the channel is sized to a fixed
// headroom and the real cap is re-read per acquire.
func TestDataSlotHonoursLiveLimit(t *testing.T) {
	prevSem := masterDataSemaphore
	prevCfg := globalConfig.Load()
	defer func() {
		masterDataSemaphore = prevSem
		if prevCfg != nil {
			globalConfig.Store(prevCfg)
		}
	}()

	masterDataSemaphore = make(chan struct{}, dataSlotHeadroom)
	cfg := config.Config{MasterConcurrencyLimit: 2}
	globalConfig.Store(&cfg)

	if !tryAcquireDataSlot() || !tryAcquireDataSlot() {
		t.Fatal("could not take the two slots the limit allows")
	}
	if tryAcquireDataSlot() {
		t.Error("took a third slot against a limit of 2")
	}

	// Raising the limit must take effect immediately: the channel cannot be resized, which is
	// why the cap is applied per acquire rather than by the channel's capacity.
	raised := config.Config{MasterConcurrencyLimit: 3}
	globalConfig.Store(&raised)
	if !tryAcquireDataSlot() {
		t.Error("a raised limit did not free a slot")
	}

	releaseDataSlot()
	releaseDataSlot()
	releaseDataSlot()
	releaseDataSlot()
	if len(masterDataSemaphore) != 0 {
		t.Errorf("slots still held after releasing them all: %d", len(masterDataSemaphore))
	}
	// Releasing with nothing held must not block or panic.
	releaseDataSlot()
}

// A limit of 0 makes the old fixed-capacity channel unbuffered, which meant no pump could ever
// start. The floor keeps one slot available whatever the config says.
func TestDataSlotFloorsAtOne(t *testing.T) {
	prevSem := masterDataSemaphore
	prevCfg := globalConfig.Load()
	defer func() {
		masterDataSemaphore = prevSem
		if prevCfg != nil {
			globalConfig.Store(prevCfg)
		}
	}()

	masterDataSemaphore = make(chan struct{}, dataSlotHeadroom)
	for _, limit := range []int{0, -5} {
		cfg := config.Config{MasterConcurrencyLimit: limit}
		globalConfig.Store(&cfg)
		if !tryAcquireDataSlot() {
			t.Errorf("limit %d left no slot at all", limit)
		}
		releaseDataSlot()
	}
}

// waitForDataSlot must return the slot to the caller, and must give up on a cancelled request
// rather than parking a FUSE read forever.
func TestWaitForDataSlot(t *testing.T) {
	prevSem := masterDataSemaphore
	prevCfg := globalConfig.Load()
	defer func() {
		masterDataSemaphore = prevSem
		if prevCfg != nil {
			globalConfig.Store(prevCfg)
		}
	}()

	masterDataSemaphore = make(chan struct{}, dataSlotHeadroom)
	cfg := config.Config{MasterConcurrencyLimit: 1}
	globalConfig.Store(&cfg)

	ok, errno := waitForDataSlot(context.Background(), time.Second, "/movies/x.mkv")
	if !ok || errno != 0 {
		t.Fatalf("waitForDataSlot on a free semaphore = %v, %v", ok, errno)
	}

	// Saturated now: a cancelled request must come back promptly with EINTR.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ok, errno = waitForDataSlot(ctx, 10*time.Second, "/movies/x.mkv")
	if ok || errno != syscall.EINTR {
		t.Errorf("cancelled wait = %v, %v; want false, EINTR", ok, errno)
	}

	// And a saturated semaphore must time out rather than hang.
	start := time.Now()
	ok, errno = waitForDataSlot(context.Background(), 50*time.Millisecond, "/movies/x.mkv")
	if ok || errno != syscall.ETIMEDOUT {
		t.Errorf("saturated wait = %v, %v; want false, ETIMEDOUT", ok, errno)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("saturated wait took %v, budget was 50ms", elapsed)
	}
	releaseDataSlot()
}

// A lazy stub's declared size is an estimate, and Getattr is where the media server learns
// how long the file is. The node holds the *vfs.Metadata it was built with, so once play-time
// resolution has the torrent's real length the attribute fill has to prefer it - otherwise
// stat() reports the estimate while every read is bounded by the real length, and the player
// seeks into a tail that is not there.
func TestFillAttrPrefersResolvedSize(t *testing.T) {
	prevCfg := globalConfig.Load()
	defer func() {
		if prevCfg != nil {
			globalConfig.Store(prevCfg)
		}
	}()
	globalConfig.Store(&config.Config{FuseBlockSize: 1 << 20})

	const path = "/movies/TestFillAttrPrefersResolvedSize.mkv"
	resolvedTargets.Delete(path)
	defer resolvedTargets.Delete(path)

	const estimate = int64(15 << 30)
	const real = int64(8 << 30)
	meta := &vfs.Metadata{Path: path, Size: estimate}

	var out fuse.Attr
	fillAttrFromMetadata(meta, &out)
	if out.Size != uint64(estimate) {
		t.Fatalf("with nothing resolved, Size = %d; want the stub's %d", out.Size, estimate)
	}

	rememberResolvedTarget(path, 2, real)
	fillAttrFromMetadata(meta, &out)
	if out.Size != uint64(real) {
		t.Errorf("after resolution, Size = %d; want the torrent's real %d", out.Size, real)
	}
	if want := (uint64(real) + 511) / 512; out.Blocks != want {
		t.Errorf("Blocks = %d; want %d, in step with the corrected size", out.Blocks, want)
	}

	// A resolved id with no length attached (the URL-index fallback in resolveTargetFile
	// returns 0) must not blank the size out.
	resolvedTargets.Delete(path)
	rememberResolvedTarget(path, 2, 0)
	fillAttrFromMetadata(meta, &out)
	if out.Size != uint64(estimate) {
		t.Errorf("with an unknown real length, Size = %d; want the stub's %d", out.Size, estimate)
	}
}
