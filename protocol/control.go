/*
 * Copyright 2025-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package protocol

func RequestsPendingChunk() ControlChunk {
	return ControlChunk{Action: "approve-requests"}
}

func IsRequestsPendingChunk(chunk ControlChunk) bool {
	return chunk.Action == "approve-requests"
}

func ParticipantsChangedChunk() ControlChunk {
	return ControlChunk{Action: "participants-changed"}
}

func IsParticipantsChangedChunk(chunk ControlChunk) bool {
	return chunk.Action == "participants-changed"
}

func EndChunk() ControlChunk {
	return ControlChunk{Action: "end"}
}

func IsEndChunk(chunk ControlChunk) bool {
	return chunk.Action == "end"
}
