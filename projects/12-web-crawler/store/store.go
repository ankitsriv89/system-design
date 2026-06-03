// Package store provides PostgreSQL and Redis persistence for the web crawler.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/ankitsriv89/12-web-crawler/crawler"
)

// DB wraps a *sql.DB with crawl-specific queries.
type DB struct {
	db *sql.DB
}

// NewDB opens a PostgreSQL connection pool.
func NewDB(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &DB{db: db}, nil
}

// Close closes the underlying pool.
func (d *DB) Close() { d.db.Close() }

// CreateJob inserts a new crawl job and returns it with the assigned ID.
func (d *DB) CreateJob(ctx context.Context, seedURL string, maxDepth int) (crawler.Job, error) {
	var j crawler.Job
	err := d.db.QueryRowContext(ctx,
		`INSERT INTO crawl_jobs(seed_url, max_depth, status, created_at)
		 VALUES($1, $2, 'pending', NOW())
		 RETURNING id, seed_url, max_depth, status, created_at`,
		seedURL, maxDepth,
	).Scan(&j.ID, &j.SeedURL, &j.MaxDepth, &j.Status, &j.CreatedAt)
	if err != nil {
		return j, fmt.Errorf("store: create job: %w", err)
	}
	return j, nil
}

// GetJob returns a crawl job by ID.
func (d *DB) GetJob(ctx context.Context, id int64) (crawler.Job, error) {
	var j crawler.Job
	err := d.db.QueryRowContext(ctx,
		`SELECT id, seed_url, max_depth, status, created_at FROM crawl_jobs WHERE id=$1`, id,
	).Scan(&j.ID, &j.SeedURL, &j.MaxDepth, &j.Status, &j.CreatedAt)
	if err != nil {
		return j, fmt.Errorf("store: get job %d: %w", id, err)
	}
	return j, nil
}

