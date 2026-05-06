# Casdoor 内测申请功能 (Beta Testing Application)

## TL;DR

> **Quick Summary**: 为 casdoor 添加用户内测资格申请功能——用户登录后在 /account 页面点击"申请内测"即可获取激活码，后续由应用携带 device_id 调用激活 API 完成设备绑定。将 activation-server 的校验逻辑合入 casdoor 作为新 API，签发 RS256 JWT。
>
> **Deliverables**:
> - `object/activation_code.go` — 激活码模型（XORM）
> - `object/beta_application.go` — 用户申请记录模型（中间表）
> - `controllers/beta.go` — 申请/激活/状态 API 控制器
> - `routers/router.go` — 新增 4 条路由
> - `web/src/account/BetaApplyPage.js` — 内测申请前端页面
> - `web/src/account/BetaApplyBackend.js` — 前端 API 客户端
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 3 waves
> **Critical Path**: Task 1 (models) → Task 4 (apply API) → Task 8 (activate API) → Task 12 (frontend)

---

## Context

### Original Request
用户需要在 casdoor 中添加内测申请功能：激活码由外部导入；用户登录后进入内测申请页面，点击即可登记并获取激活码；后续应用携带 device_id 调用激活 API 完成绑定。需参考已有的 `activation-server` 包（校验服务端）和 `activation` 包（消费端/使用方）进行开发，将 activation-server 的校验功能合入 casdoor，activation 包不合入。

### Interview Summary
**Key Discussions**:
- **两阶段流程**: ①用户在 /account/beta-apply 申请获取激活码(status=pending) → ②应用携带 device_id 调用 /api/activate-beta 完成绑定并获取 JWT
- **激活码表**: 在 casdoor 中用 XORM 新建，遵循 Owner+Name 复合主键
- **中间表主键**: Owner+Name 复合主键（casdoor 规范）
- **状态值**: pending（待激活）、activated（已激活）、expired（已过期）
- **绑定策略**: 一对一（一个激活码绑定一个 device_id）
- **JWT**: RS256 签发，含 device_id claim，匹配 activation-server 协议
- **device_id**: 由激活的应用在请求中携带，非用户在页面输入
- **前端路由**: /account/beta-apply（在已有 /account 区域加入口）
- **测试策略**: TDD（RED-GREEN-REFACTOR）

**Research Findings**:
- **activation-server** (`/activation-server/`): 独立 Go module，校验流程为查 ActivationCode 表 → 检查绑定状态 → 绑定设备 → 签发 RS256 JWT。字段: `{ID, Code(unique), Status(0=unused/1=bound), DeviceID, ActivatedAt}`
- **activation** (`/activation/`): 客户端库，提供设备指纹采集、远程激活调用、JWT 本地校验。import 路径为 `cw/pkg/activation`，不合入
- **Casdoor Cert 模型**: 已有 `object/cert.go` 的 `Cert{PK Owner+Name, PrivateKey(mediumtext), CryptoAlgorithm...}`，可复用于存储 JWT 签名私钥
- **Casdoor 路由模式**: `routers/router.go` 中 `web.Router("/api/xxx", &ApiController{}, "METHOD:Handler")`
- **Casdoor 前端**: React class components + Ant Design 5，`/account` 路由已存在（AccountPage），后端 API 通过 `controllers/base.go` ApiController 处理

### Metis Review
**Identified Gaps** (addressed):
- **JWT 签名密钥来源**: 复用 casdoor 现有 Cert 模型存储 RSA 私钥，通过配置指定 cert 名称
- **未认证激活端点安全**: /api/activate-beta 为公开端点（无需用户 session），需添加非空 device_id 校验 + 长度限制（≤512）防止滥用
- **code 过期逻辑**: 新增 code_expiry_days 配置项，默认 30 天未使用即过期
- **并发安全**: 激活流程使用 XORM 事务 + 唯一约束（activation_code UNIQUE INDEX）防止竞态条件
- **状态扩展到三态**: pending → activated/expired，与 activation-server 的 binary status 不同
- **apply 端点幂等性**: 同一用户多次申请返回已有记录，不重复分配
- **JWT 无状态验证**: 不存储 token，客户端通过公钥本地验签（与 activation-server 一致）
- **activate 端点限流**: 实现基本 IP 限流（每分钟最多 10 次），防止暴力穷举

---

## Work Objectives

### Core Objective
在 casdoor 中添加完整的内测申请激活功能：用户申请获取激活码 → 应用激活绑定设备 → 签发 JWT token。

### Concrete Deliverables
- `object/activation_code.go` — 激活码模型 + CRUD（XORM + Owner+Name PK）
- `object/beta_application.go` — 用户申请记录模型 + CRUD（中间表）
- `controllers/beta.go` — 4 个 API handler: ApplyBeta, ActivateBeta, GetBetaStatus, GetBetaApplications
- `routers/router.go` — 4 条新路由注册
- `web/src/account/BetaApplyPage.js` — 前端申请页面
- `web/src/account/BetaApplyBackend.js` — 前端 API 客户端
- `conf/app.conf` — 新增 beta 配置项
- `object/ormer.go` — 新增 2 个 Sync2 表迁移
- 单元测试文件：activation_code_test.go, beta_application_test.go, beta_controller_test.go

### Definition of Done
- [ ] `go test ./object/ -run "ActivationCode|BetaApplication"` → ALL PASS
- [ ] `go test ./controllers/ -run "Beta"` → ALL PASS
- [ ] `curl -X POST /api/apply-beta` (authenticated) → 200 + activation code
- [ ] `curl -X POST /api/activate-beta -d '{"code":"X","device_id":"device-1"}'` → 200 + JWT
- [ ] `curl /api/get-beta-status` → 200 + 申请状态
- [ ] `/account/beta-apply` 页面可正常访问和交互
- [ ] frontend build: `cd web && yarn build` → 无错误

### Must Have
- [ ] 激活码表 (activation_code) 使用 XORM + Owner+Name 复合主键
- [ ] 中间表 (beta_application) 使用 XORM + Owner+Name 复合主键，字段: user, activation_code, device_id, status, activation_time
- [ ] /api/apply-beta 端点需用户登录（session 认证）
- [ ] /api/activate-beta 端点为公开端点（无需用户认证）
- [ ] 激活码一对一绑定 device_id，不可重复激活
- [ ] RS256 JWT 签发，90天过期，与 activation-server 协议一致
- [ ] 前端用户申请后直接显示激活码（无需审批）
- [ ] TDD: 先写测试，后写实现

