/*
 * Copyright 2025-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package speech

import (
	"context"
	"io"

	"github.com/whisper-project/srv2/storage"
	"go.uber.org/zap"
)

func sLog() *zap.Logger {
	return storage.ServerLogger
}

type Manager interface {
	GenerateSpeech(ctx context.Context, profileId, speechId, text string)
	GeneratedSpeech(ctx context.Context, speechId string) (io.Reader, error)
}
