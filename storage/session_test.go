/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/whisper-project/server.golang/protocol"

	"github.com/go-test/deep"

	"github.com/whisper-project/server.golang/platform"

	"github.com/google/uuid"
)

func TestSuspendedSessionInterfaceDefinition(t *testing.T) {
	id := uuid.NewString()
	platform.StorableInterfaceTester(t, suspendedSession(id), "suspended-session-state:", id)
}

func sampleSessionState(id string) *SessionState {
	s := NewSessionState(id)
	start := time.Now().UnixMilli() - 50000
	s.StartedAt = start
	for i := 0; i < 5; i++ {
		c := fmt.Sprintf("client%d", i)
		p := fmt.Sprintf("profile%d", i)
		n := fmt.Sprintf("name%d", i)
		isWhisperer := i == 2
		np := NewParticipant(c, p, n, isWhisperer)
		np.JoinedAt = start + 1000 + 5000*int64(i)
		s.Participants[c] = np
	}
	s.PastText = []PastTextLine{
		{20000, "First line"},
		{25000, "Second line"},
		{30000, "Third line"},
	}
	return s
}

func TestSessionStateResumeSuspendResumeResume(t *testing.T) {
	id := uuid.NewString()
	s0, err := SuspendedSessionState(id)
	if s0 != nil || err != nil {
		t.Fatalf("expected nil state, got %v, %v", s0, err)
	}
	s1 := NewSessionState(id)
	if err = SuspendSessionState(s1); err != nil {
		t.Fatalf("store of new suspended state failed: %v", err)
	}
	s0, err = SuspendedSessionState(id)
	if err != nil {
		t.Fatalf("fetch of new suspended state failed: %v", err)
	}
	if diff := deep.Equal(s1, s0); diff != nil {
		t.Errorf("suspended state mismatch: %v", diff)
	}
	if s1, err = SuspendedSessionState(id); err != nil || s1 != nil {
		t.Fatalf("repeated fetch of suspended state succeeded")
	}
}

func TestSuspendedSessionPacketsInterface(t *testing.T) {
	platform.StorableInterfaceTester(t, suspendedSessionPackets("test"), "suspended-packets:", "test")
}

func TestSessionPacketsResumeSuspendResume(t *testing.T) {
	id := uuid.NewString()
	//goland:noinspection GoStructInitializationWithoutFieldNames
	packets := []protocol.ContentPacket{
		{"a", "a", "a"},
		{"b", "b", "b"},
		{"c", "c", "c"},
	}
	if err := SuspendSessionPackets(id, packets...); err != nil {
		t.Fatalf("store of new suspended packets failed: %v", err)
	}
	p2, err := SuspendedSessionPackets(id)
	if err != nil {
		t.Fatalf("fetch of new suspended packets failed: %v", err)
	}
	if diff := deep.Equal(packets, p2); diff != nil {
		t.Errorf("suspended packets mismatch: %v", diff)
	}
	// expiration is tested elsewhere
}

func TestStoredTranscriptInterfaceDefinition(t *testing.T) {
	id := uuid.NewString()
	platform.StorableInterfaceTester(t, storedTranscript(id), "stored-transcript:", id)
}

func TestNewTranscriptFetchStoreFetch(t *testing.T) {
	cId := uuid.NewString()
	tId := uuid.NewString()
	if transcript, err := StoredTranscript(tId); err != nil || transcript != nil {
		t.Errorf("expected nil transcript, got %v, %v", transcript, err)
	}
	state := sampleSessionState(cId)
	transcript := NewTranscript(tId, state)
	if transcript.WhispererName != "name2" {
		t.Errorf("Expected name to be 'name2', got '%s'", transcript.WhispererName)
	}
	if diff := deep.Equal(state.PastText, transcript.PastText); diff != nil {
		t.Errorf("transcript mismatch: %v", diff)
	}
	if err := StoreTranscript(transcript); err != nil {
		t.Fatalf("store of new transcript failed: %v", err)
	}
	retrieved, err := StoredTranscript(tId)
	if err != nil {
		t.Fatalf("fetch of stored transcript failed: %v", err)
	}
	if diff := deep.Equal(transcript, retrieved); diff != nil {
		t.Errorf("retrieved transcript mismatch: %v", diff)
	}
}