### Must NOT Have (Guardrails)
- [ ] **禁止修改** `activation-server/` 目录下任何文件
- [ ] **禁止修改** `activation/` 目录下任何文件
- [ ] **禁止修改** casdoor 现有 JWT 基础设施（OAuth token 等）
- [ ] **禁止添加**管理员审批流程
- [ ] **禁止添加**管理员管理 UI（激活码管理、申请审核等）
- [ ] **禁止添加**邮件/短信通知功能
- [ ] **禁止添加**批量导入激活码功能
- [ ] **禁止添加**激活码自动生成功能（假设码已预置在 DB）
- [ ] **禁止添加**设备管理页面（解绑、设备列表等）
- [ ] **禁止**过度抽象——不新建 service 层，直接在 object 层实现逻辑

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (go test + Makefile ut target)
- **Automated tests**: TDD (RED-GREEN-REFACTOR)
- **Framework**: Go standard library `testing` + `github.com/stretchr/testify`
- **Each task follows**: RED (failing test) → GREEN (minimal impl) → REFACTOR

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Backend API/Model**: Use Bash (curl + go test) — 发送请求，断言状态码 + 响应 JSON
- **Frontend/UI**: Use Playwright — 导航、交互、断言 DOM、截图
- **Module/Logic**: Use Bash (go test) — 导入、调用函数、比较输出

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — MAX PARALLEL):
├── Task 1: ActivationCode model + tests [quick]
├── Task 2: BetaApplication model + tests [quick]
├── Task 3: DB migration + config [quick]
└── Task 4: ApplyBeta API + tests [quick]

Wave 2 (Core Activation — MAX PARALLEL):
├── Task 5: JWT signing utility + cert integration [deep]
├── Task 6: ActivateBeta API + tests [deep]
├── Task 7: GetBetaStatus API + tests [quick]
└── Task 8: GetBetaApplications API + tests [quick]

Wave 3 (Frontend + Integration):
├── Task 9: Frontend Backend API client [quick]
├── Task 10: Frontend BetaApplyPage [visual-engineering]
└── Task 11: Frontend route + AccountPage entry [quick]

Wave FINAL (4 parallel reviews):
├── Task F1: Plan Compliance Audit [oracle]
├── Task F2: Code Quality Review [unspecified-high]
├── Task F3: Real Manual QA [unspecified-high]
└── Task F4: Scope Fidelity Check [deep]
→ Present results → Get explicit user okay
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| 1 | — | 4, 6 |
| 2 | — | 4, 6, 7, 8 |
| 3 | 1, 2 | (all await 1, 2 through model existence) |
| 4 | 1, 2 | 6, 7, 10 |
| 5 | — | 6 |
| 6 | 1, 2, 4, 5 | — |
| 7 | 2 | 9, 10 |
| 8 | 2 | — |
| 9 | 7 | 10 |
| 10 | 7, 9 | — |
| 11 | 10 | — |

### Agent Dispatch Summary

