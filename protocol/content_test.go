/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package protocol

import (
	"strings"
	"testing"
)

func TestProcessLiveChunk(t *testing.T) {
	tests := []struct {
		name            string
		oldLive         string
		chunk           ContentChunk
		expectedLive    string
		expectedPast    []string
		expectedPastIds []string
	}{
		{
			"coNewline offset",
			"hello",
			ContentChunk{Offset: CoNewline, Text: "frotz"},
			"",
			[]string{"hello"},
			[]string{"frotz"},
		},
		{
			"Offset within oldLive",
			"hello",
			ContentChunk{Offset: 3, Text: "p"},
			"help",
			nil,
			nil,
		},
		{
			"Offset beyond oldLive",
			"hi",
			ContentChunk{Offset: 4, Text: "z"},
			"hi??z",
			nil,
			nil,
		},
		{
			"Empty oldLive",
			"",
			ContentChunk{Offset: 0, Text: "new"},
			"new",
			nil,
			nil,
		},
		{
			"Chunk.Text appending to oldLive",
			"world",
			ContentChunk{Offset: 5, Text: "!"},
			"world!",
			nil,
			nil,
		},
		{
			"Chunk doesn't affect live",
			"hello!",
			ContentChunk{Offset: CoPlaySound, Text: "sound-name"},
			"hello!",
			nil,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualLive, actualPast, actualPastId := ProcessLiveChunk(tt.oldLive, tt.chunk)
			if actualLive != tt.expectedLive {
				t.Errorf("Expected live: %q, got: %q", tt.expectedLive, actualLive)
			}
			if actualPast == nil {
				if tt.expectedPast != nil {
					t.Errorf("Expected past: %v, got: nil", tt.expectedPast[0])
				}
				if tt.expectedPastIds != nil {
					t.Errorf("Expected pastId: %v, got: nil", tt.expectedPastIds[0])
				}
			} else {
				if tt.expectedPast == nil {
					t.Errorf("Expected past: %v, got: %v", tt.expectedPast, *actualPast)
				} else if tt.expectedPast[0] != *actualPast {
					t.Errorf("Expected past: %v, got: %v", tt.expectedPast[0], *actualPast)
				}
				if tt.expectedPastIds == nil {
					t.Errorf("Expected pastId: %v, got: %v", tt.expectedPastIds, *actualPastId)
				} else if tt.expectedPastIds[0] != *actualPastId {
					t.Errorf("Expected pastId: %v, got: %v", tt.expectedPastIds[0], *actualPastId)
				}
			}
		})
	}
}

func TestDiffLines(t *testing.T) {
	tests := []struct {
		name     string
		old      string
		new      string
		expected []ContentChunk
	}{
		{
			"Base case - lines differ",
			"here is some live text",
			"here is some new live text",
			[]ContentChunk{{Offset: len("here is some "), Text: "new live text"}},
		},
		{
			"Identical strings",
			"test",
			"test",
			nil,
		},
		{
			"New string longer",
			"test",
			"testing",
			[]ContentChunk{{Offset: 4, Text: "ing"}},
		},
		{
			"New string shorter",
			"testing",
			"test",
			[]ContentChunk{{Offset: 4, Text: ""}},
		},
		{
			"Suffix with a newline",
			"hello",
			"hello\nworld",
			[]ContentChunk{
				{Offset: 5, Text: ""},
				{Offset: CoNewline, Text: ""},
				{Offset: 0, Text: "world"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := DiffLines(tt.old, tt.new)
			if len(actual) != len(tt.expected) {
				t.Fatalf("Expected %d chunks, got %d", len(tt.expected), len(actual))
			}
			for i, expectedChunk := range tt.expected {
				if actual[i] != expectedChunk {
					if actual[i].Offset != CoNewline {
						t.Errorf("Chunk[%d] - Expected: %+v, got: %+v", i, expectedChunk, actual[i])
					} else {
						if expectedChunk.Offset != CoNewline || !strings.HasPrefix(actual[i].Text, "line-") {
							t.Errorf("Chunk[%d] - Expected: %+v, got: %+v", i, expectedChunk, actual[i])
						}
					}
				}
			}
		})
	}
}

func TestSuffixToChunks(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		start    int
		expected []ContentChunk
	}{
		{
			"No additional lines",
			"Text",
			4,
			nil,
		},
		{
			"Single-line addition",
			"hello\nworld",
			5,
			[]ContentChunk{
				{Offset: 5, Text: ""},
				{Offset: CoNewline, Text: ""},
				{Offset: 0, Text: "world"},
			},
		},
		{
			"Multi-line addition",
			"hello\nworld\n!",
			5,
			[]ContentChunk{
				{Offset: 5, Text: ""},
				{Offset: CoNewline, Text: ""},
				{Offset: 0, Text: "world"},
				{Offset: CoNewline, Text: ""},
				{Offset: 0, Text: "!"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := suffixToChunks(tt.text, tt.start)
			if len(actual) != len(tt.expected) {
				t.Fatalf("Expected %d chunks, got %d", len(tt.expected), len(actual))
			}
			for i, expectedChunk := range tt.expected {
				if actual[i] != expectedChunk {
					if actual[i].Offset != CoNewline {
						t.Errorf("Chunk[%d] - Expected: %+v, got: %+v", i, expectedChunk, actual[i])
					} else {
						if expectedChunk.Offset != CoNewline || !strings.HasPrefix(actual[i].Text, "line-") {
							t.Errorf("Chunk[%d] - Expected: %+v, got: %+v", i, expectedChunk, actual[i])
						}
					}
				}
			}
		})
	}
}
