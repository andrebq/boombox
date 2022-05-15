package authprogram

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
)

type (
	memStore struct {
		cache *bigcache.BigCache
	}
)

func InMemoryTokenStore() TokenStore {
	cache, _ := bigcache.NewBigCache(bigcache.DefaultConfig(10 * time.Minute))
	return &memStore{
		cache: cache,
	}
}

func (m *memStore) Save(ctx context.Context, token string) error {
	m.cache.Set(token, []byte{1})
	return nil
}

func (m *memStore) Lookup(ctx context.Context, token string) (bool, error) {
	buf, err := m.cache.Get(token)
	if err != nil {
		return false, err
	}
	return (len(buf) > 0 && buf[0] == 1), nil
}