- **Wave 1**: 4 tasks — T1-T2 → `quick`, T3 → `quick`, T4 → `quick`
- **Wave 2**: 4 tasks — T5 → `deep`, T6 → `deep`, T7-T8 → `quick`
- **Wave 3**: 3 tasks — T9 → `quick`, T10 → `visual-engineering`, T11 → `quick`
- **Wave FINAL**: 4 tasks — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. **ActivationCode 模型 + 单元测试**

  **What to do**:
  - 创建 `object/activation_code.go`
  - 定义 `ActivationCode` 结构体，xorm 标签，字段:
    - `Owner` varchar(100) notnull pk
    - `Name` varchar(100) notnull pk（激活码本身）
    - `CreatedTime` varchar(100)
    - `Status` int default:0（0=unused, 1=assigned, 2=expired）
    - `Application` varchar(100)（关联的 beta_application 的 id，空字符串=未分配）
    - `AssignedAt` varchar(100)（分配时间）
  - 实现 CRUD 函数（参照 `object/form.go` 模式）:
    - `getActivationCode(owner, name string) (*ActivationCode, error)` — 内部函数，根据 PK 查询
    - `GetActivationCode(id string) (*ActivationCode, error)` — 通过 `owner/name` 格式的 id 获取
    - `GetActivationCodes(owner string) ([]*ActivationCode, error)` — 按 owner 获取列表
    - `GetAvailableActivationCode(owner string) (*ActivationCode, error)` — 获取一个 status=0 (unused) 的码，按 created_time ASC（FIFO）
    - `AddActivationCode(code *ActivationCode) (bool, error)` — 新增
    - `UpdateActivationCode(id string, code *ActivationCode) (bool, error)` — 更新
    - `MarkActivationCodeAssigned(owner, codeName, applicationId string) error` — 标记为已分配 (status=1)
    - `MarkActivationCodeExpired(owner, codeName string) error` — 标记为已过期 (status=2)
  - **先写测试** (`object/activation_code_test.go`):
    - TestAddActivationCode: insert → verify exists
    - TestGetAvailableActivationCode: insert 2 unused + 1 assigned → get → verify receives oldest unused
    - TestMarkActivationCodeAssigned: assign → verify status=1, application field set
    - TestMarkActivationCodeExpired: expire → verify status=2
    - TestActivationCodeDuplicate: insert duplicate PK → expect error

  **Must NOT do**:
  - 不要在 object 层添加 HTTP 相关逻辑
  - 不要创建 service 层
  - 不要修改 activation-server/ 或 activation/ 下的代码

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 标准的 CRUD 模型创建，遵循 casdoor 现有模式，单一文件 + 测试
  - **Skills**: [`git-master`]
    - `git-master`: 用于参考 commit history 和现有模型变更模式

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3) — Task 1 与 Task 2 无依赖，可并行
  - **Blocks**: Task 4, Task 6, Task 3
  - **Blocked By**: None (can start immediately)

  **References** (CRITICAL):
  - `object/form.go:1-120` — 完整的 XORM 模型 + CRUD 模板，包括 struct 定义、get/Get/GetAll/Add/Update/Delete 函数签名和实现模式
  - `object/ormer.go:310-518` — CreateTables/Sync2 调用位置，需要在此处添加 `a.Engine.Sync2(new(ActivationCode))`
  - `object/cert.go:27-48` — Cert 模型的 xorm 标签写法参考（varchar, pk, mediumtext, index 等）

  **WHY Each Reference Matters**:
  - `form.go` 是最简洁干净的模型模板，无额外 auth 扩展，直接复制其函数签名和实现模式即可
  - `ormer.go` 中的 Sync2 调用是表迁移的唯一入口，必须在最后添加新表
  - `cert.go` 展示了 xorm 复合主键 (Owner+Name) 和 varchar 字段的标签写法

  **Acceptance Criteria**:
  - [ ] Test file created: `object/activation_code_test.go`
  - [ ] `go test ./object/ -run "ActivationCode" -v` → ALL PASS (4+ tests, 0 failures)
  - [ ] `go build ./...` → 无编译错误

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 添加并查询激活码 (happy path)
    Tool: Bash (go test)
    Preconditions: 测试数据库已就绪
    Steps:
      1. 运行: go test ./object/ -run "TestGetActivationCode" -v
      2. 验证: 测试输出包含 "PASS"
      3. 验证: 无 "FAIL" 字样
    Expected Result: 所有 ActivationCode 测试通过
    Evidence: .sisyphus/evidence/task-1-activation-code-test.txt

  Scenario: 获取可用激活码 (FIFO ordering)
    Tool: Bash (go test)
    Preconditions: 表中存在 3 个码: CODE-001(created最早), CODE-002, CODE-003(assigned)
    Steps:
      1. 运行: go test ./object/ -run "TestGetAvailableActivationCode" -v
      2. 断言: 返回 CODE-001 (非 CODE-003)
    Expected Result: 返回最旧未分配的码 (FIFO)
    Evidence: .sisyphus/evidence/task-1-fifo-test.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-1-activation-code-test.txt` — go test 输出
  - [ ] `task-1-fifo-test.txt` — FIFO 测试输出

  **Commit**: YES (groups with Tasks 2, 3 as Wave 1)
  - Message: `feat(beta): add ActivationCode model with CRUD and FIFO assignment`
  - Files: `object/activation_code.go`, `object/activation_code_test.go`
  - Pre-commit: `go test ./object/ -run "ActivationCode" -v`

- [x] 2. **BetaApplication 模型 + 单元测试**

  **What to do**:
  - 创建 `object/beta_application.go`
  - 定义 `BetaApplication` 结构体，字段:
    - `Owner` varchar(100) notnull pk
    - `Name` varchar(100) notnull pk（格式: "app_{user}_{timestamp}"）
    - `CreatedTime` varchar(100)
    - `User` varchar(100)（申请用户 id，格式 "owner/username"）
    - `ActivationCode` varchar(100) index（关联的激活码 name）
    - `DeviceId` varchar(512)（设备 ID，申请时为空）
    - `Status` varchar(50) default:'pending'（pending | activated | expired）
    - `ActivationTime` varchar(100)（激活时间，申请时为空）
    - `TokenExpiry` varchar(100)（JWT 过期时间）
  - 实现 CRUD 函数:
    - `getBetaApplication(owner, name string) (*BetaApplication, error)`
    - `GetBetaApplication(id string) (*BetaApplication, error)`
    - `GetBetaApplicationByUser(owner, user string) (*BetaApplication, error)` — 按用户查已有申请（幂等）
    - `GetBetaApplications(owner string) ([]*BetaApplication, error)`
    - `AddBetaApplication(app *BetaApplication) (bool, error)`
    - `UpdateBetaApplication(id string, app *BetaApplication) (bool, error)`
    - `ActivateBetaApplication(owner, name, deviceId string, tokenExpiry string) error` — 标记激活
    - `ExpireBetaApplication(owner, name string) error` — 标记过期
  - **先写测试** (`object/beta_application_test.go`):
    - TestAddBetaApplication: insert → verify
    - TestGetBetaApplicationByUser: insert → query by user → verify returns same
    - TestActivateBetaApplication: update device_id + status + time → verify
    - TestExpireBetaApplication: expire → verify status="expired"
    - TestGetBetaApplicationByUserNotFound: query non-existent → verify nil

  **Must NOT do**:
  - 不要在 BetaApplication struct 中添加认证相关字段（如 JWT token 存储）——JWT 是无状态的
  - 不要修改 activation-server/ 或 activation/ 下的代码

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 标准 CRUD 模型，复制 form.go 模式 + 业务特定查询方法
  - **Skills**: [`git-master`]
    - `git-master`: 参考 casdoor 现有模型的 git 变更历史

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Task 4, Task 6, Task 7, Task 8
  - **Blocked By**: None (can start immediately)

  **References**:
  - `object/form.go:1-120` — CRUD 模板
  - `object/ormer.go:310-518` — Sync2 调用位置
  - `object/cert.go:27-48` — 复合主键 xorm 标签参考
  - `util/random.go` — ID 生成工具（`GenerateUUID()` 用于生成 BetaApplication 的 Name）

  **Acceptance Criteria**:
  - [ ] Test file created: `object/beta_application_test.go`
  - [ ] `go test ./object/ -run "BetaApplication" -v` → ALL PASS (5+ tests, 0 failures)
  - [ ] `go build ./...` → 无编译错误

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 添加并查询申请记录 (happy path)
    Tool: Bash (go test)
    Preconditions: 测试数据库已就绪
    Steps:
      1. 运行: go test ./object/ -run "TestAddBetaApplication" -v
      2. 验证: 测试输出包含 "PASS"
    Expected Result: 插入成功，查询返回相同记录
    Evidence: .sisyphus/evidence/task-2-beta-app-test.txt

  Scenario: 按用户查询已有申请 (幂等性验证)
    Tool: Bash (go test)
    Preconditions: 用户 "built-in/testuser" 已有 pending 申请
    Steps:
      1. 运行: go test ./object/ -run "TestGetBetaApplicationByUser" -v
      2. 断言: 返回的记录的 User 字段为 "built-in/testuser"
    Expected Result: 正确返回用户的已有申请记录
    Evidence: .sisyphus/evidence/task-2-get-by-user-test.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-2-beta-app-test.txt`
  - [ ] `task-2-get-by-user-test.txt`

  **Commit**: YES (groups with Tasks 1, 3)
  - Message: `feat(beta): add BetaApplication model with status tracking`
  - Files: `object/beta_application.go`, `object/beta_application_test.go`
  - Pre-commit: `go test ./object/ -run "BetaApplication" -v`

