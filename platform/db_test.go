/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"strings"
	"testing"
)

func TestGetDb(t *testing.T) {
	env := GetConfig()
	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("Test environment %q has an invalid database URL %q", env.Name, env.DbUrl)
		}
	}()
	_, prefix := GetDb()
	if prefix != env.DbProjectPrefix+env.DbEnvPrefix {
		t.Errorf("Test environment %q has prefix %q, should be %q",
			env.Name, prefix, env.DbProjectPrefix+env.DbEnvPrefix)
	}
	if !strings.HasSuffix(prefix, ":"+env.Name[0:1]+":") {
		t.Errorf("Test environment %q has an invalid environment prefix: %q", env.Name, prefix)
	}
}

func TestGetLegacyDb(t *testing.T) {
	env := GetConfig()
	defer func() {
		if err := recover(); err != nil {
			t.Fatalf("Test environment %q has an invalid database URL %q", env.Name, env.DbUrl)
		}
	}()
	_, prefix := GetLegacyDb()
	if prefix != env.LegacyDbProjectPrefix+env.LegacyDbEnvPrefix {
		t.Errorf("Test environment %q has legacy prefix %q, should be %q",
			env.Name, prefix, env.LegacyDbProjectPrefix+env.LegacyDbEnvPrefix)
	}
}
