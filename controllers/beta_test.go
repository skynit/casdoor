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

package controllers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	beecontext "github.com/beego/beego/v2/server/web/context"
	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/util"
	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	engine, err := object.InitSqliteTestDB()
	if err != nil {
		panic(fmt.Sprintf("failed to create in-memory sqlite engine: %v", err))
	}

	// Sync tables required by ApplyBeta, GetBetaStatus, and GetBetaApplications
	err = engine.Sync2(new(object.ActivationCode), new(object.BetaApplication))
	if err != nil {
		panic(fmt.Sprintf("TestMain Sync2 failed: %v", err))
	}

	code := m.Run()

	engine.Close()
	os.Exit(code)
}

// newTestActivationCode creates an unused activation code with the given owner and name.
func newTestActivationCode(owner string, name string) *object.ActivationCode {
	return &object.ActivationCode{
		Owner:       owner,
		Name:        name,
		CreatedTime: time.Now().Format("2006-01-02T15:04:05+08:00"),
		Status:      0,
	}
}

// createMockController creates an ApiController with a mock session username.
// Uses Beego's proper context initialization so all controller methods work correctly.
func createMockController(username string) *ApiController {
	r, _ := http.NewRequest("POST", "/api/apply-beta", nil)
	w := httptest.NewRecorder()

	// Create and initialize a Beego context with the mock response writer
	ctx := beecontext.NewContext()
	ctx.Reset(&beecontext.Response{ResponseWriter: w}, r)
	ctx.Input.SetData("currentUserId", username)

	c := &ApiController{}
	c.Init(ctx, "ApiController", "ApplyBeta", c)

	return c
}

// createMockControllerGet creates an ApiController for GET requests with a custom URL.
// The URL should include query parameters as needed.
func createMockControllerGet(username string, url string) *ApiController {
	r, _ := http.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()

	ctx := beecontext.NewContext()
	ctx.Reset(&beecontext.Response{ResponseWriter: w}, r)
	ctx.Input.SetData("currentUserId", username)

	c := &ApiController{}
	c.Init(ctx, "ApiController", "GetBetaApplications", c)

	return c
}

// getResponseData extracts the Response from the controller's Data.
func getResponseData(c *ApiController) *Response {
	resp, ok := c.Data["json"].(*Response)
	if !ok {
		return nil
	}
	return resp
}

// generateTestRSAKey generates a 2048-bit RSA key for JWT signing in tests.
func generateTestRSAKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// createMockControllerPost creates an ApiController for POST requests with a JSON body.
func createMockControllerPost(method string, url string, body string) *ApiController {
	r, _ := http.NewRequest(method, url, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx := beecontext.NewContext()
	ctx.Reset(&beecontext.Response{ResponseWriter: w}, r)
	// Read request body to populate ctx.Input.RequestBody
	bodyBytes, _ := io.ReadAll(r.Body)
	ctx.Input.RequestBody = bodyBytes

	c := &ApiController{}
	c.Init(ctx, "ApiController", "ActivateBeta", c)

	return c
}

// getResponseMap extracts the Data from a Response as a map.
func getResponseMap(c *ApiController) map[string]interface{} {
	resp := getResponseData(c)
	if resp == nil {
		return nil
	}
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	err = json.Unmarshal(dataBytes, &m)
	if err != nil {
		return nil
	}
	return m
}

func TestApplyBetaSuccess(t *testing.T) {
	owner := "built-in"
	code := newTestActivationCode(owner, "BETA-CODE-001")
	ok, err := object.AddActivationCode(code)
	if err != nil {
		t.Fatalf("AddActivationCode failed: %v", err)
	}
	if !ok {
		t.Fatal("AddActivationCode returned false")
	}

	c := createMockController(owner + "/testuser")

	// Call handler (will fail until implemented - this is RED phase)
	c.ApplyBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response in c.Data, got nil")
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status=ok, got status=%s, msg=%s", resp.Status, resp.Msg)
	}

	// Response data should be a BetaApplication
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal response data: %v", err)
	}
	var betaApp object.BetaApplication
	err = json.Unmarshal(dataBytes, &betaApp)
	if err != nil {
		t.Fatalf("failed to unmarshal BetaApplication: %v", err)
	}
	if betaApp.Status != "pending" {
		t.Errorf("expected status=pending, got %s", betaApp.Status)
	}
	if betaApp.User != owner+"/testuser" {
		t.Errorf("expected User=%s/testuser, got %s", owner, betaApp.User)
	}
	if betaApp.ActivationCode != code.Name {
		t.Errorf("expected ActivationCode=%s, got %s", code.Name, betaApp.ActivationCode)
	}
}

func TestApplyBetaUnauthorized(t *testing.T) {
	// No session set
	c := createMockController("")

	c.ApplyBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response in c.Data, got nil")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status=error, got status=%s", resp.Status)
	}
	if resp.Msg != "unauthorized" {
		t.Errorf("expected msg=unauthorized, got msg=%s", resp.Msg)
	}
}

