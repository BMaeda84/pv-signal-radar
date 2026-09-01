package cache

import (
	"testing"
	"time"
)

func TestLRUCache_Basic(t *testing.T) {
	c := New(2, 1*time.Hour)

	c.Set("k1", "v1")
	c.Set("k2", "v2")

	if val, ok := c.Get("k1"); !ok || val != "v1" {
		t.Fatalf("expected to get k1=v1, got %v (ok=%v)", val, ok)
	}

	// Insert k3, should evict k2 because k1 was touched
	c.Set("k3", "v3")

	if _, ok := c.Get("k2"); ok {
		t.Fatalf("expected k2 to be evicted")
	}

	if val, ok := c.Get("k3"); !ok || val != "v3" {
		t.Fatalf("expected k3=v3, got %v", val)
	}
}

func TestLRUCache_TTL(t *testing.T) {
	c := New(5, 50*time.Millisecond)

	c.Set("k1", "v1")
	if val, ok := c.Get("k1"); !ok || val != "v1" {
		t.Fatalf("expected k1 before expiration, got %v", val)
	}

	time.Sleep(60 * time.Millisecond)

	if _, ok := c.Get("k1"); ok {
		t.Fatalf("expected k1 to be expired")
	}
}
