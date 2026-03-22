/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"errors"
	"testing"
	"time"

	"github.com/whisper-project/whisper.server2/platform"
)

func TestActivityDataInterface(t *testing.T) {
	clientId := platform.NewId("test-client-")
	i := &ActivityData{
		ClientId:     clientId,
		ClientType:   "test",
		ProfileId:    "test-profile",
		LaunchTime:   3,
		LastActivity: "test-activity",
		LastTime:     4,
	}
	var o ActivityData
	if errs := platform.RedisKeyTester(i, "activity-data:", clientId); len(errs) > 0 {
		for _, err := range errs {
			t.Error(err)
		}
	}
	if errs := platform.RedisValueTester(i, &o, func(l, r *ActivityData) bool { return *l == *r }); len(errs) > 0 {
		for _, err := range errs {
			t.Error(err)
		}
	}
}

func TestClientActivityMethods(t *testing.T) {
	clientId := platform.NewId("test-client-")
	profileId := platform.NewId("test-profile-")
	now := time.Now().UnixMilli()
	ObserveClientLaunch("test", clientId, profileId)
	a1, err := GetClientActivity(clientId)
	if err != nil {
		t.Fatalf("GetClientActivity failed: %v", err)
	}
	if a1.ClientType != "test" {
		t.Errorf("Got the wrong client type. Got %s, Want %s", a1.ClientType, "test")
	}
	if a1.ProfileId != profileId {
		t.Errorf("Got the wrong profile id. Got %s, Want %s", a1.ProfileId, profileId)
	}
	if a1.LaunchTime < now {
		t.Errorf("Got an early start. Got %v, Want no earlier than %v", a1.LaunchTime, now)
	}
	if a1.LastActivity != "launch" {
		t.Errorf("Got the wrong last activity. Got %s, Want %s", a1.LastActivity, "launch")
	}
	if a1.LastTime != a1.LaunchTime {
		t.Errorf("Got the wrong last time. Got %v, Want %v", a1.LastTime, a1.LaunchTime)
	}
	ObserveClientActivity(clientId, "shutdown")
	a2, err := GetClientActivity(clientId)
	if err != nil {
		t.Fatalf("GetClientActivity failed: %v", err)
	}
	if a2.LastActivity != "shutdown" {
		t.Errorf("Got the wrong last activity. Got %s, Want %s", a2.LastActivity, "shutdown")
	}
	if a2.LastTime <= a1.LastTime {
		t.Errorf("Got the wrong last time. Got %v, Want no earlier than %v", a2.LastTime, a1.LastTime)
	}
	if DeleteClientActivity(clientId) != nil {
		t.Errorf("DeleteClientActivity failed: %v", err)
	}
	_, err = GetClientActivity(clientId)
	if !errors.Is(err, platform.NotFoundError) {
		t.Errorf("Got the wrong error. Got %v, Want %v", err, "not found")
	}
}
