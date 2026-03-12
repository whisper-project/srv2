/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestMakeSha1(t *testing.T) {
	emptySha1Bytes, err := hex.DecodeString("da39a3ee5e6b4b0d3255bfef95601890afd80709")
	if err != nil {
		t.Fatal(err)
	}
	emptySha1Base64 := base64.URLEncoding.EncodeToString(emptySha1Bytes)
	computedSha1Base64 := MakeSha1("")
	if computedSha1Base64 != emptySha1Base64 {
		t.Errorf("computedSha1Base64 should be %q but is %q", emptySha1Base64, computedSha1Base64)
	}
}
