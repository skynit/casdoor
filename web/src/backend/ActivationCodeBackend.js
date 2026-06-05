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

import * as Setting from "../Setting";

export function getActivationCodes(owner, page = "", pageSize = "", field = "", value = "", sortField = "", sortOrder = "", activated = "") {
  return fetch(`${Setting.ServerUrl}/api/get-activation-codes?owner=${owner}&p=${page}&pageSize=${pageSize}&field=${field}&value=${value}&sortField=${sortField}&sortOrder=${sortOrder}&activated=${activated}`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}

export function getActivationCode(id) {
  return fetch(`${Setting.ServerUrl}/api/get-activation-code?id=${encodeURIComponent(id)}`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}

export function addActivationCode(code) {
  const newCode = Setting.deepCopy(code);
  return fetch(`${Setting.ServerUrl}/api/add-activation-code`, {
    method: "POST",
    credentials: "include",
    body: JSON.stringify(newCode),
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}

export function addActivationCodes(codes) {
  return fetch(`${Setting.ServerUrl}/api/add-activation-codes`, {
    method: "POST",
    credentials: "include",
    body: JSON.stringify(codes),
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}

export function deleteActivationCode(code) {
  const newCode = Setting.deepCopy(code);
  return fetch(`${Setting.ServerUrl}/api/delete-activation-code`, {
    method: "POST",
    credentials: "include",
    body: JSON.stringify(newCode),
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}

export function updateActivationCode(id, code) {
  const newCode = Setting.deepCopy(code);
  return fetch(`${Setting.ServerUrl}/api/update-activation-code?id=${encodeURIComponent(id)}`, {
    method: "POST",
    credentials: "include",
    body: JSON.stringify(newCode),
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}

export function getActivationCodeStats(owner) {
  return fetch(`${Setting.ServerUrl}/api/get-activation-code-stats?owner=${owner}`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}

export function getBetaPaused() {
  return fetch(`${Setting.ServerUrl}/api/get-beta-paused`, {
    method: "GET",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}

export function setBetaPaused(paused) {
  return fetch(`${Setting.ServerUrl}/api/set-beta-paused`, {
    method: "POST",
    credentials: "include",
    body: JSON.stringify({paused}),
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}

export function resetActivationCode(id) {
  return fetch(`${Setting.ServerUrl}/api/reset-activation-code?id=${encodeURIComponent(id)}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Accept-Language": Setting.getAcceptLanguage(),
    },
  }).then(res => res.json());
}
