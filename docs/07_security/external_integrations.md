# External Integrations & Service Providers
status: LOCKED

## Scope

Đặc tả kỹ thuật khóa danh sách nhà cung cấp dịch vụ ngoài (Identity, IAP Payment, Notifications) và mô hình bảo mật cho ngày phát hành game Thỉnh Thần.

Mục tiêu: Đảm bảo AI agent triển khai các task Auth (`IMP-006`), IAP (`IMP-053`), và An ninh (`IMP-045`) không phải tự lựa chọn nhà cung cấp, thông số mật mã hay cấu hình môi trường.

## 1. Identity Providers (IdP)

### 1.1 Danh sách Provider được duyệt
Chỉ hỗ trợ 3 nhà cung cấp xác thực liên kết (Federated Identity Providers) chính thức:
1. **Apple Sign In (`apple`):** Dành cho iOS, macOS và PC client. Xác thực JWT Identity Token qua Apple Public Keys (JWKS: `https://appleid.apple.com/auth/keys`); `iss = https://appleid.apple.com`, `aud` thuộc `APPLE_SIGNIN_CLIENT_IDS`, `exp > now()`, và claim `nonce` bắt buộc bằng `SHA-256(nonce thô client gửi kèm)` (chống replay; ADR-0069).
2. **Google Play Games / Google OAuth (`google`):**
   - **Production Path:** Bắt buộc xác thực chữ ký OIDC JWT cục bộ bằng bộ khóa công khai Google JWKS (`https://www.googleapis.com/oauth2/v3/certs`). Kiểm tra thuật toán RS256, `iss` (`https://accounts.google.com`), `aud` thuộc `GOOGLE_OIDC_CLIENT_IDS`, và thời gian hết hạn `exp > now()`. JWKS được cache trong bộ nhớ kèm cơ chế refresh tự động.
   - **Debug / Development Path Only:** Endpoint `https://oauth2.googleapis.com/tokeninfo?id_token={token}` chỉ được phép dùng cho kiểm thử thủ công/debug cục bộ; cấm dùng trên production để tránh phụ thuộc network roundtrip và rate limit của Google.
3. **Steam OpenID (`steam`):** Dành cho phiên bản phân phối PC qua Steamworks SDK. Client tạo ticket bằng `GetAuthTicketForWebApi("thinhthan-login")`; server gọi `GET https://partner.steam-api.com/ISteamUserAuth/AuthenticateUserTicket/v1/?key=STEAM_PUBLISHER_KEY&appid=STEAM_APP_ID&ticket=<hex>&identity=thinhthan-login` và chấp nhận chỉ khi `response.params.result = OK`, `steamid` khác rỗng và `vacbanned`/`publisherbanned` = false; `provider_subject = steamid` (ADR-0069).

### 1.2 First-Party Password Authentication: ENABLED (ADR-0051)
- Provider `password`: đăng ký bằng username + mật khẩu + email (không xác thực email), đăng nhập bằng username + mật khẩu.
- Quy tắc, băm Argon2id và mã lỗi: `auth.md` § Password Provider. Không có reset mật khẩu lúc ra mắt.
- Email không gửi đi đâu; server không tích hợp dịch vụ gửi mail.

## 2. In-App Purchase (IAP) Verification

### 2.1 Endpoint xác thực
- **Apple App Store:**
  - Production: `https://api.storekit.itunes.apple.com/inApps/v1/transactions/{transactionId}`
  - Sandbox: `https://api.storekit-sandbox.itunes.apple.com/inApps/v1/transactions/{transactionId}`
  - Xác thực qua App Store Server API với JWT sinh bằng private key ES256 được Apple cấp.
