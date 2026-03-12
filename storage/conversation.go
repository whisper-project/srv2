/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"bytes"
	"encoding/gob"

	"go.uber.org/zap"

	"github.com/whisper-project/server.golang/platform"
)

type Conversation struct {
	Id    string
	Owner string
	Name  string
}

func (c *Conversation) StoragePrefix() string {
	return "conversation:"
}

func (c *Conversation) StorageId() string {
	if c == nil {
		return ""
	}
	return c.Id
}

func (c *Conversation) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(c); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (c *Conversation) FromRedis(data []byte) error {
	*c = Conversation{} // dump old data
	return gob.NewDecoder(bytes.NewReader(data)).Decode(c)
}

// NewConversation creates a new conversation with the given owner (profileId) and name.
func NewConversation(owner, name string) *Conversation {
	return &Conversation{
		Id:    platform.NewId("convo-"),
		Owner: owner,
		Name:  name,
	}
}

// GetConversation returns the conversation with the given id.
func GetConversation(id string) (*Conversation, error) {
	c := &Conversation{Id: id}
	if err := platform.FetchObject(sCtx(), c); err != nil {
		sLog().Error("storage failure (load) on Conversation",
			zap.String("id", id), zap.Error(err))
		return nil, err
	}
	return c, nil
}

// SaveConversation saves the conversation with the given id.
func SaveConversation(c *Conversation) error {
	if err := platform.StoreObject(sCtx(), c); err != nil {
		sLog().Error("storage failure (save) on Conversation",
			zap.String("id", c.Id), zap.Error(err))
		return err
	}
	return nil
}

// DeleteConversation deletes the conversation with the given id.
func DeleteConversation(id string) error {
	if err := platform.DeleteStorage(sCtx(), &Conversation{Id: id}); err != nil {
		sLog().Error("storage failure (delete) on Conversation",
			zap.String("id", id), zap.Error(err))
		return err
	}
	return nil
}

// IsOwnedConversation checks if the user with the given profileId is the owner
// of the conversation with the given conversationId.
func IsOwnedConversation(profileId, conversationId string) (bool, error) {
	c, err := GetConversation(conversationId)
	if err != nil {
		return false, err
	}
	return c.Owner == profileId, nil
}

// The AllowedListeners for the conversation with the given conversationId is
// the set of profileIds for users accepted into that conversation.
type AllowedListeners string

func (a AllowedListeners) StoragePrefix() string {
	return "allowed-listeners:"
}

func (a AllowedListeners) StorageId() string {
	return string(a)
}

// IsAllowedListener checks if the conversation with the given conversationId
// has allowed the user with the given profileId as a listener.
func IsAllowedListener(conversationId, profileId string) (bool, error) {
	id := AllowedListeners(conversationId)
	ok, err := platform.IsSetMember(sCtx(), id, profileId)
	if err != nil {
		sLog().Error("storage failure (lookup) on AllowedListeners",
			zap.String("conversationId", conversationId), zap.Error(err))
	}
	return ok, err
}

// AddAllowedListener ensures that the conversation with the given conversationId
// allows the user with the given profileId as a listener.
func AddAllowedListener(conversationId, profileId string) error {
	id := AllowedListeners(conversationId)
	if err := platform.AddSetMembers(sCtx(), id, profileId); err != nil {
		sLog().Error("storage failure (add) on AllowedListeners",
			zap.String("conversationId", conversationId), zap.Error(err))
		return err
	}
	return nil
}

// RemoveAllowedListener ensures that the conversation with the given conversationId
// does not allow the user with the given profileId as a listener.
func RemoveAllowedListener(conversationId, profileId string) error {
	id := AllowedListeners(conversationId)
	if err := platform.RemoveSetMembers(sCtx(), id, profileId); err != nil {
		sLog().Error("storage failure (remove) on AllowedListeners",
			zap.String("conversationId", conversationId), zap.Error(err))
		return err
	}
	return nil
}

// ListAllowedListeners returns the list of profileIds for users
// who are allowed to listen to the conversation with the given conversationId.
func ListAllowedListeners(conversationId string) ([]string, error) {
	id := AllowedListeners(conversationId)
	list, err := platform.FetchSetMembers(sCtx(), id)
	if err != nil {
		sLog().Error("storage failure (fetch) on AllowedListeners",
			zap.String("conversationId", conversationId), zap.Error(err))
		return nil, err
	}
	return list, nil
}
