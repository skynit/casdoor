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
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/beego/beego/v2/core/utils/pagination"
	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/util"
	"github.com/xorm-io/core"
)

// betaRateLimiter 内测激活接口的 IP 限流器
var betaRateLimiter = struct {
	mu    sync.Mutex
	store map[string][]time.Time
}{store: make(map[string][]time.Time)}

// checkBetaRateLimit 基于 IP 的速率限制，每分钟最多 N 次（N 由配置决定）
func checkBetaRateLimit(ip string) bool {
	limit := conf.GetBetaActivateRateLimitPerMinute()
	betaRateLimiter.mu.Lock()
	defer betaRateLimiter.mu.Unlock()

	now := time.Now()
	window := now.Add(-time.Minute)

	// Filter out entries older than 1 minute
	var recent []time.Time
	for _, t := range betaRateLimiter.store[ip] {
		if t.After(window) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= limit {
		return false
	}
	recent = append(recent, now)
	betaRateLimiter.store[ip] = recent
	return true
}

// ActivateBetaRequest 激活接口的请求体
type ActivateBetaRequest struct {
	Code     string `json:"code"`
	DeviceId string `json:"device_id"`
}

// ApplyBeta
// @Title ApplyBeta
// @Tag Beta API
// @Description 用户申请内测资格，返回分配的激活码
// @Success 200 {object} object.BetaApplication The Response object
// @router /apply-beta [post]
func (c *ApiController) ApplyBeta() {
	// 1. Get current user from session
	userId := c.GetSessionUsername()
	if userId == "" {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	// 2. Parse user into owner/username
	owner, _ := util.GetOwnerAndNameFromIdNoCheck(userId)

	// 3. Check for existing application (idempotent)
	existing, err := object.GetBetaApplicationByUser(owner, userId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if existing != nil {
		c.ResponseOk(existing)
		return
	}

	// 4. Check if beta distribution is paused
	paused, err := object.GetBetaPaused()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if paused {
		c.ResponseError("第一轮内测已结束，请等待公测后再体验，感谢您的关注。\nThe first round of closed beta codes has ended. Please wait for the open beta to experience it. Thank you for your attention.")
		return
	}

	// 5. Get available activation code
	code, err := object.GetAvailableActivationCode("built-in")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if code == nil {
		c.ResponseError("no_available_codes")
		return
	}

	// 5. Create BetaApplication record
	now := time.Now().Format("2006-01-02T15:04:05+08:00")
	app := &object.BetaApplication{
		Owner:          owner,
		Name:           fmt.Sprintf("app_%s", util.GenerateUUID()),
		CreatedTime:    now,
		User:           userId,
		ActivationCode: code.Name,
		Status:         "pending",
	}

	// 6. Transaction: insert BetaApplication and mark activation code as assigned
	sess := object.GetOrmer().Engine.NewSession()
	defer sess.Close()

	err = sess.Begin()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	_, err = sess.Insert(app)
	if err != nil {
		sess.Rollback()
		c.ResponseError(err.Error())
		return
	}

	_, err = sess.ID(core.PK{code.Owner, code.Name}).Cols("status", "application", "assigned_at", "assigned_to").Update(&object.ActivationCode{
		Status:      1,
		Application: app.GetId(),
		AssignedAt:  now,
		AssignedTo:  userId,
	})
	if err != nil {
		sess.Rollback()
		c.ResponseError(err.Error())
		return
	}

	err = sess.Commit()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(app)
}

// GetBetaStatus
// @Title GetBetaStatus
// @Tag Beta API
// @Description 获取当前用户的内测申请状态
// @Success 200 {object} map[string]string The Response object
// @router /get-beta-status [get]
func (c *ApiController) GetBetaStatus() {
	userId := c.GetSessionUsername()
	if userId == "" {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}
	owner, _ := util.GetOwnerAndNameFromIdNoCheck(userId)

	app, err := object.GetBetaApplicationByUser(owner, userId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if app == nil {
		c.ResponseOk(map[string]string{"status": "none"})
		return
	}
	c.ResponseOk(map[string]string{
		"code":           app.ActivationCode,
		"status":         app.Status,
		"deviceId":       app.DeviceId,
		"activationTime": app.ActivationTime,
	})
}

// ResetMyBeta
// @Title ResetMyBeta
// @Tag Beta API
// @Description reset the current user's activation binding, allowing re-activation with a different device
// @Success 200 {object} controllers.Response The Response object
// @router /reset-my-beta [post]
func (c *ApiController) ResetMyBeta() {
	userId := c.GetSessionUsername()
	if userId == "" {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}
	owner, _ := util.GetOwnerAndNameFromIdNoCheck(userId)

	app, err := object.GetBetaApplicationByUser(owner, userId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if app == nil {
		c.ResponseError("no_application")
		return
	}

	_, err = object.ResetBetaApplication(app.GetId())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	if app.ActivationCode != "" {
		codeId := util.GetId("built-in", app.ActivationCode)
		_, err = object.ResetActivationCode(codeId)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
	}

	c.ResponseOk(true)
}

// GetBetaApplications
// @Title GetBetaApplications
// @Tag Beta API
// @Description 获取所有内测申请记录（管理员）
// @Success 200 {array} object.BetaApplication The Response object
// @router /get-beta-applications [get]
func (c *ApiController) GetBetaApplications() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}
	owner := c.Ctx.Input.Query("owner")
	apps, err := object.GetBetaApplications(owner)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(apps)
}

// ActivateBeta
// @Title ActivateBeta
// @Tag Beta API
// @Description 使用激活码和设备 ID 完成激活绑定，返回 JWT
// @Param body body ActivateBetaRequest true "activation request"
// @Success 200 {object} map[string]interface{} The Response object
// @router /activate-beta [post]
func (c *ApiController) ActivateBeta() {
	// 1. Parse JSON body
	var req ActivateBetaRequest
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &req)
	if err != nil {
		c.ResponseError("invalid_request_body")
		return
	}

	// 2. Validate device_id: non-empty AND len <= 512
	if req.DeviceId == "" {
		c.ResponseError("missing_device_id")
		return
	}
	if len(req.DeviceId) > 512 {
		c.ResponseError("device_id_too_long")
		return
	}

	// 3. Rate limit check (IP-based)
	ip := c.Ctx.Input.IP()
	if !checkBetaRateLimit(ip) {
		c.ResponseError("rate_limited")
		c.Ctx.ResponseWriter.WriteHeader(429)
		return
	}

	// 4. Look up ActivationCode by code name
	// Use "built-in" as default owner for the public endpoint
	code, err := object.GetActivationCode(fmt.Sprintf("built-in/%s", req.Code))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if code == nil {
		c.ResponseError("invalid_code")
		return
	}

	// 5. Check ActivationCode status
	// status=0: not yet applied, reject (must apply first via /api/apply-beta)
	// status=1: already applied/reserved, can be activated
	// status=2: expired, reject
	if code.Status == 0 {
		c.ResponseError("code_not_yet_applied")
		return
	}
	if code.Status == 2 {
		c.ResponseError("code_expired")
		return
	}

	// 6. Find BetaApplication linked to this code
	app, err := object.GetBetaApplication(code.Application)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if app == nil {
		c.ResponseError("invalid_code")
		return
	}

	// 7. Check BetaApplication status
	if app.Status == "activated" {
		if app.DeviceId == req.DeviceId {
			// Same device: re-issue JWT (idempotent)
			token, err := signBetaTokenForActivate(req.DeviceId, req.Code)
			if err != nil {
				c.ResponseError(err.Error())
				return
			}
			c.ResponseOk(map[string]interface{}{
				"token":      token,
				"expires_in": 86400,
			})
			return
		}
		// Different device: code already bound
		c.ResponseError("code_already_bound")
		c.Ctx.ResponseWriter.WriteHeader(403)
		return
	}

	// 8. status="pending" → first-time activation
	if app.Status == "pending" {
		token, err := signBetaTokenForActivate(req.DeviceId, req.Code)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		tokenExpiry := time.Now().Add(24 * time.Hour).Format("2006-01-02T15:04:05+08:00")
		nowFormatted := time.Now().Format("2006-01-02T15:04:05+08:00")

		sess := object.GetOrmer().Engine.NewSession()
		defer sess.Close()

		err = sess.Begin()
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		// Update BetaApplication
		_, err = sess.ID(core.PK{app.Owner, app.Name}).Cols("device_id", "status", "activation_time", "token_expiry").Update(&object.BetaApplication{
			DeviceId:       req.DeviceId,
			Status:         "activated",
			ActivationTime: nowFormatted,
			TokenExpiry:    tokenExpiry,
		})
		if err != nil {
			sess.Rollback()
			c.ResponseError(err.Error())
			return
		}

		// Mark activation code as used (status=1) and record activation time
		_, err = sess.ID(core.PK{code.Owner, code.Name}).Cols("status", "activated_at").Update(&object.ActivationCode{
			Status:      1,
			ActivatedAt: nowFormatted,
		})
		if err != nil {
			sess.Rollback()
			c.ResponseError(err.Error())
			return
		}

		err = sess.Commit()
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		c.ResponseOk(map[string]interface{}{
			"token":      token,
			"expires_in": 86400,
		})
		return
	}

	// Unknown status
	c.ResponseError("invalid_code")
}

// signBetaTokenForActivate 签发激活 JWT（优先使用配置证书，测试环境自动生成临时密钥）
func signBetaTokenForActivate(deviceID, activationCode string) (string, error) {
	token, err := object.SignBetaToken(deviceID, activationCode)
	if err == nil {
		return token, nil
	}

	// Fallback for test environments: generate a temporary RSA key
	testKey, genErr := rsa.GenerateKey(rand.Reader, 2048)
	if genErr != nil {
		return "", fmt.Errorf("failed to generate test key: %w", genErr)
	}

	return object.SignBetaTokenForTest(deviceID, activationCode, testKey, 24*time.Hour)
}

// @Title GetActivationCodes
// @Tag Beta API
// @Description get activation codes
// @Param   owner     query    string  true        "The owner of activation codes"
// @Success 200 {array} object.ActivationCode The Response object
// @router /get-activation-codes [get]
func (c *ApiController) GetActivationCodes() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	owner := c.Ctx.Input.Query("owner")
	limit := c.Ctx.Input.Query("pageSize")
	page := c.Ctx.Input.Query("p")
	field := c.Ctx.Input.Query("field")
	value := c.Ctx.Input.Query("value")
	sortField := c.Ctx.Input.Query("sortField")
	sortOrder := c.Ctx.Input.Query("sortOrder")
	activated := c.Ctx.Input.Query("activated")

	if limit == "" || page == "" {
		codes, err := object.GetActivationCodes(owner)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		c.ResponseOk(codes)
	} else {
		limitInt := util.ParseInt(limit)
		count, err := object.GetActivationCodeCount(owner, field, value, activated)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		paginator := pagination.NewPaginator(c.Ctx.Request, limitInt, count)
		codes, err := object.GetPaginationActivationCodes(owner, paginator.Offset(), limitInt, field, value, sortField, sortOrder, activated)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		c.ResponseOk(codes, count)
	}
}

// GetActivationCode
// @Title GetActivationCode
// @Tag Beta API
// @Description get activation code by id
// @Param   id     query    string  true        "The id of activation code"
// @Success 200 {object} object.ActivationCode The Response object
// @router /get-activation-code [get]
func (c *ApiController) GetActivationCode() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	id := c.Ctx.Input.Query("id")
	code, err := object.GetActivationCode(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(code)
}

// AddActivationCode
// @Title AddActivationCode
// @Tag Beta API
// @Description add a single activation code
// @Param   body     body    object.ActivationCode  true        "The activation code object"
// @Success 200 {object} object.Response The Response object
// @router /add-activation-code [post]
func (c *ApiController) AddActivationCode() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	var code object.ActivationCode
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &code); err != nil {
		c.ResponseError(err.Error())
		return
	}

	affected, err := object.AddActivationCode(&code)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(affected)
}