- [x] 3. **数据库迁移 + 配置**

  **What to do**:
  - 在 `object/ormer.go` 的 `createTable()` 函数末尾添加 2 行 Sync2:
    - `err = a.Engine.Sync2(new(ActivationCode))`
    - `err = a.Engine.Sync2(new(BetaApplication))`
  - 在 `conf/app.conf` 中添加 beta 相关配置项:
    - `betaJwtCertName = beta-activation-cert`（用于 JWT 签名的 cert 名称，需在 casdoor Cert 管理中预创建）
    - `betaCodeExpiryDays = 30`（未使用激活码过期天数）
    - `betaJwtExpiryHours = 2160`（90天 = 2160小时）
    - `betaActivateRateLimitPerMinute = 10`（激活端点每分钟限流）
  - 创建 `conf/beta.go` 配置读取工具:
    - `GetBetaJwtCertName() string`
    - `GetBetaCodeExpiryDays() int`
    - `GetBetaJwtExpiryHours() int`
    - `GetBetaActivateRateLimitPerMinute() int`
  - 在 `main.go` 或 `object/init.go` 的初始化序列中添加激活码过期检查（可选，或留到 Task 7 处理）

  **Must NOT do**:
  - 不要修改现有 Sync2 调用的顺序或内容
  - 不要硬编码配置值（必须通过 conf/app.conf + env var 覆盖）

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 配置文件修改 + 简单 Go 配置读取器
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES（可与 Task 1、2 在 Wave 1 启动，但依赖 Task 1、2 完成）
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4)
  - **Blocks**: Nothing directly（模型和 API 可以各自独立工作，Sync2 在启动时统一执行）
  - **Blocked By**: Task 1, Task 2（需要 ActivationCode 和 BetaApplication 类型定义）

  **References**:
  - `object/ormer.go:310-518` — 精确找到最后一个 Sync2 调用的位置（插入新行）
  - `conf/app.conf:1-30` — 现有配置格式（INI 风格，key = value）
  - `conf/conf.go:1-50` — GetConfigString/GetConfigBool/GetConfigInt64 使用模式
  - `object/init.go:1-50` — 初始化序列，了解 InitDb 的调用顺序

  **Acceptance Criteria**:
  - [ ] `object/ormer.go` 包含 `Sync2(new(ActivationCode))` 和 `Sync2(new(BetaApplication))`
  - [ ] `go build ./...` → 无编译错误
  - [ ] `conf/beta.go` 配置读取函数正确返回默认值

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 表自动创建 (happy path)
    Tool: Bash (go run)
    Preconditions: 数据库已连接（app.conf 中 driverName 配置正确）
    Steps:
      1. 启动 casdoor: go run main.go
      2. 等待启动完成（看到 "Listening on" 日志）
      3. 查询数据库: 验证 activation_code 和 beta_application 表存在
    Expected Result: 两张新表在启动时自动创建
    Evidence: .sisyphus/evidence/task-3-table-creation.txt

  Scenario: 配置读取默认值 (config default)
    Tool: Bash (go test)
    Preconditions: app.conf 中未设置 betaCodeExpiryDays
    Steps:
      1. 运行: go test ./conf/ -run "TestGetBetaCodeExpiryDays" -v
      2. 断言: 返回默认值 30
    Expected Result: 未配置时返回默认值 30
    Evidence: .sisyphus/evidence/task-3-config-default.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-3-table-creation.txt`
  - [ ] `task-3-config-default.txt`

  **Commit**: YES (groups with Tasks 1, 2)
  - Message: `feat(beta): add DB migration for ActivationCode and BetaApplication tables`
  - Files: `object/ormer.go`, `conf/app.conf`, `conf/beta.go`
  - Pre-commit: `go build ./...`

- [x] 4. **ApplyBeta API + 单元测试**

  **What to do**:
  - 创建 `controllers/beta.go`
  - 实现 `ApplyBeta` handler (POST /api/apply-beta):
    1. 检查用户是否已登录（`c.GetSessionUsername()`）
    2. 未登录 → `c.ResponseError("unauthorized")` → HTTP 401
    3. 已登录 → 查 BetaApplication 表是否有已有申请（`GetBetaApplicationByUser`）
    4. 已有申请 → 返回已有记录（幂等）
    5. 无申请 → 调用 `GetAvailableActivationCode` 获取可用码
    6. 无可用码 → `c.ResponseError("no_available_codes")` → HTTP 500
    7. 分配成功 → 创建 BetaApplication 记录（status=pending） + 标记 ActivationCode 为 assigned
    8. 使用事务确保原子性（XORM `session.Begin()` + `Commit()` / `Rollback()`）
  - **先写测试** (`controllers/beta_test.go`):
    - TestApplyBetaSuccess: 已登录用户 → 200 + code
    - TestApplyBetaUnauthorized: 未登录 → 401
    - TestApplyBetaNoCodes: 无可用码 → 500 + "no_available_codes"
    - TestApplyBetaIdempotent: 已有申请 → 200 + 返回相同记录
  - 使用 `net/http/httptest` 进行 HTTP handler 测试

  **Must NOT do**:
  - 不要在 apply 时要求 device_id（device_id 在 activate 阶段由应用提供）
  - 不要修改 activation-server/ 或 activation/ 代码

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 标准 casdoor API handler，遵循现有 controller 模式
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO（依赖 Task 1, 2 完成）
  - **Parallel Group**: Wave 1 (last in wave)
  - **Blocks**: Task 6, Task 7, Task 10
  - **Blocked By**: Task 1, Task 2

  **References**:
  - `controllers/form.go:1-120` — handler 模式模板: 读取 body → 调用 object 层 → ResponseOk/ResponseError
  - `controllers/base.go:91-100` — `getCurrentUser()` 用法，获取当前登录用户
  - `controllers/util.go:42-60` — `ResponseOk()` / `ResponseError()` 签名和用法
  - `routers/router.go:318-323` — form 路由注册模式（复制格式注册 beta 路由）
  - `object/beta_application.go` — 模型方法签名（需要调用的函数）
  - `object/activation_code.go` — GetAvailableActivationCode 和 MarkActivationCodeAssigned
  - `util/random.go` — ID 生成函数（GenerateUUID 用于生成 BetaApplication Name）

  **Acceptance Criteria**:
  - [ ] Test file created: `controllers/beta_test.go`
  - [ ] `go test ./controllers/ -run "TestApplyBeta" -v` → ALL PASS (4 tests, 0 failures)
  - [ ] `go build ./...` → 无编译错误

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 已登录用户申请成功 (happy path)
    Tool: Bash (curl + go test)
    Preconditions: 数据库有可用激活码，用户已登录（测试中模拟 session）
    Steps:
      1. 运行: go test ./controllers/ -run "TestApplyBetaSuccess" -v
      2. 断言: HTTP 200, JSON body 包含 "code" 和 "status": "pending"
      3. 断言: 数据库 BetaApplication 表中出现新记录
    Expected Result: 返回分配的激活码，状态为 pending
    Evidence: .sisyphus/evidence/task-4-apply-success.txt

  Scenario: 未登录用户申请失败 (error)
    Tool: Bash (go test)
    Preconditions: 用户未登录（无 session cookie）
    Steps:
      1. 运行: go test ./controllers/ -run "TestApplyBetaUnauthorized" -v
      2. 断言: HTTP 401
    Expected Result: 拒绝未认证请求
    Evidence: .sisyphus/evidence/task-4-apply-unauthorized.txt

  Scenario: 重复申请幂等性 (idempotent)
    Tool: Bash (go test)
    Preconditions: 用户已有 pending 申请
    Steps:
      1. 运行: go test ./controllers/ -run "TestApplyBetaIdempotent" -v
      2. 断言: HTTP 200, 返回相同 code（不创建新记录）
    Expected Result: 返回已有申请，不分配新码
    Evidence: .sisyphus/evidence/task-4-apply-idempotent.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-4-apply-success.txt`
  - [ ] `task-4-apply-unauthorized.txt`
  - [ ] `task-4-apply-idempotent.txt`

  **Commit**: YES (Wave 1 completion)
  - Message: `feat(beta): add ApplyBeta API with TDD`
  - Files: `controllers/beta.go`, `controllers/beta_test.go`, `routers/router.go`
  - Pre-commit: `go test ./controllers/ -run "TestApplyBeta" -v`

