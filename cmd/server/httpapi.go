package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"wacalls/internal/voip/core"

	"go.mau.fi/whatsmeow/types"
)

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)

	api := http.NewServeMux()
	api.HandleFunc("GET /api/auth/me", s.handleMe)
	api.HandleFunc("GET /api/sessions", s.handleSessionList)
	api.HandleFunc("POST /api/sessions", s.handleSessionCreate)
	api.HandleFunc("DELETE /api/sessions/{sid}", s.handleSessionDelete)
	api.HandleFunc("POST /api/sessions/{sid}/logout", s.handleSessionLogout)
	api.HandleFunc("POST /api/sessions/{sid}/pair", s.handleSessionPair)
	api.HandleFunc("POST /api/sessions/{sid}/calls", s.handleStartCall)
	api.HandleFunc("POST /api/sessions/{sid}/calls/{id}/webrtc", s.handleWebRTC)
	api.HandleFunc("POST /api/sessions/{sid}/calls/{id}/accept", s.handleAccept)
	api.HandleFunc("POST /api/sessions/{sid}/calls/{id}/reject", s.handleReject)
	api.HandleFunc("DELETE /api/sessions/{sid}/calls/{id}", s.handleEndCall)
	api.HandleFunc("GET /api/sessions/{sid}/history", s.handleHistory)
	api.HandleFunc("GET /api/sessions/{sid}/recordings", s.handleListRecordings)
	api.HandleFunc("GET /api/recordings/{id}/download", s.handleDownloadRecording)
	api.HandleFunc("GET /api/sessions/{sid}/webhooks", s.handleListWebhooks)
	api.HandleFunc("POST /api/sessions/{sid}/webhooks", s.handleCreateWebhook)
	api.HandleFunc("DELETE /api/sessions/{sid}/webhooks/{wid}", s.handleDeleteWebhook)
	api.HandleFunc("GET /api/sip/status", s.handleSIPStatus)
	api.HandleFunc("POST /api/sip/call", s.handleSIPCall)
	api.HandleFunc("DELETE /api/sip/call/{callID}", s.handleSIPHangup)
	api.HandleFunc("GET /api/stats", s.handleStats)
	api.HandleFunc("GET /api/events", s.handleEvents)

	mux.Handle("/api/", s.authMiddleware(api))

	if s.staticDir != "" {
		if _, err := os.Stat(s.staticDir); err == nil {
			mux.Handle("/", http.FileServer(http.Dir(s.staticDir)))
		}
	}
	return withCORS(mux)
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Client-Id")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func clientID(r *http.Request) string {
	if id := r.Header.Get("X-Client-Id"); id != "" {
		return id
	}
	return r.URL.Query().Get("clientId")
}

func (s *server) sessionByID(w http.ResponseWriter, sid string) *Session {
	sess, ok := s.sessions.Get(sid)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such session"})
		return nil
	}
	return sess
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	s.broker.serveSSE(w, r, clientID(r))
}

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		strings.TrimSpace(body.Email) == "" || strings.TrimSpace(body.Password) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}
	user, err := s.auth.Register(r.Context(), body.Email, body.Password, body.Name)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	token, err := s.auth.GenerateToken(user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		strings.TrimSpace(body.Email) == "" || strings.TrimSpace(body.Password) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}
	user, err := s.auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	token, err := s.auth.GenerateToken(user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	uid := userIDFromContext(r.Context())
	if uid == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	user, err := s.auth.GetUserByID(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *server) handleSessionList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.sessions.infos()})
}

func (s *server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "Session"
	}
	id, err := s.sessions.Create(name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (s *server) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.sessions.Delete(r.Context(), r.PathValue("sid")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleSessionLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.sessions.Logout(r.Context(), r.PathValue("sid")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleSessionPair(w http.ResponseWriter, r *http.Request) {
	if err := s.sessions.Pair(r.PathValue("sid")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleStartCall(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r.PathValue("sid")); sess != nil {
		s.doStartCall(sess, w, r)
	}
}

func (s *server) handleWebRTC(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r.PathValue("sid")); sess != nil {
		s.doWebRTC(sess, w, r)
	}
}

func (s *server) handleAccept(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r.PathValue("sid")); sess != nil {
		s.doAccept(sess, w, r)
	}
}

func (s *server) handleReject(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r.PathValue("sid")); sess != nil {
		s.doReject(sess, w, r)
	}
}

func (s *server) handleEndCall(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r.PathValue("sid")); sess != nil {
		s.doEndCall(sess, w, r)
	}
}

func (s *server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if sess := s.sessionByID(w, r.PathValue("sid")); sess != nil {
		writeJSON(w, http.StatusOK, map[string]any{"rows": s.broker.historyRows(sess.id, 50)})
	}
}

