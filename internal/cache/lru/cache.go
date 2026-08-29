package lru

import (
	"context"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
)

type Cache struct {
	lru *lru.Cache[string, string]
}

func New(cap int) (*Cache, error) {
	l, err := lru.New[string, string](cap)
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	return &Cache{l}, nil
}

func (c *Cache) Get(_ context.Context, key string) (string, bool, error) {
	val, ok := c.lru.Get(key)

	return val, ok, nil
}

func (c *Cache) Set(_ context.Context, key string, value string) error {
	c.lru.Add(key, value)

	return nil
}