- **Google Play:**
  - Google Play Developer API (Android Publisher v3): `https://androidpublisher.googleapis.com/androidpublisher/v3/applications/{packageName}/purchases/products/{productId}/tokens/{token}`
  - Xác thực qua OAuth2 Service Account với quyền `androidpublisher`.
  - Sau khi `GRANTED`: gọi `.../purchases/products/{productId}/tokens/{token}:acknowledge` (retry có backoff; Google tự hoàn tiền nếu chưa acknowledge sau 3 ngày và báo qua RTDN).
- **Steam (PC, ADR-0060):** nhà cung cấp thanh toán duy nhất trên PC là Steam Microtransactions (Steamworks Web API, publisher key):
  - Khởi tạo: `POST https://partner.steam-api.com/ISteamMicroTxn/InitTxn/v3/` (server gọi khi client gọi `/api/v1/iap/steam/init`; `orderid` do server sinh, 64-bit, lưu làm `platform_receipt = orderid`).
  - Hoàn tất: `POST https://partner.steam-api.com/ISteamMicroTxn/FinalizeTxn/v2/` sau callback `MicroTxnAuthorizationResponse_t` phía client; trạng thái `QueryTxn/v3` là nguồn quyết định duy nhất (ánh xạ trạng thái, timeout `Init` 24 h, worker 15 phút: `validation.md` § IAP Receipt Verification).
  - Hoàn tiền/chargeback: không có webhook; worker gọi `GetReport/v5` (`type=GAMESALES`) mỗi 10 phút với con trỏ `time` bền vững, xử lý các dòng `Refunded`/`Chargedback` giống notification refund (dedup theo `orderid + status` trong `iap_notification_dedup`).
  - Sandbox: `ISteamMicroTxnSandbox` khi `IAP_SANDBOX=true`.
- **Client submission:** mọi nền tảng gửi kết quả mua qua `POST /api/v1/iap/verify` (`validation.md` § IAP Receipt Verification); "regional payment gateway" trong `../03_systems/monetization.md` = Steam trên PC.

