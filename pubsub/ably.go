/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/whisper-project/srv2/platform"
	"github.com/whisper-project/srv2/protocol"
	"github.com/whisper-project/srv2/storage"

	"github.com/ably/ably-go/ably"
	"go.uber.org/zap"
)

// An ablyManager is a (singleton) pubsub manager implemented using Ably.
type ablyManager struct {
	mutex    sync.Mutex
	sessions map[string]*session
}

var ablyManagerInstance *ablyManager

// GetAblyManager returns the Ably manager.
func GetAblyManager() Manager {
	if ablyManagerInstance == nil {
		ablyManagerInstance = &ablyManager{
			sessions: make(map[string]*session),
		}
	}
	return ablyManagerInstance
}

// StartSession creates and starts a session for sessionId (unless one already exists).
func (m *ablyManager) StartSession(sessionId string, cr protocol.ContentReceiver, sr StatusReceiver) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.sessions[sessionId]; ok {
		return nil
	}
	s := &session{id: sessionId, cr: cr, sr: sr, participants: make(map[string]*participant)}
	if err := s.start(); err != nil {
		return err
	}
	m.sessions[sessionId] = s
	return nil
}

// EndSession ends the session for sessionId (if it exists).
func (m *ablyManager) EndSession(sessionId string) {
	var s *session
	defer func() {
		if s != nil {
			s.end()
		}
	}()
	m.mutex.Lock()
	defer m.mutex.Unlock()
	s, _ = m.sessions[sessionId]
	delete(m.sessions, sessionId)
}

// getSession (internal) returns the existing session for id if there is one.
func (m *ablyManager) getSession(id string) *session {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	s, _ := m.sessions[id]
	return s
}

// AddWhisperer adds the client with Whisperer capability to the session for sessionId.
// Returns whether the client is already attached to the session.
func (m *ablyManager) AddWhisperer(sessionId, clientId string) (bool, error) {
	s := m.getSession(sessionId)
	if s == nil {
		return false, fmt.Errorf("%w: %s", NoSessionError, sessionId)
	}
	return s.addWhisperer(clientId)
}

// AddListener adds the client with Listener capability to the session for sessionId.
func (m *ablyManager) AddListener(sessionId, clientId string) (bool, error) {
	s := m.getSession(sessionId)
	if s == nil {
		return false, fmt.Errorf("%w: %s", NoSessionError, sessionId)
	}
	return s.addListener(clientId)
}

// ClientToken return an Ably AuthTokenRequest appropriate to the given client in the given session.
func (m *ablyManager) ClientToken(sessionId, clientId string) ([]byte, error) {
	s := m.getSession(sessionId)
	if s == nil {
		return nil, fmt.Errorf("%w: %s", NoSessionError, sessionId)
	}
	return s.clientToken(clientId)
}

// RemoveClient removes the given client from the given session
func (m *ablyManager) RemoveClient(sessionId, clientId string) error {
	s := m.getSession(sessionId)
	if s == nil {
		return fmt.Errorf("%w: %s", NoSessionError, sessionId)
	}
	return s.removeClient(clientId)
}

// SendControl sends the given control chunk to the given client in the given session.
func (m *ablyManager) SendControl(sessionId, clientId string, chunk protocol.ControlChunk) error {
	s := m.getSession(sessionId)
	if s == nil {
		return fmt.Errorf("%w: %s", NoSessionError, sessionId)
	}
	return s.sendControl(clientId, chunk)
}

// BroadcastControl sends the given control chunk to all the clients currently in the given session.
func (m *ablyManager) BroadcastControl(sessionId string, chunk protocol.ControlChunk) error {
	s := m.getSession(sessionId)
	if s == nil {
		return fmt.Errorf("%w: %s", NoSessionError, sessionId)
	}
	return s.broadcastControl(chunk)
}

// A pubsub session.
type session struct {
	id              string
	cr              protocol.ContentReceiver
	sr              StatusReceiver
	participants    map[string]*participant
	client          *ably.Realtime
	controlId       string
	presenceId      string
	contentId       string
	controlChannel  *ably.RealtimeChannel
	presenceChannel *ably.RealtimeChannel
	contentChannel  *ably.RealtimeChannel
}

// A participant in a session.
//
// Participants may not be authorized to be part of the session;
// they may be in the "waiting" list while the Whisperer decides
// whether to approve them or not.
type participant struct {
	clientId   string
	canWhisper bool
	canListen  bool
	attached   bool
}

