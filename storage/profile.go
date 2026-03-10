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

var EmailProfileMap = platform.StorableMap("email-profile-map")

type Profile struct {
	Id        string
	Name      string
	EmailHash string
	Secret    string
}

func (p *Profile) StoragePrefix() string {
	return "profile:"
}

func (p *Profile) StorageId() string {
	if p == nil {
		return ""
	}
	return p.Id
}

func (p *Profile) SetStorageId(id string) error {
	if p == nil {
		return fmt.Errorf("can't set id of nil %T", p)
	}
	p.Id = id
	return nil
}

func (p *Profile) Copy() platform.StructPointer {
	if p == nil {
		return nil
	}
	n := new(Profile)
	*n = *p
	return n
}

func (p *Profile) Downgrade(a any) (platform.StructPointer, error) {
	if o, ok := a.(Profile); ok {
		return &o, nil
	}
	if o, ok := a.(*Profile); ok {
		return o, nil
	}
	return nil, fmt.Errorf("not a %T: %#v", p, a)
}

func NewProfile(emailHash string) *Profile {
	if emailHash == "" {
		panic("email hash required for new profile")
	}
	return &Profile{
		Id:        uuid.NewString(),
		EmailHash: emailHash,
		Secret:    uuid.NewString(),
	}
}

type WhisperConversationMap string

func (p WhisperConversationMap) StoragePrefix() string {
	return "whisper-conversations:"
}

func (p WhisperConversationMap) StorageId() string {
	return string(p)
}

// NewLaunchProfile creates a launch profile for a hashed email from a client and records it in the database.
func NewLaunchProfile(clientType, hashedEmail, clientId string) (*Profile, error) {
	p := NewProfile(hashedEmail)
	if err := platform.SaveFields(sCtx(), p); err != nil {
		sLog().Error("Save Fields failure on new profile creation",
			zap.String("profileId", p.Id), zap.Error(err))
		return nil, err
	}
	cleanup := false
	defer func() {
		if !cleanup {
			return
		}
		_ = platform.MapRemove(sCtx(), EmailProfileMap, hashedEmail)
		_ = platform.DeleteStorage(sCtx(), p)
	}()
	if err := SetEmailProfile(p.EmailHash, p.Id); err != nil {
		cleanup = true
		return nil, err
	}
	if _, err := AddWhisperConversation(p.Id, "Conversation 1"); err != nil {
		cleanup = true
		return nil, err
	}
	ObserveClientLaunch(clientType, clientId, p.Id)
	return p, nil
}

func WhisperConversation(profileId string, name string) (string, error) {
	if name == "" {
		return "", nil
	}
	key := WhisperConversationMap(profileId)
	conversationId, err := platform.MapGet(sCtx(), key, name)
	if err != nil {
		sLog().Error("platform error on whisper conversation retrieval",
			zap.String("profileId", profileId), zap.String("name", name), zap.Error(err))
		return "", err
	}
	return conversationId, nil
}

func WhisperConversations(profileId string) (map[string]string, error) {
	key := WhisperConversationMap(profileId)
	cMap, err := platform.MapGetAll(sCtx(), key)
	if err != nil {
		sLog().Error("platform error on whisper conversations retrieval",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, err
	}
	return cMap, nil
}

func AddWhisperConversation(profileId string, name string) (string, error) {
	key := WhisperConversationMap(profileId)
	conversation := NewConversation(profileId, name)
	if err := platform.SaveFields(sCtx(), conversation); err != nil {
		sLog().Error("Save Fields failure on whisper conversation creation",
			zap.String("conversationId", conversation.Id), zap.Error(err))
		return "", err
	}
	if err := platform.MapSet(sCtx(), key, name, conversation.Id); err != nil {
		sLog().Error("platform error on whisper conversation creation",
			zap.String("profileId", profileId), zap.String("name", name), zap.Error(err))
		return "", err
	}
	return conversation.Id, nil
}

func DeleteWhisperConversation(profileId string, name string) error {
	key := WhisperConversationMap(profileId)
	if err := platform.MapRemove(sCtx(), key, name); err != nil {
		sLog().Error("platform error on whisper conversation deletion",
			zap.String("profileId", profileId), zap.String("name", name), zap.Error(err))
		return err
	}
	return nil
}

func EmailProfile(hashedEmail string) (string, error) {
	profileId, err := platform.MapGet(sCtx(), EmailProfileMap, hashedEmail)
	if err != nil {
		sLog().Error("EmailProfileMap failure", zap.String("email", hashedEmail), zap.Error(err))
		return "", err
	}
	return profileId, nil
}

func SetEmailProfile(hashedEmail, profileId string) error {
	if err := platform.MapSet(sCtx(), EmailProfileMap, hashedEmail, profileId); err != nil {
		sLog().Error("EmailProfileMap set failure",
			zap.String("email", hashedEmail), zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	return nil
}
