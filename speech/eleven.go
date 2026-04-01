/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"

	"github.com/whisper-project/whisper.server2/platform"
	"go.uber.org/zap"
)

var elevenManagerInstance *Manager

func GetElevenManager() *Manager {
	if elevenManagerInstance == nil {
		elevenManagerInstance = newManager("eleven", 300, getElevenCore())
	}
	return elevenManagerInstance
}

type elevenCore struct {
	profileTokens platform.StorableMap
	profileVoices platform.StorableMap
}

var elevenCoreInstance *elevenCore
var elevenDefaultVoice = &elevenVoice{
	VoiceId:   "onwK4e9ZLuTAKqWW03F9",
	VoiceName: "Daniel - Steady Broadcaster",
	ModelId:   "eleven_flash_v2_5",
}

func getElevenCore() *elevenCore {
	if elevenCoreInstance == nil {
		elevenCoreInstance = &elevenCore{
			profileTokens: platform.StorableMap("elevenlabs-profile-tokens"),
			profileVoices: platform.StorableMap("elevenlabs-profile-settings"),
		}
	}
	return elevenCoreInstance
}

type elevenVoice struct {
	VoiceId      string   `json:"voiceId"`
	VoiceName    string   `json:"voiceName"`
	ModelId      string   `json:"modelId"`
	Dictionaries []string `json:"dictionaries"`
}

func (ev *elevenVoice) Marshal() string {
	b, _ := json.Marshal(ev)
	return string(b)
}

func (ev *elevenVoice) Unmarshal(s string) {
	_ = json.Unmarshal([]byte(s), ev)
}

func (ec *elevenCore) getProfileToken(ctx context.Context, profileId string) string {
	token, err := platform.GetMapValue(ctx, ec.profileTokens, profileId)
	if token == "" {
		if err != nil {
			// notest
			sLog().Error("storage failure (get) on ElevenLabs profile token",
				zap.String("profileId", profileId), zap.Error(err))
		}
		return platform.GetConfig().ElevenLabsApiKey
	}
	return token
}

