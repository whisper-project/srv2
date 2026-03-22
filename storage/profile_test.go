/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"errors"
	"testing"

	"github.com/whisper-project/whisper.server2/platform"

	"github.com/google/uuid"
)

func TestProfileInterface(t *testing.T) {
	id := platform.NewId("test-profile-")
	p := &Profile{Id: id, Name: "test", EmailHash: platform.MakeSha1("test@test.com"), Secret: uuid.NewString()}
	var n Profile
	if errs := platform.RedisKeyTester(p, "profile:", id); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
	if errs := platform.RedisValueTester(p, &n, func(l, r *Profile) bool { return *l == *r }); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func TestOwnedConversationMapInterface(t *testing.T) {
	if errs := platform.RedisKeyTester(OwnedConversationMap("test"), "owned-conversations:", "test"); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func TestProfileMethods(t *testing.T) {
	profileId := platform.NewId("test-profile-")
	p, err := CreateNewUser(
		profileId, platform.NewId("test-client-"), "test-type",
		"test-user-name", platform.MakeSha1("test-user@test.com"))
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	if pId, err := GetEmailProfile(p.EmailHash); pId != profileId || err != nil {
		t.Errorf("expected id by email: %v but id: %v, err: %v", profileId, pId, err)
	}
	c1Id, err := GetOwnedConversationIdByName(profileId, "Conversation 1")
	if err != nil {
		t.Errorf("failed to get 'Conversation 1' id by name: %v", err)
	}
	c1, err := GetConversation(c1Id)
	if err != nil {
		t.Errorf("failed to get 'Conversation 1' by name: %v", err)
	}
	if c1.Owner != profileId {
		t.Errorf("expected 'Conversation 1' to be owned by %v, but the owner is %v", profileId, c1.Owner)
	}
	c2, err := CreateNewOwnedConversation(profileId, "Conversation 2")
	if err != nil {
		t.Errorf("failed to create 'Conversation 2': %v", err)
	}
	c2Id, err := GetOwnedConversationIdByName(profileId, "Conversation 2")
	if err != nil {
		t.Errorf("failed to get 'Conversation 2' id by name: %v", err)
	}
	if c2Id != c2.Id {
		t.Errorf("expected 'Conversation 2' id to be %v, but got %v", c2.Id, c2Id)
	}
	if err := DeleteOwnedConversation(profileId, c1.Name); err != nil {
		t.Errorf("failed to delete 'Conversation 1': %v", err)
	}
	if c, err := GetConversation(c1Id); !errors.Is(err, platform.NotFoundError) {
		t.Errorf("expected 'Conversation 1' to be deleted, but got %v, err: %v", c, err)
	}
	if id, err := GetOwnedConversationIdByName(profileId, "Conversation 1"); id != "" || err != nil {
		t.Errorf("expected 'Conversation 1' to be deleted, but got id: %v, err: %v", id, err)
	}
	if errs := DeleteExistingUser(profileId); errs != nil {
		t.Errorf("failed to delete user: %v", errs)
	}
	if c, err := GetConversation(c2Id); !errors.Is(err, platform.NotFoundError) {
		t.Errorf("expected 'Conversation 2' to be deleted, but got %v, err: %v", c, err)
	}
	if m, err := GetOwnedConversationsNameToIdMap(profileId); len(m) != 0 || err != nil {
		t.Errorf("expected the deleted user's conversation map to be empty, but map: %v, err: %v", m, err)
	}
	if _, err := GetProfile(profileId); !errors.Is(err, platform.NotFoundError) {
		t.Errorf("expected the deleted user to be deleted, but got err: %v", err)
	}
}
