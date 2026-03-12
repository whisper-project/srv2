/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"errors"
	"testing"

	"github.com/go-test/deep"
	"github.com/whisper-project/srv2/platform"

	"github.com/google/uuid"
)

func TestConversationInterface(t *testing.T) {
	id := uuid.NewString()
	c := &Conversation{Id: id}
	var n *Conversation
	platform.RedisKeyTester(t, c, "conversation:", id)
	platform.RedisValueTester(t, c, n, func(l, r *Conversation) bool { return l == r })
}

func TestAllowedListenersInterface(t *testing.T) {
	id := uuid.NewString()
	a := AllowedListeners(id)
	platform.RedisKeyTester(t, a, "allowed-listeners:", id)
}

func TestConversationMethods(t *testing.T) {
	id := platform.NewId("test-convo-")
	c1 := NewConversation("owner", "name")
	c1.Id = id
	if err := SaveConversation(c1); err != nil {
		t.Fatalf("failed to save conversation: %v", err)
	}
	c2, err := GetConversation(id)
	if err != nil {
		t.Fatalf("failed to get conversation: %v", err)
	}
	if *c2 != *c1 {
		t.Errorf("Expected conversations to be equal, got c2: %v, c1: %v", c2, c1)
	}
	if isOwner, err := IsOwnedConversation("owner", id); err != nil || !isOwner {
		t.Errorf("Expected conversation to be owned by 'owner', got isOwner: %v, err: %v", isOwner, err)
	}
	if listeners, err := ListAllowedListeners(id); err != nil || len(listeners) != 0 {
		t.Errorf("Expected conversation to have no listeners, got listeners: %v, err: %v", listeners, err)
	}
	if err := AddAllowedListener(id, "listener1"); err != nil {
		t.Errorf("failed to add listener: %v", err)
	}
	if err := AddAllowedListener(id, "listener2"); err != nil {
		t.Errorf("failed to add listener: %v", err)
	}
	if isOk, err := IsAllowedListener(id, "listener1"); err != nil || !isOk {
		t.Errorf("Expected listener to be allowed, got isOk: %v, err: %v", isOk, err)
	}
	if listeners, err := ListAllowedListeners(id); err != nil || len(listeners) != 2 {
		t.Errorf("Expected conversation to have 2 listeners, got listeners: %v, err: %v", listeners, err)
	}
	if err := RemoveAllowedListener(id, "listener1"); err != nil {
		t.Errorf("failed to remove listener: %v", err)
	}
	if isOk, err := IsAllowedListener(id, "listener1"); err != nil || isOk {
		t.Errorf("Expected listener to be removed, got isOk: %v, err: %v", isOk, err)
	}
	if isOk, err := IsAllowedListener(id, "listener2"); err != nil || !isOk {
		t.Errorf("Expected listener to be allowed, got isOk: %v, err: %v", isOk, err)
	}
	if listeners, err := ListAllowedListeners(id); err != nil || deep.Equal(listeners, []string{"listener2"}) != nil {
		t.Errorf("Expected listener list to contain only listener2, got listeners: %v, err: %v", listeners, err)
	}
	if err := RemoveAllowedListener(id, "listener2"); err != nil {
		t.Errorf("failed to remove listener: %v", err)
	}
	if listeners, err := ListAllowedListeners(id); err != nil || len(listeners) != 0 {
		t.Errorf("Expected listener list to be empty, got listeners: %v, err: %v", listeners, err)
	}
	if err := DeleteConversation(id); err != nil {
		t.Errorf("failed to delete conversation: %v", err)
	}
	if _, err := GetConversation(id); !errors.Is(err, platform.NotFoundError) {
		t.Errorf("Expected conversation to be deleted, got err: %v", err)
	}
}
