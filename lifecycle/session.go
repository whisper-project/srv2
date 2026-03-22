/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/whisper-project/srv2/speech"

	"go.uber.org/zap"

	"github.com/whisper-project/srv2/protocol"
	"github.com/whisper-project/srv2/pubsub"
	"github.com/whisper-project/srv2/storage"
)

var (
	ably                = pubsub.GetAblyManager()
	resemble            = speech.GetResembleManager()
	sessions            = make(map[string]*Session)
	AlreadyPresentError = fmt.Errorf("already present")
	NotPresentError     = fmt.Errorf("not present")
)

// A Session is one continuous instance of a conversation with a single
// Whisperer (possibly on multiple client devices) and multiple Listeners.
//
// Sessions are kept in memory on one server instance. If that server
// shuts down while the session is in progress, it saves the session's
// state so that it can be reloaded and resumed on a different server.
type Session struct {
	Id           string // the conversation ID this is a session for
	Owner        string // the ID of the Whisperer that owns the conversation
	Pubsub       pubsub.Manager
	speech       speech.Manager
	state        *storage.SessionState
	cr           protocol.ContentReceiver
	sr           pubsub.StatusReceiver
	cancel       context.CancelFunc
	livePackets  []protocol.ContentPacket
	liveText     string
	overlap      []protocol.ContentChunk
	shuttingDown bool
	transcriptId string
}

// AuthenticateParticipant gets an appropriate pubsub token for a client.
// If it returns a nil token, then the client cannot authenticate against the session.
func AuthenticateParticipant(conversationId, clientId string) (json.RawMessage, error) {
	s, ok := sessions[conversationId]
	if !ok {
		return nil, nil
	}
	tok, err := s.Pubsub.ClientToken(conversationId, clientId)
	if err != nil {
		sLog().Error("ably client token failure",
			zap.String("sessionId", conversationId), zap.String("clientId", clientId),
			zap.Error(err))
		return nil, err
	}
	return tok, nil
}

// GetSession finds or creates a Session for the given conversation.
func GetSession(conversationId string) (*Session, error) {
	if s, ok := sessions[conversationId]; ok {
		return s, nil
	}
	state, err := storage.LoadSessionState(conversationId)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = storage.NewSessionState(conversationId)
	}
	c, err := storage.GetConversation(conversationId)
	if err != nil {
		return nil, err
	}
	s := &Session{
		Id:     conversationId,
		Owner:  c.Owner,
		Pubsub: ably,
		speech: resemble,
		state:  state,
		cr:     make(protocol.ContentReceiver, 1024), // never stall
		sr:     make(pubsub.StatusReceiver, 1024),    // never stall
	}
	if err = s.start(); err != nil {
		sLog().Error("session start failure",
			zap.String("sessionId", conversationId), zap.Error(err))
		return nil, err
	}
	sessions[conversationId] = s
	return s, nil
}

// EndAllSessions force terminates all current conversation sessions.
func EndAllSessions() int {
	count := len(sessions)
	for _, s := range sessions {
		s.End()
	}
	return count
}

// ShutdownAllSessions gets all running sessions ready for handoff to a new server instance.
// It's meant to be invoked as a goroutine.
// When it's finished, it notifies with the number of sessions that were shut down.
func ShutdownAllSessions(notify chan int) {
	count := len(sessions)
	if count == 0 {
		notify <- 0
		return
	}
	completed := make(chan string, count)
	for _, s := range sessions {
		s.Shutdown(completed)
	}
	for done := 0; done < count; done++ {
		id := <-completed
		if err := storage.AddSuspendedSession(id); err != nil {
			sLog().Error("suspend session failure", zap.String("sessionId", id), zap.Error(err))
		}
	}
	notify <- count
}

