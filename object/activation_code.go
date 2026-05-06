// Copyright 2025 The Casdoor Authors. All Rights Reserved.
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
	"fmt"
	"time"

	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/util"
	"github.com/xorm-io/core"
)

// ActivationCode represents an activation code
type ActivationCode struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	Status      int    `xorm:"int" json:"status"`
	Application string `xorm:"varchar(100)" json:"application"`
	AssignedAt  string `xorm:"varchar(100)" json:"assignedAt"`
	AssignedTo  string `xorm:"varchar(100)" json:"assignedTo"`
}

func getActivationCode(owner string, name string) (*ActivationCode, error) {
	code := ActivationCode{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(&code)
	if err != nil {
		return &code, err
	}
	if existed {
		return &code, nil
	}
	return nil, nil
}

func GetActivationCode(id string) (*ActivationCode, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return nil, err
	}
	return getActivationCode(owner, name)
}

func GetActivationCodes(owner string) ([]*ActivationCode, error) {
	codes := []*ActivationCode{}
	err := ormer.Engine.Desc("created_time").Find(&codes, &ActivationCode{Owner: owner})
	if err != nil {
		return codes, err
	}
	return codes, nil
}

func GetPaginationActivationCodes(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*ActivationCode, error) {
	codes := []*ActivationCode{}
	session := ormer.Engine.Prepare()
	if offset >= 0 && limit > 0 {
		session.Limit(limit, offset)
	}
	if owner != "" {
		session = session.And("owner = ?", owner)
	}
	if field != "" && value != "" {
		if util.FilterField(field) {
			session = session.And(fmt.Sprintf("%s like ?", util.CamelToSnakeCase(field)), "%"+value+"%")
		}
	}
	if sortField == "" || sortOrder == "" {
		sortField = "created_time"
	}
	if sortOrder == "ascend" {
		session = session.Asc(util.CamelToSnakeCase(sortField))
	} else {
		session = session.Desc(util.CamelToSnakeCase(sortField))
	}
	err := session.Find(&codes)
	if err != nil {
		return codes, err
	}
	return codes, nil
}

func GetActivationCodeCount(owner string, field, value string) (int64, error) {
	session := ormer.Engine.Prepare()
	if owner != "" {
		session = session.And("owner = ?", owner)
	}
	if field != "" && value != "" {
		if util.FilterField(field) {
			session = session.And(fmt.Sprintf("%s like ?", util.CamelToSnakeCase(field)), "%"+value+"%")
		}
	}
	return session.Count(&ActivationCode{})
}

// GetAvailableActivationCode returns the oldest unused activation code for the given owner (FIFO).
func GetAvailableActivationCode(owner string) (*ActivationCode, error) {
	code := &ActivationCode{}
	existed, err := ormer.Engine.Asc("created_time").Where("status = ? AND owner = ?", 0, owner).Limit(1).Get(code)
	if err != nil {
		return code, err
	}
	if existed {
		return code, nil
	}
	return nil, nil
}

func AddActivationCode(code *ActivationCode) (bool, error) {
	affected, err := ormer.Engine.Insert(code)
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

func UpdateActivationCode(id string, code *ActivationCode) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return false, err
	}
	_, err = ormer.Engine.ID(core.PK{owner, name}).AllCols().Update(code)
	if err != nil {
		return false, err
	}
	return true, nil
}

// MarkActivationCodeAssigned marks a code as assigned to an application and user.
func MarkActivationCodeAssigned(id string, applicationId, assignedTo string) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return false, err
	}
	code := &ActivationCode{
		Status:      1,
		Application: applicationId,
		AssignedAt:  time.Now().Format("2006-01-02T15:04:05+08:00"),
		AssignedTo:  assignedTo,
	}
	_, err = ormer.Engine.ID(core.PK{owner, name}).Cols("status", "application", "assigned_at", "assigned_to").Update(code)
	if err != nil {
		return false, err
	}
	return true, nil
}

// MarkActivationCodeExpired marks a code as expired by setting status=2.
func MarkActivationCodeExpired(id string) (bool, error) {
	owner, name, err := util.GetOwnerAndNameFromIdWithError(id)
	if err != nil {
		return false, err
	}
	code := &ActivationCode{
		Status: 2,
	}
	_, err = ormer.Engine.ID(core.PK{owner, name}).Cols("status").Update(code)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (code *ActivationCode) GetId() string {
	return code.Owner + "/" + code.Name
}

func MarkExpiredCodes() error {
	days := conf.GetBetaCodeExpiryDays()
	expiryTime := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02T15:04:05+08:00")
	_, err := ormer.Engine.Where("status = 0 AND created_time < ?", expiryTime).Cols("status").Update(&ActivationCode{Status: 2})
	return err
}

func DeleteActivationCode(owner, name string) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{owner, name}).Delete(&ActivationCode{})
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}