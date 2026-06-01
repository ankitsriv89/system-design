// Package store contains all external persistence adapters.
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/lib/pq"

	"github.com/ankitsriv89/pastebin/paste"
)

// DB implements paste.Repository using PostgreSQL.
type DB struct {
	db *sql.DB
}

func NewDB(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	return &DB{db: db}, nil
}

func (d *DB) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *DB) Save(ctx context.Context, p *paste.Paste) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO pastes
			(id, owner_id, title, language, visibility, size_bytes, object_key, expires_at, burn_after_read, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		p.ID, nullStr(p.OwnerID), nullStr(p.Title), nullStr(p.Language),
		string(p.Visibility), p.SizeBytes, p.ObjectKey,
		p.ExpiresAt, p.BurnAfterRead, p.CreatedAt,
	)
	return err
}

func (d *DB) Get(ctx context.Context, id string) (*paste.Paste, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT id, owner_id, title, language, visibility, size_bytes, object_key,
		       expires_at, burn_after_read, created_at
		FROM pastes WHERE id = $1`, id)

	p := &paste.Paste{}
	var ownerID, title, language sql.NullString
	err := row.Scan(
		&p.ID, &ownerID, &title, &language,
		&p.Visibility, &p.SizeBytes, &p.ObjectKey,
		&p.ExpiresAt, &p.BurnAfterRead, &p.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, paste.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.OwnerID = ownerID.String
	p.Title = title.String
	p.Language = language.String
	return p, nil
}

func (d *DB) Delete(ctx context.Context, id string) error {
	res, err := d.db.ExecContext(ctx, `DELETE FROM pastes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return paste.ErrNotFound
	}
	return nil
}

func (d *DB) ListExpired(ctx context.Context, limit int) ([]string, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id FROM pastes WHERE expires_at IS NOT NULL AND expires_at < NOW() LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return ids, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
