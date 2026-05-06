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
	"time"

	"github.com/casdoor/casdoor/util"
	"github.com/xorm-io/core"
)

// BetaApplication 用户内测申请记录表（中间表）。
// Status 流转: pending（待激活）→ activated（已激活）→ expired（已过期）。
type BetaApplication struct {
	Owner          string `xorm:"varchar(100) notnull pk" json:"owner"`           // 组织
	Name           string `xorm:"varchar(100) notnull pk" json:"name"`            // 主键名（格式 app_{uuid}）
	CreatedTime    string `xorm:"varchar(100)" json:"createdTime"`                // 创建时间
	User           string `xorm:"varchar(100)" json:"user"`                       // 申请用户（owner/username）
	ActivationCode string `xorm:"varchar(100) index" json:"activationCode"`       // 关联激活码
	DeviceId       string `xorm:"varchar(512)" json:"deviceId"`                   // 设备 ID（激活时填入）
	Status         string `xorm:"varchar(50) default 'pending'" json:"status"`    // 状态: pending/activated/expired
	ActivationTime string `xorm:"varchar(100)" json:"activationTime"`             // 激活时间
	TokenExpiry    string `xorm:"varchar(100)" json:"tokenExpiry"`                // JWT 过期时间
}

func getBetaApplication(owner string, name string) (*BetaApplication, error) {
	app := BetaApplication{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(&app)
	if err != nil {
		return &app, err
	}

	if existed {
		return &app, nil
	}
	return nil, nil
}

func GetBetaApplication(id string) (*BetaApplication, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return nil, err
	}
	return getBetaApplication(owner, name)
}

func GetBetaApplicationByUser(owner string, user string) (*BetaApplication, error) {
	app := &BetaApplication{}
	existed, err := ormer.Engine.Where("owner = ? AND user = ?", owner, user).Get(app)
	if err != nil {
		return app, err
	}
	if existed {
		return app, nil
	}
	return nil, nil
}

func GetBetaApplications(owner string) ([]*BetaApplication, error) {
	apps := []*BetaApplication{}
	err := ormer.Engine.Desc("created_time").Find(&apps, &BetaApplication{Owner: owner})
	if err != nil {
		return apps, err
	}
	return apps, nil
}

func AddBetaApplication(app *BetaApplication) (bool, error) {
	affected, err := ormer.Engine.Insert(app)
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

func UpdateBetaApplication(id string, app *BetaApplication) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return false, err
	}
	_, err = ormer.Engine.ID(core.PK{owner, name}).AllCols().Update(app)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ActivateBetaApplication(owner, name, deviceId, tokenExpiry string) (bool, error) {
	now := time.Now().Format("2006-01-02T15:04:05+08:00")
	_, err := ormer.Engine.ID(core.PK{owner, name}).Cols("device_id", "status", "activation_time", "token_expiry").Update(&BetaApplication{
		DeviceId:       deviceId,
		Status:         "activated",
		ActivationTime: now,
		TokenExpiry:    tokenExpiry,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func ExpireBetaApplication(owner, name string) (bool, error) {
	_, err := ormer.Engine.ID(core.PK{owner, name}).Cols("status").Update(&BetaApplication{
		Status: "expired",
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (app *BetaApplication) GetId() string {
	return app.Owner + "/" + app.Name
}
