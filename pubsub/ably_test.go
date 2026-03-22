/*
 * Copyright 2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package pubsub

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/whisper-project/srv2/protocol"
)

func TestManagerGolden1(t *testing.T) {
	session, clients := MakeBots("golden-session-1", 1, 1)
	actions := ActionList{
		{-1, "start"},
		{-1, "add-whisperer|whisper-bot-1"},
		{-1, "add-listener|listener-bot-1"},
		{1, "start"},
		{0, "start"},
		{0, "whisper|This is a test."},
		{0, "whisper|\n"},
	}
	AnimateBots(t, actions, session, clients)
}

func TestManagerGolden2(t *testing.T) {
	session, clients := MakeBots("golden-session-2", 2, 1)
	actions := ActionList{
		{-1, "start"},
		{-1, "add-whisperer|whisper-bot-1"},
		{0, "start"},
		{-1, "add-listener|listener-bot-1"},
		{2, "start"},
		{-1, "add-whisperer|whisper-bot-2"},
		{1, "start"},
		{0, "whisper|This is whisperer 1 speaking."},
		{1, "whisper|This is whisperer 2 speaking."},
		{0, "whisper|\n"},
		{1, "whisper|\n"},
	}
	AnimateBots(t, actions, session, clients)
}

func MakeBots(sessionId string, wCount, lCount int) (*RoboSession, []*RoboClient) {
	clients := make([]*RoboClient, wCount+lCount)
	errorReports := make(chan RoboErrorReport, 10)
	controlReports := make(chan RoboControlReport, 10)
	contentReports := make(chan RoboContentReport, 10)
	for i := 0; i < wCount+lCount; i++ {
		id := fmt.Sprintf("whisper-bot-%d", i+1)
		if i >= wCount {
			id = fmt.Sprintf("listener-bot-%d", i+1-wCount)
		}
		clients[i] = &RoboClient{
			sessionId:      sessionId,
			clientId:       id,
			isWhisperer:    i < wCount,
			actionFeed:     make(chan string, 10),
			errorReports:   errorReports,
			controlReports: controlReports,
			contentReports: contentReports,
		}
	}
	session := &RoboSession{
		sessionId:    sessionId,
		actionFeed:   make(chan string, 10),
		errorReports: errorReports,
		cr:           make(protocol.ContentReceiver, 10),
		sr:           make(StatusReceiver, 10),
	}
	return session, clients
}

type ActionList []struct {
	target int // 0-based client index, negative means it's a session action
	action string
}

func AnimateBots(t *testing.T, actions ActionList, session *RoboSession, bots []*RoboClient) {
	var wg sync.WaitGroup
	wg.Go(session.animate)
	for _, bot := range bots {
		wg.Go(bot.animate)
	}
	botsDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(botsDone)
	}()
	lastWhisper := ""
	startupPacketReceived := false
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	t.Logf("starting run at %s...", time.Now().Format("15:04:05.000"))
	start := time.Now()
	for index := 0; index >= 0; {
		select {
		case <-timer.C:
			if index >= len(actions) {
				t.Logf("%s: actions complete", time.Since(start))
				for _, bot := range bots {
					close(bot.actionFeed)
				}
				close(session.actionFeed)
				continue
			}
			t.Logf("%s: sending action to %d: %s", time.Since(start), actions[index].target, actions[index].action)
			target, action := actions[index].target, actions[index].action
			index++
			if target < 0 {
				session.actionFeed <- action
				timer.Reset(500 * time.Millisecond)
				break
			}
			if target >= len(bots) {
				t.Fatalf("Invalid target: %d of %d", target, len(bots))
			}
			if strings.HasPrefix(action, "whisper|") {
				lastWhisper = action[len("whisper|"):]
			}
			bots[target].actionFeed <- action
			timer.Reset(time.Second)
		case report := <-bots[0].errorReports:
			t.Errorf("%s reports error: %v", report.clientId, report.err)
		case report := <-bots[0].contentReports:
			analyzeContentChunk(t, start, report.chunk, lastWhisper, report.clientId)
		case status, more := <-session.sr:
			if !more {
				t.Logf("%s: session status receiver closed", time.Since(start))
				index = -1
				continue
			}
			t.Logf("%s: session sent status: %+v\n", time.Since(start), status)
		case packet, more := <-session.cr:
			if !more {
				t.Logf("%s: session content receiver closed", time.Since(start))
				index = -1
				continue
			}
			if startupPacketReceived {
				chunk := protocol.ParseContentChunk(packet.Data)
				analyzeContentChunk(t, start, chunk, lastWhisper, "session")
			} else {
				startupPacketReceived = true
				t.Logf("%s: session received startup packet: %#v\n", time.Since(start), packet)
			}
		}
	}
	t.Logf("%s: waiting up to 5 seconds for bots to finish...", time.Since(start))
	timer.Reset(5 * time.Second)
	select {
	case <-timer.C:
		t.Logf("%s: timeout!", time.Since(start))
	case <-botsDone:
		t.Logf("%s: bots done!", time.Since(start))
	}
}

func analyzeContentChunk(t *testing.T, start time.Time, chunk protocol.ContentChunk, lastWhisper string, source string) {
	t.Helper()
	if lastWhisper == "" {
		t.Errorf("%s: %s got content without a whisper: %v", time.Since(start), source, chunk)
	} else if lastWhisper == "\n" && chunk.Offset == protocol.CoNewline && strings.HasPrefix(chunk.Text, "line-") {
		t.Logf("%s: %s got the correct newline chunk\n", time.Since(start), source)
	} else if chunk.Offset == 0 && chunk.Text == lastWhisper {
		t.Logf("%s: %s got the correct text chunk for %q\n", time.Since(start), source, lastWhisper)
	} else {
		t.Errorf("%s: %s got an incorrect chunk after whisper %q: %v", time.Since(start), source, lastWhisper, chunk)
	}
}
