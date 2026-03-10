/*
 * Copyright 2025-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package speech

import (
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

type mockManager struct {
	textCache            map[string]string
	inverseTextCache     map[string]string
	generatedSpeechPaths map[string]string
}

func (m *mockManager) GenerateSpeech(text string) (string, error) {
	id := uuid.NewString()
	cacheKey := strings.ToLower(strings.TrimSpace(text))
	m.textCache[cacheKey] = id
	m.inverseTextCache[id] = text
	// mock doesn't actually generate speech yet
	return id, nil
}

func (m *mockManager) GeneratedSpeech(id string) (io.Reader, error) {
	_, ok := m.generatedSpeechPaths[id]
	if !ok {
		return nil, fmt.Errorf("no speech found for id %s", id)
	}
	return nil, fmt.Errorf("not implemented")
}

func NewMockManager() Manager {
	return &mockManager{
		textCache:            make(map[string]string),
		inverseTextCache:     make(map[string]string),
		generatedSpeechPaths: make(map[string]string),
	}
}
