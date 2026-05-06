// Copyright 2026 The Casdoor Authors. All Rights Reserved.
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

package object

import (
	"testing"
	"time"

	"github.com/casdoor/casdoor/util"
)

func setupBetaTestDB(t *testing.T) {
	t.Helper()
	if ormer == nil || ormer.Engine == nil {
		t.Skip("database not initialized, skipping integration test")
	}
	err := ormer.Engine.Sync2(new(BetaApplication))
	if err != nil {
		t.Fatalf("Sync2 failed: %v", err)
	}
}

func TestAddBetaApplication(t *testing.T) {
	setupBetaTestDB(t)

	owner := "testorg"
	name := "app_" + util.GenerateUUID()
	user := owner + "/testuser"
	activationCode := "TEST_CODE_ABC"
	now := time.Now().Format("2006-01-02T15:04:05+08:00")

	app := &BetaApplication{
		Owner:          owner,
		Name:           name,
		CreatedTime:    now,
		User:           user,
		ActivationCode: activationCode,
		DeviceId:       "",
		Status:         "pending",
	}

	ok, err := AddBetaApplication(app)
	if err != nil {
		t.Fatalf("AddBetaApplication failed: %v", err)
	}
	if !ok {
		t.Fatal("AddBetaApplication returned false")
	}

	// Verify retrieval
	retrieved, err := GetBetaApplication(owner + "/" + name)
	if err != nil {
		t.Fatalf("GetBetaApplication failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetBetaApplication returned nil")
	}
	if retrieved.Name != name {
		t.Errorf("expected Name=%s, got %s", name, retrieved.Name)
	}
	if retrieved.Owner != owner {
		t.Errorf("expected Owner=%s, got %s", owner, retrieved.Owner)
	}
	if retrieved.Status != "pending" {
		t.Errorf("expected Status=pending, got %s", retrieved.Status)
	}
	if retrieved.User != user {
		t.Errorf("expected User=%s, got %s", user, retrieved.User)
	}
	if retrieved.ActivationCode != activationCode {
		t.Errorf("expected ActivationCode=%s, got %s", activationCode, retrieved.ActivationCode)
	}
	if retrieved.DeviceId != "" {
		t.Errorf("expected empty DeviceId, got %s", retrieved.DeviceId)
	}
}

func TestGetBetaApplicationByUser(t *testing.T) {
	setupBetaTestDB(t)

	owner := "testorg"
	user := owner + "/testuser2"
	name := "app_" + util.GenerateUUID()
	now := time.Now().Format("2006-01-02T15:04:05+08:00")

	app := &BetaApplication{
		Owner:          owner,
		Name:           name,
		CreatedTime:    now,
		User:           user,
		ActivationCode: "CODE_XYZ",
		Status:         "pending",
	}

	_, err := AddBetaApplication(app)
	if err != nil {
		t.Fatalf("AddBetaApplication failed: %v", err)
	}

	retrieved, err := GetBetaApplicationByUser(owner, user)
	if err != nil {
		t.Fatalf("GetBetaApplicationByUser failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetBetaApplicationByUser returned nil for existing user")
	}
	if retrieved.User != user {
		t.Errorf("expected User=%s, got %s", user, retrieved.User)
	}
	if retrieved.Owner != owner {
		t.Errorf("expected Owner=%s, got %s", owner, retrieved.Owner)
	}
}

func TestActivateBetaApplication(t *testing.T) {
	setupBetaTestDB(t)

	owner := "testorg"
	name := "app_" + util.GenerateUUID()
	user := owner + "/testuser3"
	now := time.Now().Format("2006-01-02T15:04:05+08:00")

	app := &BetaApplication{
		Owner:          owner,
		Name:           name,
		CreatedTime:    now,
		User:           user,
		ActivationCode: "CODE_ACTIVATE",
		Status:         "pending",
	}

	_, err := AddBetaApplication(app)
	if err != nil {
		t.Fatalf("AddBetaApplication failed: %v", err)
	}

	deviceId := "device-12345-abcde"
	tokenExpiry := time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02T15:04:05+08:00")

	ok, err := ActivateBetaApplication(owner, name, deviceId, tokenExpiry)
	if err != nil {
		t.Fatalf("ActivateBetaApplication failed: %v", err)
	}
	if !ok {
		t.Fatal("ActivateBetaApplication returned false")
	}

	retrieved, err := GetBetaApplication(owner + "/" + name)
	if err != nil {
		t.Fatalf("GetBetaApplication failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetBetaApplication returned nil after activation")
	}
	if retrieved.Status != "activated" {
		t.Errorf("expected Status=activated, got %s", retrieved.Status)
	}
	if retrieved.DeviceId != deviceId {
		t.Errorf("expected DeviceId=%s, got %s", deviceId, retrieved.DeviceId)
	}
	if retrieved.TokenExpiry != tokenExpiry {
		t.Errorf("expected TokenExpiry=%s, got %s", tokenExpiry, retrieved.TokenExpiry)
	}
	if retrieved.ActivationTime == "" {
		t.Error("expected non-empty ActivationTime")
	}
}

func TestExpireBetaApplication(t *testing.T) {
	setupBetaTestDB(t)

	owner := "testorg"
	name := "app_" + util.GenerateUUID()
	user := owner + "/testuser4"
	now := time.Now().Format("2006-01-02T15:04:05+08:00")

	app := &BetaApplication{
		Owner:          owner,
		Name:           name,
		CreatedTime:    now,
		User:           user,
		ActivationCode: "CODE_EXPIRE",
		Status:         "pending",
	}

	_, err := AddBetaApplication(app)
	if err != nil {
		t.Fatalf("AddBetaApplication failed: %v", err)
	}

	ok, err := ExpireBetaApplication(owner, name)
	if err != nil {
		t.Fatalf("ExpireBetaApplication failed: %v", err)
	}
	if !ok {
		t.Fatal("ExpireBetaApplication returned false")
	}

	retrieved, err := GetBetaApplication(owner + "/" + name)
	if err != nil {
		t.Fatalf("GetBetaApplication failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetBetaApplication returned nil after expire")
	}
	if retrieved.Status != "expired" {
		t.Errorf("expected Status=expired, got %s", retrieved.Status)
	}
}

func TestGetBetaApplicationByUserNotFound(t *testing.T) {
	setupBetaTestDB(t)

	retrieved, err := GetBetaApplicationByUser("nonexistent", "nonexistent/nouser")
	if err != nil {
		t.Fatalf("GetBetaApplicationByUser failed: %v", err)
	}
	if retrieved != nil {
		t.Error("expected nil for nonexistent user, got non-nil")
	}
}
