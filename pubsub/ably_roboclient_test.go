/*
 * Copyright 2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/whisper-project/srv2/platform"
	"github.com/whisper-project/srv2/protocol"
)

type RoboErrorReport struct {
	clientId string
	err      error
}

type RoboControlReport struct {
	clientId string
	chunk    protocol.ControlChunk
}

type RoboContentReport struct {
	clientId string
	chunk    protocol.ContentChunk
}

type RoboSession struct {
	sessionId    string
	actionFeed   chan string
	errorReports chan RoboErrorReport
	cr           protocol.ContentReceiver
	sr           StatusReceiver
	manager      Manager
}

func (rs *RoboSession) animate() {
	ended := false
	defer func() {
		if !ended {
			rs.end()
		}
	}()
	for spec := range rs.actionFeed {
		if ended {
			rs.errorReports <- RoboErrorReport{"session", fmt.Errorf("got action %q after end", spec)}
			continue
		}
		action, clientId, _ := strings.Cut(spec, "|")
		switch action {
		case "start":
			rs.start()
		case "end":
			ended = true
			rs.end()
		case "add-whisperer":
			rs.addWhisperer(clientId)
		case "add-listener":
			rs.addListener(clientId)
		case "remove":
			rs.remove(clientId)
		case "send-control":
			clientId, data, found := strings.Cut(clientId, "|")
			if !found {
				rs.errorReports <- RoboErrorReport{"session", fmt.Errorf("invalid control spec %q", spec)}
				continue
			}
			rs.sendControl(clientId, protocol.ParseControlChunk(data))
		case "broadcast-control":
			chunk := protocol.ParseControlChunk(clientId)
			rs.broadcastControl(chunk)
		default:
			rs.errorReports <- RoboErrorReport{"session", fmt.Errorf("unknown action %q", spec)}
		}
	}
}

func (rs *RoboSession) start() {
	rs.manager = GetAblyManager()
	if err := rs.manager.StartSession(rs.sessionId, rs.cr, rs.sr); err != nil {
		rs.errorReports <- RoboErrorReport{"session", fmt.Errorf("start error: %w", err)}
		return
	}
}

func (rs *RoboSession) end() {
	rs.manager.EndSession(rs.sessionId)
}

func (rs *RoboSession) addWhisperer(clientId string) {
	attached, err := rs.manager.AddWhisperer(rs.sessionId, clientId)
	if err != nil {
		rs.errorReports <- RoboErrorReport{"session", fmt.Errorf("add whisperer error: %w", err)}
	}
	rs.sr <- ClientStatus{clientId, attached}
}

func (rs *RoboSession) addListener(clientId string) {
	attached, err := rs.manager.AddListener(rs.sessionId, clientId)
	if err != nil {
		rs.errorReports <- RoboErrorReport{"session", fmt.Errorf("add listener error: %w", err)}
	}
	rs.sr <- ClientStatus{clientId, attached}
}

func (rs *RoboSession) remove(clientId string) {
	if err := rs.manager.RemoveClient(rs.sessionId, clientId); err != nil {
		rs.errorReports <- RoboErrorReport{"session", fmt.Errorf("remove client error: %w", err)}
	}
}

func (rs *RoboSession) sendControl(clientId string, chunk protocol.ControlChunk) {
	if err := rs.manager.SendControl(rs.sessionId, clientId, chunk); err != nil {
		rs.errorReports <- RoboErrorReport{"session", fmt.Errorf("send control error: %w", err)}
	}
}

func (rs *RoboSession) broadcastControl(chunk protocol.ControlChunk) {
	if err := rs.manager.BroadcastControl(rs.sessionId, chunk); err != nil {
		rs.errorReports <- RoboErrorReport{"session", fmt.Errorf("broadcast control error: %w", err)}
	}
}

type RoboClient struct {
	clientId        string
	sessionId       string
	isWhisperer     bool
	actionFeed      chan string
	errorReports    chan RoboErrorReport
	controlReports  chan RoboControlReport
	contentReports  chan RoboContentReport
	client          *ably.Realtime
	controlId       string
	contentId       string
	presenceId      string
	controlChannel  *ably.RealtimeChannel
	contentChannel  *ably.RealtimeChannel
	presenceChannel *ably.RealtimeChannel
}

func (rc *RoboClient) animate() {
	ended := false
	defer func() {
		if !ended {
			rc.end()
		}
	}()
	for spec := range rc.actionFeed {
		if ended {
			rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("got action %q after end", spec)}
			continue
		}
		action, data, _ := strings.Cut(spec, "|")
		switch action {
		case "start":
			rc.start()
		case "disconnect":
			rc.disconnect()
		case "reconnect":
			rc.reconnect()
		case "whisper":
			rc.whisper(data)
		case "end":
			ended = true
			rc.end()
		default:
			rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("unknown action %q", spec)}
		}
	}
}

func (rc *RoboClient) start() {
	c, err := ably.NewRealtime(
		ably.WithClientID(rc.clientId),
		ably.WithAuthCallback(rc.getAuthToken),
		ably.WithEchoMessages(false),
		ably.WithAutoConnect(true),
	)
	if err != nil {
		rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("client create error: %w", err)}
		return
	}
	defer func() {
		if err != nil {
			c.Close()
		}
	}()
	rc.controlId = fmt.Sprintf("%s:%s", rc.sessionId, "control")
	rc.contentId = fmt.Sprintf("%s:%s", rc.sessionId, "content")
	rc.presenceId = fmt.Sprintf("%s:%s", rc.sessionId, "presence")
	controlChannel := c.Channels.Get(rc.controlId)
	contentChannel := c.Channels.Get(rc.contentId)
	presenceChannel := c.Channels.Get(rc.presenceId)
	if err = presenceChannel.Presence.Enter(context.Background(), "roboClient starting session"); err != nil {
		rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("enter error: %w", err)}
	}
	if _, err = controlChannel.SubscribeAll(context.Background(), rc.controlReceiver); err != nil {
		rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("subscribe error: %w", err)}
	}
	if _, err = contentChannel.SubscribeAll(context.Background(), rc.contentReceiver); err != nil {
		rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("subscribe error: %w", err)}
	}
	rc.client = c
	rc.controlChannel = controlChannel
	rc.contentChannel = contentChannel
	rc.presenceChannel = presenceChannel
}

func (rc *RoboClient) disconnect() {
	if err := rc.presenceChannel.Presence.Leave(context.Background(), "simulated disconnect"); err != nil {
		rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("leave error: %w", err)}
	}
}

func (rc *RoboClient) reconnect() {
	if err := rc.presenceChannel.Presence.Enter(context.Background(), "simulated reconnect"); err != nil {
		rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("enter error: %w", err)}
	}
}

func (rc *RoboClient) end() {
	_ = rc.presenceChannel.Presence.Leave(context.Background(), "roboClient ending session")
	_ = rc.controlChannel.Detach(context.Background())
	_ = rc.contentChannel.Detach(context.Background())
	// give above time to complete before closing client
	go func() {
		time.Sleep(1 * time.Second)
		rc.client.Close()
	}()
}

func (rc *RoboClient) whisper(data string) {
	chunk := protocol.ContentChunk{0, data}
	if data == "\n" {
		chunk = protocol.ContentChunk{protocol.CoNewline, platform.NewId("line-")}
	}
	if err := rc.contentChannel.Publish(context.Background(), "all", chunk.String()); err != nil {
		rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("content publish failure: %w", err)}
	}
}

func (rc *RoboClient) getAuthToken(context.Context, ably.TokenParams) (ably.Tokener, error) {
	mgr := GetAblyManager()
	tok, err := mgr.ClientToken(rc.sessionId, rc.clientId)
	if err != nil {
		rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("token fetch error: %w", err)}
		return nil, err
	}
	var req ably.TokenRequest
	if err = json.NewDecoder(bytes.NewReader(tok)).Decode(&req); err != nil {
		rc.errorReports <- RoboErrorReport{rc.clientId, fmt.Errorf("token decode error: %w", err)}
		return nil, err
	}
	return ably.Tokener(req), nil
}

func (rc *RoboClient) controlReceiver(msg *ably.Message) {
	if msg.Name == "all" || msg.Name == rc.clientId || (rc.isWhisperer && msg.Name == "whisperer") {
		chunk := protocol.ParseControlChunk(msg.Data.(string))
		rc.controlReports <- RoboControlReport{rc.clientId, chunk}
	}
}

func (rc *RoboClient) contentReceiver(msg *ably.Message) {
	if msg.Name == "all" || msg.Name == rc.clientId {
		chunk := protocol.ParseContentChunk(msg.Data.(string))
		rc.contentReports <- RoboContentReport{rc.clientId, chunk}
	}
}
