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

type roboClientSession struct {
	clientId        string
	sessionId       string
	isWhisperer     bool
	actionFeed      chan string
	errorReports    chan error
	controlReports  chan protocol.ControlChunk
	contentReports  chan protocol.ContentChunk
	client          *ably.Realtime
	controlId       string
	contentId       string
	presenceId      string
	controlChannel  *ably.RealtimeChannel
	contentChannel  *ably.RealtimeChannel
	presenceChannel *ably.RealtimeChannel
}

func (rcs *roboClientSession) animate() {
	for spec := range rcs.actionFeed {
		action, data, _ := strings.Cut(spec, "|")
		switch action {
		case "start":
			rcs.start()
		case "whisper":
			rcs.whisper(data)
		}
	}
	rcs.end()
}

func (rcs *roboClientSession) start() {
	c, err := ably.NewRealtime(
		ably.WithClientID(rcs.clientId),
		ably.WithAuthCallback(rcs.getAuthToken),
		ably.WithEchoMessages(false),
		ably.WithAutoConnect(true),
	)
	if err != nil {
		rcs.errorReports <- fmt.Errorf("error creating a realtime client: %w", err)
		return
	}
	rcs.controlId = fmt.Sprintf("%s:%s", rcs.sessionId, "control")
	rcs.contentId = fmt.Sprintf("%s:%s", rcs.sessionId, "content")
	rcs.presenceId = fmt.Sprintf("%s:%s", rcs.sessionId, "presence")
	controlChannel := c.Channels.Get(rcs.controlId)
	contentChannel := c.Channels.Get(rcs.contentId)
	presenceChannel := c.Channels.Get(rcs.presenceId)
	defer func() {
		if err != nil {
			c.Close()
		}
	}()
	if err = presenceChannel.Presence.Enter(context.Background(), "roboClient starting session"); err != nil {
		rcs.errorReports <- fmt.Errorf("error entering roboClient: %w", err)
		return
	}
	if _, err = controlChannel.SubscribeAll(context.Background(), rcs.controlReceiver); err != nil {
		rcs.errorReports <- fmt.Errorf("error subscribing roboClient for control: %w", err)
		return
	}
	if _, err = contentChannel.SubscribeAll(context.Background(), rcs.contentReceiver); err != nil {
		rcs.errorReports <- fmt.Errorf("error subscribing roboClient for content: %w", err)
		return
	}
	rcs.client = c
	rcs.controlChannel = controlChannel
	rcs.contentChannel = contentChannel
	rcs.presenceChannel = presenceChannel
}

func (rcs *roboClientSession) end() {
	_ = rcs.presenceChannel.Presence.Leave(context.Background(), "roboClient ending session")
	_ = rcs.controlChannel.Detach(context.Background())
	_ = rcs.contentChannel.Detach(context.Background())
	// give above time to complete before closing client
	go func() {
		time.Sleep(1 * time.Second)
		rcs.client.Close()
	}()
	close(rcs.controlReports)
	close(rcs.contentReports)
	close(rcs.errorReports)
}

func (rcs *roboClientSession) whisper(data string) {
	chunk := protocol.ContentChunk{0, data}
	if data == "\n" {
		chunk = protocol.ContentChunk{protocol.CoNewline, platform.NewId("line-")}
	}
	if err := rcs.contentChannel.Publish(context.Background(), "all", chunk.String()); err != nil {
		rcs.errorReports <- fmt.Errorf("content publish failure: %w", err)
	}
}

func (rcs *roboClientSession) getAuthToken(context.Context, ably.TokenParams) (ably.Tokener, error) {
	mgr := GetAblyManager()
	tok, err := mgr.ClientToken(rcs.sessionId, rcs.clientId)
	if err != nil {
		rcs.errorReports <- fmt.Errorf("fetch ClientToken error for %q: %w", rcs.sessionId, err)
		return nil, err
	}
	var req ably.TokenRequest
	if err = json.NewDecoder(bytes.NewReader(tok)).Decode(&req); err != nil {
		rcs.errorReports <- fmt.Errorf("decode ClientToken error for %q: %w", rcs.sessionId, err)
		return nil, err
	}
	return ably.Tokener(req), nil
}

func (rcs *roboClientSession) controlReceiver(msg *ably.Message) {
	if msg.Name == "all" || msg.Name == rcs.clientId || (rcs.isWhisperer && msg.Name == "whisperer") {
		chunk := protocol.ParseControlChunk(msg.Data.(string))
		rcs.controlReports <- chunk
	}
}

func (rcs *roboClientSession) contentReceiver(msg *ably.Message) {
	if msg.Name == "all" || msg.Name == rcs.clientId {
		chunk := protocol.ParseContentChunk(msg.Data.(string))
		rcs.contentReports <- chunk
	}
}