func TestApplyBetaNoCodes(t *testing.T) {
	owner := "nocode-org"
	c := createMockController(owner + "/testuser")

	c.ApplyBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response in c.Data, got nil")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status=error, got status=%s", resp.Status)
	}
	if resp.Msg != "no_available_codes" {
		t.Errorf("expected msg=no_available_codes, got msg=%s", resp.Msg)
	}
}

func TestApplyBetaIdempotent(t *testing.T) {
	owner := "idempotent-org"
	user := owner + "/idemuser"
	codeName := "BETA-CODE-IDEM"

	// Set up an available activation code
	code := newTestActivationCode(owner, codeName)
	_, err := object.AddActivationCode(code)
	if err != nil {
		t.Fatalf("AddActivationCode failed: %v", err)
	}

	// Insert an existing BetaApplication for this user (simulating previous application)
	existingApp := &object.BetaApplication{
		Owner:          owner,
		Name:           "app_" + util.GenerateUUID(),
		CreatedTime:    time.Now().Format("2006-01-02T15:04:05+08:00"),
		User:           user,
		ActivationCode: codeName,
		Status:         "pending",
	}
	_, err = object.AddBetaApplication(existingApp)
	if err != nil {
		t.Fatalf("AddBetaApplication (existing) failed: %v", err)
	}

	// Now call ApplyBeta for the same user - should return existing application
	c := createMockController(user)

	c.ApplyBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response in c.Data, got nil")
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status=ok, got status=%s, msg=%s", resp.Status, resp.Msg)
	}

	// Verify the response contains the existing application (same code)
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal response data: %v", err)
	}
	var betaApp object.BetaApplication
	err = json.Unmarshal(dataBytes, &betaApp)
	if err != nil {
		t.Fatalf("failed to unmarshal BetaApplication: %v", err)
	}
	if betaApp.ActivationCode != codeName {
		t.Errorf("expected ActivationCode=%s (existing), got %s", codeName, betaApp.ActivationCode)
	}
	if betaApp.Name != existingApp.Name {
		t.Errorf("expected Name=%s (existing), got %s", existingApp.Name, betaApp.Name)
	}

	// Verify only one BetaApplication record exists for this user
	retrieved, err := object.GetBetaApplicationByUser(owner, user)
	if err != nil {
		t.Fatalf("GetBetaApplicationByUser failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected existing BetaApplication, got nil")
	}
	if retrieved.Name != existingApp.Name {
		t.Errorf("expected existing Name=%s, got %s", existingApp.Name, retrieved.Name)
	}
}

func TestGetBetaStatusExists(t *testing.T) {
	owner := "status-org"
	user := owner + "/testuser"

	// Insert a BetaApplication for this user
	app := &object.BetaApplication{
		Owner:          owner,
		Name:           "app_teststatus",
		CreatedTime:    time.Now().Format("2006-01-02T15:04:05+08:00"),
		User:           user,
		ActivationCode: "CODE-STATUS",
		Status:         "pending",
	}
	_, err := object.AddBetaApplication(app)
	if err != nil {
		t.Fatalf("AddBetaApplication failed: %v", err)
	}

	c := createMockController(user)
	c.GetBetaStatus()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status=ok, got status=%s, msg=%s", resp.Status, resp.Msg)
	}

	// Parse response data as map[string]string
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var statusData map[string]string
	err = json.Unmarshal(dataBytes, &statusData)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if statusData["status"] != "pending" {
		t.Errorf("expected status=pending, got %s", statusData["status"])
	}
	if statusData["code"] != "CODE-STATUS" {
		t.Errorf("expected code=CODE-STATUS, got %s", statusData["code"])
	}
}

func TestGetBetaStatusNone(t *testing.T) {
	c := createMockController("nonexistent-org/testuser")
	c.GetBetaStatus()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status=ok, got status=%s, msg=%s", resp.Status, resp.Msg)
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var statusData map[string]string
	err = json.Unmarshal(dataBytes, &statusData)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if statusData["status"] != "none" {
		t.Errorf("expected status=none, got %s", statusData["status"])
	}
}

func TestGetBetaStatusUnauthorized(t *testing.T) {
	c := createMockController("")
	c.GetBetaStatus()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status=error, got status=%s", resp.Status)
	}
	if resp.Msg != "unauthorized" {
		t.Errorf("expected msg=unauthorized, got msg=%s", resp.Msg)
	}
}

