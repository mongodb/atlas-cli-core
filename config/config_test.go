// Copyright 2025 MongoDB Inc
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

package config

import (
	"testing"

	"github.com/mongodb/atlas-cli-core/mocks"
	"go.uber.org/mock/gomock"
)

func TestVerifyConfigVersion(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*mocks.MockStore)
		expectedVer int64
		wantErr     bool
	}{
		{
			name: "version matches expected",
			setupMock: func(m *mocks.MockStore) {
				m.EXPECT().GetProfileNames().Return([]string{"default"})
				m.EXPECT().IsSetGlobal("version").Return(true)
				m.EXPECT().GetGlobalValue("version").Return(int64(2))
			},
			expectedVer: 2,
			wantErr:     false,
		},
		{
			name: "missing version",
			setupMock: func(m *mocks.MockStore) {
				m.EXPECT().GetProfileNames().Return([]string{"default"})
				m.EXPECT().IsSetGlobal("version").Return(false)
			},
			expectedVer: 2,
			wantErr:     true,
		},
		{
			name: "version newer than expected",
			setupMock: func(m *mocks.MockStore) {
				m.EXPECT().GetProfileNames().Return([]string{"default"})
				m.EXPECT().IsSetGlobal("version").Return(true)
				m.EXPECT().GetGlobalValue("version").Return(int64(3))
			},
			expectedVer: 2,
			wantErr:     true,
		},
		{
			name: "version older than expected",
			setupMock: func(m *mocks.MockStore) {
				m.EXPECT().GetProfileNames().Return([]string{"default"})
				m.EXPECT().IsSetGlobal("version").Return(true)
				m.EXPECT().GetGlobalValue("version").Return(int64(1))
			},
			expectedVer: 2,
			wantErr:     true,
		},
		{
			name: "no profiles set",
			setupMock: func(m *mocks.MockStore) {
				m.EXPECT().GetProfileNames().Return([]string{})
			},
			expectedVer: 2,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockStore(ctrl)
			tt.setupMock(mockStore)

			err := verifyConfigVersion(tt.expectedVer, mockStore)
			if (err != nil) != tt.wantErr {
				t.Errorf("verifyConfigVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
