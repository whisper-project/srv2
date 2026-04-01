/*
 * Copyright 2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package speech

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/whisper-project/whisper.server2/platform"
)

func GenerateRetrieveSpeechTester(t *testing.T, m *Manager) {
	ctx := context.Background()
	profileId := platform.NewId("test-profile-")
	speechId1 := platform.NewId("test-speech-")
	text1 := "This is the last thing I was expecting."
	speechId2 := platform.NewId("test-speech-")
	text2 := "Really? It was the very first thing I was expecting."
	start1 := time.Now()
	m.GenerateSpeech(ctx, profileId, speechId1, text1)
	_, err := m.GeneratedSpeech(ctx, speechId1)
	end1 := time.Now()
	elapsed1 := time.Since(start1)
	t.Logf("Generated speech %s in %s", speechId1, elapsed1)
	if err != nil {
		t.Fatalf("Failed to retrieve speech %s: %v", speechId1, err)
	}
	start2 := time.Now()
	m.GenerateSpeech(ctx, profileId, speechId2, text2)
	_, err = m.GeneratedSpeech(ctx, speechId1)
	t.Logf("Retrieved speech %s again in %s", speechId1, time.Since(end1))
	if err != nil {
		t.Fatalf("Failed to retrieve speech %s again: %v", speechId1, err)
	}
	_, err = m.GeneratedSpeech(ctx, speechId2)
	elapsed2 := time.Since(start2)
	t.Logf("Generated speech %s in %s", speechId2, elapsed2)
	if err != nil {
		t.Fatalf("Failed to retrieve speech %s: %v", speechId2, err)
	}
	time.Sleep(3 * time.Second)
	if _, err = m.GeneratedSpeech(ctx, speechId1); err == nil {
		t.Errorf("Speech %s should have expired but didn't", speechId1)
	} else if !errors.Is(err, platform.NotFoundError) {
		t.Errorf("Expected NotFoundError, got %v", err)
	}
	if _, err = m.GeneratedSpeech(ctx, speechId2); err == nil {
		t.Errorf("Speech %s should have expired but didn't", speechId2)
	} else if !errors.Is(err, platform.NotFoundError) {
		t.Errorf("Expected NotFoundError, got %v", err)
	}
}
