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

	"github.com/whisper-project/whisper.server2/platform"
)

var testEc = &elevenCore{
	profileTokens: platform.StorableMap(platform.NewId("test-elevenlabs-profile-tokens-")),
	profileVoices: platform.StorableMap(platform.NewId("test-elevenlabs-profile-voices-")),
}

func TestGetElevenCore(t *testing.T) {
	_ = getElevenCore()
}

//goland:noinspection DuplicatedCode
func TestElevenProfileToken(t *testing.T) {
	env := platform.GetConfig()
	goodToken := env.ElevenLabsApiKey
	if goodToken == "" {
		t.Skip("No ElevenLabs API key in the environment")
		return
	}
	// change the environment so we can test error cases
	replacement := "replacement-token"
	env.ElevenLabsApiKey = replacement
	defer func() { env.ElevenLabsApiKey = goodToken }()
	// now do the testing
	ctx := context.Background()
	profileId := platform.NewId("test-profile-")
	if err := testEc.registerProfileToken(ctx, profileId, "invalid-token"); err == nil {
		t.Error("expected an error registering an invalid token")
	}
	if tok := testEc.getProfileToken(ctx, profileId); tok != replacement {
		t.Errorf("expected token to be %s, got %s", replacement, tok)
	}
	if err := testEc.registerProfileToken(ctx, profileId, goodToken); err != nil {
		t.Fatalf("failed to register a good token: %v", err)
	}
	if tok := testEc.getProfileToken(ctx, profileId); tok != goodToken {
		t.Errorf("expected token to be %s, got %s", goodToken, tok)
	}
}

func TestElevenProfileVoice(t *testing.T) {
	env := platform.GetConfig()
	goodToken := env.ElevenLabsApiKey
	if goodToken == "" {
		t.Skip("resemble token not set")
		return
	}
	//goland:noinspection SpellCheckingInspection
	testVoice := &elevenVoice{"e0bmllAjevQ9bvU71kzO", "Lisa", "eleven_flash_v2_5", nil}
	profileId := platform.NewId("test-profile-")
	if v := testEc.getProfileVoice(context.Background(), profileId); v != elevenDefaultVoice {
		t.Errorf("expected default voice, got %+v", *v)
	}
	if err := testEc.registerProfileVoice(context.Background(), profileId, testVoice); err != nil {
		t.Fatalf("failed to register a new voice: %v", err)
	}
	if v := testEc.getProfileVoice(context.Background(), profileId); v.VoiceId != testVoice.VoiceId {
		t.Errorf("expected voice to be %+v, got %+v", *testVoice, *v)
	}
}

func TestElevenListVoices(t *testing.T) {
	if platform.GetConfig().ElevenLabsApiKey == "" {
		t.Skip("No ElevenLabs API key in the environment")
		return
	}
	ctx := context.Background()
	profileId := platform.NewId("test-profile-")
	voices, err := testEc.listVoices(ctx, profileId, "", "default", "")
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(voices) != 21 {
		t.Errorf("expected 21 voices, got %d", len(voices))
	}
	t.Logf("Found %d voices:", len(voices))
	voiceIds := make(map[string]string, len(voices))
	for _, v := range voices {
		voiceIds[v.Name] = v.VoiceId
	}
	names := slices.Sorted(maps.Keys(voiceIds))
	for i, name := range names {
		t.Logf("  %3d: %s [id: %s]", i, name, voiceIds[name])
	}
}

func TestElevenTtsRequest(t *testing.T) {
	if platform.GetConfig().ElevenLabsApiKey == "" {
		t.Skip("resemble token not set")
		return
	}
	ctx := context.Background()
	profileId := platform.NewId("test-profile-")
	b, err := testEc.TextToSpeech(ctx, profileId, "When I first tried it, I didn't think I'd like it.")
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

func TestGetElevenManager(t *testing.T) {
	m1 := GetElevenManager()
	m2 := GetElevenManager()
	if m1 != m2 {
		t.Errorf("Expected same manager instance, got different instances")
	}
}

func TestElevenGenerateRetrieveSpeec(t *testing.T) {
	testEm := newManager("test-eleven", 2, testEc)
	GenerateRetrieveSpeechTester(t, testEm)
}