- [x] 5. **JWT 签名工具 + Cert 集成**

  **What to do**:
  - 创建 `object/beta_jwt.go`
  - 实现 JWT 签名和验证函数:
    - `SignBetaToken(deviceId string) (string, error)` — 使用 RS256 签发 JWT
      1. 从 casdoor Cert 表中读取指定 cert（通过 `conf.GetBetaJwtCertName()` 获取 cert 名）
      2. 解析 PrivateKey PEM 为 `*rsa.PrivateKey`
      3. 构造 Claims: `{device_id, type="activation", iat=now, exp=now+90d, iss="casdoor"}`
      4. 签名返回 JWT 字符串
    - `GetBetaPublicKey(certName string) (*rsa.PublicKey, error)` — 从 cert 的证书 PEM 或从私钥推导公钥
  - 确保与 activation-server 的 JWT 格式兼容:
    - Claims 结构: `{device_id: string, type: "activation"}` + 标准 `RegisteredClaims`
    - 签名算法: RS256
    - Token 格式: 三段式 JWT (header.payload.signature)
  - **先写测试** (`object/beta_jwt_test.go`):
    - TestSignBetaToken: 签名 → 验证签名有效
    - TestSignBetaTokenClaims: 签名 → 解码验证 device_id, type, exp
    - TestSignBetaTokenExpired: 生成过期 token → 验证 ParseWithClaims 返回 TokenExpired 错误
    - TestBetaTokenTamperProof: 修改 payload → 验证验签失败
  - 测试中动态生成临时 RSA 密钥对（不需要实际 cert）

  **Must NOT do**:
  - 不要在 JWT 中包含用户信息（device_id 足够）
  - 不要修改 casdoor 现有的 OAuth JWT 逻辑
  - 不要存储 JWT token（无状态设计）

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 涉及 JWT 加密标准、RSA 密钥解析、与 activation-server 协议兼容性验证，需要仔细理解 JWT 规范
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 4 无直接代码依赖，仅逻辑上后行）
  - **Parallel Group**: Wave 2 (with Tasks 6, 7, 8)
  - **Blocks**: Task 6
  - **Blocked By**: Task 3（需要 conf/beta.go 的配置读取函数）

  **References**:
  - `activation-server/token.go:1-80` — JWT 签名实现参考: ActivationClaims struct, signToken() RSA 签名逻辑
  - `activation-server/main.go:30-68` — 激活流程中 JWT 签发部分
  - `activation/token.go:1-100` — 客户端 JWT 解析验证逻辑（确认 claim 字段名和类型格式）
  - `object/cert.go:27-48` — Cert 模型的 PrivateKey 字段存储格式
  - `certificate/` 目录 — casdoor 现有证书处理代码（密钥解析、PEM 处理）
  - `go.mod` — 确认 `github.com/golang-jwt/jwt/v5` 已在依赖中

  **WHY Each Reference Matters**:
  - `activation-server/token.go` 定义了 ActivationClaims 结构和 signToken 的 RS256 签名方式，必须匹配
  - `activation/token.go` 的 ParseActivationToken 和 ValidateTokenForDevice 决定了 token 的字段名必须为 "device_id", "type"
  - `object/cert.go` 的 PrivateKey 是 PEM 格式文本，需要用 `crypto/x509.ParsePKCS8PrivateKey` 解析

  **Acceptance Criteria**:
  - [ ] Test file created: `object/beta_jwt_test.go`
  - [ ] `go test ./object/ -run "BetaJwt|BetaToken" -v` → ALL PASS (4+ tests, 0 failures)
  - [ ] `go build ./...` → 无编译错误
  - [ ] 生成的 JWT 能用 `activation/token.go` 的 `ParseActivationToken()` 成功解析

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 签发并验证 JWT (happy path)
    Tool: Bash (go test)
    Preconditions: 测试中动态生成 RSA 密钥对
    Steps:
      1. 运行: go test ./object/ -run "TestSignBetaToken" -v
      2. 断言: token 非空，jwt.Parse 成功
    Expected Result: 签名成功，token 可被公钥验证
    Evidence: .sisyphus/evidence/task-5-sign-verify.txt

  Scenario: JWT 与 activation-server 兼容性验证 (compatibility)
    Tool: Bash (go test)
    Preconditions: 参考 activation-server/token.go 的 claim 结构
    Steps:
      1. 签发 token (device_id="test-device-001")
      2. 解码 payload, 断言: claims["device_id"] == "test-device-001", claims["type"] == "activation"
      3. 断言: exp > iat, exp - iat ≈ 7776000 (90天)
    Expected Result: claim 结构与 activation-server 一致
    Evidence: .sisyphus/evidence/task-5-compat-check.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-5-sign-verify.txt`
  - [ ] `task-5-compat-check.txt`

  **Commit**: YES
  - Message: `feat(beta): add JWT signing utility with RS256 and activation-server compatibility`
  - Files: `object/beta_jwt.go`, `object/beta_jwt_test.go`
  - Pre-commit: `go test ./object/ -run "BetaJwt|BetaToken" -v`

- [x] 6. **ActivateBeta API + 单元测试**

  **What to do**:
  - 在 `controllers/beta.go` 中实现 `ActivateBeta` handler (POST /api/activate-beta):
    1. 读取 request body: `{code: string, device_id: string}`
    2. 验证 device_id 非空且 ≤512 字符
    3. 确认 activation code 的格式正确（Name 字段非空）
    4. 查 ActivationCode 表 → 不存在 → `c.ResponseError("invalid_code")`
    5. 查 BetaApplication 表通过 activation_code 关联
    6. 检查 ActivationCode status:
       - status=2 (expired) → `c.ResponseError("code_expired")`
    7. 检查 BetaApplication status:
       - status="activated" 且 device_id 相同 → 重发 JWT（幂等）
       - status="activated" 且 device_id 不同 → `c.ResponseError("code_already_bound")`
    8. 首次激活: status="pending" → 使用事务:
       a. 更新 BetaApplication: device_id, status="activated", activation_time=now
       b. 更新 ActivationCode: status=1 (bound)
       c. 调用 `SignBetaToken(deviceId)` 签发 JWT
       d. 返回 `{token, expires_in}`
    9. 事务失败 → `c.ResponseError("activation_failed")`
  - 实现限流（通过 conf 中的 rate limit 配置，简单内存 map 或使用 casdoor 现有限流机制）
  - **先写测试** (`controllers/beta_test.go`):
    - TestActivateBetaSuccess: 有效 code + device_id → 200 + JWT
    - TestActivateBetaInvalidCode: 不存在的 code → 400 + "invalid_code"
    - TestActivateBetaAlreadyBoundDifferentDevice: 已绑定其他设备 → 403 + "code_already_bound"
    - TestActivateBetaSameDeviceRetry: 同设备重激活 → 200 + 新 JWT（幂等）
    - TestActivateBetaExpiredCode: code 已过期 → 400 + "code_expired"
    - TestActivateBetaEmptyDeviceId: device_id 为空 → 400 + "missing_device_id"
    - TestActivateBetaDeviceIdTooLong: device_id > 512 → 400
    - TestActivateBetaRateLimit: 连续请求超过限制 → 429

  **Must NOT do**:
  - 不要在 activate 端点要求用户登录认证
  - 不要修改 activation-server/ 代码

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 涉及事务管理、并发安全、限流、JWT 签发，复杂度较高
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO（依赖 Task 4, 5）
  - **Parallel Group**: Wave 2 (with Tasks 7, 8)
  - **Blocks**: Nothing
  - **Blocked By**: Task 1, Task 2, Task 4, Task 5

  **References**:
  - `activation-server/main.go:30-68` — 原始激活流程: 查码 → 检查绑定 → 更新 → 签发 JWT（完整参考）
  - `activation-server/db.go:1-80` — findCode() 和 bindDevice() 实现
  - `controllers/beta.go:ApplyBeta` handler（Task 4 产物）— 同类 handler 模式
  - `controllers/base.go:91-100` — getCurrentUser() 用法
  - `controllers/util.go:42-60` — ResponseOk/ResponseError
  - `object/beta_jwt.go` — SignBetaToken (Task 5 产物)
  - `routers/router.go` — 路由注册位置

  **Acceptance Criteria**:
  - [ ] `go test ./controllers/ -run "TestActivateBeta" -v` → ALL PASS (8 tests, 0 failures)
  - [ ] 激活成功后 BetaApplication.status="activated", ActivationCode.status=1
  - [ ] 返回的 JWT 可用公钥验证

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 首次激活成功 (happy path)
    Tool: Bash (curl)
    Preconditions: 数据库有可用激活码 CODE-001，已分配 pending BetaApplication
    Steps:
      1. curl -X POST http://localhost:8000/api/activate-beta -H "Content-Type: application/json" -d '{"code":"CODE-001","device_id":"device-abc123"}'
      2. 断言: HTTP 200
      3. 断言: JSON body.token 非空（JWT 三段式）
      4. 断言: JSON body.expires_in ≈ 7776000
    Expected Result: 返回有效 JWT token
    Failure Indicators: 非 200 状态码，或 token 为空
    Evidence: .sisyphus/evidence/task-6-activate-success.txt

  Scenario: 重复激活同一设备 (幂等)
    Tool: Bash (curl)
    Preconditions: CODE-001 已绑定 device-abc123
    Steps:
      1. 再次 curl POST /api/activate-beta -d '{"code":"CODE-001","device_id":"device-abc123"}'
      2. 断言: HTTP 200, 返回新 token
    Expected Result: 同设备重激活成功，返回新 JWT
    Evidence: .sisyphus/evidence/task-6-activate-idempotent.txt

  Scenario: 不同设备尝试激活已绑定码 (error)
    Tool: Bash (curl)
    Preconditions: CODE-001 已绑定 device-abc123
    Steps:
      1. curl POST /api/activate-beta -d '{"code":"CODE-001","device_id":"device-xyz789"}'
      2. 断言: HTTP 403
      3. 断言: JSON body.msg 包含 "code_already_bound"
    Expected Result: 拒绝不同设备激活
    Evidence: .sisyphus/evidence/task-6-activate-already-bound.txt

  Scenario: device_id 为空 (validation)
    Tool: Bash (curl)
    Steps:
      1. curl POST /api/activate-beta -d '{"code":"CODE-001","device_id":""}'
      2. 断言: HTTP 400
    Expected Result: 拒绝空 device_id
    Evidence: .sisyphus/evidence/task-6-activate-empty-device.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-6-activate-success.txt`
  - [ ] `task-6-activate-idempotent.txt`
  - [ ] `task-6-activate-already-bound.txt`
  - [ ] `task-6-activate-empty-device.txt`

  **Commit**: YES
  - Message: `feat(beta): add ActivateBeta API with binding, JWT issuance, and rate limiting`
  - Files: `controllers/beta.go`, `controllers/beta_test.go`
  - Pre-commit: `go test ./controllers/ -run "TestActivateBeta" -v`