// ListJobs returns the most recent N crawl jobs.
func (d *DB) ListJobs(ctx context.Context, limit int) ([]crawler.Job, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, seed_url, max_depth, status, created_at
		 FROM crawl_jobs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list jobs: %w", err)
	}
	defer rows.Close()
	var jobs []crawler.Job
	for rows.Next() {
		var j crawler.Job
		if err := rows.Scan(&j.ID, &j.SeedURL, &j.MaxDepth, &j.Status, &j.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: list jobs scan: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// UpdateJobStatus sets a job's status.
func (d *DB) UpdateJobStatus(ctx context.Context, id int64, status string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE crawl_jobs SET status=$1 WHERE id=$2`, status, id)
	return err
}

// EnqueueURL inserts a URL into the frontier (ignores duplicates).
func (d *DB) EnqueueURL(ctx context.Context, rawURL, host string, priority int, depth int) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO url_frontier(url, host, priority, depth, status, next_fetch_at, created_at)
		 VALUES($1, $2, $3, $4, 'pending', NOW(), NOW())
		 ON CONFLICT (url) DO NOTHING`,
		rawURL, host, priority, depth,
	)
	if err != nil {
		return fmt.Errorf("store: enqueue url: %w", err)
	}
	return nil
}

// ClaimURLs picks up to n pending URLs and marks them as fetching.
// Returns the claimed entries.
func (d *DB) ClaimURLs(ctx context.Context, n int) ([]crawler.URLEntry, error) {
	rows, err := d.db.QueryContext(ctx,
		`UPDATE url_frontier
		 SET status='fetching'
		 WHERE id IN (
		     SELECT id FROM url_frontier
		     WHERE status='pending' AND next_fetch_at <= NOW()
		     ORDER BY priority DESC, next_fetch_at ASC
		     LIMIT $1
		     FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, url, host, priority, next_fetch_at, status, created_at`, n)
	if err != nil {
		return nil, fmt.Errorf("store: claim urls: %w", err)
	}
	defer rows.Close()
	var entries []crawler.URLEntry
	for rows.Next() {
		var e crawler.URLEntry
		if err := rows.Scan(&e.ID, &e.URL, &e.Host, &e.Priority, &e.NextFetchAt, &e.Status, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: claim scan: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// MarkURLDone marks a frontier entry done or failed.
func (d *DB) MarkURLDone(ctx context.Context, id int64, status string, rescheduleAfter time.Duration) error {
	nextFetch := time.Now().Add(rescheduleAfter)
	_, err := d.db.ExecContext(ctx,
		`UPDATE url_frontier SET status=$1, next_fetch_at=$2 WHERE id=$3`,
		status, nextFetch, id,
	)
	return err
}

// UpsertPageFetch records a fetch result (insert or update by url_hash).
func (d *DB) UpsertPageFetch(ctx context.Context, pf crawler.PageFetch) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO page_fetches(url_hash, url, status_code, content_hash, body_size, fetched_at, error)
		 VALUES($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (url_hash) DO UPDATE
		   SET status_code=EXCLUDED.status_code,
		       content_hash=EXCLUDED.content_hash,
		       body_size=EXCLUDED.body_size,
		       fetched_at=EXCLUDED.fetched_at,
		       error=EXCLUDED.error`,
		pf.URLHash, pf.URL, pf.StatusCode, pf.ContentHash, pf.BodySize, pf.FetchedAt, pf.Error,
	)
	if err != nil {
		return fmt.Errorf("store: upsert page fetch: %w", err)
	}
	return nil
}

// GetPageFetch retrieves a page fetch record by URL hash.
func (d *DB) GetPageFetch(ctx context.Context, urlHash string) (crawler.PageFetch, error) {
	var pf crawler.PageFetch
	var errStr sql.NullString
	err := d.db.QueryRowContext(ctx,
		`SELECT url_hash, url, status_code, content_hash, body_size, fetched_at, error
		 FROM page_fetches WHERE url_hash=$1`, urlHash,
	).Scan(&pf.URLHash, &pf.URL, &pf.StatusCode, &pf.ContentHash, &pf.BodySize, &pf.FetchedAt, &errStr)
	if errStr.Valid {
		pf.Error = errStr.String
	}
	if err != nil {
		return pf, fmt.Errorf("store: get page fetch: %w", err)
	}
	return pf, nil
}

// FrontierStats returns counts by status.
func (d *DB) FrontierStats(ctx context.Context) (map[string]int64, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM url_frontier GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("store: frontier stats: %w", err)
	}
	defer rows.Close()
	stats := make(map[string]int64, 4)
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[status] = count
	}
	return stats, rows.Err()
}

// RecentFetches returns the most recent fetched pages.
func (d *DB) RecentFetches(ctx context.Context, limit int) ([]crawler.PageFetch, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT url_hash, url, status_code, content_hash, body_size, fetched_at, COALESCE(error,'')
		 FROM page_fetches ORDER BY fetched_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: recent fetches: %w", err)
	}
	defer rows.Close()
	var pages []crawler.PageFetch
	for rows.Next() {
		var p crawler.PageFetch
		if err := rows.Scan(&p.URLHash, &p.URL, &p.StatusCode, &p.ContentHash, &p.BodySize, &p.FetchedAt, &p.Error); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// Cache wraps a Redis client with crawler-specific helpers.
type Cache struct {
	rdb *redis.Client
}

// NewCache creates a Redis client.
func NewCache(addr, password string, db int) *Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Cache{rdb: rdb}
}

// Close closes the Redis client.
func (c *Cache) Close() { c.rdb.Close() }

// Ping checks Redis connectivity.
func (c *Cache) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// IsSeen returns true if a URL hash has been seen before (Bloom-filter-lite via SET).
func (c *Cache) IsSeen(ctx context.Context, urlHash string) (bool, error) {
	val, err := c.rdb.SIsMember(ctx, "crawl:seen", urlHash).Result()
	return val, err
}

// MarkSeen adds a URL hash to the seen set.
func (c *Cache) MarkSeen(ctx context.Context, urlHash string) error {
	return c.rdb.SAdd(ctx, "crawl:seen", urlHash).Err()
}

// SetRobots caches a RobotsRule for a host for ttl duration.
func (c *Cache) SetRobots(ctx context.Context, host string, rule *crawler.RobotsRule, ttl time.Duration) error {
	data, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, "robots:"+host, data, ttl).Err()
}

// GetRobots retrieves a cached RobotsRule or returns nil if missing.
func (c *Cache) GetRobots(ctx context.Context, host string) (*crawler.RobotsRule, error) {
	data, err := c.rdb.Get(ctx, "robots:"+host).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rule crawler.RobotsRule
	if err := json.Unmarshal(data, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// IncrFetchCount increments the per-host fetch counter (for rate-limiting awareness).
func (c *Cache) IncrFetchCount(ctx context.Context, host string) (int64, error) {
	key := "fetchcount:" + host
	cnt, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// Expire the key after 1 minute so counts are rolling.
	c.rdb.Expire(ctx, key, time.Minute)
	return cnt, nil
}

// SeenCount returns the total number of seen URL hashes.
func (c *Cache) SeenCount(ctx context.Context) (int64, error) {
	return c.rdb.SCard(ctx, "crawl:seen").Result()
}
