/*
 * Copyright 2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package speech

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/whisper-project/srv2/platform"
	"go.uber.org/zap"
)

type resembleCore struct {
	profileTokens platform.StorableMap
	profileVoices platform.StorableMap
}

var resembleCoreInstance *resembleCore

func getResembleCore() *resembleCore {
	if resembleCoreInstance == nil {
		resembleCoreInstance = &resembleCore{
			profileTokens: "resemble-profile-tokens",
			profileVoices: "resemble-profile-voices",
		}
	}
	return resembleCoreInstance
}

func (rc *resembleCore) registerProfileToken(ctx context.Context, profileId, token string) error {
	if !rc.validateApiToken(ctx, token) {
		sLog().Error("failed to validate the token", zap.String("profileId", profileId))
		return errors.New("failed to validate the token")
	}
	if err := platform.SetMapValue(ctx, rc.profileTokens, profileId, token); err != nil {
		// notest
		sLog().Error("storage failure (set) on Resemble profile token",
			zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	return nil
}

func (rc *resembleCore) getProfileToken(ctx context.Context, profileId string) string {
	token, err := platform.GetMapValue(ctx, rc.profileTokens, profileId)
	if token == "" {
		if err != nil {
			// notest
			sLog().Error("storage failure (get) on Resemble profile token",
				zap.String("profileId", profileId), zap.Error(err))
		}
		return platform.GetConfig().ResembleToken
	}
	return token
}

func (rc *resembleCore) registerProfileVoice(ctx context.Context, profileId string, voice *resembleVoice) error {
	if err := platform.SetMapValue(ctx, rc.profileVoices, profileId, voice.Marshal()); err != nil {
		// notest
		sLog().Error("storage failure (set) on Resemble profile voice",
			zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	return nil
}

func (rc *resembleCore) getProfileVoice(ctx context.Context, profileId string) resembleVoice {
	s, err := platform.GetMapValue(ctx, rc.profileVoices, profileId)
	if err != nil {
		// notest
		sLog().Error("storage failure (get) on Resemble profile voice",
			zap.String("profileId", profileId), zap.Error(err))
		return resembleDefaultVoiceItem
	}
	var voice resembleVoice
	voice.Unmarshal(s)
	if voice.Uuid == "" {
		return resembleDefaultVoiceItem
	}
	return voice
}

func (rc *resembleCore) validateApiToken(ctx context.Context, token string) bool {
	endpoint := "https://app.resemble.ai/api/v2/account"
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		// notest
		sLog().Error("failed to prepare a request for voice info", zap.Error(err))
		return false
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// notest
		sLog().Error("the voice info request failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// notest
		sLog().Error("bad status on the account request", zap.Int("status", resp.StatusCode))
		return false
	}
	var response resembleGenericResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		// notest
		sLog().Error("failed to decode account response", zap.Error(err))
		return false
	}
	return response.Success
}

func (rc *resembleCore) listVoices(ctx context.Context, profileId string) ([]resembleVoice, error) {
	const endpoint = "https://app.resemble.ai/api/v2/voices"
	const query = "?page=1&page_size=500"
	token := rc.getProfileToken(ctx, profileId)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+query, nil)
	if err != nil {
		sLog().Error("failed to prepare a request to list voices",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, fmt.Errorf("failed to list voices: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// notest
		sLog().Error("failed to list voices",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, fmt.Errorf("failed to list voices: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// notest
		sLog().Error("failed to list voices",
			zap.String("profileId", profileId), zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("failed to list voices: %s", resp.Status)
	}
	var response resembleListVoicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		// notest
		sLog().Error("failed to decode resemble voices",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, fmt.Errorf("failed to decode resemble voices: %w", err)
	}
	if !response.Success {
		// notest
		sLog().Error("got a non-success response to the list voices request",
			zap.String("profileId", profileId))
		return nil, fmt.Errorf("failed to list voices: %s", resp.Status)
	}
	items := response.Items
	return items, nil
}

func (rc *resembleCore) textToSpeech(ctx context.Context, profileId, text string) ([]byte, error) {
	const endpoint = "https://f.cluster.resemble.ai/synthesize"
	token := rc.getProfileToken(context.Background(), profileId)
	voice := rc.getProfileVoice(context.Background(), profileId)
	body := resembleTtsRequest{voice.Uuid, text, "chatterbox-turbo", "mp3"}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body.Marshal()))
	if err != nil {
		sLog().Error("failed to create a TTS request", zap.Error(err))
		return nil, fmt.Errorf("failure during text-to-speech: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// notest
		sLog().Error("failed to perform a TTS request", zap.Error(err))
		return nil, fmt.Errorf("failure during text-to-speech: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// notest
		sLog().Error("failed to perform a TTS request", zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("failure during text-to-speech: %s", resp.Status)
	}
	var response resembleTtsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		// notest
		sLog().Error("failed to decode TTS response", zap.Error(err))
		return nil, fmt.Errorf("failure during text-to-speech: %w", err)
	}
	if !response.Success {
		// notest
		sLog().Error("TTS request got a non-success response",
			zap.String("profileId", profileId), zap.String("text", text))
		return nil, fmt.Errorf("TTS request failed")
	}
	return response.extractAudio()
}

type resembleGenericResponse struct {
	Success bool `json:"success"`
}

type resembleListVoicesResponse struct {
	Success bool            `json:"success"`
	Items   []resembleVoice `json:"items"`
}

type resembleVoice struct {
	Uuid string `json:"uuid"`
	Name string `json:"name"`
}

func (r *resembleVoice) Marshal() string {
	b, _ := json.Marshal(r)
	return string(b)
}

func (r *resembleVoice) Unmarshal(s string) {
	_ = json.Unmarshal([]byte(s), r)
}

var resembleDefaultVoiceItem = resembleVoice{
	Uuid: "55592656",
	Name: "Ember",
}

type resembleTtsRequest struct {
	Uuid   string `json:"voice_uuid"`
	Text   string `json:"data"`
	Model  string `json:"model"`
	Format string `json:"output_format"`
}

func (rb *resembleTtsRequest) Marshal() []byte {
	b, _ := json.Marshal(rb)
	return b
}

type resembleTtsResponse struct {
	Success  bool    `json:"success"`
	Audio    string  `json:"audio_content"`
	Duration float32 `json:"duration"`
}

func (rb *resembleTtsResponse) extractAudio() ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(rb.Audio)
	if err != nil {
		sLog().Error("failed to decode base64 audio", zap.Error(err))
		return nil, fmt.Errorf("failed to decode base64 audio: %w", err)
	}
	return b, nil
}
