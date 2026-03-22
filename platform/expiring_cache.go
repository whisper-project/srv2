/*
 * Copyright 2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// An ExpiringCache is a map from strings to blobs where
// each entry has a limited lifetime.
type ExpiringCache struct {
	Id  string
	Ttl time.Duration
}

// NewExpiringCache creates a new ExpiringCache where each entry's lifetime is ttlSecs.
func NewExpiringCache(id string, ttlSecs uint) *ExpiringCache {
	return &ExpiringCache{Id: id, Ttl: time.Duration(ttlSecs) * time.Second}
}

// AddBlob adds the given blob to the cache with the given id.
func (e *ExpiringCache) AddBlob(ctx context.Context, id string, blob []byte) error {
	db, dbPrefix := GetDb()
	key := dbPrefix + e.Id
	res1 := db.HSet(ctx, key, id, string(blob))
	if err := res1.Err(); err != nil {
		return err
	}
	res2 := db.HExpire(ctx, key, e.Ttl, id)
	if err := res2.Err(); err != nil {
		// if we can't expire the entry, delete it
		_ = db.HDel(ctx, key, id)
		return err
	}
	return nil
}

// GetBlob returns the blob with the given key, or nil if it doesn't exist.
func (e *ExpiringCache) GetBlob(ctx context.Context, id string) ([]byte, error) {
	db, dbPrefix := GetDb()
	key := dbPrefix + e.Id
	res := db.HGet(ctx, key, id)
	if err := res.Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	return []byte(res.Val()), nil
}