### 2.2 Server Notifications & Webhooks
- **Apple Server Notifications v2:** Nhận qua webhook endpoint `/api/v1/iap/apple/webhook`. Payload là signed JWS. Server verify chữ ký bằng chứng chỉ Apple root CA.
- **Google Cloud Pub/Sub RTDN:** Nhận qua webhook endpoint `/api/v1/iap/google/webhook`. Bắt buộc xác thực OIDC JWT Bearer Token trong header `Authorization` do Google gửi kèm theo [Google Pub/Sub Push Authentication](https://docs.cloud.google.com/pubsub/docs/authenticate-push-subscriptions), kiểm tra `aud` = `GOOGLE_RTDN_AUDIENCE` và `email` = `GOOGLE_RTDN_SERVICE_ACCOUNT_EMAIL` (§ 4).
- **Idempotency:** Mỗi notification chứa `notificationUUID` (Apple) hoặc `messageId` (Google). Server ghi nhận vào bảng `iap_notification_dedup` trước khi xử lý, loại trừ trùng lặp.

## 3. Shared Auth Rate-Limiting (Không dùng Redis)

Quy chuẩn điều phối giới hạn tốc độ yêu cầu (Rate Limiting) trong tiến trình world duy nhất (ADR-0052) mà không vi phạm quy tắc cấm Redis:

1. **Local In-Process Tier (Level 1):**
   - Edge subsystem duy trì in-memory Token Bucket cài bằng Go standard library (deterministic refill theo `time.Now`, không dependency ngoài).
   - Áp dụng cho kết nối TCP thô, chống tấn công SYN flood và spam frame WebSocket.
2. **Global Database Tier (Level 2):**
   - Áp dụng cho các thao tác nhạy cảm cần sống sót qua restart: auth endpoints, gameplay ticket, IAP (`rate_limits.md` § Authentication).
   - Khóa theo phạm vi (ADR-0064): `key_hash = HMAC-SHA-256(key = ACCOUNT_SIGNAL_SALT, action || ':' || scope_kind || ':' || scope_value)` với `scope_kind ∈ ACCOUNT | USERNAME | IP | IP_DEVICE` (`scope_value` = `account_id`, `username_key`, IP /32 IPv4 · /64 IPv6, hoặc IP + ':' + `device_id`) (ADR-0069: khóa có salt, không tính ngược được từ username/IP). Mỗi (action, scope_kind) có `limit` và `window_seconds` riêng trong `rate_limits.md`.
   - Bảng PostgreSQL `rate_limit_counters` (cửa sổ trượt xấp xỉ 2 cửa sổ cố định):
     ```text
     key_hash         BYTEA PRIMARY KEY
     window_seconds   INTEGER NOT NULL
     window_start     TIMESTAMPTZ NOT NULL   -- date_bin(window_seconds, now(), epoch)
     current_count    INTEGER NOT NULL
     previous_count   INTEGER NOT NULL DEFAULT 0
     ```
     Upsert atomic: nếu `window_start` hiện tại khớp thì `current_count + 1`; nếu là cửa sổ kế tiếp thì `previous_count = current_count, current_count = 1`; xa hơn thì cả hai reset (`previous_count = 0, current_count = 1`).
     `effective = current_count + previous_count × (1 − elapsed_in_window / window_seconds)`; `effective > limit` → `RATE_LIMITED` (BACKOFF, `retry_after_ms` = thời gian tới khi `effective ≤ limit`). Hàng không đổi trong 24 h bị xóa bởi job dọn dẹp.
   - Backoff lũy tiến cho đăng nhập sai (`auth_failure_backoff`):
     ```text
     key_hash               BYTEA PRIMARY KEY   -- HMAC-SHA-256(ACCOUNT_SIGNAL_SALT, 'auth.password.login:USERNAME:' || username_key), cùng cho 'IP'
     consecutive_failures   INTEGER NOT NULL
     locked_until           TIMESTAMPTZ NULL
     last_failure_at        TIMESTAMPTZ NOT NULL
     ```
     Mỗi lần sai: `consecutive_failures + 1`; từ lần thứ 5 trở đi `locked_until = now + min(30 s × 2^(consecutive_failures − 5), 900 s)`. Trong lúc khóa, mật khẩu **vẫn được kiểm tra**: đúng thì đăng nhập thành công; sai thì trả `RATE_LIMITED` với `retry_after_ms` (cùng hình dạng/độ trễ với `AUTH_INVALID`, không lộ username tồn tại) (ADR-0069). Đăng nhập đúng xóa hàng USERNAME đó và hàng IP của IP nguồn; `consecutive_failures` giảm 1 mỗi 10 phút không có lỗi mới (tính khi đọc: `max(0, n − floor((now − last_failure_at)/600 s))`); hàng không có lỗi trong 24 h bị xóa.

## 4. Environment Config & Secrets Schema

Mọi cấu hình môi trường được nạp qua biến môi trường tiêu chuẩn (không commit file `.env` vào repository):

| Biến môi trường | Bắt buộc | Mô tả |
|---|---|---|
| `DATABASE_URL` | Có | Chuỗi kết nối PostgreSQL (pgx pool format) |
| `SERVER_PORT` | Có | Cổng TCP mở listener WSS (mặc định 8080) |
| `WORLD_CCU_CAP` | Có | Số session attached tối đa trước khi bật login queue; đặt bằng CCU đo được ở release gate 10k (`../08_scale_ops/capacity.md`, `session.md` § Login Queue, ADR-0052) |
| `IAP_SANDBOX` | Không | `true` \| `false` (mặc định `false`) |
| `APPLE_KEY_ID` | Khi bật IAP | Key ID cấp bởi Apple Developer Portal |
| `APPLE_ISSUER_ID` | Khi bật IAP | Issuer ID của App Store Connect |
| `APPLE_PRIVATE_KEY_PEM` | Khi bật IAP | Nội dung private key ES256 |
| `OPERATOR_TOTP_KEY` | Luôn luôn | 32-byte key (base64) mã hóa AES-256-GCM cho `operators.totp_secret_encrypted` (`auth.md` § Operator) |
| `GOOGLE_SERVICE_ACCOUNT_JSON` | Khi bật IAP | Nội dung JSON của Google Service Account |
| `STEAM_APP_ID` | Khi bật IAP | Steam App ID của game |
| `STEAM_PUBLISHER_KEY` | Khi bật IAP | Steamworks Web API publisher key (MicroTxn, GetReport) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Không | Địa chỉ OpenTelemetry collector |
| `GOOGLE_OIDC_CLIENT_IDS` | Có | Danh sách OAuth client ID (phân tách bằng dấu phẩy: Android, Windows) chấp nhận làm `aud` của Google ID token (§ 1.1) |
| `APPLE_SIGNIN_CLIENT_IDS` | Có | Danh sách Apple Services ID / bundle ID chấp nhận làm `aud` của Apple identity token (§ 1.1) |
| `APPLE_BUNDLE_ID` | Khi bật IAP | Bundle ID dùng cho App Store Server API và kiểm tra `bundleId` của giao dịch |
| `GOOGLE_PLAY_PACKAGE_NAME` | Khi bật IAP | `packageName` của ứng dụng Android cho Android Publisher API |
| `GOOGLE_RTDN_AUDIENCE` | Khi bật IAP | `aud` bắt buộc của OIDC token Pub/Sub push (= URL webhook `/api/v1/iap/google/webhook`) |
| `GOOGLE_RTDN_SERVICE_ACCOUNT_EMAIL` | Khi bật IAP | `email` bắt buộc của OIDC token Pub/Sub push |
| `ADMIN_BIND_ADDR` | Luôn luôn | `host:port` của listener admin HTTPS trên mạng vận hành riêng (`auth.md` § Operator); không bao giờ là địa chỉ public |
| `TLS_TERMINATION` | Có | `SERVER` \| `PROXY` (`../05_network/protocol.md` § TLS) |
| `TLS_CERT_FILE`, `TLS_KEY_FILE` | Khi `TLS_TERMINATION=SERVER` | Đường dẫn chứng chỉ/khóa PEM của listener public |
| `ACCOUNT_SIGNAL_SALT` | Luôn luôn | 32-byte (base64) salt cho `account_login_history` hash (`../06_data/data_model.md`) và khóa HMAC của `rate_limit_counters` / `auth_failure_backoff` (§ 3) |
| `ERASURE_LEDGER_SALT` | Luôn luôn | 32-byte (base64) salt cho `account_id_hash` của erasure ledger (`../08_scale_ops/backup_recovery.md`) |
| `BACKUP_STORAGE_URL` | Luôn luôn | URL kho S3-compatible của Owner Setup cho erasure ledger (world process chỉ ghi thêm prefix `erasure-ledger/`; `../08_scale_ops/backup_recovery.md`, IMP-056) |
| `BACKUP_STORAGE_CREDENTIALS_FILE` | Luôn luôn | Đường dẫn file credential chỉ-ghi-thêm cho prefix đó (quyền 0600, world host) |
| `PGBACKREST_REPO1_S3_KEY`, `PGBACKREST_REPO1_S3_KEY_SECRET`, `PGBACKREST_REPO1_CIPHER_PASS` | Trên PostgreSQL host | Credential repository và mật khẩu mã hóa của pgBackRest (biến môi trường gốc của pgBackRest; không nạp vào world process) |

## Invariants

```text
first-party password = enabled (provider `password`, ADR-0051); federated: Apple, Google, Steam; no password reset at launch
IAP verification = server-to-server qua Apple/Google/Steam official APIs; client chỉ gửi receipt qua POST /api/v1/iap/verify
rate limiting = in-memory L1 + PostgreSQL L2; cấm dùng Redis
secrets = environment variables only; không hardcode trong repo
```
