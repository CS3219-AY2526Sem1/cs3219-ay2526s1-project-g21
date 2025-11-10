package feedback

import (
	"sync"
	"time"

	"peerprep/ai/internal/models"
)

// ContextCache stores request contexts temporarily for feedback collection
// Uses in-memory storage with TTL to avoid database overhead
type ContextCache struct {
	cache map[string]*cacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

type cacheEntry struct {
	context   *models.RequestContext
	expiresAt time.Time
}

// NewContextCache creates a new context cache with the specified TTL
func NewContextCache(ttl time.Duration) *ContextCache {
	cc := &ContextCache{
		cache: make(map[string]*cacheEntry),
		ttl:   ttl,
	}

	// Start background cleanup goroutine
	go cc.cleanupLoop()

	return cc
}

// Set stores a request context with TTL
func (cc *ContextCache) Set(requestID string, ctx *models.RequestContext) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cache[requestID] = &cacheEntry{
		context:   ctx,
		expiresAt: time.Now().Add(cc.ttl),
	}
}

// Get retrieves a request context if it exists and hasn't expired
func (cc *ContextCache) Get(requestID string) (*models.RequestContext, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	entry, exists := cc.cache[requestID]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.context, true
}

// Delete removes a request context from cache
func (cc *ContextCache) Delete(requestID string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	delete(cc.cache, requestID)
}

// cleanupLoop runs periodically to remove expired entries
func (cc *ContextCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cc.cleanup()
	}
}

// cleanup removes expired entries from cache
func (cc *ContextCache) cleanup() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()
	for requestID, entry := range cc.cache {
		if now.After(entry.expiresAt) {
			delete(cc.cache, requestID)
		}
	}
}

// Size returns the current number of cached contexts
func (cc *ContextCache) Size() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return len(cc.cache)
}
