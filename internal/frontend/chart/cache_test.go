package chart

import (
	"errors"
	"testing"
)

func TestFrameCacheExactByteBoundLRUReplacementAndOwnership(t *testing.T) {
	t.Parallel()
	firstFrame := testFrame(1)
	secondFrame := testFrame(2)
	budget := firstFrame.ByteSize() + secondFrame.ByteSize()
	cache, err := NewFrameCache(budget)
	if err != nil {
		t.Fatal(err)
	}
	firstKey := testQuery("first")
	secondKey := testQuery("second")
	thirdKey := testQuery("third")
	if err := cache.Put(firstKey, firstFrame); err != nil {
		t.Fatal(err)
	}
	if err := cache.Put(secondKey, secondFrame); err != nil {
		t.Fatal(err)
	}
	if stats := cache.Stats(); stats.Used != budget || stats.Entries != 2 {
		t.Fatalf("exact-bound stats = %+v", stats)
	}
	owned, found := cache.Get(firstKey)
	if !found {
		t.Fatal("first key missing")
	}
	owned.Layers[0].Segments[0].Start.X = 999
	again, _ := cache.Get(firstKey)
	if again.Layers[0].Segments[0].Start.X == 999 {
		t.Fatal("published frame shares cache storage")
	}
	if err := cache.Put(thirdKey, firstFrame); err != nil {
		t.Fatal(err)
	}
	if _, found := cache.Get(secondKey); found {
		t.Fatal("least-recent second key was not evicted")
	}
	if err := cache.Put(firstKey, secondFrame); err != nil {
		t.Fatal(err)
	}
	stats := cache.Stats()
	if stats.Used > stats.Budget || stats.Evictions != 1 {
		t.Fatalf("final stats = %+v", stats)
	}
}

func TestFrameCacheRejectsOversizedEntryWithoutEvicting(t *testing.T) {
	t.Parallel()
	frame := testFrame(1)
	cache, err := NewFrameCache(frame.ByteSize())
	if err != nil {
		t.Fatal(err)
	}
	key := testQuery("kept")
	if err := cache.Put(key, frame); err != nil {
		t.Fatal(err)
	}
	if err := cache.Put(testQuery("oversized"), testFrame(10)); !errors.Is(err, ErrCacheBudget) {
		t.Fatalf("oversized error = %v", err)
	}
	if _, found := cache.Get(key); !found {
		t.Fatal("oversized insert evicted existing entry")
	}
}

func testFrame(segmentCount int) Frame {
	segments := make([]Segment32, segmentCount)
	for index := range segments {
		segments[index] = Segment32{Start: Point32{X: float32(index)}, End: Point32{X: float32(index + 1)}, Style: StylePrimary}
	}
	return Frame{Viewport: Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}, Data: Rect{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}, Layers: []PreparedLayer{{ID: "line", Kind: LayerLine, Segments: segments}}}
}

func testQuery(key string) Query {
	return Query{SeriesKey: key, MinimumX: 0, MaximumX: 10, Resolution: 100}
}
