/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

func TestS3PutGetListDeleteEncryptedBlob(t *testing.T) {
	env := GetConfig()
	root := env.AwsRootPath
	if root == "" {
		t.Skip("Skipping S3 test because no AWS root path has been set")
	}
	parentPath := env.Name + "/" + "testing"
	blobName := NewId("test-blob-")
	content := "This is a test. This is only a test."
	inStream := strings.NewReader(content)
	err := S3PutEncryptedBlob(context.Background(), parentPath, blobName, inStream)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = S3DeleteBlob(context.Background(), parentPath, blobName)
		if err != nil {
			t.Fatal(err)
		}
	}()
	b := bytes.Buffer{}
	err = S3GetEncryptedBlob(context.Background(), parentPath, blobName, &b)
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != content {
		t.Errorf("Retrieved content does not match original content: %q != %q", b.String(), content)
	}
	blobs, err := S3ListBlobs(context.Background(), parentPath)
	if err != nil {
		t.Fatal(err)
	}
	count1 := len(blobs)
	if count1 == 0 {
		t.Errorf("Expected at least 1 blob, got none")
	}
	if len(blobs[0]) == 0 {
		t.Errorf("Expected non-empty blob names, got %q", blobs[0])
	}
	if !slices.Contains(blobs, blobName) {
		t.Errorf("Can't find %q in: %q", blobName, blobs)
	}
	err = S3DeleteBlob(context.Background(), parentPath, blobName)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err = S3ListBlobs(context.Background(), parentPath)
	if err != nil {
		t.Fatal(err)
	}
	count2 := len(blobs)
	if count2 != count1-1 {
		t.Errorf("Expected %d blobs, got %d", count1-1, count2)
	}
	if slices.Contains(blobs, blobName) {
		t.Errorf("Found %q (after delete) in: %q", blobName, blobs)
	}
}
