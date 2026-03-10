/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"bytes"
	"encoding/gob"
	"time"

	"go.uber.org/zap"

	"github.com/whisper-project/server.golang/platform"
)

// ActivityData tracks the attributes of a client
// at the time of its most recent launch, and when
// it last made a server request in that launch.
//
// Times are in epoch millis
type ActivityData struct {
	ClientId     string
	ClientType   string
	ProfileId    string
	LaunchTime   int64
	LastActivity string
	LastTime     int64
}

func (a *ActivityData) StoragePrefix() string {
	return "activity-data:"
}

func (a *ActivityData) StorageId() string {
	if a == nil {
		return ""
	}
	return a.ClientId
}

func (a *ActivityData) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (a *ActivityData) FromRedis(b []byte) error {
	*a = ActivityData{} // dump old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(a)
}

// GetClientActivity returns the last activity recorded for a given clientId.
func GetClientActivity(clientId string) (*ActivityData, error) {
	a := &ActivityData{ClientId: clientId}
	if err := platform.LoadObject(sCtx(), a); err != nil {
		sLog().Error("storage failure (load) on ActivityData",
			zap.String("clientId", clientId), zap.Error(err))
		return nil, err
	}
	return a, nil
}

// SaveClientActivity saves the activity data for a given clientId.
func SaveClientActivity(a *ActivityData) error {
	if err := platform.SaveObject(sCtx(), a); err != nil {
		sLog().Error("storage failure (save) on ActivityData",
			zap.String("clientId", a.ClientId), zap.Error(err))
		return err
	}
	return nil
}

// DeleteClientActivity deletes the activity data for a given clientId.
func DeleteClientActivity(clientId string) error {
	if err := platform.DeleteStorage(sCtx(), &ActivityData{ClientId: clientId}); err != nil {
		sLog().Error("storage failure (delete) on ActivityData",
			zap.String("clientId", clientId), zap.Error(err))
		return err
	}
	return nil
}

// ObserveClientLaunch records a client launch
//
// Failures are logged but not returned because they don't affect the client.
func ObserveClientLaunch(clientType, clientId, profileId string) {
	now := time.Now().UnixMilli()
	a := &ActivityData{
		ClientType:   clientType,
		ClientId:     clientId,
		ProfileId:    profileId,
		LaunchTime:   now,
		LastActivity: "launch",
		LastTime:     now,
	}
	_ = SaveClientActivity(a)
}

// ObserveClientActivity records the last request received
// from an already-launched client.
//
// Failures are logged but not returned because they don't affect the client.
func ObserveClientActivity(clientId string, activity string) {
	now := time.Now().UnixMilli()
	a, err := GetClientActivity(clientId)
	if err != nil {
		return
	}
	a.LastActivity = activity
	a.LastTime = now
	_ = SaveClientActivity(a)
}
