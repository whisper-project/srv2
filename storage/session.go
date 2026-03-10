/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"context"
	"errors"
	"time"

	"github.com/whisper-project/server.golang/protocol"

	"github.com/redis/go-redis/v9"
	"github.com/whisper-project/server.golang/platform"
)

var SuspendedSessionList = platform.StorableList("suspended-session-list")

func SuspendSession(id string) error {
	if err := platform.PushRange(context.Background(), SuspendedSessionList, true, id); err != nil {
		return err
	}
	return nil
}

func RemoveSuspendedSession(id string) error {
	if err := platform.RemoveElement(context.Background(), SuspendedSessionList, 1, id); err != nil {
		return err
	}
	return nil
}

func WaitForSuspendedSession(timeout time.Duration) (string, error) {
	id, err := platform.FetchOneBlocking(context.Background(), SuspendedSessionList, false, timeout)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

type SessionState struct {
	Id           string
	Participants ParticipantMap
	Waitlist     []*Participant
	PastText     []PastTextLine
	StartedAt    int64
	EndedAt      int64
}

func NewSessionState(id string) *SessionState {
	return &SessionState{
		Id:           id,
		Participants: make(ParticipantMap),
		StartedAt:    time.Now().UnixMilli(),
	}
}

type ParticipantMap map[string]*Participant

type Participant struct {
	ClientId    string `json:"clientId"`
	ProfileId   string `json:"profileId"`
	Name        string `json:"name"`
	IsWhisperer bool   `json:"isWhisperer"`
	IsOnline    bool   `json:"isOnline"`
	JoinedAt    int64  `json:"joinedAt"`
}

func NewParticipant(clientId, profileId, name string, isWhisperer bool) *Participant {
	return &Participant{
		ClientId:    clientId,
		ProfileId:   profileId,
		Name:        name,
		IsWhisperer: isWhisperer,
		JoinedAt:    time.Now().UnixMilli(),
	}
}

type PastTextLine struct {
	Time int64
	Text string
}

type suspendedSession string

func (s suspendedSession) StoragePrefix() string {
	return "suspended-session-state:"
}

func (s suspendedSession) StorageId() string {
	return string(s)
}

func SuspendSessionState(s *SessionState) error {
	if err := platform.StoreGob(context.Background(), suspendedSession(s.Id), s); err != nil {
		return err
	}
	return nil
}

func SuspendedSessionState(id string) (*SessionState, error) {
	var state SessionState
	if err := platform.FetchGob(context.Background(), suspendedSession(id), &state); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	// once the state is picked up, it's maintained by the server, and so not accurate.
	_ = platform.DeleteStorage(context.Background(), suspendedSession(id))
	return &state, nil
}

type suspendedSessionPackets string

func (s suspendedSessionPackets) StoragePrefix() string {
	return "suspended-packets:"
}

func (s suspendedSessionPackets) StorageId() string {
	return string(s)
}

func SuspendedSessionPackets(id string) ([]protocol.ContentPacket, error) {
	packetStrings, err := platform.FetchRange(context.Background(), suspendedSessionPackets(id), 0, -1)
	if err != nil {
		return nil, err
	}
	// once the packets are picked up, any new ones will be ignored,
	// so expire them as soon as the old server has finished shutting down.
	_ = platform.SetExpiration(context.Background(), suspendedSessionPackets(id), 30)
	packets := make([]protocol.ContentPacket, len(packetStrings))
	for i, s := range packetStrings {
		packets[i] = protocol.ParseContentPacket(s)
	}
	return packets, nil
}

func SuspendSessionPackets(id string, packets ...protocol.ContentPacket) error {
	if len(packets) == 0 {
		return nil
	}
	packetStrings := make([]string, len(packets))
	for i, p := range packets {
		packetStrings[i] = p.String()
	}
	loc := suspendedSessionPackets(id)
	if err := platform.PushRange(context.Background(), loc, false, packetStrings...); err != nil {
		return err
	}
	return nil
}

type Transcript struct {
	Id             string
	ConversationId string
	WhispererName  string
	StartTime      int64
	EndTime        int64
	PastText       []PastTextLine
}

func NewTranscript(id string, state *SessionState) *Transcript {
	whispererName := "Unknown Whisperer"
	for _, p := range state.Participants {
		if p.IsWhisperer {
			whispererName = p.Name
			break
		}
	}
	return &Transcript{
		Id:             id,
		ConversationId: state.Id,
		WhispererName:  whispererName,
		StartTime:      state.StartedAt,
		EndTime:        state.EndedAt,
		PastText:       state.PastText,
	}
}

type storedTranscript string

func (s storedTranscript) StoragePrefix() string {
	return "stored-transcript:"
}

func (s storedTranscript) StorageId() string {
	return string(s)
}

func StoreTranscript(t *Transcript) error {
	if err := platform.StoreGob(context.Background(), storedTranscript(t.Id), t); err != nil {
		return err
	}
	var yearSecs int64 = 365 * 24 * 60 * 60
	if err := platform.SetExpiration(context.Background(), storedTranscript(t.Id), 1*yearSecs); err != nil {
		return err
	}
	return nil
}

func StoredTranscript(id string) (*Transcript, error) {
	var t Transcript
	if err := platform.FetchGob(context.Background(), storedTranscript(id), &t); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}
