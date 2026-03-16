/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

// Package pubsub manages sessions of conversations.
//
// All the members of a session are subscribers to the
// session data being published from the whisperer (typed packets) and from the
// server (vocalized speech).
//
// Each available pubsub provider implements the same interface, and each
// session uses one of the available providers.
package pubsub

import (
	"errors"

	"github.com/whisper-project/srv2/protocol"
)

// The Manager interface is what each pubsub implementation provides.
//
// The documentation here describes how these methods are used by the conversation
// manager over the course of a session. The pubsub implementations will each
// provide more detail about how they implement the methods.
type Manager = interface {
	// StartSession is called by the conversation manager whenever it starts a session
	// for a conversation. The conversation manager must pass in channels on which
	// the pubsub manager can send it the whispered session content and updates
	// about client status in the session.
	//
	// The pubsub manager will never create more than one session for a given sessionId,
	// so conversation managers needn't worry about race conditions leading them to start
	// sessions more than once.
	StartSession(sessionId string, cr protocol.ContentReceiver, sr StatusReceiver) error
	// EndSession is called by the conversation manager to terminate an existing session.
	//
	// Like StartSession, it doesn't matter how many times this is called with the same sessionId.
	EndSession(sessionId string)
	// AddWhisperer is called by the conversation manager to add a Whisperer to a session.
	//
	// The same user may act as a Whisperer from multiple clients, so this may be called
	// multiple times (with a different clientId each time) for the same session.
	AddWhisperer(sessionId, clientId string) (bool, error)
	// AddListener is called by the conversation manager to add a Listener to a session.
	AddListener(sessionId, clientId string) (bool, error)
	// ClientToken is called by the conversation manager after enrolling a client in a session
	// in order to get the client an authorization token appropriate to its role in the pubsub session.
	//
	// If a pubsub manager does not use authorization with clients, this should return an empty
	// token and no error.
	//
	// If a pubsub manager uses authorization leases, this will be called whenever the client needs
	// to renew their lease.
	ClientToken(sessionId, clientId string) ([]byte, error)
	// RemoveClient is called by the conversation manager when a client opts out of a session
	// (possibly by shutting down). The pubsub manager may hear from the client even after this
	// call is made, but it should ignore/refuse any such interactions.
	RemoveClient(sessionId, clientId string) error
	// Send is called by the conversation manager to communicate with a specific client.
	Send(sessionId, clientId, packet string) error
	// Broadcast is called by the conversation manager to communicate with all clients.
	Broadcast(sessionId, packet string) error
}

// A NoSessionError (or actually an error wrapping it) is returned whenever a call
// (other than StartSession or EndSession)
// is made on a sessionId that hasn't been started.
var NoSessionError error = errors.New("no such session")

// A ClientStatus gives the state of a client in a session.
type ClientStatus struct {
	ClientId string
	IsOnline bool
}

// A StatusReceiver is used by the pubsub manager to notify
// the server of updates to client status in a session.
type StatusReceiver chan ClientStatus
