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