// StartAllSuspendedSessions gets all suspended sessions running in this server instance.
// It's meant to be invoked as a goroutine and stops when there are no more suspended sessions.
func StartAllSuspendedSessions() {
	for {
		id, err := storage.WaitForSuspendedSession(30)
		if err != nil {
			sLog().Error("retrieve suspended session failure", zap.Error(err))
			return
		}
		if id == "" {
			// timeout
			return
		}
		if _, err = GetSession(id); err != nil {
			sLog().Error("resume session first failure", zap.String("sessionId", id), zap.Error(err))
		}
		if _, err = GetSession(id); err != nil {
			sLog().Error("resume session second failure", zap.String("sessionId", id), zap.Error(err))
			sLog().Info("giving up on starting suspended sessions")
		}
		err = storage.RemoveSuspendedSession(id)
		if err != nil {
			sLog().Error("remove suspended session failure", zap.String("sessionId", id), zap.Error(err))
			sLog().Info("giving up on starting suspended sessions")
		}
	}
}

// Shutdown gets a session ready for handoff to a new server instance
// (presumably because this one is terminating). It saves the state of
// the session and saves all the packets in the current live text of
// the session so they can be processed by the next server. It also keeps
// listening and saving content packets for 10 seconds to give the next
// server time to start up and resume the session. It notifies the session ID
// on the argument channel when the 10 seconds have passed and the server
// can finish shutting down.
func (s *Session) Shutdown(notify chan string) {
	s.shuttingDown = true
	delete(sessions, s.Id)
	go func() {
		time.Sleep(10 * time.Second)
		s.cancel()
		s.Pubsub.EndSession(s.Id)
		if err := storage.SaveSessionState(s.state); err != nil {
			sLog().Error("session suspend failure", zap.String("sessionId", s.Id), zap.Error(err))
		}
		notify <- s.Id
	}()
}

// End terminates a session at the request of the Whisperer. All
// participants are notified that the session is ending, and then the
// session is destroyed. If the session is being transcribed, then
// the transcript is finalized and saved, and its ID is returned.
func (s *Session) End() string {
	delete(sessions, s.Id)
	s.state.EndedAt = time.Now().UnixMilli()
	if err := s.Pubsub.BroadcastControl(s.Id, protocol.EndChunk()); err != nil {
		sLog().Error("ably broadcast failure when ending the session",
			zap.String("sessionId", s.Id), zap.Error(err))
	}
	s.cancel()
	s.Pubsub.EndSession(s.Id)
	if err := s.saveTranscript(); err != nil {
		sLog().Error("session save transcript failure",
			zap.String("sessionId", s.Id), zap.Error(err))
	}
	return s.transcriptId
}

// AddWhisperer adds the client to the session as a Whisperer.
func (s *Session) AddWhisperer(clientId string, profileId string, name string) error {
	defer s.notifyNeedsAuth()
	return s.newParticipant(clientId, profileId, name, true)
}

// AddListener adds the client to the session as a Listener
func (s *Session) AddListener(clientId, profileId, name string) error {
	// if this client was waiting, they are now approved
	for i, p := range s.state.Waitlist {
		if p.ClientId == clientId {
			s.state.Waitlist = append(s.state.Waitlist[:i], s.state.Waitlist[i+1:]...)
			break
		}
	}
	return s.newParticipant(clientId, profileId, name, false)
}

// AddListenerRequest asks the Whisperer to admit a Listener
func (s *Session) AddListenerRequest(clientId, profileId, name string) error {
	for _, p := range s.state.Waitlist {
		if p.ClientId == clientId {
			return AlreadyPresentError
		}
	}
	s.state.Waitlist = append(s.state.Waitlist, storage.NewParticipant(clientId, profileId, name, false))
	s.notifyNeedsAuth()
	return nil
}

// Participants returns the list of current participants
func (s *Session) Participants() []storage.Participant {
	participants := make([]storage.Participant, 0, len(s.state.Participants))
	for _, p := range s.state.Participants {
		participants = append(participants, *p)
	}
	return participants
}

