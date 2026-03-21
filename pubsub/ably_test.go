/*
 * Copyright 2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package pubsub

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/whisper-project/srv2/platform"
	"github.com/whisper-project/srv2/protocol"
)

func TestManagerGolden1(t *testing.T) {
	manager := GetAblyManager()
	sessionId := platform.NewId("test-session-")
	whispererId := platform.NewId("test-whisperer-client-")
	listenerId := platform.NewId("test-listener-client-")
	whispererErrs := make(chan error, 10)
	listenerErrs := make(chan error, 10)
	whispererControls := make(chan protocol.ControlChunk, 10)
	whispererContents := make(chan protocol.ContentChunk, 10)
	listenerControls := make(chan protocol.ControlChunk, 10)
	listenerContents := make(chan protocol.ContentChunk, 10)
	sessionContent := make(protocol.ContentReceiver, 10)
	sessionStatus := make(StatusReceiver, 10)
	remoteSenders := 8
	whispererActions := make(chan string)
	listenerActions := make(chan string)
	whispererRcs := &roboClientSession{
		sessionId:      sessionId,
		clientId:       whispererId,
		isWhisperer:    true,
		actionFeed:     whispererActions,
		errorReports:   whispererErrs,
		controlReports: whispererControls,
		contentReports: whispererContents,
	}
	go whispererRcs.animate()
	listenerRcs := &roboClientSession{
		sessionId:      sessionId,
		clientId:       listenerId,
		isWhisperer:    false,
		actionFeed:     listenerActions,
		errorReports:   listenerErrs,
		controlReports: listenerControls,
		contentReports: listenerContents,
	}
	go listenerRcs.animate()
	if err := manager.StartSession(sessionId, sessionContent, sessionStatus); err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}
	if attached, err := manager.AddWhisperer(sessionId, whispererId); err != nil {
		t.Fatalf("Failed to add whisperer: %v", err)
	} else if attached {
		t.Errorf("Whisperer is already attached??")
	}
	if attached, err := manager.AddListener(sessionId, listenerId); err != nil {
		t.Fatalf("Failed to add listener: %v", err)
	} else if attached {
		t.Errorf("Listener is already attached??")
	}
	actions := []string{
		"listener|start",
		"whisperer|start",
		"whisperer|whisper|this is a test",
		"whisperer|whisper|\n",
	}
	lastWhisper := ""
	startupPacketReceived := false
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for index := 0; index >= 0; {
		timer.Reset(time.Second)
		select {
		case <-timer.C:
			if index >= len(actions) {
				index = -1
				break
			}
			target, action, _ := strings.Cut(actions[index], "|")
			index++
			if target == "whisperer" {
				if strings.HasPrefix(action, "whisper|") {
					lastWhisper = action[len("whisper|"):]
				}
				whispererActions <- action
			} else {
				listenerActions <- action
			}
		case err := <-whispererErrs:
			t.Errorf("Whisperer error: %v", err)
		case err := <-listenerErrs:
			t.Errorf("Listener error: %v", err)
		case chunk := <-whispererContents:
			t.Errorf("Whisperer got content: %v", chunk)
		case chunk := <-listenerContents:
			analyzeContentChunk(t, chunk, lastWhisper, "Listener")
		case status := <-sessionStatus:
			fmt.Printf("Received status: %v\n", status)
		case packet := <-sessionContent:
			if startupPacketReceived {
				chunk := protocol.ParseContentChunk(packet.Data)
				analyzeContentChunk(t, chunk, lastWhisper, "Session")
			} else {
				startupPacketReceived = true
				fmt.Printf("Received startup packet: %v\n", packet)
			}
		}
	}
	close(whispererActions)
	close(listenerActions)
	manager.EndSession(sessionId)
	for remoteSenders > 0 {
		select {
		case err, more := <-whispererErrs:
			if !more {
				remoteSenders--
			} else {
				t.Errorf("Received late Whisperer error: %v", err)
			}
		case err, more := <-listenerErrs:
			if !more {
				remoteSenders--
			} else {
				t.Errorf("Received late Listener error: %v", err)
			}
		case chunk, more := <-whispererControls:
			if !more {
				remoteSenders--
			} else {
				t.Errorf("Received late Whisperer control: %v", chunk)
			}
		case chunk, more := <-whispererContents:
			if !more {
				remoteSenders--
			} else {
				t.Errorf("Received Whisperer content: %v", chunk)
			}
		case chunk, more := <-listenerControls:
			if !more {
				remoteSenders--
			} else {
				t.Errorf("Received late Listener control: %v", chunk)
			}
		case chunk, more := <-listenerContents:
			if !more {
				remoteSenders--
			} else {
				t.Errorf("Received late Listener content: %v", chunk)
			}
		case status, more := <-sessionStatus:
			if !more {
				remoteSenders--
			} else {
				fmt.Printf("Received status: %v\n", status)
			}
		case packet, more := <-sessionContent:
			if !more {
				remoteSenders--
			} else {
				t.Errorf("Received late session content: %v", packet)
			}
		}
	}
}

func analyzeContentChunk(t *testing.T, chunk protocol.ContentChunk, lastWhisper string, source string) {
	t.Helper()
	if lastWhisper == "" {
		t.Errorf("%s got content without a whisper: %v", source, chunk)
	} else if lastWhisper == "\n" && chunk.Offset == protocol.CoNewline && strings.HasPrefix(chunk.Text, "line-") {
		fmt.Printf("%s got the correct newline chunk\n", source)
	} else if chunk.Offset == 0 && chunk.Text == lastWhisper {
		fmt.Printf("%s got the correct text chunk for %q\n", source, lastWhisper)
	} else {
		t.Errorf("%s got an incorrect chunk after whisper %q: %v", source, lastWhisper, chunk)
	}
}
