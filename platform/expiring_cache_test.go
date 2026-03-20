/*
 * Copyright 2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"context"
	"testing"
	"time"
)

func TestExpiringCache(t *testing.T) {
	ctx := context.Background()
	cacheId := NewId("test-cache-")
	cache := NewExpiringCache(cacheId, 1)
	if err := cache.AddBlob(ctx, "id1", []byte("value1")); err != nil {
		t.Fatalf("Failed to add <id1, value1> to cache: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if v, err := cache.GetBlob(ctx, "id1"); err != nil || string(v) != "value1" {
		t.Errorf("Got the wrong value for id1: %v, err: %v", v, err)
	}
	if err := cache.AddBlob(ctx, "id2", []byte("value2")); err != nil {
		t.Fatalf("Failed to add <id2, value2> to cache: %v", err)
	}
	time.Sleep(800 * time.Millisecond)
	if v, err := cache.GetBlob(ctx, "id1"); err != nil {
		t.Fatalf("Err getting expired value for id1: %v", err)
	} else if v != nil {
		t.Errorf("Should have gotten no value for id1 but got %q", string(v))
	}
	if v, err := cache.GetBlob(ctx, "id2"); err != nil || string(v) != "value2" {
		t.Errorf("Got the wrong value for id2: %v, err: %v", string(v), err)
	}
	time.Sleep(1000 * time.Millisecond)
	if v, err := cache.GetBlob(ctx, "id2"); err != nil {
		t.Fatalf("Err getting expired value for id2: %v", err)
	} else if v != nil {
		t.Errorf("Should have gotten no value for id2 but got %q", string(v))
	}
}
