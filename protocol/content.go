/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package protocol

import (
	"strings"

	"github.com/whisper-project/whisper.server2/platform"
)

// ProcessLiveChunk "plays" an incoming content chunk against the current live text.
//
// It produces as outputs the new live text, any created line of past text, and the id of that line.
//
// Note that not all chunks actually change text. If you pass, for example, a chunk
// that says to play a sound, it will have no effect.
//
// If the offset of the chunk is longer than the current live text, the missing
// space is filled with '?' characters.
func ProcessLiveChunk(oldLive string, chunk ContentChunk) (newLive string, newPast *string, newPastId *string) {
	// offsets less than CoNewline don't change the live text
	if chunk.Offset < CoNewline {
		return oldLive, nil, nil
	}
	// if this is a newline, we move the live text to past text and use the newline ID as the past text ID
	if chunk.Offset == CoNewline {
		return "", &oldLive, &chunk.Text
	}
	// this is a chunk that extends or shortens the live text
	if chunk.Offset > len(oldLive) {
		oldLive += strings.Repeat("?", chunk.Offset-len(oldLive))
	}
	return oldLive[0:chunk.Offset] + chunk.Text, nil, nil
}

// DiffLines creates the chunks that are sent when a user, whose current live
// Text is `old`, alters that live Text to be `new` (by typing, or by a cut/paste
// that contains multiple lines).
//
// If the old and new strings are identical, the returned slice will be empty.
// If the new string has no newlines, the returned slice will have one chunk.
// Otherwise, the returned slice will have multiple chunks, and they have
// to be processed in order to get the correct live and past Text at the end.
func DiffLines(oldLive, newLive string) []ContentChunk {
	for i := 0; i < len(oldLive) && i < len(newLive); i++ {
		if oldLive[i] != newLive[i] {
			return suffixToChunks(newLive, i)
		}
	}
	// fell through: either one is a proper substring of the other or they are identical
	if len(oldLive) < len(newLive) {
		return suffixToChunks(newLive, len(oldLive))
	}
	if len(oldLive) > len(newLive) {
		return []ContentChunk{{Offset: len(newLive), Text: ""}}
	}
	return nil
}

func suffixToChunks(s string, start int) []ContentChunk {
	if len(s) <= start {
		return nil
	}
	lines := strings.Split(s[start:], "\n")
	result := make([]ContentChunk, 1, len(lines)*2-1)
	result[0] = ContentChunk{Offset: start, Text: lines[0]}
	for i := 1; i < len(lines); i++ {
		result = append(result, ContentChunk{Offset: CoNewline, Text: platform.NewId("line-")})
		result = append(result, ContentChunk{Offset: 0, Text: lines[i]})
	}
	return result
}
