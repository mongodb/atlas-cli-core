// Copyright 2026 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserAgent(t *testing.T) {
	// CURSOR_AGENT is the first marker the SDK checks, so it is detected
	// regardless of any other agent markers set in the environment.
	t.Setenv("CURSOR_AGENT", "1")

	userAgent := UserAgent("1.2.3")

	assert.True(t, strings.HasPrefix(userAgent, AtlasCLI+"/1.2.3 ("), userAgent)
	assert.True(t, strings.HasSuffix(userAgent, " ai-agent/cursor"), userAgent)
}
