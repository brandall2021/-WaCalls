package main

import (
	"context"
	"database/sql"
)

type RecordingRow struct {
	ID        string
	SessionID string
	CallID    string
	Duration  int64
	FilePath  string
	FileSize  int64
}

type recordingStore struct{ db *sql.DB }

func newRecordingStore(ctx context.Context, db *sql.DB) (*recordingStore, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS recordings (
		id         TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		call_id    TEXT NOT NULL,
		duration   BIGINT,
		file_path  TEXT,
		file_size  BIGINT
	)`)
	if err != nil {
		return nil, err
	}
	return &recordingStore{db: db}, nil
}

func (s *recordingStore) Save(ctx context.Context, r *RecordingRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO recordings (id, session_id, call_id, duration, file_path, file_size) VALUES ($1, $2, $3, $4, $5, $6)`,
		r.ID, r.SessionID, r.CallID, r.Duration, r.FilePath, r.FileSize,
	)
	return err
}

func (s *recordingStore) Update(ctx context.Context, id string, duration, fileSize int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE recordings SET duration = $1, file_size = $2 WHERE id = $3`,
		duration, fileSize, id,
	)
	return err
}

func (s *recordingStore) List(ctx context.Context, sessionID string) ([]RecordingRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, call_id, duration, file_path, file_size FROM recordings WHERE session_id = $1 ORDER BY id`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecordingRow
	for rows.Next() {
		var r RecordingRow
		if err := rows.Scan(&r.ID, &r.SessionID, &r.CallID, &r.Duration, &r.FilePath, &r.FileSize); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *recordingStore) Get(ctx context.Context, id string) (*RecordingRow, error) {
	var r RecordingRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, session_id, call_id, duration, file_path, file_size FROM recordings WHERE id = $1`,
		id,
	).Scan(&r.ID, &r.SessionID, &r.CallID, &r.Duration, &r.FilePath, &r.FileSize)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
