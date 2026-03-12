/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package snapins

type ContentReceiver func(string)

type ConversationManager interface {
	// EnsureSession makes sure a pub-sub session exists for the given conversation.
	//
	// If one doesn't already exist, it's created, and arrangements are made so
	// that all content chunks sent from clients go to the receiver.
	EnsureSession(conversationId string, receiver ContentReceiver) error
	EndSession(conversationId string) error
	SetWhisperer(conversationId, profileId, clientId string) (string, error)
	AddListener(conversationId, profileId, clientId string) (string, error)
	RemoveListener(conversationId, profileId, clientId string) error
}
