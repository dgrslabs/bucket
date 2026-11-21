package mocks

import (
	"context"
	"errors"

	"github.com/dgrslabs/bucket/internal/domain"
	"github.com/stretchr/testify/mock"
)

type CacheManager struct {
	mock.Mock
}

var _ domain.CacheManager = (*CacheManager)(nil)

func (m *CacheManager) SetBucketValue(ctx context.Context, key string, value any) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *CacheManager) GetBucketValue(ctx context.Context, key string) (any, error) {
	args := m.Called(key)
	return args.Get(0), args.Error(1)
}

func (m *CacheManager) DeleteBucketValue(ctx context.Context, key string) error {
	args := m.Called(key)
	return args.Error(0)
}

type NoopCache struct{}

func NewNoopCache() domain.CacheManager {
	return &NoopCache{}
}

func (n *NoopCache) SetBucketValue(ctx context.Context, key string, value any) error { return nil }
func (n *NoopCache) GetBucketValue(ctx context.Context, key string) (any, error) {
	return nil, errors.New("cache miss")
}
func (n *NoopCache) DeleteBucketValue(ctx context.Context, key string) error { return nil }

type NoopLRUCache struct{}

func NewNoopLRUCache() domain.LRUCache {
	return &NoopLRUCache{}
}

func (m *NoopLRUCache) Close()                            {}
func (M *NoopLRUCache) Get(key string) ([]byte, bool)     { return nil, false }
func (M *NoopLRUCache) Add(key string, value []byte) bool { return true }
func (M *NoopLRUCache) Hits() uint64                      { return 0 }
func (M *NoopLRUCache) Misses() uint64                    { return 0 }
