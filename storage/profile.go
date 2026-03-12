/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"bytes"
	"encoding/gob"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/whisper-project/srv2/platform"
)

// A Profile represents a user.
//
// We keep a hash of the user's email to avoid PII
// while still having a way of validating the user's email against their profile
// when they want to recover their secret.
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

func (p *Profile) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(p); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (p *Profile) FromRedis(b []byte) error {
	*p = Profile{} // dump old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(p)
}

// GetProfile returns the profile with the given id.
func GetProfile(id string) (*Profile, error) {
	p := &Profile{Id: id}
	if err := platform.FetchObject(sCtx(), p); err != nil {
		sLog().Error("storage failure (load) on Profile",
			zap.String("id", id), zap.Error(err))
		return nil, err
	}
	return p, nil
}

// SaveProfile saves the given profile p.
func SaveProfile(p *Profile) error {
	if err := platform.StoreObject(sCtx(), p); err != nil {
		sLog().Error("storage failure (save) on Profile",
			zap.String("id", p.Id), zap.Error(err))
		return err
	}
	return nil
}

// DeleteProfile deletes the profile with the given id.
func DeleteProfile(id string) error {
	if err := platform.DeleteStorage(sCtx(), &Profile{Id: id}); err != nil {
		sLog().Error("storage failure (delete) on Profile",
			zap.String("id", id), zap.Error(err))
		return err
	}
	return nil
}

// The EmailProfileMap maps each user's hashed email to that user's profile id.
var EmailProfileMap = platform.StorableMap("email-profile-map")

// GetEmailProfile returns the profile id for the given hashed email.
func GetEmailProfile(hashedEmail string) (string, error) {
	profileId, err := platform.GetMapValue(sCtx(), EmailProfileMap, hashedEmail)
	if err != nil {
		sLog().Error("EmailProfileMap failure", zap.String("email", hashedEmail), zap.Error(err))
		return "", err
	}
	return profileId, nil
}

