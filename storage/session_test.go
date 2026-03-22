/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/whisper-project/whisper.server2/protocol"

	"github.com/go-test/deep"

	"github.com/whisper-project/whisper.server2/platform"
)

func TestSuspendedSessionIdAddWaitRemoveWait(t *testing.T) {
	id1 := platform.NewId("test-session-state-")
	if err := AddSuspendedSession(id1); err != nil {
		t.Fatalf("failed to add suspended session id %v: %s", id1, err)
	}
	id2, err := WaitForSuspendedSession(1)
	if err != nil {
		t.Fatalf("failed to wait for suspended session id: %s", err)
	}
	if id2 != id1 {
		t.Errorf("retrieved session id %v does not match added session id %v", id2, id1)
	}
	if err := RemoveSuspendedSession(id2); err != nil {
		t.Fatalf("failed to remove retrieved session id %v: %s", id2, err)
	}
	id3, err := WaitForSuspendedSession(1)
	if err != nil {
		t.Fatalf("failed to wait for suspended session id: %s", err)
	}
	if id3 != "" {
		t.Errorf("expected empty session id but got %v", id3)
	}
}

func TestSessionStateInterface(t *testing.T) {
	id := platform.NewId("test-session-state-")
	s := newSampleSessionState(id)
	var n SessionState
	if errs := platform.RedisKeyTester(s, "session-state:", id); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
	if errs := platform.RedisValueTester(s, &n, func(l, r *SessionState) bool { return deep.Equal(l, r) == nil }); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func newSampleSessionState(id string) *SessionState {
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
	id := platform.NewId("test-session-state-")
	s0, err := LoadSessionState(id)
	if s0 != nil || !errors.Is(err, platform.NotFoundError) {
		t.Fatalf("expected nil state, got %v, err %v", s0, err)
	}
	s1 := newSampleSessionState(id)
	if err = SaveSessionState(s1); err != nil {
		t.Fatalf("store of new suspended state failed: %v", err)
	}
	s0, err = LoadSessionState(id)
	if err != nil {
		t.Fatalf("fetch of new suspended state failed: %v", err)
	}
	if diff := deep.Equal(s1, s0); diff != nil {
		t.Errorf("suspended state mismatch: %v", diff)
	}
	s3, err := LoadSessionState(id)
	if s3 != nil || !errors.Is(err, platform.NotFoundError) {
		t.Fatalf("expected nil state, got %v, err %v", s3, err)
	}
}

func TestSuspendedSessionPacketsInterface(t *testing.T) {
	if errs := platform.RedisKeyTester(SuspendedSessionPackets("test"), "suspended-packets:", "test"); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func TestSessionPacketsResumeSuspendResume(t *testing.T) {
	id := platform.NewId("test-session-packets-")
	if p0, err := LoadSuspendedSessionPackets(id); len(p0) != 0 || err != nil {
		t.Errorf("expected no packets, got packets %v, err %v", p0, err)
	}
	packets := []protocol.ContentPacket{
		{"a", "a", "a"},
		{"b", "b", "b"},
		{"c", "c", "c"},
	}
	if err := SaveSuspendedSessionPackets(id, packets...); err != nil {
		t.Fatalf("store of new suspended packets failed: %v", err)
	}
	p2, err := LoadSuspendedSessionPackets(id)
	if err != nil {
		t.Fatalf("fetch of new suspended packets failed: %v", err)
	}
	if diff := deep.Equal(packets, p2); diff != nil {
		t.Errorf("suspended packets mismatch: %v", diff)
	}
	// expiration is tested elsewhere
}

func TestTranscriptInterface(t *testing.T) {
	id := platform.NewId("test-transcript-")
	t1 := &Transcript{
		Id:             id,
		ConversationId: platform.NewId("test-conversation-"),
		WhispererName:  "test-whisperer-name",
		StartTime:      10000,
		EndTime:        50000,
		PastText: []PastTextLine{
			{20000, "First line"},
			{25000, "Second line"},
			{30000, "Third line"},
		},
	}
	var t2 Transcript
	if errs := platform.RedisKeyTester(t1, "transcript:", id); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
	if errs := platform.RedisValueTester(t1, &t2, func(l, r *Transcript) bool { return deep.Equal(l, r) == nil }); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func TestNewTranscriptFetchStoreFetchDeleteFetch(t *testing.T) {
	cId := platform.NewId("test-convo-")
	tId := platform.NewId("test-transcript-")
	if transcript, err := LoadTranscript(tId); !errors.Is(err, platform.NotFoundError) || transcript != nil {
		t.Errorf("expected nil transcript, got %v, err %v", transcript, err)
	}
	state := newSampleSessionState(cId)
	transcript := CreateSessionTranscript(tId, state)
	if transcript.WhispererName != "name2" {
		t.Errorf("Expected name to be 'name2', got '%s'", transcript.WhispererName)
	}
	if diff := deep.Equal(state.PastText, transcript.PastText); diff != nil {
		t.Errorf("transcript mismatch: %v", diff)
	}
	if err := SaveTranscript(transcript); err != nil {
		t.Fatalf("store of new transcript failed: %v", err)
	}
	retrieved, err := LoadTranscript(tId)
	if err != nil {
		t.Fatalf("fetch of stored transcript failed: %v", err)
	}
	if diff := deep.Equal(transcript, retrieved); diff != nil {
		t.Errorf("retrieved transcript mismatch: %v", diff)
	}
	if err := DeleteTranscript(tId); err != nil {
		t.Errorf("delete of deleted transcript failed: %v", err)
	}
	if transcript, err := LoadTranscript(tId); !errors.Is(err, platform.NotFoundError) || transcript != nil {
		t.Errorf("expected nil transcript, got %v, err %v", transcript, err)
	}
}
