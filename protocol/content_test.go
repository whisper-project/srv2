/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package protocol

import "testing"

func TestProcessLiveChunk(t *testing.T) {
	tests := []struct {
		name         string
		oldLive      string
		chunk        ContentChunk
		expectedLive string
		expectedPast []string
	}{
		{
			"coNewline offset",
			"hello",
			ContentChunk{Offset: CoNewline, Text: ""},
			"",
			[]string{"hello"},
		},
		{
			"Offset within oldLive",
			"hello",
			ContentChunk{Offset: 3, Text: "p"},
			"help",
			nil,
		},
		{
			"Offset beyond oldLive",
			"hi",
			ContentChunk{Offset: 4, Text: "z"},
			"hi??z",
			nil,
		},
		{
			"Empty oldLive",
			"",
			ContentChunk{Offset: 0, Text: "new"},
			"new",
			nil,
		},
		{
			"Chunk.Text appending to oldLive",
			"world",
			ContentChunk{Offset: 5, Text: "!"},
			"world!",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualLive, actualPast := ProcessLiveChunk(tt.oldLive, tt.chunk)
			if actualLive != tt.expectedLive {
				t.Errorf("Expected live: %q, got: %q", tt.expectedLive, actualLive)
			}
			if len(actualPast) != len(tt.expectedPast) {
				t.Fatalf("Expected past length: %d, got: %d", len(tt.expectedPast), len(actualPast))
			}
			for i, expectedPast := range tt.expectedPast {
				if actualPast[i] != expectedPast {
					t.Errorf("Past[%d] - Expected: %q, got: %q", i, expectedPast, actualPast[i])
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
			"Suffix with newline",
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
					t.Errorf("Chunk[%d] - Expected: %+v, got: %+v", i, expectedChunk, actual[i])
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
			"Single line addition",
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
					t.Errorf("Chunk[%d] - Expected: %+v, got: %+v", i, expectedChunk, actual[i])
				}
			}
		})
	}
}