- [x] 7. **GetBetaStatus + GetBetaApplications API + 测试**

  **What to do**:
  - 在 `controllers/beta.go` 中实现 `GetBetaStatus` handler (GET /api/get-beta-status):
    1. 检查用户登录状态（`c.GetSessionUsername()`）
    2. 查 BetaApplication 表按当前用户
    3. 有记录 → 返回 `{code, status, device_id, activation_time}`
    4. 无记录 → 返回 `{status: "none"}`
  - 实现 `GetBetaApplications` handler (GET /api/get-beta-applications) — admin only:
    1. 检查用户是否为 admin（`c.IsAdmin()`）
    2. 查 BetaApplication 表按 owner 返回列表（分页支持）
    3. 非 admin → 401
  - 在 `routers/router.go` 中注册 GET 路由
  - **先写测试**:
    - TestGetBetaStatusExists: 已有申请 → 200 + 状态信息
    - TestGetBetaStatusNone: 无申请 → 200 + status="none"
    - TestGetBetaStatusUnauthorized: 未登录 → 401
    - TestGetBetaApplicationsAdmin: admin → 200 + 列表
    - TestGetBetaApplicationsNonAdmin: 非 admin → 401

  **Must NOT do**:
  - GetBetaStatus 不要返回敏感信息（如完整 token）

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 简单的查询 API，遵循现有 controller 模式
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 5, 6 在 Wave 2 并行）
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 8)
  - **Blocks**: Task 9, Task 10
  - **Blocked By**: Task 2, Task 4

  **References**:
  - `controllers/form.go` — GET handler 模式
  - `controllers/base.go:51-58` — `IsAdmin()` 用法
  - `routers/router.go:318-323` — GET 路由注册模式

  **Acceptance Criteria**:
  - [ ] `go test ./controllers/ -run "TestGetBetaStatus|TestGetBetaApplications" -v` → ALL PASS
  - [ ] `go build ./...` → 无编译错误

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 查询已有申请状态 (happy path)
    Tool: Bash (curl)
    Preconditions: 用户已登录，已有 pending 申请
    Steps:
      1. curl http://localhost:8000/api/get-beta-status -b "session_cookie"
      2. 断言: HTTP 200, JSON body.status == "pending", body.code 非空
    Expected Result: 返回申请状态和激活码
    Evidence: .sisyphus/evidence/task-7-get-status.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-7-get-status.txt`

  **Commit**: YES (groups with Task 8)
  - Message: `feat(beta): add GetBetaStatus and GetBetaApplications endpoints`
  - Files: `controllers/beta.go`, `routers/router.go`
  - Pre-commit: `go test ./controllers/ -run "TestGetBetaStatus|TestGetBetaApplications" -v`

- [x] 8. **路由注册 + 过期码清理**

  **What to do**:
  - 在 `routers/router.go` 中注册所有 4 条 beta 路由（选择合适位置插入）:
    - `web.Router("/api/apply-beta", &controllers.ApiController{}, "POST:ApplyBeta")`
    - `web.Router("/api/activate-beta", &controllers.ApiController{}, "POST:ActivateBeta")`
    - `web.Router("/api/get-beta-status", &controllers.ApiController{}, "GET:GetBetaStatus")`
    - `web.Router("/api/get-beta-applications", &controllers.ApiController{}, "GET:GetBetaApplications")`
  - 实现过期码标记逻辑（在 `object/activation_code.go` 中添加）:
    - `MarkExpiredCodes() error` — 查询 status=0 且 created_time 超过 betaCodeExpiryDays 的码，标记为 status=2
  - 在 `object/init.go` 的 InitDb 中添加调用 `MarkExpiredCodes()`（启动时执行一次）

  **Must NOT do**:
  - 不要修改现有路由的注册方式

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 路由注册是简单的一行代码；过期清理是简单的数据库操作
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES（与 Task 7 并行）
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 7)
  - **Blocks**: Nothing
  - **Blocked By**: Task 1, Task 2, Task 4, Task 6（路由注册需要 handler 存在）

  **References**:
  - `routers/router.go:318-323` — form 路由注册模式（精确的格式和缩进）
  - `routers/authz_filter.go` — 了解 authz filter 如何工作（beta 路由是否需要 authz）
  - `object/init.go:1-80` — InitDb 调用顺序

  **Acceptance Criteria**:
  - [ ] `routers/router.go` 包含 4 条 beta 路由
  - [ ] `go build ./...` → 无编译错误
  - [ ] `MarkExpiredCodes()` 正确标记超过 betaCodeExpiryDays 的未使用码

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 路由可达 (happy path)
    Tool: Bash (curl)
    Preconditions: casdoor 已启动，用户已登录
    Steps:
      1. curl -X POST http://localhost:8000/api/apply-beta -b "session_cookie"
      2. 断言: HTTP 200 或 500（取决于有无可用码），至少不是 404
    Expected Result: 路由注册成功，无 404
    Evidence: .sisyphus/evidence/task-8-route-reachable.txt

  Scenario: 过期码自动标记 (cleanup)
    Tool: Bash (go test)
    Preconditions: 插入一个 created_time 超过 30 天的 status=0 的 ActivationCode
    Steps:
      1. 运行: go test ./object/ -run "TestMarkExpiredCodes" -v
      2. 断言: 该码的 status 变为 2
    Expected Result: 过期码被标记为 expired
    Evidence: .sisyphus/evidence/task-8-expire-cleanup.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-8-route-reachable.txt`
  - [ ] `task-8-expire-cleanup.txt`

  **Commit**: YES (groups with Task 7)
  - Message: `feat(beta): register beta API routes and add expired code cleanup`
  - Files: `routers/router.go`, `object/activation_code.go`, `object/init.go`
  - Pre-commit: `go build ./...`

