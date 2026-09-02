package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adrone13/url-shortener/internal/shortener"
)

const uniqueViolation = "23505"

type Repo struct {
	conn *pgxpool.Pool
}

func New(conn *pgxpool.Pool) *Repo {
	return &Repo{conn: conn}
}

func (r *Repo) Save(ctx context.Context, link shortener.Link) error {
	_, err := r.conn.Exec(ctx,
		`INSERT INTO links (code, original_url) VALUES ($1, $2)`,
		link.Code, link.OriginalURL,
	)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == uniqueViolation {
			return fmt.Errorf("%w: code %q already exists", shortener.ErrCodeExists, link.Code)
		}
		return fmt.Errorf("save link: %w", err)
	}

	return nil
}

func (r *Repo) Get(ctx context.Context, code string) (string, error) {
	var originalURL string

	err := r.conn.QueryRow(ctx,
		`SELECT original_url FROM links WHERE code = $1`,
		code,
	).Scan(&originalURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", shortener.ErrNotFound
		}
		return "", fmt.Errorf("get link: %w", err)
	}

	return originalURL, nil
}

func (r *Repo) List(ctx context.Context) ([]shortener.Link, error) {
	rows, err := r.conn.Query(ctx, `SELECT code, original_url FROM links`)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()

	links, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[shortener.Link])
	if err != nil {
		return nil, fmt.Errorf("scan links: %w", err)
	}

	return links, nil
}
