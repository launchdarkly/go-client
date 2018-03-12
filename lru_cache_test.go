package ldclient

import (
  "testing"

  "github.com/stretchr/testify/assert"
)

func TestLRUCache(t *testing.T) {
  cache := newLruCache(1)
  assert.False(t, cache.add(1))
  assert.True(t, cache.add(1))
  assert.False(t, cache.add(2))
  assert.False(t, cache.add(1))
}

func TestEmptyLRUCache(t *testing.T) {
  zeroCache := newLruCache(0)
  zeroCache.add(1)
  assert.False(t,  zeroCache.add(1))
}