// Starting a session creates a new client and then connects it
// to the content and presence channels
// for the session. The content channel is where the Whisperer will
// talk and also where the generated speech will be sent. The presence
// channel is used to track when participants join and leave.
func (s *session) start() error {
	sLog().Info("opening ably client", zap.String("sessionId", s.id))
	hubId := fmt.Sprintf("%s:%s", storage.ServerId, s.id)
	s.controlId = fmt.Sprintf("%s:%s", s.id, "control")
	s.presenceId = fmt.Sprintf("%s:%s", s.id, "presence")
	s.contentId = fmt.Sprintf("%s:%s", s.id, "content")
	client, err := ably.NewRealtime(
		ably.WithClientID(hubId),
		ably.WithKey(platform.GetConfig().AblyPublishKey),
		ably.WithEchoMessages(false),
		ably.WithAutoConnect(true),
	)
	if err != nil {
		sLog().Error("ably client create failure", zap.String("sessionId", s.id), zap.Error(err))
		return err
	}
	defer func() {
		if err != nil {
			client.Close()
		}
	}()
	controlChannel := client.Channels.Get(s.controlId)
	presenceChannel := client.Channels.Get(s.presenceId)
	contentChannel := client.Channels.Get(s.contentId)
	_, err = presenceChannel.Presence.SubscribeAll(context.Background(), s.presenceReceiver())
	if err != nil {
		sLog().Error("ably presence subscribe failure", zap.String("sessionId", s.id), zap.Error(err))
		return err
	}
	contentChannel.Once(ably.ChannelEventAttached, func(_ ably.ChannelStateChange) {
		sLog().Info("attached the Ably content channel", zap.String("sessionId", s.id))
		// signal the content receiver that we are attached
		s.cr <- protocol.AttachPacket
	})
	_, err = contentChannel.SubscribeAll(context.Background(), s.contentReceiver())
	if err != nil {
		sLog().Error("ably content subscribe failure", zap.String("sessionId", s.id), zap.Error(err))
		return err
	}
	s.client = client
	s.controlChannel = controlChannel
	s.presenceChannel = presenceChannel
	s.contentChannel = contentChannel
	return nil
}

// Ending a session detaches from the content and presence channels
// and then closes the session client.
func (s *session) end() {
	sLog().Info("ending session", zap.String("sessionId", s.id))
	ctx := context.Background()
	if err := s.contentChannel.Detach(ctx); err != nil {
		sLog().Error("ably failure detaching the content channel",
			zap.String("sessionId", s.id), zap.Error(err))
	}
	if err := s.presenceChannel.Detach(ctx); err != nil {
		sLog().Error("ably failure detaching the presence channel",
			zap.String("sessionId", s.id), zap.Error(err))
	}
	// Delay the client close to allow the detach operations to complete.
	go func() {
		time.Sleep(1 * time.Second)
		s.client.Close()
		sLog().Info("closed ably client", zap.String("sessionId", s.id))
	}()
	close(s.cr)
	close(s.sr)
}

// addWhisperer ensures that the client has access to the session with
// whispering privileges. Returns whether the client is already attached
// to the session.
func (s *session) addWhisperer(clientId string) (bool, error) {
	if p, ok := s.participants[clientId]; ok {
		p.canWhisper = true
		p.canListen = true
		return p.attached, nil
	}
	attached := s.updatePresence(clientId)
	l := &participant{clientId: clientId, canWhisper: true, canListen: true, attached: attached}
	s.participants[clientId] = l
	return attached, nil
}

// addListener ensures that the client has access to the session with
// listening privileges.
func (s *session) addListener(clientId string) (bool, error) {
	if p, ok := s.participants[clientId]; ok {
		p.canListen = true
		return p.attached, nil
	}
	attached := s.updatePresence(clientId)
	l := &participant{clientId: clientId, canWhisper: false, canListen: true, attached: attached}
	s.participants[clientId] = l
	return attached, nil
}

// addWaitLister adds the client as a participant but does not authorize them to
// listen or whisper in the session.
func (s *session) addWaitLister(clientId string) (bool, error) {
	if p, ok := s.participants[clientId]; ok {
		return p.attached, nil
	}
	attached := s.updatePresence(clientId)
	l := &participant{clientId: clientId, canWhisper: false, canListen: false, attached: attached}
	s.participants[clientId] = l
	return attached, nil
}

