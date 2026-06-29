package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLRUCache_Basic(t *testing.T) {
	c := NewLRUCache(3)

	c.Set("k1", "v1", 1*time.Second)
	c.Set("k2", "v2", 1*time.Second)
	c.Set("k3", "v3", 1*time.Second)

	val, found := c.Get("k1")
	assert.True(t, found)
	assert.Equal(t, "v1", val)

	// Exceed capacity, should evict oldest (k2, since k1 was accessed)
	c.Set("k4", "v4", 1*time.Second)

	_, found = c.Get("k2")
	assert.False(t, found) // evicted

	val, found = c.Get("k1")
	assert.True(t, found)
	assert.Equal(t, "v1", val)
}

func TestLRUCache_Expiration(t *testing.T) {
	c := NewLRUCache(5)
	c.Set("k1", "v1", 50*time.Millisecond)

	val, found := c.Get("k1")
	assert.True(t, found)
	assert.Equal(t, "v1", val)

	time.Sleep(60 * time.Millisecond)

	_, found = c.Get("k1")
	assert.False(t, found) // expired
}