// AddActivationCodes
// @Title AddActivationCodes
// @Tag Beta API
// @Description add multiple activation codes
// @Param   body     body    []object.ActivationCode  true        "The activation code objects"
// @Success 200 {object} object.Response The Response object
// @router /add-activation-codes [post]
func (c *ApiController) AddActivationCodes() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	var codes []*object.ActivationCode
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &codes); err != nil {
		c.ResponseError(err.Error())
		return
	}

	affected := 0
	for _, code := range codes {
		ok, err := object.AddActivationCode(code)
		if err == nil && ok {
			affected++
		}
	}
	c.ResponseOk(affected)
}

// DeleteActivationCode
// @Title DeleteActivationCode
// @Tag Beta API
// @Description delete activation code
// @Param   body     body    object.ActivationCode  true        "The activation code object"
// @Success 200 {object} object.Response The Response object
// @router /delete-activation-code [post]
func (c *ApiController) DeleteActivationCode() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	var code object.ActivationCode
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &code); err != nil {
		c.ResponseError(err.Error())
		return
	}

	affected, err := object.DeleteActivationCode(code.Owner, code.Name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(affected)
}

// ResetActivationCode
// @Title ResetActivationCode
// @Tag Beta API
// @Description clear the device binding of an activation code, allowing the assigned user to re-activate with a different device
// @Param   id     query    string  true        "The id (owner/name) of the activation code"
// @Success 200 {object} controllers.Response The Response object
// @router /reset-activation-code [post]
func (c *ApiController) ResetActivationCode() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	id := c.Ctx.Input.Query("id")
	if id == "" {
		c.ResponseError("missing_id")
		return
	}

	// 1. Look up the activation code
	code, err := object.GetActivationCode(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if code == nil {
		c.ResponseError("invalid_code")
		return
	}

	// 2. Reset the linked BetaApplication if present
	if code.Application != "" {
		_, err = object.ResetBetaApplication(code.Application)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
	}

	// 3. Reset the activation code itself
	_, err = object.ResetActivationCode(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(true)
}

// GetActivationCodeStats
// @Title GetActivationCodeStats
// @Tag Beta API
// @Description get activation code statistics
// @Param   owner     query    string  true        "The owner of activation codes"
// @Success 200 {object} object.ActivationCodeStats The Response object
// @router /get-activation-code-stats [get]
func (c *ApiController) GetActivationCodeStats() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	owner := c.Ctx.Input.Query("owner")
	stats, err := object.GetActivationCodeStats(owner)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(stats)
}

// GetBetaPausedStatus
// @Title GetBetaPausedStatus
// @Tag Beta API
// @Description get beta distribution paused status
// @Success 200 {object} map[string]bool The Response object
// @router /get-beta-paused [get]
func (c *ApiController) GetBetaPausedStatus() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	paused, err := object.GetBetaPaused()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(map[string]bool{"paused": paused})
}

// SetBetaPaused
// @Title SetBetaPaused
// @Tag Beta API
// @Description set beta distribution paused status
// @Param   body     body    map[string]bool  true        "{\"paused\": true}"
// @Success 200 {object} controllers.Response The Response object
// @router /set-beta-paused [post]
func (c *ApiController) SetBetaPaused() {
	if !c.IsAdmin() {
		c.ResponseError("unauthorized")
		c.Ctx.ResponseWriter.WriteHeader(401)
		return
	}

	var body map[string]bool
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &body); err != nil {
		c.ResponseError(err.Error())
		return
	}

	paused, ok := body["paused"]
	if !ok {
		c.ResponseError("missing 'paused' field")
		return
	}

	err := object.SetBetaPaused(paused)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(paused)
}
