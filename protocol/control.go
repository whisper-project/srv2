/*
 * Copyright 2025-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package protocol

func RequestsPendingPacket() string {
	return ControlChunk{Action: "approve-requests"}.String()
}

func IsRequestsPendingPacket(packet string) bool {
	return ParseControlChunk(packet).Action == "approve-requests"
}

func ParticipantsChangedPacket() string {
	return ControlChunk{Action: "participants-changed"}.String()
}

func IsParticipantsChangedPacket(packet string) bool {
	return ParseControlChunk(packet).Action == "participants-changed"
}

func PastTextSpeechIdPacket(packetId, sequenceNum, speechId string) string {
	return ControlChunk{
		Action: "past-text-speech-id",
		Args:   []string{packetId, sequenceNum, speechId},
	}.String()
}

// IsPastTextSpeechIdPacket checks if the given packet has action "past-text-speech-id".
// Returns a boolean indicating whether this is a past text speech packet.
// If it is, it additionally returns:
// - the packetId that produced the past text,
// - which line of past text this was in the lines produced by that packet, and
// - the speech ID to request from the server for the generated speech.
func IsPastTextSpeechIdPacket(packet string) (bool, string, string, string) {
	if chunk := ParseControlChunk(packet); chunk.Action == "past-text-speech-id" {
		return true, chunk.Args[0], chunk.Args[1], chunk.Args[2]
	}
	return false, "", "", ""
}

func EndPacket() string {
	return ControlChunk{Action: "end"}.String()
}

func IsEndPacket(packet string) bool {
	return ParseControlChunk(packet).Action == "end"
}
