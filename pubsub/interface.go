/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package pubsub

import "github.com/whisper-project/srv2/protocol"

type Manager = interface {
	StartSession(sessionId string, cr protocol.ContentReceiver, sr StatusReceiver) error
	EndSession(sessionId string) error
	AddWhisperer(sessionId, clientId string) (bool, error)
	AddListener(sessionId, clientId string) (bool, error)
	ClientToken(sessionId, clientId string) ([]byte, error)
	RemoveClient(sessionId, clientId string) error
	Send(sessionId, clientId, packet string) error
	Broadcast(sessionId, packet string) error
}
