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
	"time"

	"github.com/whisper-project/srv2/platform"
	"github.com/whisper-project/srv2/protocol"
	"github.com/whisper-project/srv2/storage"

	"github.com/ably/ably-go/ably"
	"go.uber.org/zap"
)

func sLog() *zap.Logger {
	return storage.ServerLogger
}

type ClientStatus struct {
	ClientId string
	IsOnline bool
}

type StatusReceiver chan ClientStatus

type AblyManager struct {
	sessions map[string]*session
}

func (m *AblyManager) StartSession(sessionId string, cr protocol.ContentReceiver, sr StatusReceiver) error {
	if _, ok := m.sessions[sessionId]; ok {
		return fmt.Errorf("session %s already started", sessionId)
	}
	s := &session{id: sessionId, cr: cr, sr: sr}
	if err := s.start(); err != nil {
		return err
	}
	m.sessions[sessionId] = s
	return nil
}

func (m *AblyManager) EndSession(sessionId string) error {
	s, ok := m.sessions[sessionId]
	if !ok {
		return fmt.Errorf("no session %s", sessionId)
	}
	s.end()
	delete(m.sessions, sessionId)
	return nil
}

func (m *AblyManager) AddWhisperer(sessionId, clientId string) (bool, error) {
	s, ok := m.sessions[sessionId]
	if !ok {
		return false, fmt.Errorf("no session %s", sessionId)
	}
	return s.addWhisperer(clientId)
}

func (m *AblyManager) AddListener(sessionId, clientId string) (bool, error) {
	s, ok := m.sessions[sessionId]
	if !ok {
		return false, fmt.Errorf("no session %s", sessionId)
	}
	return s.addListener(clientId)
}

func (m *AblyManager) ClientToken(sessionId, clientId string) ([]byte, error) {
	s, ok := m.sessions[sessionId]
	if !ok {
		return nil, fmt.Errorf("no session %s", sessionId)
	}
	return s.clientToken(clientId)
}

func (m *AblyManager) RemoveClient(sessionId, clientId string) error {
	s, ok := m.sessions[sessionId]
	if !ok {
		return fmt.Errorf("no session %s", sessionId)
	}
	return s.removeClient(clientId)
}

func (m *AblyManager) Send(sessionId, clientId, packet string) error {
	s, ok := m.sessions[sessionId]
	if !ok {
		return fmt.Errorf("no session %s", sessionId)
	}
	return s.send(clientId, packet)
}

func (m *AblyManager) Broadcast(sessionId, packet string) error {
	s, ok := m.sessions[sessionId]
	if !ok {
		return fmt.Errorf("no session %s", sessionId)
	}
	return s.broadcast(packet)
}

func NewAblyManager() *AblyManager {
	return &AblyManager{
		sessions: make(map[string]*session),
	}
}

type session struct {
	id              string
	cr              protocol.ContentReceiver
	sr              StatusReceiver
	client          *ably.Realtime
	controlId       string
	presenceId      string
	contentId       string
	controlChannel  *ably.RealtimeChannel
	presenceChannel *ably.RealtimeChannel
	contentChannel  *ably.RealtimeChannel
	participants    map[string]*participant
}

type participant struct {
	clientId   string
	canWhisper bool
	canListen  bool
	attached   bool
}

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
		sLog().Info("ably content channel attached", zap.String("sessionId", s.id))
		// signal the content receiver that we are attached
		s.cr <- protocol.ContentPacket{}
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

func (s *session) end() {
	sLog().Info("ending session", zap.String("sessionId", s.id))
	ctx := context.Background()
	if err := s.contentChannel.Detach(ctx); err != nil {
		sLog().Error("ably failure detaching content channel",
			zap.String("sessionId", s.id), zap.Error(err))
	}
	if err := s.presenceChannel.Detach(ctx); err != nil {
		sLog().Error("ably failure detaching presence channel",
			zap.String("sessionId", s.id), zap.Error(err))
	}
	go func() {
		time.Sleep(1 * time.Second)
		s.client.Close()
		sLog().Info("closed ably client", zap.String("sessionId", s.id))
	}()
}

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

func (s *session) addWaitLister(clientId string) (bool, error) {
	if p, ok := s.participants[clientId]; ok {
		return p.attached, nil
	}
	attached := s.updatePresence(clientId)
	l := &participant{clientId: clientId, canWhisper: false, canListen: false, attached: attached}
	s.participants[clientId] = l
	return attached, nil
}

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
	}
	if p.canListen {
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

func (s *session) removeClient(clientId string) error {
	_, ok := s.participants[clientId]
	if !ok {
		return fmt.Errorf("unknown client: %s", clientId)
	}
	delete(s.participants, clientId)
	return nil
}

func (s *session) send(clientId, packet string) error {
	p, ok := s.participants[clientId]
	if !ok {
		return fmt.Errorf("unknown client: %s", clientId)
	}
	err := s.controlChannel.Publish(context.Background(), p.clientId, packet)
	if err != nil {
		sLog().Error("ably failure publishing to control channel",
			zap.String("sessionId", s.id), zap.String("clientId", p.clientId), zap.Error(err))
	}
	return err
}

func (s *session) broadcast(packet string) error {
	err := s.controlChannel.Publish(context.Background(), "all", packet)
	if err != nil {
		sLog().Error("ably failure publishing to control channel",
			zap.String("sessionId", s.id), zap.Error(err))
	}
	return err
}

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

func (s *session) contentReceiver() func(*ably.Message) {
	return func(msg *ably.Message) {
		packet := protocol.ContentPacket{
			PacketId: msg.ID,
			ClientId: msg.ClientID,
			Data:     msg.String(),
		}
		sLog().Debug("received content packet",
			zap.String("sessionId", s.id),
			zap.Any("packet", packet),
		)
		s.cr <- packet
	}
}

func (s *session) presenceReceiver() func(*ably.PresenceMessage) {
	return func(msg *ably.PresenceMessage) {
		p, ok := s.participants[msg.ClientID]
		if !ok {
			return
		}
		sLog().Debug("received presence message",
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
			// we shouldn't get these, because clients are not sending updates
			sLog().Warn("received an unexpected presence message",
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
