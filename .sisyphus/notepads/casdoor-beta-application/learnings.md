# Learnings: Casdoor Beta Application

## Task 1: ActivationCode Model (2026-05-02)

### Patterns Followed
- XORM composite primary key: `Owner` + `Name` with `xorm:"varchar(100) notnull pk"` tags
- Timestamps stored as `varchar(100)` (not `time.Time`), matching casdoor conventions
- `ormer.Engine` global engine for all DB operations
- `util.GetOwnerAndNameFromIdWithError(id)` for parsing "owner/name" composite ID
- `core.PK{owner, name}` for ID-based XORM updates
- Time format: `"2006-01-02T15:04:05+08:00"` (ISO 8601 with timezone offset)

### Function Signatures
- Internal getter: `getActivationCode(owner, name string)` unexported
- Public getters: `GetActivationCode(id string)`, `GetActivationCodes(owner string)`
- FIFO lookup: `GetAvailableActivationCode(owner string)` using `Asc("created_time").Limit(1)`
- Mutations: `AddActivationCode`, `UpdateActivationCode`, `MarkActivationCodeAssigned`, `MarkActivationCodeExpired`
- `MarkActivationCodeAssigned(id, applicationId string)` — uses `Cols()` for selective update (status, application, assigned_at)
- `MarkActivationCodeExpired(id string)` — uses `Cols("status")` for minimal update

### Test Setup
- In-memory SQLite via `xorm.NewEngine("sqlite", ":memory:")` (driver is "sqlite", NOT "sqlite3")
- `TestMain(m *testing.M)` pattern to initialize DB once for all tests
- `ormer = &Ormer{Engine: engine}` to set the global ormer for tests
- Helper `newTestActivationCode(owner, status)` for DRY test data creation
- No testify/gomega — pure Go `testing` package (matching existing tests)
- White-box tests (`package object`) to access unexported `getActivationCode` and `Ormer` type

### Gotchas
- `modernc.org/sqlite` registers as driver "sqlite" — using "sqlite3" causes panic
- Pre-existing compilation errors in `permission_rbac_dedup_test.go` block `go test ./object/` from compiling (unrelated to this task)
- `beta_application_test.go` already defines `setupTestDB(t *testing.T)` — renamed to `setupActivationCodeTestDB()` to avoid conflict
- The `init global config instance failed` warning is harmless when running tests with TestMain override

### Decisions
- Used `Cols()` for Mark* functions (selective column update) vs `AllCols()` for UpdateActivationCode (full update)
- ActivationCode.Status values: 0=unused, 1=assigned, 2=expired

## Task 2: BetaApplication Model (2026-05-02)

### Patterns Followed
- Same XORM composite PK pattern: `Owner` + `Name` with `xorm:"varchar(100) notnull pk"`
- Timestamps as `varchar(100)` using `time.Now().Format("2006-01-02T15:04:05+08:00")`
- CRUD pattern from `object/form.go`: private `getBetaApplication(owner,name)`, public `GetBetaApplication(id)` using `util.GetOwnerAndNameFromIdWithError(id)`
- `ormer.Engine.ID(core.PK{owner, name})` for PK-based operations
- `ormer.Engine.Cols(...)` for partial field updates
- `core.PK` import from `github.com/xorm-io/core`

### Function Signatures
- Internal: `getBetaApplication(owner, name string)` — unexported
- Public: `GetBetaApplication(id string)`, `GetBetaApplicationByUser(owner, user string)`, `GetBetaApplications(owner string)`
- Mutations: `AddBetaApplication`, `UpdateBetaApplication`, `ActivateBetaApplication`, `ExpireBetaApplication`
- `ActivateBetaApplication(owner, name, deviceId, tokenExpiry string)` — uses `Cols("device_id", "status", "activation_time", "token_expiry")`
- `ExpireBetaApplication(owner, name string)` — uses `Cols("status")` for minimal update
- `GetBetaApplicationByUser(owner, user string)` uses `ormer.Engine.Where("owner = ? AND user = ?", owner, user).Get(...)`

