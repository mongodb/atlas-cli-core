// Copyright 2024 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build unit

package config

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestProfile_Rename(t *testing.T) {
	tests := []struct {
		name    string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "default",
			wantErr: require.NoError,
		},
		{
			name:    "default-123",
			wantErr: require.NoError,
		},
		{
			name:    "default-test",
			wantErr: require.NoError,
		},
		{
			name:    "default.123",
			wantErr: require.Error,
		},
		{
			name:    "default.test",
			wantErr: require.Error,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Profile{
				name: tt.name,
				fs:   afero.NewMemMapFs(),
			}
			tt.wantErr(t, p.Rename(tt.name), fmt.Sprintf("Rename(%v)", tt.name))
		})
	}
}

func TestProfile_SetName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "default",
			wantErr: require.NoError,
		},
		{
			name:    "default-123",
			wantErr: require.NoError,
		},
		{
			name:    "default-test",
			wantErr: require.NoError,
		},
		{
			name:    "default.123",
			wantErr: require.Error,
		},
		{
			name:    "default.test",
			wantErr: require.Error,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Profile{
				name: tt.name,
				fs:   afero.NewMemMapFs(),
			}
			tt.wantErr(t, p.SetName(tt.name), fmt.Sprintf("SetName(%v)", tt.name))
		})
	}
}
