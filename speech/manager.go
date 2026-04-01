/*
 * Copyright 2025-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package speech

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/whisper-project/whisper.server2/platform"
	"github.com/whisper-project/whisper.server2/storage"
	"go.uber.org/zap"
)

func sLog() *zap.Logger {
	return storage.ServerLogger
}

type Generator interface {
	TextToSpeech(ctx context.Context, profileId, text string) ([]byte, error)
}

type Manager struct {
	core             Generator
	generatedSpeech  *platform.ExpiringCache
	generationErrors *platform.ExpiringCache
	signals          map[string]chan struct{}
}

func newManager(name string, ttlSecs uint, core Generator) *Manager {
	speechId := platform.NewId(name + "-speech-")
	errorsId := platform.NewId(name + "-errors-")
	return &Manager{
		core:             core,
		generatedSpeech:  platform.NewExpiringCache(speechId, ttlSecs),
		generationErrors: platform.NewExpiringCache(errorsId, ttlSecs),
		signals:          make(map[string]chan struct{}),
	}
}

func (m *Manager) GenerateSpeech(ctx context.Context, profileId, speechId, text string) {
	m.signals[speechId] = make(chan struct{})
	// mark the speech as sent for generation
	_ = m.generatedSpeech.AddBlob(ctx, speechId, []byte{})
	go func() {
		defer func() {
			close(m.signals[speechId])
		}()
		b, err := m.core.TextToSpeech(ctx, profileId, text)
		if err != nil {
			// notest
			_ = m.generationErrors.AddBlob(ctx, speechId, []byte(err.Error()))
			sLog().Error("failed to generate speech", zap.String("speechId", speechId), zap.Error(err))
			return
		}
		if err = m.generatedSpeech.AddBlob(ctx, speechId, b); err != nil {
			// notest
			sLog().Error("failed to cache generated speech", zap.String("speechId", speechId), zap.Error(err))
		}
	}()
}

func (m *Manager) GeneratedSpeech(ctx context.Context, speechId string) (io.Reader, error) {
	signal, ok := m.signals[speechId]
	if ok {
		select {
		case <-ctx.Done():
			// notest
			sLog().Error("speech retrieval canceled", zap.String("speechId", speechId))
			return nil, ctx.Err()
		case <-signal:
			delete(m.signals, speechId)
		}
	}
	b, err := m.generatedSpeech.GetBlob(ctx, speechId)
	if err != nil {
		sLog().Error("failed to retrieve generated speech",
			zap.String("speechId", speechId), zap.Error(err))
		return nil, err
	}
	if b == nil {
		sLog().Info("speech not found", zap.String("speechId", speechId))
		return nil, fmt.Errorf("requested speech has expired: %w", platform.NotFoundError)
	}
	if len(b) == 0 {
		msg, _ := m.generationErrors.GetBlob(ctx, speechId)
		if msg == nil {
			msg = []byte("unknown error")
		}
		sLog().Info("speech generation failed",
			zap.String("speechId", speechId), zap.String("cause", string(msg)))
		return nil, fmt.Errorf("speech generation failed: %s", string(msg))
	}
	return bytes.NewReader(b), nil
}
