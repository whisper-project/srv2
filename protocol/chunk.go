/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package protocol

import (
	"fmt"
	"strconv"
	"strings"
)

type ControlChunk struct {
	Action string
	Args   []string
}

func (c ControlChunk) String() string {
	return fmt.Sprintf("%s|%s", c.Action, strings.Join(c.Args, "|"))
}

func ParseControlChunk(s string) ControlChunk {
	left, right, found := strings.Cut(s, "|")
	if !found || right == "" {
		return ControlChunk{Action: left, Args: nil}
	}
	return ControlChunk{Action: left, Args: strings.Split(right, "|")}
}

type ContentChunk struct {
	Offset int
	Text   string
}

const (
	CoNewline   = -1    // Shift current live Text to past Text (ignore chunk Text)
	CoPlaySound = -2    // Play local sound resource named by the chunk Text
	CoIgnore    = -1000 // Ignore this packet (used for recovery from transmission errors)
)

var ccNames = map[int]string{
	CoNewline:   "newline",
	CoPlaySound: "play sound",
	CoIgnore:    "ignore",
}

func (c ContentChunk) String() string {
	return fmt.Sprintf("%d|%s", c.Offset, c.Text)
}

func (c ContentChunk) DebugString() string {
	if c.Offset >= 0 {
		return fmt.Sprintf("%d|%s", c.Offset, c.Text)
	}
	name := ccNames[c.Offset]
	if name == "" {
		return fmt.Sprintf("unknown offset %d: %s", c.Offset, c.Text)
	}
	if c.Text == "" {
		return name
	}
	return fmt.Sprintf("%s: %s", name, c.Text)
}

func ParseContentChunk(s string) ContentChunk {
	left, right, found := strings.Cut(s, "|")
	if !found {
		return ContentChunk{Offset: CoIgnore, Text: s}
	}
	offset, err := strconv.Atoi(left)
	if err != nil {
		return ContentChunk{Offset: CoIgnore, Text: s}
	}
	return ContentChunk{Offset: offset, Text: right}
}

type ContentPacket struct {
	PacketId string
	ClientId string
	Data     string
}

type ContentReceiver chan ContentPacket

func (c ContentPacket) String() string {
	return fmt.Sprintf("%s|%s|%s", c.PacketId, c.ClientId, c.Data)
}

func ParseContentPacket(s string) ContentPacket {
	packetId, right, found := strings.Cut(s, "|")
	if !found {
		return ContentPacket{PacketId: packetId, ClientId: "", Data: ""}
	}
	clientId, data, found := strings.Cut(right, "|")
	if !found {
		return ContentPacket{PacketId: packetId, ClientId: clientId, Data: ""}
	}
	return ContentPacket{PacketId: packetId, ClientId: clientId, Data: data}
}