// Requesters return the list of those who have asked to be allowed to join
func (s *Session) Requesters() []storage.Participant {
	requestors := make([]storage.Participant, 0, len(s.state.Waitlist))
	for _, p := range s.state.Waitlist {
		requestors = append(requestors, *p)
	}
	return requestors
}

// RemoveClient removes the client from the session.
func (s *Session) RemoveClient(clientId string) error {
	if _, ok := s.state.Participants[clientId]; !ok {
		for i, p := range s.state.Waitlist {
			if p.ClientId == clientId {
				s.state.Waitlist = append(s.state.Waitlist[:i], s.state.Waitlist[i+1:]...)
				return nil
			}
		}
		return NotPresentError
	}
	if err := s.Pubsub.RemoveClient(s.Id, clientId); err != nil {
		sLog().Error("ably remove client failure",
			zap.String("sessionId", s.Id), zap.String("clientId", clientId),
			zap.Error(err))
		return err
	}
	delete(s.state.Participants, clientId)
	return nil
}

// Transcribe marks a session for transcription and returns the ID
// of the transcription.
func (s *Session) Transcribe() string {
	s.transcriptId = uuid.NewString()
	return s.transcriptId
}

func (s *Session) start() error {
	if err := s.Pubsub.StartSession(s.Id, s.cr, s.sr); err != nil {
		sLog().Error("ably start session failure",
			zap.String("sessionId", s.Id), zap.Error(err))
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.monitorParticipants(ctx)
	go s.transcribeContent(ctx)
	for _, p := range s.state.Participants {
		var err error
		if p.IsWhisperer {
			p.IsOnline, err = s.Pubsub.AddWhisperer(s.Id, p.ClientId)
			if err != nil {
				sLog().Error("ably add whisperer failure",
					zap.String("sessionId", s.Id), zap.String("clientId", p.ClientId),
					zap.Error(err))
				cancel()
				return err
			}
		} else {
			p.IsOnline, err = s.Pubsub.AddListener(s.Id, p.ClientId)
			if err != nil {
				sLog().Error("ably add listener failure",
					zap.String("sessionId", s.Id), zap.String("clientId", p.ClientId),
					zap.Error(err))
				cancel()
				return err
			}
		}
	}
	s.notifyNeedsAuth()
	return nil
}

func (s *Session) newParticipant(clientId, profileId, name string, isWhisperer bool) error {
	if _, ok := s.state.Participants[clientId]; ok {
		return AlreadyPresentError
	}
	p := storage.NewParticipant(clientId, profileId, name, isWhisperer)
	s.state.Participants[clientId] = p
	var err error
	var msg string
	if isWhisperer {
		msg = "ably add whisperer failure"
		p.IsOnline, err = s.Pubsub.AddWhisperer(s.Id, p.ClientId)
	} else {
		msg = "ably add listener failure"
		p.IsOnline, err = s.Pubsub.AddListener(s.Id, p.ClientId)
	}
	if err != nil {
		sLog().Error(msg,
			zap.String("sessionId", s.Id), zap.String("clientId", p.ClientId),
			zap.Error(err))
		return err
	}
	return nil
}

func (s *Session) notifyNeedsAuth() {
	if len(s.state.Waitlist) > 0 {
		for _, p := range s.state.Participants {
			if p.IsWhisperer && p.IsOnline {
				chunk := protocol.RequestsPendingChunk()
				if err := s.Pubsub.SendControl(s.Id, "whisperer", chunk); err != nil {
					sLog().Error("ably send failure to Whisperer",
						zap.String("sessionId", s.Id), zap.String("clientId", "whisperer"),
						zap.String("chunk", chunk.String()), zap.Error(err))
				}
				break
			}
		}
	}
}

func (s *Session) monitorParticipants(ctx context.Context) {
	slog.Info("monitoring participants started", zap.String("sessionId", s.Id))
	for {
		select {
		case <-ctx.Done():
			slog.Info("monitoring participants stopped", zap.String("sessionId", s.Id))
			return
		case status := <-s.sr:
			p, ok := s.state.Participants[status.ClientId]
			if !ok {
				continue
			}
			p.IsOnline = status.IsOnline
			if p.IsWhisperer && status.IsOnline {
				s.notifyNeedsAuth()
			}
			chunk := protocol.ParticipantsChangedChunk()
			if err := s.Pubsub.BroadcastControl(s.Id, chunk); err != nil {
				sLog().Error("ably broadcast failure",
					zap.String("sessionId", s.Id),
					zap.String("packet", chunk.String()), zap.Error(err))
			}
		}
	}
}

func (s *Session) transcribeContent(ctx context.Context) {
	slog.Info("transcribing content started", zap.String("sessionId", s.Id))
	// wait for the first packet, which always comes as soon as pubsub is online
	<-s.cr
	// process the packets received but not processed by the prior server
	// while we were starting up
	processedIds := s.processSuspendedPackets()
	packetsToCheck := len(processedIds)
	for {
		select {
		case <-ctx.Done():
			if s.shuttingDown {
				slog.Info("saving live packets at shutdown", zap.String("sessionId", s.Id))
				if len(s.livePackets) > 0 {
					if err := storage.SaveSuspendedSessionPackets(s.Id, s.livePackets...); err != nil {
						sLog().Error("error saving suspended packets",
							zap.String("sessionId", s.Id), zap.Error(err))
					}
				}
			}
			slog.Info("transcribing content stopped", zap.String("sessionId", s.Id))
			return
		case packet := <-s.cr:
			if s.shuttingDown {
				// if we're shutting down, leave all packets for the next server
				s.livePackets = append(s.livePackets, packet)
				if err := storage.SaveSuspendedSessionPackets(s.Id, s.livePackets...); err != nil {
					sLog().Error("error saving suspended packets",
						zap.String("sessionId", s.Id), zap.Error(err))
				} else {
					s.livePackets = nil
				}
				continue
			}
			if packetsToCheck > 0 {
				// if we've just started up, make sure we don't process a packet
				// that may also have been received and saved by the prior server
				packetsToCheck--
				if processedIds[packet.PacketId] {
					continue
				}
			}
			s.transcribeOnePacket(packet)
		}
	}
}

func (s *Session) processSuspendedPackets() (packetIds map[string]bool) {
	packets, err := storage.LoadSuspendedSessionPackets(s.Id)
	if err != nil {
		sLog().Error("failure loading suspended packets at session startup",
			zap.String("sessionId", s.Id), zap.Error(err))
		return
	}
	packetIds = make(map[string]bool, len(packets))
	for _, p := range packets {
		packetIds[p.PacketId] = true
	}
	return
}

func (s *Session) transcribeOnePacket(packet protocol.ContentPacket) {
	live, past, pastId := protocol.ProcessLiveChunk(s.liveText, protocol.ParseContentChunk(packet.Data))
	if past != nil {
		s.state.PastText = append(s.state.PastText, storage.PastTextLine{time.Now().UnixMilli(), *past})
		s.speech.GenerateSpeech(storage.ServerContext, s.Owner, *pastId, *past)
		s.livePackets = nil
	} else {
		s.livePackets = append(s.livePackets, packet)
	}
	s.liveText = live
}

func (s *Session) saveTranscript() error {
	lines := s.state.PastText
	if s.liveText != "" {
		lines = append(lines, storage.PastTextLine{time.Now().UnixMilli(), s.liveText})
	}
	t := storage.CreateSessionTranscript(s.transcriptId, s.state)
	if err := storage.SaveTranscript(t); err != nil {
		sLog().Error("transcript save failure",
			zap.String("sessionId", s.Id), zap.String("transcriptId", s.transcriptId),
			zap.Error(err))
		return err
	}
	s.transcriptId = ""
	return nil
}
