/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"testing"

	"github.com/whisper-project/server.golang/platform"

	"github.com/google/uuid"
)

func TestConversationInterfaceDefinition(t *testing.T) {
	id := uuid.NewString()
	var p *Conversation = &Conversation{Id: id}
	var n *Conversation
	platform.StorableInterfaceTester(t, p, "conversation:", id)
	platform.StructPointerInterfaceTester(t, n, p, *p, "conversation:", id)
}

func TestNewConversation(t *testing.T) {
	c := NewConversation("owner", "name")
	if c.Id == "" {
		t.Error("id not set")
	}
	if c.Owner != "owner" {
		t.Errorf("Expected owner to be 'owner', got '%s'", c.Owner)
	}
	if c.Name != "name" {
		t.Errorf("Expected name to be 'name', got '%s'", c.Name)
	}
}

func TestAllowedListenersInterface(t *testing.T) {
	id := uuid.NewString()
	a := AllowedListeners(id)
	platform.StorableInterfaceTester(t, a, "allowed-listeners:", id)
}
