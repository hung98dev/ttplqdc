package auth

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// Handler serves the 12 auth/account HTTPS routes of auth.md
// § HTTPS Endpoints on a stdlib mux.
type Handler struct {
	svc *Service
	mux *http.ServeMux
}

// NewHandler mounts the endpoint table.
func NewHandler(svc *Service) *Handler {
	h := &Handler{svc: svc, mux: http.NewServeMux()}
	h.mux.HandleFunc("POST /api/v1/auth/login/{provider}", h.federatedLogin)
	h.mux.HandleFunc("POST /api/v1/auth/password/register", h.register)
	h.mux.HandleFunc("POST /api/v1/auth/password/login", h.passwordLogin)
	h.mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	h.mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	h.mux.HandleFunc("POST /api/v1/auth/password/change", h.passwordChange)
	h.mux.HandleFunc("POST /api/v1/auth/link/{provider}", h.link)
	h.mux.HandleFunc("POST /api/v1/auth/unlink/{provider}", h.unlink)
	h.mux.HandleFunc("GET /api/v1/account", h.getAccount)
	h.mux.HandleFunc("POST /api/v1/gameplay/ticket", h.ticket)
	h.mux.HandleFunc("POST /api/v1/account/delete", h.deleteAccount)
	h.mux.HandleFunc("POST /api/v1/account/delete/cancel", h.cancelDelete)
	return h
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// ---------------------------------------------------------------------------
// transport plumbing

type errorBody struct {
	ErrorCode      string `json:"error_code"`
	Retryability   string `json:"retryability"`
	RetryAfterMs   int64  `json:"retry_after_ms,omitempty"`
	QueuePosition  int32  `json:"queue_position,omitempty"`
	SafeMessageKey string `json:"safe_message_key"`
}

// writeError serializes the wire error body (errors.md).
func writeError(w http.ResponseWriter, err error) {
	ae := AsAPIError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.Status)
	_ = json.NewEncoder(w).Encode(errorBody{
		ErrorCode:      ae.Code,
		Retryability:   ae.Retryability,
		RetryAfterMs:   ae.RetryAfterMs,
		QueuePosition:  ae.QueuePosition,
		SafeMessageKey: ae.SafeMessageKey,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decode reads a JSON object body; empty body decodes to the zero value.
func decode(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64<<10))
	if err := dec.Decode(v); err != nil {
		if err.Error() == "EOF" {
			return nil
		}
		return ErrBadRequest
	}
	return nil
}

// bearer extracts the Authorization: Bearer token.
func bearer(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", ErrAuthInvalid
	}
	return strings.TrimPrefix(h, "Bearer "), nil
}

// clientIP resolves the effective client IP. With proxy TLS termination
// the listener runs behind a loopback hop: X-Forwarded-For is honored
// only when the direct peer is loopback, and the first entry is the
// client (external_integrations.md §4).
func clientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer != nil && peer.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
			if ip := net.ParseIP(first); ip != nil {
				return ip
			}
		}
	}
	return peer
}

// meta builds LoginMeta from the request + body fields.
func meta(r *http.Request, deviceID string, clientBuild uint32, platformStr string) LoginMeta {
	return LoginMeta{
		IP:         clientIP(r),
		DeviceID:   deviceID,
		Platform:   platformStr,
		AppVersion: r.Header.Get("X-Client-Version"),
	}
}

func parsePlatform(s string) protocolv1.ClientPlatform {
	switch strings.ToUpper(s) {
	case "WINDOWS":
		return protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS
	case "ANDROID":
		return protocolv1.ClientPlatform_CLIENT_PLATFORM_ANDROID
	}
	return protocolv1.ClientPlatform_CLIENT_PLATFORM_UNSPECIFIED
}

// ---------------------------------------------------------------------------
// request/response bodies

type tokenResponseBody struct {
	AccountID        string `json:"account_id"`
	AccessToken      string `json:"access_token"`
	AccessExpiresAt  string `json:"access_expires_at"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresAt string `json:"refresh_expires_at"`
	IsNewAccount     bool   `json:"is_new_account"`
}

func tokenBody(t TokenResponse) tokenResponseBody {
	return tokenResponseBody{
		AccountID:        t.AccountID.String(),
		AccessToken:      t.AccessToken,
		AccessExpiresAt:  t.AccessExpiresAt.Format(time.RFC3339),
		RefreshToken:     t.RefreshToken,
		RefreshExpiresAt: t.RefreshExpiresAt.Format(time.RFC3339),
		IsNewAccount:     t.IsNewAccount,
	}
}

// ---------------------------------------------------------------------------
// endpoints

type federatedLoginReq struct {
	ProviderToken string `json:"provider_token"`
	DeviceID      string `json:"device_id"`
	ClientBuild   uint32 `json:"client_build"`
	Platform      string `json:"platform"`
	Nonce         string `json:"nonce"`
}

func (h *Handler) federatedLogin(w http.ResponseWriter, r *http.Request) {
	var req federatedLoginReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	resp, err := h.svc.FederatedLogin(r.Context(), r.PathValue("provider"),
		req.ProviderToken, req.Nonce, meta(r, req.DeviceID, req.ClientBuild, req.Platform))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokenBody(resp))
}

type registerReq struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Email       string `json:"email"`
	DeviceID    string `json:"device_id"`
	ClientBuild uint32 `json:"client_build"`
	Platform    string `json:"platform"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	resp, err := h.svc.Register(r.Context(), req.Username, req.Email, req.Password,
		meta(r, req.DeviceID, req.ClientBuild, req.Platform))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokenBody(resp))
}

