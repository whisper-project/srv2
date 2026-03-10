/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/whisper-project/server.golang/platform"

	"github.com/google/uuid"
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

func (c *Conversation) SetStorageId(id string) error {
	if c == nil {
		return fmt.Errorf("can't set id of nil %T", c)
	}
	c.Id = id
	return nil
}

func (c *Conversation) Copy() platform.StructPointer {
	if c == nil {
		return nil
	}
	n := new(Conversation)
	*n = *c
	return n
}

func (c *Conversation) Downgrade(a any) (platform.StructPointer, error) {
	if o, ok := a.(Conversation); ok {
		return &o, nil
	}
	if o, ok := a.(*Conversation); ok {
		return o, nil
	}
	return nil, fmt.Errorf("not a %T: %#v", c, a)
}

func NewConversation(owner, name string) *Conversation {
	return &Conversation{
		Id:    uuid.NewString(),
		Owner: owner,
		Name:  name,
	}
}

func IsOwnedConversation(profileId, conversationId string) (bool, error) {
	var conversation Conversation
	if conversationId == "" {
		sLog().Info("Empty conversation id")
		return false, nil
	}
	if err := platform.LoadFields(sCtx(), &conversation); err != nil {
		sLog().Error("Load Fields failure on conversation retrieval",
			zap.String("conversationId", conversation.Id), zap.Error(err))
		return false, err
	}
	if conversation.Owner != profileId {
		sLog().Info("Conversation owner mismatch",
			zap.String("conversationId", conversation.Id), zap.String("profileId", profileId))
		return false, nil
	}
	return true, nil
}

type AllowedListeners string

func (a AllowedListeners) StoragePrefix() string {
	return "allowed-listeners:"
}

func (a AllowedListeners) StorageId() string {
	return string(a)
}

func MakeAllowedListener(profileId, conversationId string) error {
	id := AllowedListeners(conversationId)
	if err := platform.AddMembers(sCtx(), id, profileId); err != nil {
		sLog().Error("storage failure adding allowed listener",
			zap.String("conversationId", conversationId), zap.String("profileId", profileId),
			zap.Error(err))
		return err
	}
	return nil
}

func IsAllowedListener(profileId, conversationId string) (bool, error) {
	id := AllowedListeners(conversationId)
	ok, err := platform.IsMember(sCtx(), id, profileId)
	if err != nil {
		sLog().Error("storage failure retrieving allowed listener",
			zap.String("conversationId", conversationId), zap.String("profileId", profileId),
			zap.Error(err))
	}
	return ok, err
}
