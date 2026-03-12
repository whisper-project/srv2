/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"errors"
	"testing"

	"github.com/whisper-project/srv2/platform"

	"github.com/google/uuid"
)

func TestProfileInterface(t *testing.T) {
	id := platform.NewId("test-profile-")
	p := &Profile{Id: id, Name: "test", EmailHash: platform.MakeSha1("test@test.com"), Secret: uuid.NewString()}
	var n Profile
	platform.RedisKeyTester(t, p, "profile:", id)
	platform.RedisValueTester(t, p, &n, func(l, r *Profile) bool { return l == r })
}

func TestOwnedConversationMapInterface(t *testing.T) {
	platform.RedisKeyTester(t, OwnedConversationMap("test"), "owned-conversations:", "test")
}

func TestProfileMethods(t *testing.T) {
	p, err := CreateNewUser(
		platform.NewId("test-client-"),
		"test-type",
		"test-user-name",
		platform.MakeSha1("test-user@test.com"))
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	if pId, err := GetEmailProfile(p.EmailHash); pId != p.Id || err != nil {
		t.Errorf("expected id by email: %v but id: %v, err: %v", p.Id, pId, err)
	}
	c1Id, err := GetOwnedConversationIdByName(p.Id, "Conversation 1")
	if err != nil {
		t.Errorf("failed to get 'Conversation 1' id by name: %v", err)
	}
	if c1, err := GetConversation(c1Id); err != nil {
		t.Errorf("failed to get 'Conversation 1' by name: %v", err)
	} else if c1 != nil && c1.Owner != p.Id {
		t.Errorf("expected 'Conversation 1' to be owned by %v, but the owner is %v", p.Id, c1.Owner)
	}
	c2, err := CreateNewOwnedConversation(p.Id, "Conversation 2")
	if err != nil {
		t.Errorf("failed to create 'Conversation 2': %v", err)
	}
	c2Id, err := GetOwnedConversationIdByName(p.Id, "Conversation 2")
	if err != nil {
		t.Errorf("failed to get 'Conversation 2' id by name: %v", err)
	}
	if c2Id != c2.Id {
		t.Errorf("expected 'Conversation 2' id to be %v, but got %v", c2.Id, c2Id)
	}
	if err := DeleteOwnedConversation(p.Id, c1Id); err != nil {
		t.Errorf("failed to delete 'Conversation 1': %v", err)
	}
	if _, err := GetConversation(c1Id); !errors.Is(err, platform.NotFoundError) {
		t.Errorf("expected 'Conversation 1' to be deleted, but got err: %v", err)
	}
	if id, err := GetOwnedConversationIdByName(p.Id, "Conversation 1"); id != "" || err != nil {
		t.Errorf("expected 'Conversation 1' to be deleted, but got id: %v, err: %v", id, err)
	}
	if errs := DeleteExistingUser(p.Id); errs != nil {
		t.Errorf("failed to delete user: %v", errs)
	}
	if _, err := GetConversation(c2Id); !errors.Is(err, platform.NotFoundError) {
		t.Errorf("expected 'Conversation 2' to be deleted, but got err: %v", err)
	}
	if m, err := GetOwnedConversationsNameToIdMap(p.Id); m != nil || err != nil {
		t.Errorf("expected the deleted user's conversation map to be empty, but map: %v, err: %v", m, err)
	}
	if _, err := GetProfile(p.Id); !errors.Is(err, platform.NotFoundError) {
		t.Errorf("expected the deleted user to be deleted, but got err: %v", err)
	}
}