type passwordLoginReq struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DeviceID    string `json:"device_id"`
	ClientBuild uint32 `json:"client_build"`
	Platform    string `json:"platform"`
}

func (h *Handler) passwordLogin(w http.ResponseWriter, r *http.Request) {
	var req passwordLoginReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	resp, err := h.svc.PasswordLogin(r.Context(), req.Username, req.Password,
		meta(r, req.DeviceID, req.ClientBuild, req.Platform))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokenBody(resp))
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	resp, err := h.svc.Refresh(r.Context(), req.RefreshToken, req.DeviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokenBody(resp))
}

type logoutReq struct {
	Scope string `json:"scope"`
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	tok, err := bearer(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req logoutReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if req.Scope == "" {
		req.Scope = "SESSION"
	}
	if err := h.svc.Logout(r.Context(), tok, req.Scope); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type passwordChangeReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) passwordChange(w http.ResponseWriter, r *http.Request) {
	tok, err := bearer(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req passwordChangeReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	resp, err := h.svc.PasswordChange(r.Context(), tok, req.CurrentPassword, req.NewPassword)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokenBody(resp))
}

type linkReq struct {
	ProviderToken   string `json:"provider_token"`
	CurrentPassword string `json:"current_password"`
	Nonce           string `json:"nonce"`
}

func (h *Handler) link(w http.ResponseWriter, r *http.Request) {
	tok, err := bearer(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req linkReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Link(r.Context(), tok, r.PathValue("provider"),
		req.ProviderToken, req.CurrentPassword, req.Nonce)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

type unlinkReq struct {
	CurrentPassword string `json:"current_password"`
	ProviderToken   string `json:"provider_token"`
	Nonce           string `json:"nonce"`
}

func (h *Handler) unlink(w http.ResponseWriter, r *http.Request) {
	tok, err := bearer(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req unlinkReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Unlink(r.Context(), tok, r.PathValue("provider"),
		req.CurrentPassword, req.ProviderToken, req.Nonce)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}

func (h *Handler) getAccount(w http.ResponseWriter, r *http.Request) {
	tok, err := bearer(r)
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := h.svc.GetAccount(r.Context(), tok)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type ticketReq struct {
	ClientBuild     uint32 `json:"client_build"`
	Platform        string `json:"platform"`
	ProtocolMajor   uint32 `json:"protocol_major"`
	ProtocolMinor   uint32 `json:"protocol_minor"`
	ContentRevision string `json:"content_revision"`
}

func (h *Handler) ticket(w http.ResponseWriter, r *http.Request) {
	tok, err := bearer(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req ticketReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	res, err := h.svc.Ticket(r.Context(), tok, req.ClientBuild,
		parsePlatform(req.Platform), req.ProtocolMajor, req.ProtocolMinor,
		req.ContentRevision)
	if err != nil {
		writeError(w, err)
		return
	}
	if res.Credential == "" {
		// Queue position answer: 503 SERVER_OVERLOADED carrying
		// queue_position + retry_after_ms (auth.md § gameplay/ticket).
		writeError(w, ErrServerOverloaded(res.QueuePosition, res.RetryAfterMs))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ticket":             res.Credential,
		"ticket_expires_at":  res.ExpiresAt.Format(time.RFC3339),
		"wss_url":            res.WSSURL,
		"protocol_minor_min": res.ProtocolMinMinor,
		"client_build_min":   res.MinBuild,
		"content_revision":   res.ContentRevision,
	})
}

type deleteReq struct {
	CurrentPassword string `json:"current_password"`
	ProviderToken   string `json:"provider_token"`
	Nonce           string `json:"nonce"`
}

func (h *Handler) deleteAccount(w http.ResponseWriter, r *http.Request) {
	tok, err := bearer(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req deleteReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.DeleteAccount(r.Context(), tok,
		req.CurrentPassword, req.ProviderToken, req.Nonce); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) cancelDelete(w http.ResponseWriter, r *http.Request) {
	tok, err := bearer(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.CancelDeletion(r.Context(), tok); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// compile-time context use.
var _ = context.Background