- [x] 9. **前端 API 客户端 + AccountPage 入口按钮**

  **What to do**:
  - 在 `web/src/account/` 下创建 `BetaApplyBackend.js`:
    - `applyBeta()` — POST /api/apply-beta → 返回 `{code, status}`
    - `getBetaStatus()` — GET /api/get-beta-status → 返回 `{code, status, device_id, activation_time}`
  - 修改 `web/src/account/AccountPage.js`:
    - 在现有 UserEditPage 上方或下方添加"内测申请"入口按钮/卡片
    - 点击跳转到 `/account/beta-apply`
  - 在 `web/src/ManagementPage.js` 中添加 `/account/beta-apply` 路由:
    - `import BetaApplyPage from "./account/BetaApplyPage"`
    - 在 `/account` 相关 Route 区域添加 `<Route exact path="/account/beta-apply" render={(props) => <BetaApplyPage account={account} {...props} />} />`
  - 在 `web/src/locales/en/data.json` 中添加 i18n 字符串:
    - `"beta:Beta Testing Application"`, `"beta:Apply for Beta"`, `"beta:Your Activation Code"`, `"beta:Copy Code"`, `"beta:Apply"`

  **Must NOT do**:
  - 不要修改 casdoor 的全局样式/主题
  - 不要在 AccountPage 中复杂化现有布局（保持最小侵入）

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 简单的 fetch wrapper + 路由注册 + i18n 字符串添加
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (with Tasks 10, 11)
  - **Blocks**: Task 11
  - **Blocked By**: Task 7（需要 API 端点就绪才能写正确的 Backend 调用格式）

  **References**:
  - `web/src/backend/FormBackend.js:1-30` — Backend API 客户端模板: fetch + Setting.ServerUrl + credentials:include + JSON 处理
  - `web/src/account/AccountPage.js:1-26` — 现有 AccountPage 组件，需要在此添加按钮
  - `web/src/ManagementPage.js` — 路由注册位置（搜索 "AccountPage" 找到已注册的 /account 路由附近）
  - `web/src/locales/en/data.json` — i18n 字符串格式参考（namespace: 模式）

  **Acceptance Criteria**:
  - [ ] `BetaApplyBackend.js` 正确调用 `applyBeta()` 和 `getBetaStatus()`
  - [ ] AccountPage 中出现"内测申请"入口
  - [ ] `/account/beta-apply` 路由可访问
  - [ ] `cd web && yarn build` → 无构建错误

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: AccountPage 显示入口按钮 (smoke test)
    Tool: Playwright
    Preconditions: 用户已登录
    Steps:
      1. 导航到 http://localhost:8000/account
      2. 等待页面加载 (timeout: 10s)
      3. 查找文本 "内测申请" 或 "Beta Testing"
    Expected Result: 页面上可见入口链接/按钮
    Evidence: .sisyphus/evidence/task-9-account-entry.png
  ```

  **Evidence to Capture**:
  - [ ] `task-9-account-entry.png` — AccountPage 截图

  **Commit**: YES (groups with Task 10)
  - Message: `feat(beta): add BetaApplyBackend API client and AccountPage entry point`
  - Files: `web/src/account/BetaApplyBackend.js`, `web/src/account/AccountPage.js`, `web/src/ManagementPage.js`, `web/src/locales/en/data.json`
  - Pre-commit: `cd web && yarn build`

- [x] 10. **前端 BetaApplyPage 页面**

  **What to do**:
  - 创建 `web/src/account/BetaApplyPage.js`
  - 页面需为 **React class 组件**（与 casdoor 现有风格一致）:
    1. `componentDidMount()` 调用 `getBetaStatus()` 检查申请状态
    2. **无申请** (status="none"): 显示"申请内测资格"标题 + 说明文字 + "申请"按钮
    3. 点击"申请" → 调用 `applyBeta()`:
       - 成功 → 显示激活码（大号字体 + 复制按钮）
       - 失败 → Ant Design `message.error()` 显示错误信息
    4. **已有申请** (status="pending"): 显示"激活码: XXXX-XXXX" + 提示"请将激活码用于应用激活" + "复制"按钮
    5. **已激活** (status="activated"): 显示"已激活" + 设备 ID + 激活时间
  - 使用 Ant Design 组件: `Card`, `Button`, `Spin` (loading), `Typography.Paragraph` (copyable code)
  - 参考 `web/src/account/AccountPage.js` 的 class 组件模式和 import 风格

  **Must NOT do**:
  - 不要使用 React hooks（functional components + useState/useEffect）——与 casdoor 风格不一致
  - 不要要求用户输入 device_id
  - 不要添加额外的安全验证

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 前端 UI 组件创作，涉及状态管理、条件渲染、Ant Design 组件使用
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (with Tasks 9, 11)
  - **Blocks**: Task 11
  - **Blocked By**: Task 7, Task 9

  **References**:
  - `web/src/account/AccountPage.js:1-26` — 现有 AccountPage 风格参考（class 组件 + account props）
  - `web/src/UserEditPage.js:1-30` — React class 组件模式: state 定义, render, componentDidMount
  - `web/src/Setting.js` — `showMessage("success", ...)` / `showMessage("error", ...)` 错误提示用法
  - `web/src/locales/en/data.json` — i18n key 引用方式: `i18next.t("beta:Apply for Beta")`

  **Acceptance Criteria**:
  - [ ] 无申请时显示"申请"按钮
  - [ ] 已有申请时显示激活码和复制按钮（申请按钮隐藏）
  - [ ] 已激活时显示设备 ID 和激活时间
  - [ ] 加载中显示 Spin
  - [ ] `cd web && yarn build` → 无构建错误

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 首次访问申请页面 (first visit)
    Tool: Playwright
    Preconditions: 用户已登录，无已有申请
    Steps:
      1. 导航到 http://localhost:8000/account/beta-apply
      2. 等待页面加载 (timeout: 10s)
      3. 断言: 可见 "内测申请" 或 "Beta Testing" 文本
      4. 断言: 可见 "申请" / "Apply" 按钮
    Expected Result: 显示申请入口
    Evidence: .sisyphus/evidence/task-10-beta-apply-page.png

  Scenario: 点击申请获取激活码 (apply)
    Tool: Playwright
    Preconditions: 用户已登录，无已有申请，数据库有可用激活码
    Steps:
      1. 导航到 /account/beta-apply
      2. 点击文本为 "申请" / "Apply" 的按钮
      3. 等待加载完成 (timeout: 10s)
      4. 断言: 可见激活码文本（匹配类似 "CODE-" 格式）
      5. 断言: 可见复制图标或按钮
    Expected Result: 显示分配激活码和复制功能
    Evidence: .sisyphus/evidence/task-10-apply-success.png

  Scenario: 已有申请再次访问 (return visit)
    Tool: Playwright
    Preconditions: 用户已有 pending 申请
    Steps:
      1. 导航到 /account/beta-apply
      2. 等待页面加载
      3. 断言: 可见之前分配的激活码
      4. 断言: 不显示 "申请" 按钮
    Expected Result: 显示已有激活码，隐藏申请按钮
    Evidence: .sisyphus/evidence/task-10-existing-application.png
  ```

  **Evidence to Capture**:
  - [ ] `task-10-beta-apply-page.png`
  - [ ] `task-10-apply-success.png`
  - [ ] `task-10-existing-application.png`

  **Commit**: YES (groups with Task 9)
  - Message: `feat(beta): add BetaApplyPage with status display and code copy`
  - Files: `web/src/account/BetaApplyPage.js`
  - Pre-commit: `cd web && yarn build`

