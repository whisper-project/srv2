/*
 * Copyright 2025-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package speech

import "io"

type Manager interface {
	GenerateSpeech(text string) (string, error)
	GeneratedSpeech(id string) (io.Reader, error)
}
