/*
 * Copyright 2025-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package protocol

import "testing"

func TestRequestsPendingChunk(t *testing.T) {
	expected := "approve-requests|"
	result := RequestsPendingChunk()

	if result.String() != expected {
		t.Errorf("RequestsPendingChunk() failed, got %q, want %q", result, expected)
	}
}

func TestIsRequestsPendingChunk(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected bool
	}{
		{
			name:     "Correct requests pending chunk",
			data:     "approve-requests|",
			expected: true,
		},
		{
			name:     "Incorrect requests pending chunk",
			data:     "wrong-chunk|",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := ParseControlChunk(tt.data)
			result := IsRequestsPendingChunk(chunk)

			if result != tt.expected {
				t.Errorf("IsRequestsPendingChunk(%q) failed, got %v, want %v", tt.data, result, tt.expected)
			}
		})
	}
}

func TestParticipantsChangedChunk(t *testing.T) {
	expected := "participants-changed|"
	result := ParticipantsChangedChunk()

	if result.String() != expected {
		t.Errorf("ParticipantsChangedChunk() failed, got %q, want %q", result, expected)
	}
}

func TestIsParticipantsChangedChunk(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected bool
	}{
		{
			name:     "Correct participants changed chunk",
			data:     "participants-changed|",
			expected: true,
		},
		{
			name:     "Incorrect participants changed chunk",
			data:     "wrong-chunk|",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := ParseControlChunk(tt.data)
			result := IsParticipantsChangedChunk(chunk)

			if result != tt.expected {
				t.Errorf("IsParticipantsChangedChunk(%q) failed, got %v, want %v", tt.data, result, tt.expected)
			}
		})
	}
}

func TestEndChunk(t *testing.T) {
	p1 := EndChunk()
	e1 := "end|"
	if p1.String() != e1 {
		t.Errorf("EndChunk() failed, got %v, want %v", p1, e1)
	}
	if !IsEndChunk(p1) {
		t.Errorf("IsEndChunk() didn't recognize chunk")
	}
	p2 := ParticipantsChangedChunk()
	if IsEndChunk(p2) {
		t.Errorf("IsEndChunk() recognized invalid chunk")
	}
}
