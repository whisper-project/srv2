/*
 * Copyright 2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package speech

import (
	"context"
	"maps"
	"os"
	"slices"
	"testing"

	"github.com/whisper-project/srv2/platform"
)

var testrc = &resembleCore{
	profileTokens: platform.StorableMap(platform.NewId("test-resemble-profile-tokens-")),
	profileVoices: platform.StorableMap(platform.NewId("test-resemble-profile-voices-")),
}

func TestGetResembleCore(t *testing.T) {
	_ = getResembleCore()
}

func TestResembleProfileToken(t *testing.T) {
	env := platform.GetConfig()
	goodToken := env.ResembleToken
	if goodToken == "" {
		t.Skip("resemble token not set")
		return
	}
	// change the environment so we can test error cases
	replacement := "replacement-token"
	env.ResembleToken = replacement
	defer func() { env.ResembleToken = goodToken }()
	// now do the testing
	ctx := context.Background()
	profileId := platform.NewId("test-profile-")
	if err := testrc.registerProfileToken(ctx, profileId, "invalid-token"); err == nil {
		t.Error("expected error registering an invalid token")
	}
	if tok := testrc.getProfileToken(ctx, profileId); tok != replacement {
		t.Errorf("expected token to be %s, got %s", replacement, tok)
	}
	if err := testrc.registerProfileToken(ctx, profileId, goodToken); err != nil {
		t.Fatalf("failed to register a good token: %v", err)
	}
	if tok := testrc.getProfileToken(ctx, profileId); tok != goodToken {
		t.Errorf("expected token to be %s, got %s", goodToken, tok)
	}
}

func TestResembleProfileVoice(t *testing.T) {
	env := platform.GetConfig()
	goodToken := env.ResembleToken
	if goodToken == "" {
		t.Skip("resemble token not set")
		return
	}
	testVoice := resembleVoice{"38a0b764", "Aaron"}
	profileId := platform.NewId("test-profile-")
	if v := testrc.getProfileVoice(context.Background(), profileId); v != resembleDefaultVoiceItem {
		t.Errorf("expected default voice, got %s", v.Uuid)
	}
	if err := testrc.registerProfileVoice(context.Background(), profileId, &testVoice); err != nil {
		t.Fatalf("failed to register a new voice: %v", err)
	}
	if v := testrc.getProfileVoice(context.Background(), profileId); v != testVoice {
		t.Errorf("expected voice to be %v, got %v", testVoice, v)
	}
}

func TestResembleListVoices(t *testing.T) {
	if platform.GetConfig().ResembleToken == "" {
		t.Skip("resemble token not set")
		return
	}
	ctx := context.Background()
	profileId := platform.NewId("test-profile-")
	voices, err := testrc.listVoices(ctx, profileId)
	if err != nil {
		t.Fatal(err.Error())
	}
	t.Logf("Found %d voices:", len(voices))
	voiceIds := make(map[string]string, len(voices))
	for _, v := range voices {
		voiceIds[v.Name] = v.Uuid
	}
	names := slices.Sorted(maps.Keys(voiceIds))
	for i, name := range names {
		t.Logf("  %3d: %s [id: %s]", i, name, voiceIds[name])
	}
}

func TestResembleTtsRequest(t *testing.T) {
	if platform.GetConfig().ResembleToken == "" {
		t.Skip("resemble token not set")
		return
	}
	ctx := context.Background()
	profileId := platform.NewId("test-profile-")
	b, err := testrc.textToSpeech(ctx, profileId, "When I first tried it, I didn't think I'd like it.")
	if err != nil {
		t.Fatalf("failed to generate TTS: %v", err)
	}
	t.Logf("Generated TTS audio with %d bytes", len(b))
	f, err := os.CreateTemp("", "test-tts-*.mp3")
	if err != nil {
		t.Fatalf("failed to create a temp file: %v", err)
	}
	t.Logf("Writing TTS audio to %s", f.Name())
	defer f.Close()
	_, err = f.Write(b)
	if err != nil {
		t.Fatalf("failed to write TTS audio to file: %v", err)
	}
}