// clientToken generates a server-signed token request for the client with
// appropriate privileges for the client. All clients can subscribe to the
// control channel for the session, but only listeners and whisperers are
// authorized to be in the content channel.
func (s *session) clientToken(clientId string) ([]byte, error) {
	p, ok := s.participants[clientId]
	if !ok {
		return nil, nil
	}
	capabilities := map[string][]string{
		s.presenceId: {"presence"},
		s.controlId:  {"subscribe"},
	}
	if p.canWhisper {
		capabilities[s.contentId] = []string{"publish", "subscribe"}
	} else if p.canListen {
		capabilities[s.contentId] = []string{"subscribe"}
	}
	payload, err := json.Marshal(capabilities)
	if err != nil {
		return nil, err
	}
	params := ably.TokenParams{ClientID: clientId, Capability: string(payload)}
	request, err := s.client.Auth.CreateTokenRequest(&params)
	if err != nil {
		return nil, err
	}
	payload, err = json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// removeClient ensures the client is not a participant in the session.
//
// Removing a non-existent client is a no-op.
func (s *session) removeClient(clientId string) error {
	if _, ok := s.participants[clientId]; !ok {
		return nil
	}
	delete(s.participants, clientId)
	return nil
}

// sendControl sends a control channel chunk to the given client
func (s *session) sendControl(clientId string, chunk protocol.ControlChunk) error {
	p, ok := s.participants[clientId]
	if !ok {
		return fmt.Errorf("unknown client: %s", clientId)
	}
	err := s.controlChannel.Publish(context.Background(), p.clientId, chunk.String())
	if err != nil {
		sLog().Error("ably failure publishing to the control channel",
			zap.String("sessionId", s.id), zap.String("clientId", p.clientId), zap.Error(err))
	}
	return err
}

// broadcastControl sends a control channel chunk to all clients.
func (s *session) broadcastControl(chunk protocol.ControlChunk) error {
	err := s.controlChannel.Publish(context.Background(), "all", chunk.String())
	if err != nil {
		sLog().Error("ably failure publishing to the control channel",
			zap.String("sessionId", s.id), zap.Error(err))
	}
	return err
}

// updatePresence gets the latest presence information for the channel
// and checks whether the given clientId is present.
func (s *session) updatePresence(clientId string) bool {
	msgs, err := s.presenceChannel.Presence.Get(context.Background())
	if err != nil {
		return false
	}
	for _, m := range msgs {
		if m.ClientID == clientId {
			return true
		}
	}
	return false
}

// contentReceiver transfers all messages received on the content channel
// to the content receiver registered when the session was started.
func (s *session) contentReceiver() func(*ably.Message) {
	return func(msg *ably.Message) {
		data, ok := msg.Data.(string)
		if !ok {
			data = protocol.ContentChunk{protocol.CoIgnore, "Invalid content data type"}.String()
		}
		packet := protocol.ContentPacket{
			PacketId: msg.ID,
			Data:     data,
		}
		sLog().Debug("received a content packet",
			zap.String("sessionId", s.id),
			zap.Any("packet", packet),
		)
		s.cr <- packet
	}
}

// presenceReceiver watches for presence messages from participants
// and, if their status changes, passes that status update to the
// status receiver registered when the session was started.
func (s *session) presenceReceiver() func(*ably.PresenceMessage) {
	return func(msg *ably.PresenceMessage) {
		p, ok := s.participants[msg.ClientID]
		if !ok {
			return
		}
		sLog().Debug("received a presence message",
			zap.String("sessionId", s.id),
			zap.String("clientId", msg.ClientID),
			zap.String("action", msg.Action.String()),
		)
		attached := p.attached
		switch msg.Action {
		case ably.PresenceActionEnter, ably.PresenceActionPresent:
			attached = true
		case ably.PresenceActionLeave, ably.PresenceActionAbsent:
			attached = false
		case ably.PresenceActionUpdate:
			// we shouldn't get these because clients are not sending updates
			sLog().Warn("received an unexpected presence update",
				zap.String("sessionId", s.id),
				zap.String("clientId", p.clientId),
				zap.String("action", msg.Action.String()),
			)
		default:
			sLog().Warn("received an unknown presence action",
				zap.String("sessionId", s.id),
				zap.String("clientId", p.clientId),
				zap.String("action", msg.Action.String()),
			)
		}
		if attached != p.attached {
			p.attached = attached
			s.sr <- ClientStatus{ClientId: p.clientId, IsOnline: attached}
		}
	}
}