- [x] 11. **端到端集成验证 + 前端构建**

  **What to do**:
  - 确认所有后端 API 正常工作:
    1. 用户登录 → Apply → 获取 code + 记录 pending
    2. 应用携带 code + device_id 激活 → 返回 JWT
    3. 验证 JWT 可用公钥验证且包含 device_id
    4. 验证同设备再次激活返回新 JWT（幂等）
    5. 验证不同设备激活被拒绝
  - 确认前端构建成功: `cd web && yarn build`
  - 确认 `go build ./...` 无错误
  - 确认所有相关 `go test` 通过
  - 处理任何集成冲突

  **Must NOT do**:
  - 不要修改测试以外的业务逻辑

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 验证 + 构建，确认全局一致性
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO（依赖所有前序任务）
  - **Parallel Group**: Wave 3 (last in wave)
  - **Blocks**: Final Verification Wave (F1-F4)
  - **Blocked By**: Task 10

  **References**:
  - `routers/router.go` — 确认路由注册
  - 所有前序任务产物

  **Acceptance Criteria**:
  - [ ] 完整两阶段流程端到端通过
  - [ ] `cd web && yarn build` → BUILD SUCCESS
  - [ ] `go build ./...` → BUILD SUCCESS
  - [ ] `go test ./object/ -run "ActivationCode|BetaApplication|BetaJwt" -v` → ALL PASS
  - [ ] `go test ./controllers/ -run "Beta" -v` → ALL PASS

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 端到端流程 (E2E)
    Tool: Bash
    Preconditions: casdoor 已启动，有可用激活码
    Steps:
      1. 登录获取 session cookie
      2. curl POST /api/apply-beta → 200 + code
      3. curl POST /api/activate-beta -d '{"code":"...", "device_id":"e2e-test-device"}' → 200 + token
      4. 解码 token 验证: device_id=="e2e-test-device"
      5. curl GET /api/get-beta-status → 200 + status=="activated"
    Expected Result: 完整流程通过
    Evidence: .sisyphus/evidence/task-11-e2e-flow.txt

  Scenario: 前端构建 (build)
    Tool: Bash
    Preconditions: Node.js + Yarn 已安装
    Steps:
      1. cd web && yarn build
      2. 断言: exit code = 0
      3. 断言: web/build/ 目录下存在 index.html
    Expected Result: 前端构建成功
    Evidence: .sisyphus/evidence/task-11-build-output.txt
  ```

  **Evidence to Capture**:
  - [ ] `task-11-e2e-flow.txt`
  - [ ] `task-11-build-output.txt`

  **Commit**: YES (Wave 3 completion)
  - Message: `feat(beta): E2E integration verification and frontend build`
  - Files: (any integration fixes across multiple files)
  - Pre-commit: `go build ./... && cd web && yarn build`

---

## Final Verification Wave

> 4 review agents run in PARALLEL after ALL implementation tasks.
> ALL must APPROVE. Present consolidated results and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `go build ./...` + `go test ./...`. Review all changed files for: `panic()` without recovery, empty catch, `fmt.Print` debug lines, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names (data/result/item/temp).
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill)
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: apply → activate → status query → re-activate. Test edge cases: empty state, invalid input, rapid double-clicks.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Wave | Tasks | Message Pattern |
|------|-------|----------------|
| 1 | 1-4 | `feat(beta): add ActivationCode and BetaApplication models with ApplyBeta API` |
| 2 | 5-8 | `feat(beta): add JWT signing, ActivateBeta API, status endpoints, and routing` |
| 3 | 9-11 | `feat(beta): add BetaApplyPage frontend and E2E integration` |

---

## Success Criteria

### Verification Commands
```bash
# All backend tests
go test ./object/ -run "ActivationCode|BetaApplication|BetaJwt" -v
go test ./controllers/ -run "Beta" -v

# Full build
go build ./...

# Frontend build
cd web && yarn build
```

### Final Checklist
- [ ] All "Must Have" present (6 items)
- [ ] All "Must NOT Have" absent (9 items)
- [ ] All unit tests pass
- [ ] Frontend builds without errors
- [ ] Two-stage flow works end-to-end: apply → activate → JWT
- [ ] JWT compatible with activation-server format (RS256 + device_id + type claims)
- [ ] Idempotent apply (same user returns same record)
- [ ] Idempotent re-activation (same device gets new JWT)
- [ ] Different device rejected on already-bound code
- [ ] Expired code rejected
- [ ] Rate limiting functional on activate endpoint