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

func TestProfileInterfaceDefinition(t *testing.T) {
	id := uuid.NewString()
	var p *Profile = &Profile{Id: id}
	var n *Profile
	platform.StorableInterfaceTester(t, p, "profile:", id)
	platform.StructPointerInterfaceTester(t, n, p, *p, "profile:", id)
}

func TestWhisperConversationMapInterfaceDefinition(t *testing.T) {
	platform.StorableInterfaceTester(t, WhisperConversationMap("test"), "whisper-conversations:", "test")
}
