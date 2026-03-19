/*
 * Copyright 2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package protocol

import (
	"testing"

	"github.com/go-test/deep"
)

// Tests for ControlChunk.String()
func TestControlChunk_String(t *testing.T) {
	tests := []struct {
		name     string
		chunk    ControlChunk
		expected string
	}{
		{
			name:     "Single action with no args",
			chunk:    ControlChunk{Action: "quit", Args: []string{}},
			expected: "quit|",
		},
		{
			name:     "Single action with a single empty arg",
			chunk:    ControlChunk{Action: "quit", Args: []string{""}},
			expected: "quit|",
		},
		{
			name:     "Action with multiple args",
			chunk:    ControlChunk{Action: "active", Args: []string{"arg1", "arg2"}},
			expected: "active|arg1|arg2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.chunk.String(); result != tt.expected {
				t.Errorf("ControlChunk.String() failed, got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Tests for ParseControlChunk()
func TestParseControlChunk(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ControlChunk
	}{
		{
			name:     "Parse with action only, no separator",
			input:    "quit",
			expected: ControlChunk{Action: "quit", Args: nil},
		},
		{
			name:     "Parse with action and arguments",
			input:    "add|arg1|arg2",
			expected: ControlChunk{Action: "add", Args: []string{"arg1", "arg2"}},
		},
		{
			name:     "Parse with action and empty single argument",
			input:    "quit|",
			expected: ControlChunk{Action: "quit", Args: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseControlChunk(tt.input)
			if result.Action != tt.expected.Action || deep.Equal(result.Args, tt.expected.Args) != nil {
				t.Errorf("ParseControlChunk() failed, got %+v, want %+v", result, tt.expected)
			}
		})
	}
}

// Tests for ContentChunk.String()
func TestContentChunk_String(t *testing.T) {
	tests := []struct {
		name     string
		chunk    ContentChunk
		expected string
	}{
		{
			name:     "Valid chunk",
			chunk:    ContentChunk{Offset: 10, Text: "HelloWorld"},
			expected: "10|HelloWorld",
		},
		{
			name:     "Empty chunk",
			chunk:    ContentChunk{Offset: 0, Text: ""},
			expected: "0|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.chunk.String(); result != tt.expected {
				t.Errorf("ContentChunk.String() failed, got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Tests for ContentChunk.DebugString()
func TestContentChunk_DebugString(t *testing.T) {
	tests := []struct {
		name     string
		chunk    ContentChunk
		expected string
	}{
		{
			name:     "Positive offset",
			chunk:    ContentChunk{Offset: 5, Text: "Positive"},
			expected: "5|Positive",
		},
		{
			name:     "Known negative offset",
			chunk:    ContentChunk{Offset: CoNewline, Text: ""},
			expected: "newline",
		},
		{
			name:     "Known negative offset with value",
			chunk:    ContentChunk{Offset: CoPlaySound, Text: "some-sound"},
			expected: "play sound: some-sound",
		},
		{
			name:     "Unknown negative offset",
			chunk:    ContentChunk{Offset: -999, Text: "UnknownError"},
			expected: "unknown offset -999: UnknownError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.chunk.DebugString(); result != tt.expected {
				t.Errorf("ContentChunk.DebugString() failed, got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Tests for ParseContentChunk()
func TestParseContentChunk(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ContentChunk
	}{
		{
			name:     "Valid positive input",
			input:    "1|HelloChunk",
			expected: ContentChunk{Offset: 1, Text: "HelloChunk"},
		},
		{
			name:     "Valid negative input",
			input:    "-3|Live Text",
			expected: ContentChunk{Offset: -3, Text: "Live Text"},
		},
		{
			name:     "Non-numeric offset",
			input:    "abc|Invalid",
			expected: ContentChunk{Offset: CoIgnore, Text: "abc|Invalid"},
		},
		{
			name:     "Malformed input",
			input:    "MalformedText",
			expected: ContentChunk{Offset: CoIgnore, Text: "MalformedText"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseContentChunk(tt.input)
			if result != tt.expected {
				t.Errorf("ParseContentChunk() failed, got %+v, want %+v", result, tt.expected)
			}
		})
	}
}

// Tests for ContentPacket.String()
func TestContentPacket_String(t *testing.T) {
	tests := []struct {
		name     string
		packet   ContentPacket
		expected string
	}{
		{
			name:     "Valid packet",
			packet:   ContentPacket{PacketId: "1", Data: "3|x"},
			expected: "1|3|x",
		},
		{
			name:     "Empty fields",
			packet:   ContentPacket{PacketId: "", Data: ""},
			expected: "|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.packet.String(); result != tt.expected {
				t.Errorf("ContentPacket.String() failed, got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Tests for ParseContentPacket()
func TestParseContentPacket(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ContentPacket
	}{
		{
			name:     "Valid input",
			input:    "1|3|x",
			expected: ContentPacket{PacketId: "1", Data: "3|x"},
		},
		{
			name:     "Missing data",
			input:    "1|",
			expected: ContentPacket{PacketId: "1", Data: ""},
		},
		{
			name:     "Packet ID only",
			input:    "packet1",
			expected: ContentPacket{PacketId: "packet1", Data: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseContentPacket(tt.input)
			if result != tt.expected {
				t.Errorf("ParseContentPacket() failed, got %+v, want %+v", result, tt.expected)
			}
		})
	}
}
