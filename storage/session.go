/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"time"

	"github.com/whisper-project/whisper.server2/platform"
	"github.com/whisper-project/whisper.server2/protocol"
	"go.uber.org/zap"

	"github.com/redis/go-redis/v9"
)

// The SuspendedSessionQueue keeps track of the ids of sessions that are in progress while
// one server shuts down and another takes over.
var SuspendedSessionQueue = platform.StorableList("suspended-session-list")

// AddSuspendedSession adds a session to the back of the queue.
func AddSuspendedSession(id string) error {
	if err := platform.PushListMembers(context.Background(), SuspendedSessionQueue, true, id); err != nil {
		return err
	}
	return nil
}

// RemoveSuspendedSession removes a session from the queue by session id.
func RemoveSuspendedSession(id string) error {
	if err := platform.RemoveListElement(context.Background(), SuspendedSessionQueue, 1, id); err != nil {
		return err
	}
	return nil
}

// WaitForSuspendedSession blocks until a session is available in the front of the queue
// or until the timeout (in seconds) is reached. It returns the session id, or "" on timeout.
func WaitForSuspendedSession(timeout uint) (string, error) {
	wait := time.Duration(timeout) * time.Second
	id, err := platform.FetchListMemberBlocking(context.Background(), SuspendedSessionQueue, false, wait)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

// A SessionState holds the current state of a session _other than_
// the current live text packets. At the end of the session, the
// session transcript is created from the past text.
type SessionState struct {
	Id           string
	Participants ParticipantMap
	Waitlist     []*Participant
	PastText     []PastTextLine
	StartedAt    int64 // epoch millis of start time
	EndedAt      int64 // epoch millis of end time, or 0 if not yet ended
}

func (s *SessionState) StoragePrefix() string {
	return "session-state:"
}

func (s *SessionState) StorageId() string {
	if s == nil {
		return ""
	}
	return s.Id
}

func (s *SessionState) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (s *SessionState) FromRedis(b []byte) error {
	*s = SessionState{}
	return gob.NewDecoder(bytes.NewReader(b)).Decode(s)
}

// SaveSessionState saves the session state to the database.
//
// Since sessions are always monitored by a single server, the
// SessionState is generally kept in memory. But when a session
// is passed from one server to another, it is saved by the
// server that is shutting down and loaded by the server that
// is starting up.
func SaveSessionState(s *SessionState) error {
	if err := platform.StoreObject(sCtx(), s); err != nil {
		sLog().Error("storage failure (save) on SessionState",
			zap.String("id", s.Id), zap.Error(err))
		return err
	}
	return nil
}

// LoadSessionState loads the session state from the database.
//
// Once the session has been loaded by the server that is starting up,
// its saved state is no longer needed. So, after successful
// retrieval, the state is deleted from the database.
func LoadSessionState(id string) (*SessionState, error) {
	s := &SessionState{Id: id}
	if err := platform.FetchObject(sCtx(), s); err != nil {
		sLog().Error("storage failure (load) on SessionState",
			zap.String("id", id), zap.Error(err))
		return nil, err
	}
	_ = platform.DeleteStorage(sCtx(), s)
	return s, nil
}

// NewSessionState starts a new session with the given id, returning its state.
func NewSessionState(id string) *SessionState {
	return &SessionState{
		Id:           id,
		Participants: make(ParticipantMap),
		StartedAt:    time.Now().UnixMilli(),
	}
}

// A ParticipantMap is a map of conversation participants by their client ID.
type ParticipantMap map[string]*Participant

// A Participant represents a participant in a conversation.
//
// It's serialized to JSON when transmitted to clients.
type Participant struct {
	ClientId    string `json:"clientId"`
	ProfileId   string `json:"profileId"`
	Name        string `json:"name"`
	IsWhisperer bool   `json:"isWhisperer"`
	IsOnline    bool   `json:"isOnline"`
	JoinedAt    int64  `json:"joinedAt"`
}

// NewParticipant creates a new participant and marks them as joining the session.
func NewParticipant(clientId, profileId, name string, isWhisperer bool) *Participant {
	return &Participant{
		ClientId:    clientId,
		ProfileId:   profileId,
		Name:        name,
		IsWhisperer: isWhisperer,
		JoinedAt:    time.Now().UnixMilli(),
	}
}

// A PastTextLine has a creation timestamp (in millis) and the text of the line.
type PastTextLine struct {
	Time int64
	Text string
}

// The SuspendedSessionPackets for a session ID is the chronologically ordered
// list of live text packets (serialized as strings) that were received by
// the server being suspended during the overlap between it shutting down
// and the new server starting up.
type SuspendedSessionPackets string

func (s SuspendedSessionPackets) StoragePrefix() string {
	return "suspended-packets:"
}

func (s SuspendedSessionPackets) StorageId() string {
	return string(s)
}

// SaveSuspendedSessionPackets saves the given packets at the end of the list.
func SaveSuspendedSessionPackets(id string, packets ...protocol.ContentPacket) error {
	if len(packets) == 0 {
		// notest
		return nil
	}
	packetStrings := make([]string, len(packets))
	for i, p := range packets {
		packetStrings[i] = p.String()
	}
	loc := SuspendedSessionPackets(id)
	if err := platform.PushListMembers(context.Background(), loc, false, packetStrings...); err != nil {
		sLog().Error("storage failure (save) on suspended packets",
			zap.String("session_id", id), zap.Error(err))
		return err
	}
	return nil
}

// LoadSuspendedSessionPackets loads the packets from the list.
//
// Unlike the load of session state, this does not immediately delete the loaded list,
// because the server that is suspending may still try to save more packets to it.
// Instead, the list is set to expire 30 seconds after being loaded, which gives
// the server that is shutting down time to complete its exit. (The new server won't
// need any of the additional packets saved by the old server, because the new server
// doesn't fetch the suspended packets until it's subscribed to the session and
// receiving live packets directly.)
func LoadSuspendedSessionPackets(id string) ([]protocol.ContentPacket, error) {
	packetStrings, err := platform.FetchListRange(sCtx(), SuspendedSessionPackets(id), 0, -1)
	if err != nil {
		return nil, err
	}
	if err = platform.SetExpiration(sCtx(), SuspendedSessionPackets(id), 30); err != nil {
		// log but don't return this error, because it just means the suspended packets will
		// hang around in the database until they are cleaned up.
		// notest
		sLog().Error("storage failure (set expiration) on suspended packets",
			zap.String("session_id", id), zap.Error(err))
	}
	packets := make([]protocol.ContentPacket, len(packetStrings))
	for i, s := range packetStrings {
		packets[i] = protocol.ParseContentPacket(s)
	}
	return packets, nil
}

// A Transcript holds all the session metadata needed to show a textual transcript
// of a session once it's over.
type Transcript struct {
	Id             string
	ConversationId string
	WhispererName  string
	StartTime      int64
	EndTime        int64
	PastText       []PastTextLine
}

func (t *Transcript) StoragePrefix() string {
	return "transcript:"
}

func (t *Transcript) StorageId() string {
	if t == nil {
		return ""
	}
	return t.Id
}

func (t *Transcript) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(t); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (t *Transcript) FromRedis(b []byte) error {
	*t = Transcript{} // dump old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(t)
}

func CreateSessionTranscript(id string, state *SessionState) *Transcript {
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

// SaveTranscript saves the given transcript to the database.
//
// Transcripts are stored for a max of 1 year.
func SaveTranscript(t *Transcript) error {
	if err := platform.StoreObject(sCtx(), t); err != nil {
		sLog().Error("storage failure (save) on Transcript",
			zap.String("id", t.Id), zap.Error(err))
		return err
	}
	const yearSecs int64 = 365 * 24 * 60 * 60
	if err := platform.SetExpiration(sCtx(), t, 1*yearSecs); err != nil {
		// log but don't return this error, because it just means the transcript will
		// hang around in the database until it is cleaned up.
		// notest
		sLog().Error("storage failure (expiration) on Transcript",
			zap.String("id", t.Id), zap.Error(err))
	}
	return nil
}

// LoadTranscript loads the given transcript from the database.
// If the transcript is not found, nil (with no error) is returned.
//
// When a transcript is loaded, its expiration date is reset, so the
// recipient has a year to access it.
func LoadTranscript(id string) (*Transcript, error) {
	t := &Transcript{Id: id}
	if err := platform.FetchObject(sCtx(), t); err != nil {
		if errors.Is(err, redis.Nil) {
			// notest
			return nil, nil
		}
		return nil, err
	}
	const yearSecs int64 = 365 * 24 * 60 * 60
	if err := platform.SetExpiration(sCtx(), t, 1*yearSecs); err != nil {
		// log but don't return this error, because the transcript still got loaded.
		// notest
		sLog().Error("storage failure (expiration) on Transcript",
			zap.String("id", t.Id), zap.Error(err))
	}
	return t, nil
}

// DeleteTranscript deletes the transcript with the given ID from the database.
//
// Deleting a non-existent transcript is a no-op.
func DeleteTranscript(id string) error {
	t := &Transcript{Id: id}
	if err := platform.DeleteStorage(sCtx(), t); err != nil {
		sLog().Error("storage failure (delete) on Transcript",
			zap.String("id", t.Id), zap.Error(err))
		return err
	}
	return nil
}
