package feedback

import (
	"testing"
	"time"

	"peerprep/ai/internal/models"
)

func newTestContext() *models.RequestContext {
	return &models.RequestContext{
		RequestID:   "abc",
		RequestType: "explain",
		Prompt:      "prompt",
		Response:    "response",
	}
}

func TestContextCacheSetGet(t *testing.T) {
	cache := NewContextCache(time.Hour)
	ctx := newTestContext()
	cache.Set("abc", ctx)

	got, ok := cache.Get("abc")
	if !ok {
		t.Fatal("expected to retrieve cached context")
	}
	if got != ctx {
		t.Fatal("expected same pointer from cache")
	}
	if cache.Size() != 1 {
		t.Fatalf("expected size 1, got %d", cache.Size())
	}
}

func TestContextCacheExpiration(t *testing.T) {
	cache := NewContextCache(10 * time.Millisecond)
	cache.Set("abc", newTestContext())
	time.Sleep(20 * time.Millisecond)

	if _, ok := cache.Get("abc"); ok {
		t.Fatal("expected cache entry to expire")
	}
}

func TestContextCacheDeleteAndCleanup(t *testing.T) {
	cache := NewContextCache(time.Hour)
	cache.Set("abc", newTestContext())
	cache.Delete("abc")

	if cache.Size() != 0 {
		t.Fatalf("expected empty cache after delete, got %d", cache.Size())
	}

	cache = NewContextCache(-time.Second)
	cache.Set("abc", newTestContext())
	cache.cleanup()

	if cache.Size() != 0 {
		t.Fatalf("expected cleanup to remove expired entry, got %d", cache.Size())
	}
}