func TestGetBetaApplicationsAdmin(t *testing.T) {
	owner := "admin-org"
	// Use "app/" prefix to bypass DB user lookup (IsAppUser returns true → isGlobalAdmin)
	adminUser := "app/admin-test"

	// Insert some BetaApplications
	app1 := &object.BetaApplication{
		Owner:          owner,
		Name:           "app1",
		CreatedTime:    time.Now().Format("2006-01-02T15:04:05+08:00"),
		User:           owner + "/user1",
		ActivationCode: "CODE-1",
		Status:         "pending",
	}
	_, err := object.AddBetaApplication(app1)
	if err != nil {
		t.Fatalf("AddBetaApplication failed: %v", err)
	}

	app2 := &object.BetaApplication{
		Owner:          owner,
		Name:           "app2",
		CreatedTime:    time.Now().Format("2006-01-02T15:04:05+08:00"),
		User:           owner + "/user2",
		ActivationCode: "CODE-2",
		Status:         "activated",
	}
	_, err = object.AddBetaApplication(app2)
	if err != nil {
		t.Fatalf("AddBetaApplication failed: %v", err)
	}

	c := createMockControllerGet(adminUser, "/api/get-beta-applications?owner="+owner)
	c.GetBetaApplications()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status=ok, got status=%s, msg=%s", resp.Status, resp.Msg)
	}

	// Parse response data as list
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var apps []object.BetaApplication
	err = json.Unmarshal(dataBytes, &apps)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 applications, got %d", len(apps))
	}
}

func TestGetBetaApplicationsNonAdmin(t *testing.T) {
	c := createMockController("normal-org/normaluser")
	c.GetBetaApplications()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status=error, got status=%s", resp.Status)
	}
	if resp.Msg != "unauthorized" {
		t.Errorf("expected msg=unauthorized, got msg=%s", resp.Msg)
	}
}

// ============ ActivateBeta Tests ============

// helperSetupTestCode creates an activation code and linked BetaApplication for testing.
// Returns the code, the BetaApplication, and the code name.
func helperSetupTestCode(owner string, codeName string, status string) (*object.ActivationCode, *object.BetaApplication) {
	code := newTestActivationCode(owner, codeName)
	_, err := object.AddActivationCode(code)
	if err != nil {
		panic(fmt.Sprintf("AddActivationCode failed: %v", err))
	}

	app := &object.BetaApplication{
		Owner:          owner,
		Name:           "app_" + util.GenerateUUID(),
		CreatedTime:    time.Now().Format("2006-01-02T15:04:05+08:00"),
		User:           owner + "/testuser",
		ActivationCode: codeName,
		Status:         status,
	}
	_, err = object.AddBetaApplication(app)
	if err != nil {
		panic(fmt.Sprintf("AddBetaApplication failed: %v", err))
	}

	// Link code to application
	code.Application = app.Owner + "/" + app.Name
	code.Status = 1
	_, err = object.UpdateActivationCode(code.GetId(), code)
	if err != nil {
		panic(fmt.Sprintf("UpdateActivationCode failed: %v", err))
	}

	return code, app
}

// TestActivateBetaSuccess tests a valid code+device_id activation on a pending app.
func TestActivateBetaSuccess(t *testing.T) {
	owner := "built-in"
	codeName := "ACTIVATE-SUCCESS"

	code, _ := helperSetupTestCode(owner, codeName, "pending")
	_ = code

	body := fmt.Sprintf(`{"code":"%s","device_id":"device-abc"}`, codeName)
	c := createMockControllerPost("POST", "/api/activate-beta", body)

	c.ActivateBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status=ok, got status=%s, msg=%s", resp.Status, resp.Msg)
	}

	respMap := getResponseMap(c)
	if respMap == nil {
		t.Fatal("expected response map, got nil")
	}
	if respMap["token"] == nil || respMap["token"].(string) == "" {
		t.Error("expected token in response")
	}
	if respMap["expires_in"] == nil {
		t.Error("expected expires_in in response")
	}
}

// TestActivateBetaInvalidCode tests a non-existent code.
func TestActivateBetaInvalidCode(t *testing.T) {
	c := createMockControllerPost("POST", "/api/activate-beta", `{"code":"NONEXISTENT-CODE","device_id":"device-abc"}`)

	c.ActivateBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status=error, got status=%s", resp.Status)
	}
	if resp.Msg != "invalid_code" {
		t.Errorf("expected msg=invalid_code, got msg=%s", resp.Msg)
	}
}

// TestActivateBetaAlreadyBoundDifferentDevice tests that a code already bound to one device
// cannot be activated with a different device.
func TestActivateBetaAlreadyBoundDifferentDevice(t *testing.T) {
	owner := "built-in"
	codeName := "BOUND-DIFF-DEV"

	_, app := helperSetupTestCode(owner, codeName, "activated")
	// Mark the BetaApplication as already activated with device-A
	app.DeviceId = "device-A"
	_, err := object.UpdateBetaApplication(app.GetId(), app)
	if err != nil {
		t.Fatalf("UpdateBetaApplication failed: %v", err)
	}

	// Try activating with device-B
	body := fmt.Sprintf(`{"code":"%s","device_id":"device-B"}`, codeName)
	c := createMockControllerPost("POST", "/api/activate-beta", body)

	c.ActivateBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status=error, got status=%s, msg=%s", resp.Status, resp.Msg)
	}
	if resp.Msg != "code_already_bound" {
		t.Errorf("expected msg=code_already_bound, got msg=%s", resp.Msg)
	}
}

