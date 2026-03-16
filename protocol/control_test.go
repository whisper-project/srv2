/*
 * Copyright 2025-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package protocol

import "testing"

func TestRequestsPendingPacket(t *testing.T) {
	expected := "approve-requests|"
	result := RequestsPendingPacket()

	if result != expected {
		t.Errorf("RequestsPendingPacket() failed, got %q, want %q", result, expected)
	}
}

func TestIsRequestsPendingPacket(t *testing.T) {
	tests := []struct {
		name     string
		packet   string
		expected bool
	}{
		{
			name:     "Correct requests pending packet",
			packet:   "approve-requests|",
			expected: true,
		},
		{
			name:     "Incorrect requests pending packet",
			packet:   "wrong-packet|",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRequestsPendingPacket(tt.packet)

			if result != tt.expected {
				t.Errorf("IsRequestsPendingPacket(%q) failed, got %v, want %v", tt.packet, result, tt.expected)
			}
		})
	}
}

func TestParticipantsChangedPacket(t *testing.T) {
	expected := "participants-changed|"
	result := ParticipantsChangedPacket()

	if result != expected {
		t.Errorf("ParticipantsChangedPacket() failed, got %q, want %q", result, expected)
	}
}

func TestIsParticipantsChangedPacket(t *testing.T) {
	tests := []struct {
		name     string
		packet   string
		expected bool
	}{
		{
			name:     "Correct participants changed packet",
			packet:   "participants-changed|",
			expected: true,
		},
		{
			name:     "Incorrect participants changed packet",
			packet:   "wrong-packet|",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsParticipantsChangedPacket(tt.packet)

			if result != tt.expected {
				t.Errorf("IsParticipantsChangedPacket(%q) failed, got %v, want %v", tt.packet, result, tt.expected)
			}
		})
	}
}

func TestPastTextSpeechIdPacket(t *testing.T) {
	p1 := PastTextSpeechIdPacket("1", "2", "3")
	e1 := "past-text-speech-id|1|2|3"
	if p1 != e1 {
		t.Errorf("PastTextSpeechIdPacket() failed, got %v, want %v", p1, e1)
	}
	ok1, pId1, num1, sId1 := IsPastTextSpeechIdPacket(p1)
	if !ok1 {
		t.Errorf("IsPastTextSpeechIdPacket() didn't recognize packet")
	}
	if pId1 != "1" || num1 != "2" || sId1 != "3" {
		t.Errorf("IsPastTextSpeechIdPacket() returned [%v, %v, %v], want [1, 2, 3]", pId1, num1, sId1)
	}
	p2 := ParticipantsChangedPacket()
	ok2, _, _, _ := IsPastTextSpeechIdPacket(p2)
	if ok2 {
		t.Errorf("IsPastTextSpeechIdPacket() recognized invalid packet")
	}
}

func TestEndPacket(t *testing.T) {
	p1 := EndPacket()
	e1 := "end|"
	if p1 != e1 {
		t.Errorf("EndPacket() failed, got %v, want %v", p1, e1)
	}
	if !IsEndPacket(p1) {
		t.Errorf("IsEndPacket() didn't recognize packet")
	}
	p2 := ParticipantsChangedPacket()
	if IsEndPacket(p2) {
		t.Errorf("IsEndPacket() recognized invalid packet")
	}
}
