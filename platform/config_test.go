/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"testing"
)

func TestGetConfig(t *testing.T) {
	name := GetConfig().Name
	fmt.Printf("Testing configuration is %q\n", name)
	if _, ok := knownEnvironments[name]; !ok {
		t.Fatalf("Initial configuration %q is not a known environment", name)
	}
}

func TestFindEnvFile(t *testing.T) {
	if os.Getenv("DOTENV_KEY") != "" {
		t.Skip("Skipping search for environment files because DOTENV_KEY is set")
	}
	files := findEnvFiles()
	if len(files) != 0 {
		t.Errorf("Found environment files when none were specified: %q", files)
	}
	names := slices.Collect(maps.Keys(knownEnvironments))
	files = findEnvFiles(names...)
	if len(files) != len(names) {
		t.Errorf("Expected %d environment files, got %d: %v", len(names), len(files), files)
	}
	files = findEnvFiles("foo", "bar", "baz")
	if len(files) != 0 {
		t.Errorf("Found environment files when invalid names were specified: %v", files)
	}
}
