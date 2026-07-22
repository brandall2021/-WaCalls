package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

type WebhookEvent struct {
	Type      string         `json:"type"`
	SessionID string         `json:"session_id"`
	Timestamp int64          `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

type WebhookDispatcher struct {
	store  *webhookStore
	log    *slog.Logger
	client *http.Client
	mu     sync.Mutex
}

func NewWebhookDispatcher(store *webhookStore, log *slog.Logger) *WebhookDispatcher {
	return &WebhookDispatcher{
		store: store,
		log:   log,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func generateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (d *WebhookDispatcher) Dispatch(sessionID string, event WebhookEvent) {
	configs, err := d.store.ListActive(nil, sessionID)
	if err != nil {
		d.log.Error("webhook: failed to list configs", "err", err)
		return
	}
	if len(configs) == 0 {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		d.log.Error("webhook: failed to marshal event", "err", err)
		return
	}

	for _, cfg := range configs {
		if !cfg.EventsMatches(event.Type) {
			continue
		}
		go d.deliver(cfg, event.Type, payload)
	}
}

func (d *WebhookDispatcher) deliver(cfg WebhookConfigRow, eventType string, payload []byte) {
	deliveryID := newSessionID()
	sig := signPayload(payload, cfg.Secret)

	headers := map[string]string{
		"Content-Type":        "application/json",
		"X-Webhook-Signature": sig,
		"X-Webhook-Event":     eventType,
		"X-Webhook-ID":        deliveryID,
	}

	var lastErr error
	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest("POST", cfg.URL, bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			break
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			d.log.Warn("webhook delivery failed", "config_id", cfg.ID, "attempt", attempt, "err", err)
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				d.store.SaveDelivery(nil, &WebhookDeliveryRow{
					ID:         deliveryID,
					ConfigID:   cfg.ID,
					EventType:  eventType,
					Payload:    string(payload),
					Status:     "delivered",
					StatusCode: resp.StatusCode,
					Attempts:   attempt,
				})
				d.log.Info("webhook delivered", "config_id", cfg.ID, "event", eventType, "status", resp.StatusCode)
				return
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			d.log.Warn("webhook delivery rejected", "config_id", cfg.ID, "attempt", attempt, "status", resp.StatusCode)
		}

		if attempt < maxAttempts {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			time.Sleep(backoff)
		}
	}

	d.store.SaveDelivery(nil, &WebhookDeliveryRow{
		ID:        deliveryID,
		ConfigID:  cfg.ID,
		EventType: eventType,
		Payload:   string(payload),
		Status:    "failed",
		Attempts:  maxAttempts,
	})
	d.log.Error("webhook delivery failed after retries", "config_id", cfg.ID, "event", eventType, "err", lastErr)
}

func (w *WebhookConfigRow) EventsMatches(eventType string) bool {
	if w.Events == "*" {
		return true
	}
	for _, e := range strings.Split(w.Events, ",") {
		if strings.TrimSpace(e) == eventType {
			return true
		}
	}
	return false
}
