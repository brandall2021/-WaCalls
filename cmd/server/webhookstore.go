package main

import (
	"context"
	"database/sql"
)

type WebhookConfigRow struct {
	ID        string
	SessionID string
	URL       string
	Events    string
	Secret    string
	Active    bool
}

type WebhookDeliveryRow struct {
	ID         string
	ConfigID   string
	EventType  string
	Payload    string
	Status     string
	StatusCode int
	Attempts   int
}

type webhookStore struct{ db *sql.DB }

func newWebhookStore(ctx context.Context, db *sql.DB) (*webhookStore, error) {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS webhook_configs (
			id         TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			url        TEXT NOT NULL,
			events     TEXT NOT NULL,
			secret     TEXT NOT NULL,
			active     BOOLEAN NOT NULL DEFAULT TRUE
		)
	`)
	if err != nil {
		return nil, err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id          TEXT PRIMARY KEY,
			config_id   TEXT NOT NULL,
			event_type  TEXT NOT NULL,
			payload     TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'pending',
			status_code INT,
			attempts    INT NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return nil, err
	}
	return &webhookStore{db: db}, nil
}

func (s *webhookStore) Create(ctx context.Context, w *WebhookConfigRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webhook_configs (id, session_id, url, events, secret, active) VALUES ($1, $2, $3, $4, $5, $6)`,
		w.ID, w.SessionID, w.URL, w.Events, w.Secret, w.Active,
	)
	return err
}

func (s *webhookStore) List(ctx context.Context, sessionID string) ([]WebhookConfigRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, url, events, secret, active FROM webhook_configs WHERE session_id = $1 ORDER BY id`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookConfigRow
	for rows.Next() {
		var r WebhookConfigRow
		if err := rows.Scan(&r.ID, &r.SessionID, &r.URL, &r.Events, &r.Secret, &r.Active); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *webhookStore) Get(ctx context.Context, id string) (*WebhookConfigRow, error) {
	var r WebhookConfigRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, session_id, url, events, secret, active FROM webhook_configs WHERE id = $1`,
		id,
	).Scan(&r.ID, &r.SessionID, &r.URL, &r.Events, &r.Secret, &r.Active)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *webhookStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhook_configs WHERE id = $1`, id)
	return err
}

func (s *webhookStore) ListActive(ctx context.Context, sessionID string) ([]WebhookConfigRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, url, events, secret, active FROM webhook_configs WHERE session_id = $1 AND active = TRUE`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookConfigRow
	for rows.Next() {
		var r WebhookConfigRow
		if err := rows.Scan(&r.ID, &r.SessionID, &r.URL, &r.Events, &r.Secret, &r.Active); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *webhookStore) SaveDelivery(ctx context.Context, d *WebhookDeliveryRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries (id, config_id, event_type, payload, status, status_code, attempts) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		d.ID, d.ConfigID, d.EventType, d.Payload, d.Status, d.StatusCode, d.Attempts,
	)
	return err
}

func (s *webhookStore) UpdateDelivery(ctx context.Context, d *WebhookDeliveryRow) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE webhook_deliveries SET status = $1, status_code = $2, attempts = $3 WHERE id = $4`,
		d.Status, d.StatusCode, d.Attempts, d.ID,
	)
	return err
}