func (s *server) doStartCall(sess *Session, w http.ResponseWriter, r *http.Request) {
	s.log.Info("doStartCall called", "session", sess.id)
	if sess.client.Store.ID == nil {
		s.log.Warn("doStartCall: not paired")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not paired"})
		return
	}
	var body struct {
		Phone      string `json:"phone"`
		DurationMs int    `json:"duration_ms"`
		Record     bool   `json:"record"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Phone) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "phone required"})
		return
	}
	owner := clientID(r)
	if other := s.broker.ownerActiveCall(owner); other != "" {
		s.log.Warn("doStartCall: operator already on a call", "owner", owner, "active_call", other)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "operator already on a call"})
		return
	}
	if sess.reg.count() > 0 {
		s.log.Warn("doStartCall: session already has active call", "count", sess.reg.count())
		writeJSON(w, http.StatusConflict, map[string]string{"error": "session already has an active call, hang up first"})
		return
	}
	if max := s.sessions.maxCalls; max > 0 && sess.reg.count() >= max {
		s.log.Warn("doStartCall: max concurrent calls", "count", sess.reg.count(), "max", max)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "max concurrent calls"})
		return
	}
	peer := types.NewJID(normalizePhone(body.Phone), types.DefaultUserServer)

	s.log.Info("doStartCall: starting outgoing call", "phone", body.Phone, "peer", peer.String(), "owner", owner)
	callID, err := sess.startOutgoing(r.Context(), peer, false)
	if err != nil {
		s.log.Error("doStartCall: startOutgoing failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.broker.upsertCall(CallRecord{
		SessionID: sess.id, CallID: callID, Owner: &owner, Direction: "outbound", Peer: peer.String(),
		StartedAt: time.Now().UnixMilli(), Status: StatusRinging,
	})
	if body.DurationMs > 0 {
		go func() {
			time.Sleep(time.Duration(body.DurationMs) * time.Millisecond)
			s.log.Info("doStartCall: call duration expired, ending", "call_id", callID, "duration_ms", body.DurationMs)
			sess.terminateCall(callID, core.EndCallReasonTimeout)
		}()
	}
	s.log.Info("doStartCall: call created", "call_id", callID)
	writeJSON(w, http.StatusOK, map[string]any{"call": map[string]string{"callId": callID}})
}

func (s *server) doWebRTC(sess *Session, w http.ResponseWriter, r *http.Request) {
	callID := r.PathValue("id")
	s.log.Info("doWebRTC called", "session", sess.id, "call_id", callID)
	ac, ok := sess.reg.get(callID)
	if !ok {
		s.log.Warn("doWebRTC: call not found", "call_id", callID)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such call"})
		return
	}
	var body struct {
		SDPOffer string `json:"sdp_offer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SDPOffer == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sdp_offer required"})
		return
	}
	bridge, answer, err := NewBridge(body.SDPOffer, s.log)
	if err != nil {
		s.log.Error("doWebRTC: bridge creation failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	bridge.OnBrowserPCM = func(pcm []float32) {
		if ac.recorder != nil {
			ac.recorder.WritePCM(pcm)
		}
		ac.cm.FeedCapturedPCM(pcm)
	}
	bridge.OnTerminalICE = func() {
		go sess.terminateCall(callID, core.EndCallReasonUserEnded)
	}
	sess.setBridge(callID, bridge)
	s.log.Info("doWebRTC: bridge created successfully", "call_id", callID)
	writeJSON(w, http.StatusOK, map[string]string{"sdp_answer": answer})
}