// TestActivateBetaSameDeviceRetry tests that re-activating with the same device_id
// returns a new JWT (idempotent re-issue).
func TestActivateBetaSameDeviceRetry(t *testing.T) {
	owner := "built-in"
	codeName := "SAME-DEV-RETRY"

	_, app := helperSetupTestCode(owner, codeName, "activated")
	// Mark as already activated with device-X
	app.DeviceId = "device-X"
	_, err := object.UpdateBetaApplication(app.GetId(), app)
	if err != nil {
		t.Fatalf("UpdateBetaApplication failed: %v", err)
	}

	// Re-activate with the same device-X
	body := fmt.Sprintf(`{"code":"%s","device_id":"device-X"}`, codeName)
	c := createMockControllerPost("POST", "/api/activate-beta", body)

	c.ActivateBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status=ok, got status=%s, msg=%s", resp.Status, resp.Msg)
	}

	respMap := getResponseMap(c)
	if respMap == nil {
		t.Fatal("expected response map, got nil")
	}
	if respMap["token"] == nil || respMap["token"].(string) == "" {
		t.Error("expected token in response")
	}
	if respMap["expires_in"] == nil {
		t.Error("expected expires_in in response")
	}
}

// TestActivateBetaExpiredCode tests that an expired code cannot be activated.
func TestActivateBetaExpiredCode(t *testing.T) {
	owner := "built-in"
	codeName := "EXPIRED-CODE"

	code, _ := helperSetupTestCode(owner, codeName, "pending")
	// Mark the code as expired (status=2)
	code.Status = 2
	_, err := object.UpdateActivationCode(code.GetId(), code)
	if err != nil {
		t.Fatalf("UpdateActivationCode (expire) failed: %v", err)
	}

	body := fmt.Sprintf(`{"code":"%s","device_id":"device-abc"}`, codeName)
	c := createMockControllerPost("POST", "/api/activate-beta", body)

	c.ActivateBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status=error, got status=%s", resp.Status)
	}
	if resp.Msg != "code_expired" {
		t.Errorf("expected msg=code_expired, got msg=%s", resp.Msg)
	}
}

// TestActivateBetaEmptyDeviceId tests that empty device_id returns an error.
func TestActivateBetaEmptyDeviceId(t *testing.T) {
	c := createMockControllerPost("POST", "/api/activate-beta", `{"code":"SOME-CODE","device_id":""}`)

	c.ActivateBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status=error, got status=%s", resp.Status)
	}
	if resp.Msg != "missing_device_id" {
		t.Errorf("expected msg=missing_device_id, got msg=%s", resp.Msg)
	}
}

// TestActivateBetaDeviceIdTooLong tests that device_id longer than 512 chars returns error.
func TestActivateBetaDeviceIdTooLong(t *testing.T) {
	longDeviceId := strings.Repeat("x", 513)
	body := fmt.Sprintf(`{"code":"SOME-CODE","device_id":"%s"}`, longDeviceId)
	c := createMockControllerPost("POST", "/api/activate-beta", body)

	c.ActivateBeta()

	resp := getResponseData(c)
	if resp == nil {
		t.Fatal("expected Response, got nil")
	}
	if resp.Status != "error" {
		t.Fatalf("expected status=error, got status=%s", resp.Status)
	}
	if resp.Msg != "device_id_too_long" {
		t.Errorf("expected msg=device_id_too_long, got msg=%s", resp.Msg)
	}
}

// TestActivateBetaRateLimit tests that more than 10 requests/minute are rate-limited.
func TestActivateBetaRateLimit(t *testing.T) {
	owner := "built-in"
	codeName := "RATE-LIMIT-TEST"

	code, _ := helperSetupTestCode(owner, codeName, "pending")
	_ = code

	// Send 11 rapid requests (limit is 10 per minute)
	var rateLimited bool
	for i := 0; i < 11; i++ {
		body := fmt.Sprintf(`{"code":"%s","device_id":"device-rate-%d"}`, codeName, i)
		c := createMockControllerPost("POST", "/api/activate-beta", body)

		c.ActivateBeta()

		resp := getResponseData(c)
		if resp != nil && resp.Status == "error" && resp.Msg == "rate_limited" {
			rateLimited = true
			break
		}
	}

	if !rateLimited {
		t.Error("expected rate_limited response after 10+ requests, but got none")
	}
}