// SetEmailProfile maps the given hashed email to the given profile id.
func SetEmailProfile(hashedEmail, profileId string) error {
	if err := platform.SetMapValue(sCtx(), EmailProfileMap, hashedEmail, profileId); err != nil {
		sLog().Error("EmailProfileMap set failure",
			zap.String("email", hashedEmail), zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	return nil
}

// RemoveEmailProfile removes the mapping for the given hashed email.
func RemoveEmailProfile(hashedEmail string) error {
	if err := platform.MapRemoveKey(sCtx(), EmailProfileMap, hashedEmail); err != nil {
		sLog().Error("EmailProfileMap remove failure",
			zap.String("email", hashedEmail), zap.Error(err))
		return err
	}
	return nil
}

// The OwnedConversationMap of a profile id maps, for each conversation owned
// by the user with that profile id, the conversation's name to its id.
type OwnedConversationMap string

func (p OwnedConversationMap) StoragePrefix() string {
	return "owned-conversations:"
}

func (p OwnedConversationMap) StorageId() string {
	return string(p)
}

// AddOwnedConversation adds the given conversation to its owner's name->id map.
func AddOwnedConversation(c *Conversation) error {
	key := OwnedConversationMap(c.Owner)
	if err := platform.SetMapValue(sCtx(), key, c.Name, c.Id); err != nil {
		sLog().Error("storage failure (add) on OwnedConversationMap",
			zap.String("profileId", c.Owner), zap.String("name", c.Name), zap.Error(err))
		return err
	}
	return nil
}

// RemoveOwnedConversation removes the given conversation from its owner's name->id map
func RemoveOwnedConversation(c *Conversation) error {
	key := OwnedConversationMap(c.Owner)
	if err := platform.MapRemoveKey(sCtx(), key, c.Name); err != nil {
		sLog().Error("storage failure (remove) on OwnedConversationMap",
			zap.String("profileId", c.Owner), zap.String("name", c.Name), zap.Error(err))
		return err
	}
	return nil
}

// GetOwnedConversationIdByName retrieves the conversation id for a given profile id and conversation name.
//
// If the profile doesn't own the conversation, an empty conversation id (and no error) is returned.
func GetOwnedConversationIdByName(profileId string, name string) (string, error) {
	key := OwnedConversationMap(profileId)
	conversationId, err := platform.GetMapValue(sCtx(), key, name)
	if err != nil {
		sLog().Error("storage failure (lookup) on OwnedConversationMap",
			zap.String("profileId", profileId), zap.String("name", name), zap.Error(err))
		return "", err
	}
	return conversationId, nil
}

// GetOwnedConversationsNameToIdMap returns a name->id map of all conversations owned by the given profile id.
func GetOwnedConversationsNameToIdMap(profileId string) (map[string]string, error) {
	key := OwnedConversationMap(profileId)
	cMap, err := platform.GetMapAll(sCtx(), key)
	if err != nil {
		sLog().Error("storage failure (lookup) on OwnedConversationMap",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, err
	}
	return cMap, nil
}

// CreateNewUser creates a new user based on info received from a client.
//
// New users get a profile and single conversation. The client is marked as launched against the profile.
// Everything is saved to storage.
//
// It's an error to call this if there is already a profile for the given hashedEmail.
func CreateNewUser(clientId, clientType, name, hashedEmail string) (*Profile, error) {
	id := platform.NewId("profile-")
	p := &Profile{
		Id:        id,
		Name:      name,
		EmailHash: hashedEmail,
		Secret:    uuid.NewString(),
	}
	if err := SaveProfile(p); err != nil {
		return nil, err
	}
	cleanup := false
	defer func() {
		if !cleanup {
			return
		}
		_ = RemoveEmailProfile(hashedEmail)
		_ = DeleteProfile(id)
	}()
	if err := SetEmailProfile(hashedEmail, id); err != nil {
		cleanup = true
		return nil, err
	}
	if _, err := CreateNewOwnedConversation(id, "Conversation 1"); err != nil {
		cleanup = true
		return nil, err
	}
	ObserveClientLaunch(clientType, clientId, p.Id)
	return p, nil
}

// DeleteExistingUser deletes the user with the given profileId.
//
// Before deleting the user, it deletes all the user's owned conversations,
// and it removes the user's mapping from the EmailProfileMap.
//
// Errors are logged but don't stop the operation, and any errors encountered
// are returned in a slice after everything is done.
//
// Asking to delete a non-existent user is a no-op.
func DeleteExistingUser(profileId string) (errs []error) {
	p, _ := GetProfile(profileId)
	if p == nil {
		// there's nothing to delete
		return
	}
	if err := RemoveEmailProfile(p.EmailHash); err != nil {
		errs = append(errs, err)
	}
	cIds, err := GetOwnedConversationsNameToIdMap(profileId)
	if err != nil {
		errs = append(errs, err)
	}
	for _, cId := range cIds {
		c, err := GetConversation(cId)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := RemoveOwnedConversation(c); err != nil {
			errs = append(errs, err)
		}
		if err := DeleteConversation(cId); err != nil {
			errs = append(errs, err)
		}
	}
	if err := DeleteProfile(profileId); err != nil {
		errs = append(errs, err)
	}
	return
}

// CreateNewOwnedConversation creates a new conversation with the given name
// for the user with the given profileId, returning the new conversation.
//
// It saves the conversation to storage and adds it to the user's owned conversation map.
//
// It's an error to call this if there is already a conversation with the given name owned by the given profileId.
func CreateNewOwnedConversation(profileId string, name string) (*Conversation, error) {
	conversation := NewConversation(profileId, name)
	if err := SaveConversation(conversation); err != nil {
		return nil, err
	}
	cleanup := false
	defer func() {
		if !cleanup {
			return
		}
		_ = DeleteConversation(conversation.Id)
	}()
	if err := AddOwnedConversation(conversation); err != nil {
		cleanup = true
		return nil, err
	}
	return conversation, nil
}

// DeleteOwnedConversation deletes the conversation with the given name owned by the user with the given profileId.
//
// It removes the conversation from the user's owned conversation map and deletes the conversation from storage.
// If the user doesn't own the conversation, this is a no-op.
func DeleteOwnedConversation(profileId string, name string) error {
	convoId, err := GetOwnedConversationIdByName(profileId, name)
	if err != nil {
		return err
	}
	if convoId == "" {
		return nil
	}
	c, err := GetConversation(convoId)
	if err != nil {
		return err
	}
	if err := RemoveOwnedConversation(c); err != nil {
		return err
	}
	if c.Owner != profileId {
		// shouldn't happen, but if it does, we relink the conversation to the correct owner
		sLog().Error("attempted to delete conversation linked to a different profile",
			zap.String("profileId", profileId), zap.Any("conversation", c))
		if err := AddOwnedConversation(c); err != nil {
			sLog().Error("failed to relink conversation to the correct profile",
				zap.Any("conversation", c), zap.Error(err))
			return err
		}
		return nil
	}
	return DeleteConversation(convoId)
}