func (s *server) doAccept(sess *Session, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.log.Info("doAccept called", "session", sess.id, "call_id", id)
	ac, ok := sess.reg.get(id)
	if !ok {
		s.log.Warn("doAccept: call not found in registry", "call_id", id)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such call"})
		return
	}
	owner := clientID(r)
	if other := s.broker.ownerActiveCall(owner); other != "" && other != id {
		s.log.Warn("doAccept: operator already on a call", "owner", owner, "active_call", other)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "operator already on a call"})
		return
	}
	if !s.broker.setOwner(id, owner) {
		s.log.Warn("doAccept: call claimed by another client", "call_id", id, "owner", owner)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "claimed by another client"})
		return
	}
	s.broker.emitIncomingClaimed(sess.id, id, owner)
	s.log.Info("doAccept: accepting call", "call_id", id, "owner", owner)
	if err := ac.cm.AcceptCall(r.Context(), id); err != nil {
		s.log.Error("doAccept: AcceptCall failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.log.Info("doAccept: call accepted successfully", "call_id", id)
	writeJSON(w, http.StatusOK, map[string]any{"call": map[string]string{"callId": id}})
}

func (s *server) doReject(sess *Session, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.log.Info("doReject called", "session", sess.id, "call_id", id)
	if ac, ok := sess.reg.get(id); ok {
		_ = ac.cm.RejectCall(r.Context(), id, core.EndCallReasonDeclined)
	}
	sess.removeCall(id)
	s.broker.endCall(id, string(core.EndCallReasonDeclined))
	s.log.Info("doReject: call rejected", "call_id", id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) doEndCall(sess *Session, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.log.Info("doEndCall called", "session", sess.id, "call_id", id)
	if ac, ok := sess.reg.get(id); ok {
		s.log.Info("doEndCall: ending call via CallManager", "call_id", id)
		_ = ac.cm.EndCall(r.Context(), core.EndCallReasonUserEnded)
	} else {
		s.log.Warn("doEndCall: call not found in registry, cleaning broker only", "call_id", id)
	}
	sess.removeCall(id)
	s.broker.endCall(id, string(core.EndCallReasonUserEnded))
	s.log.Info("doEndCall: cleanup done", "call_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func normalizePhone(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "+")
	var b strings.Builder
	for _, c := range p {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func (s *server) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	sess := s.sessionByID(w, r.PathValue("sid"))
	if sess == nil {
		return
	}
	recordings, err := s.sessions.recordingStore.List(r.Context(), sess.id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, recordings)
}

func (s *server) handleDownloadRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := s.sessions.recordingStore.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "recording not found"})
		return
	}
	file, err := os.Open(rec.FilePath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.wav", rec.CallID))
	http.ServeContent(w, r, rec.FilePath, time.Time{}, file)
}

func (s *server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	sess := s.sessionByID(w, r.PathValue("sid"))
	if sess == nil {
		return
	}
	webhooks, err := s.sessions.webhookStore.List(r.Context(), sess.id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, webhooks)
}

func (s *server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	sess := s.sessionByID(w, r.PathValue("sid"))
	if sess == nil {
		return
	}
	var body struct {
		URL    string `json:"url"`
		Events string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url required"})
		return
	}
	if body.Events == "" {
		body.Events = "*"
	}
	cfg := &WebhookConfigRow{
		ID:        newSessionID(),
		SessionID: sess.id,
		URL:       body.URL,
		Events:    body.Events,
		Secret:    generateSecret(),
		Active:    true,
	}
	if err := s.sessions.webhookStore.Create(r.Context(), cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	sess := s.sessionByID(w, r.PathValue("sid"))
	if sess == nil {
		return
	}
	wid := r.PathValue("wid")
	if err := s.sessions.webhookStore.Delete(r.Context(), wid); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleSIPStatus(w http.ResponseWriter, r *http.Request) {
	if s.sipUA == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	calls := s.sipUA.ListCalls()
	type callInfo struct {
		ID    string `json:"id"`
		Peer  string `json:"peer"`
		State int    `json:"state"`
	}
	var list []callInfo
	for _, c := range calls {
		list = append(list, callInfo{ID: c.ID, Peer: c.RemoteURI, State: int(c.State)})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":  true,
		"asterisk": s.sipUA.Config().AsteriskAddr,
		"ext":      s.sipUA.Config().Extension,
		"calls":    list,
	})
}

func (s *server) handleSIPCall(w http.ResponseWriter, r *http.Request) {
	if s.sipUA == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "SIP bridge not enabled"})
		return
	}
	var body struct {
		PeerURI string `json:"peer_uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PeerURI == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "peer_uri required"})
		return
	}
	callID, err := s.sipUA.MakeCall(body.PeerURI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"call_id": callID})
}

func (s *server) handleSIPHangup(w http.ResponseWriter, r *http.Request) {
	if s.sipUA == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "SIP bridge not enabled"})
		return
	}
	callID := r.PathValue("callID")
	if err := s.sipUA.Hangup(callID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	totalSessions := 0
	connectedSessions := 0
	for _, info := range s.sessions.infos() {
		totalSessions++
		if info.State == "open" {
			connectedSessions++
		}
	}

	var totalRecordings int64
	var totalDurationMs int64
	var totalFileSize int64
	_ = s.sessions.recordingStore.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(duration), 0), COALESCE(SUM(file_size), 0) FROM recordings`,
	).Scan(&totalRecordings, &totalDurationMs, &totalFileSize)

	var totalWebhooks int
	_ = s.sessions.recordingStore.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM webhook_configs`,
	).Scan(&totalWebhooks)

	var delivered int
	var failed int
	_ = s.sessions.recordingStore.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM webhook_deliveries WHERE status = 'delivered'`,
	).Scan(&delivered)
	_ = s.sessions.recordingStore.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM webhook_deliveries WHERE status = 'failed'`,
	).Scan(&failed)

	activeCalls := 0
	s.sessions.broker.mu.RLock()
	for _, c := range s.sessions.broker.calls {
		if c.Status != StatusEnded {
			activeCalls++
		}
	}
	s.sessions.broker.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": map[string]any{
			"total":     totalSessions,
			"connected": connectedSessions,
		},
		"calls": map[string]any{
			"active": activeCalls,
		},
		"recordings": map[string]any{
			"total":       totalRecordings,
			"duration_ms": totalDurationMs,
			"size_bytes":  totalFileSize,
		},
		"webhooks": map[string]any{
			"configs":   totalWebhooks,
			"delivered": delivered,
			"failed":    failed,
		},
	})
}