func (ec *elevenCore) registerProfileToken(ctx context.Context, profileId, token string) error {
	if !ec.validateApiToken(ctx, token) {
		sLog().Error("failed to validate the token", zap.String("profileId", profileId))
		return errors.New("failed to validate the apiKey")
	}
	if err := platform.SetMapValue(ctx, ec.profileTokens, profileId, token); err != nil {
		// notest
		sLog().Error("storage failure (set) on ElevenLabs profile token",
			zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	return nil
}

// validateApiToken returns true if the given API key is valid on ElevenLabs
func (ec *elevenCore) validateApiToken(ctx context.Context, token string) bool {
	uri := "https://api.elevenlabs.io/v1/user"
	req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
	if err != nil {
		// notest
		sLog().Error("failed to prepare a request for settings", zap.Error(err))
		return false
	}
	req.Header.Set("xi-api-key", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		sLog().Error("failed to execute a request for settings", zap.Error(err))
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (ec *elevenCore) getProfileVoice(ctx context.Context, profileId string) *elevenVoice {
	s, err := platform.GetMapValue(ctx, ec.profileVoices, profileId)
	if err != nil {
		// notest
		sLog().Error("storage failure (get) on ElevenLabs profile voice",
			zap.String("profileId", profileId), zap.Error(err))
		return elevenDefaultVoice
	}
	var voice elevenVoice
	voice.Unmarshal(s)
	if voice.VoiceId == "" {
		return elevenDefaultVoice
	}
	return &voice
}

func (ec *elevenCore) registerProfileVoice(ctx context.Context, profileId string, voice *elevenVoice) error {
	if err := platform.SetMapValue(ctx, ec.profileVoices, profileId, voice.Marshal()); err != nil {
		// notest
		sLog().Error("storage failure (set) on Resemble profile voice",
			zap.String("profileId", profileId), zap.Error(err))
		return err
	}
	return nil
}

// validateVoiceId returns non-nil voice settings if the voiceId is valid
func (ec *elevenCore) validateVoiceId(ctx context.Context, profileId, voiceId string) (*elevenVoice, error) {
	uri := fmt.Sprintf("https://api.us.elevenlabs.io/v2/voices/%s", voiceId)
	req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
	if err != nil {
		// notest
		sLog().Error("failed to prepare a request for voice info", zap.Error(err))
		return nil, err
	}
	req.Header.Set("xi-api-key", ec.getProfileToken(ctx, profileId))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// notest
		sLog().Error("the voice info request failed", zap.Error(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		sLog().Error("got a bad status on voice info request",
			zap.String("voiceId", voiceId), zap.Int("status_code", resp.StatusCode))
		return nil, nil
	}
	var v elevenVoiceInfoItem
	if err = json.NewDecoder(resp.Body).Decode(&v); err != nil {
		sLog().Error("failed to decode voice info", zap.Error(err))
		return nil, err
	}
	modelId := ec.bestAvailableModelId(v.HighQualityBaseModelIds)
	if modelId == "" {
		// notest
		sLog().Error("no usable model for voice",
			zap.String("voiceId", voiceId), zap.Any("voiceInfo", v))
		return nil, nil
	}
	settings := &elevenVoice{
		VoiceId:   voiceId,
		VoiceName: v.Name,
		ModelId:   modelId,
	}
	return settings, nil
}

// bestAvailableModelId returns the cheapest, highest quality usable model id from the given list.
func (ec *elevenCore) bestAvailableModelId(availableModelIds []string) string {
	usableModelIds := []string{"eleven_flash_v2_5", "eleven_flash_v2", "eleven_turbo_v2_5", "eleven_turbo_v2"}
	for _, id := range availableModelIds {
		if slices.Contains(usableModelIds, id) {
			return id
		}
	}
	return ""
}

type elevenVoiceInfoItem struct {
	VoiceId                 string            `json:"voice_id"`
	Name                    string            `json:"name"`
	Category                string            `json:"category"`
	Labels                  map[string]string `json:"labels"`
	Description             string            `json:"description"`
	PreviewUrl              string            `json:"preview_url"`
	HighQualityBaseModelIds []string          `json:"high_quality_base_model_ids"`
	CollectionIds           []string          `json:"collection_ids"`
	IsOwner                 bool              `json:"is_owner"`
}

type elevenVoiceInfoList struct {
	Voices        []elevenVoiceInfoItem `json:"voices"`
	HasMore       bool                  `json:"has_more"`
	TotalCount    int                   `json:"total_count"`
	NextPageToken string                `json:"next_page_token"`
}

func (ec *elevenCore) listVoices(ctx context.Context, profileId string,
	collectionId, kind, category string) ([]elevenVoiceInfoItem, error) {
	var voices []elevenVoiceInfoItem
	var nextPageToken string
	hasMore := true
	baseUri := "https://api.us.elevenlabs.io/v2/voices?page_size=100"
	if collectionId != "" {
		baseUri += "&collection_id=" + url.QueryEscape(collectionId)
	}
	if kind != "" {
		baseUri += "&voice_type=" + url.QueryEscape(kind)
	}
	if category != "" {
		baseUri += "&category=" + url.QueryEscape(category)
	}
	for hasMore {
		uri := baseUri
		if nextPageToken != "" {
			uri += "&page_token=" + nextPageToken
		}
		req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("xi-api-key", ec.getProfileToken(ctx, profileId))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var v elevenVoiceInfoList
		err = json.NewDecoder(resp.Body).Decode(&v)
		if err != nil {
			return nil, err
		}
		voices = append(voices, v.Voices...)
		hasMore = v.HasMore
		nextPageToken = v.NextPageToken
	}
	return voices, nil
}

type elevenAccountInfo struct {
	Tier                           string `json:"tier"`
	CharacterCount                 int64  `json:"character_count"`
	CharacterLimit                 int64  `json:"character_limit"`
	CanExtendCharacterLimit        bool   `json:"can_extend_character_limit"`
	AllowedToExtendCharacterLimit  bool   `json:"allowed_to_extend_character_limit"`
	VoiceSlotsUsed                 int64  `json:"voice_slots_used"`
	ProfessionalVoiceSlotsUsed     int64  `json:"professional_voice_slots_used"`
	VoiceLimit                     int64  `json:"voice_limit"`
	VoiceAddEditCounter            int64  `json:"voice_add_edit_counter"`
	ProfessionalVoiceLimit         int64  `json:"professional_voice_limit"`
	CanExtendVoiceLimit            bool   `json:"can_extend_voice_limit"`
	CanUseInstantVoiceCloning      bool   `json:"can_use_instant_voice_cloning"`
	CanUseProfessionalVoiceCloning bool   `json:"can_use_professional_voice_cloning"`
	Status                         string `json:"status"`
	HasOpenInvoices                bool   `json:"has_open_invoices"`
	MaxCharacterLimitExtension     int64  `json:"max_character_limit_extension"`
	NextCharacterCountResetUnix    int64  `json:"next_character_count_reset_unix"`
	MaxVoiceAddEdits               int64  `json:"max_voice_add_edits"`
	Currency                       string `json:"currency"`
	BillingPeriod                  string `json:"billing_period"`
	CharacterRefreshPeriod         string `json:"character_refresh_period"`
}

func (ec *elevenCore) getAccountInfo(ctx context.Context, profileId string) (*elevenAccountInfo, error) {
	uri := "https://api.us.elevenlabs.io/v1/user/subscription"
	req, err := http.NewRequestWithContext(ctx, "GET", uri, nil)
	if err != nil {
		sLog().Error("Failed to create ElevenLabs account info request: %v", zap.Error(err))
		return nil, err
	}
	req.Header.Set("xi-api-key", ec.getProfileToken(ctx, profileId))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		sLog().Error("Failed to get ElevenLabs account info: %v",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		break
	default:
		sLog().Error("Unexpected status getting ElevenLabs account info",
			zap.String("profileId", profileId), zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	var info elevenAccountInfo
	if err = json.NewDecoder(resp.Body).Decode(&info); err != nil {
		sLog().Error("Failed to decode account info response: %v",
			zap.String("profileId", profileId), zap.Error(err))
		return nil, err
	}
	return &info, nil
}

func (ec *elevenCore) TextToSpeech(ctx context.Context, profileId, text string) ([]byte, error) {
	token := ec.getProfileToken(ctx, profileId)
	voice := ec.getProfileVoice(ctx, profileId)
	bodyDict := map[string]any{
		"text":     text,
		"model_id": voice.ModelId,
	}
	if len(voice.Dictionaries) > 0 {
		var dicts []map[string]string
		for _, d := range voice.Dictionaries {
			dicts = append(dicts, map[string]string{"pronunciation_dictionary_id": d})
		}
		bodyDict["pronunciation_dictionary_locators"] = dicts
	}
	body, err := json.Marshal(bodyDict)
	if err != nil {
		// notest
		sLog().Error("Failed to marshal text-to-speech request body", zap.Error(err))
		return nil, err
	}
	uri := "https://api.us.elevenlabs.io/v1/text-to-speech/" + voice.VoiceId + "/stream"
	req, err := http.NewRequestWithContext(ctx, "POST", uri, bytes.NewReader(body))
	if err != nil {
		// notest
		sLog().Error("Failed to create a text-to-speech request", zap.Error(err))
		return nil, err
	}
	req.Header.Set("xi-api-key", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// notest
		sLog().Error("Failed to perform a TTS request", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// notest
		sLog().Error("Unexpected TTS response status",
			zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return nil, fmt.Errorf("unexpected TTS response status: %d", resp.StatusCode)
	}
	return body, nil
}