### Test Setup
- 5 tests: TestAddBetaApplication, TestGetBetaApplicationByUser, TestActivateBetaApplication, TestExpireBetaApplication, TestGetBetaApplicationByUserNotFound
- `setupBetaTestDB(t *testing.T)` — named differently from Task 1's `setupActivationCodeTestDB()` to avoid collision
- Tests skip if `ormer == nil || ormer.Engine == nil` (graceful in no-DB environments)
- White-box tests (`package object`) to access unexported `getBetaApplication`

### Gotchas
- Pre-existing compilation errors in `permission_rbac_dedup_test.go` block `go test ./object/` from compiling (NOT related to BetaApplication)
- `go build ./object/` passes cleanly (exit code 0) — confirms BetaApplication code compiles
- LSP diagnostics: 0 errors on `beta_application.go` and `beta_application_test.go`
- Naming collision resolved: renamed `setupTestDB` to `setupBetaTestDB` to avoid conflict with Task 1's test file

### Decisions
- `GetBetaApplicationByUser` takes explicit `owner, user` params (not ID-based) to support the lookup pattern
- `ActivationCode` field stores the code Name (which IS the code string per casdoor conventions)
- Status values: "pending" → "activated" → "expired" (string enums)
- No JWT token in model — JWT is stateless, only TokenExpiry is stored

## Task 5: Beta JWT Signing Utility (2026-05-03)

### Patterns Followed
- `activation-server/token.go` signToken pattern: RS256 + `jwt.NewWithClaims(jwt.SigningMethodRS256, claims)` + `token.SignedString(privateKey)`
- Claims struct: `BetaClaims` with `DeviceID`, `Type` (both JSON-tagged), embedding `jwt.RegisteredClaims`
- Claims fields: `device_id="<id>"`, `type="activation"`, `iss="casdoor"`, `iat=<now>`, `exp=<now+90d>`
- Cert loading via `getCert("admin", certName)` — unexported function, accessible from same `object` package

### Key Insight: PKCS#1 vs PKCS#8
- Casdoor's `generateRsaKeys()` in `token_jwt_key.go` uses `x509.MarshalPKCS1PrivateKey()` with PEM type `"RSA PRIVATE KEY"` → PKCS#1 format
- Activation-server's `loadPrivateKey()` uses `x509.ParsePKCS8PrivateKey()` → PKCS#8 format
- MUST handle both: try `x509.ParsePKCS1PrivateKey` first, fall back to `x509.ParsePKCS8PrivateKey`

### Function Signatures
- `SignBetaTokenForTest(deviceID string, privateKey *rsa.PrivateKey, duration time.Duration) (string, error)` — exported for testing, uses explicit key
- `SignBetaToken(deviceID string) (string, error)` — production: loads cert from DB via `conf.GetBetaJwtCertName()`, then delegates to `SignBetaTokenForTest`
- `GetBetaPublicKey() (*rsa.PublicKey, error)` — extracts public key from stored private key PEM
- `parsePrivateKey(pemData string) (*rsa.PrivateKey, error)` — unexported, handles PKCS#1 and PKCS#8

### Test Design (TDD)
- 4 tests: `TestSignBetaToken`, `TestSignBetaTokenClaims`, `TestSignBetaTokenExpired`, `TestBetaTokenTamperProof`
- Generate temporary RSA key pair via `rsa.GenerateKey(rand.Reader, 2048)` — no DB dependency
- `generateTestRSAKey(t *testing.T)` helper returns `(*rsa.PrivateKey, *rsa.PublicKey)`
- Tamper-proof test: split JWT on `.`, modify middle (payload) section → verify parse fails
- Expired token test: pass negative duration to `SignBetaTokenForTest` → verify parse fails with "expired" error

### Cross-Package Compatibility
- `object.BetaClaims` and `activation.ActivationClaims` have identical JSON tags (`device_id`, `type`, plus `jwt.RegisteredClaims` embedded fields)
- Tokens generated by `SignBetaTokenForTest` are fully parseable by `activation.ParseActivationToken()` and pass `activation.ValidateTokenForDevice()`
- White-box verification confirmed: sign with object → parse with activation → pass

### Gotchas
- Pre-existing compilation errors in `permission_rbac_dedup_test.go` block `go test ./object/` (must temporarily rename to run tests)
- `go build ./object/` passes cleanly — confirms new code compiles
- `init global config instance failed` warning is harmless during package tests
- LSP diagnostics: 0 errors on both `beta_jwt.go` and `beta_jwt_test.go`
