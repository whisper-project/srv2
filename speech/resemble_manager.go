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

	"github.com/whisper-project/srv2/platform"
	"go.uber.org/zap"
)

type resembleManager struct {
	core            *resembleCore
	generatedSpeech *platform.ExpiringCache
	signals         map[string]chan struct{}
}

var resembleManagerInstance *resembleManager

func GetResembleManager() Manager {
	if resembleManagerInstance == nil {
		resembleManagerInstance = &resembleManager{
			core:            getResembleCore(),
			generatedSpeech: platform.NewExpiringCache("resemble-speech", 300),
			signals:         make(map[string]chan struct{}),
		}
	}
	return resembleManagerInstance
}

func (rm *resembleManager) GenerateSpeech(ctx context.Context, profileId, speechId, text string) {
	rm.signals[speechId] = make(chan struct{})
	go func() {
		defer func() {
			close(rm.signals[speechId])
		}()
		b, err := rm.core.textToSpeech(ctx, profileId, text)
		if err != nil {
			// notest
			sLog().Error("failed to generate speech", zap.String("speechId", speechId), zap.Error(err))
			return
		}
		if err = rm.generatedSpeech.AddBlob(ctx, speechId, b); err != nil {
			// notest
			sLog().Error("failed to cache generated speech", zap.String("speechId", speechId), zap.Error(err))
		}
	}()
}

func (rm *resembleManager) GeneratedSpeech(ctx context.Context, speechId string) (io.Reader, error) {
	signal, ok := rm.signals[speechId]
	if ok {
		select {
		case <-ctx.Done():
			// notest
			sLog().Error("speech retrieval canceled", zap.String("speechId", speechId))
			return nil, ctx.Err()
		case <-signal:
			delete(rm.signals, speechId)
		}
	}
	b, err := rm.generatedSpeech.GetBlob(ctx, speechId)
	if err != nil {
		sLog().Error("failed to retrieve generated speech",
			zap.String("speechId", speechId), zap.Error(err))
		return nil, err
	}
	if b == nil {
		sLog().Info("speech not found", zap.String("speechId", speechId))
		return nil, fmt.Errorf("requested speech has expired: %w", platform.NotFoundError)
	}
	return bytes.NewReader(b), nil
}
