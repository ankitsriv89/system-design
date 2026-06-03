// Package store implements PostgreSQL corpus storage and Redis prefix-index serving.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ankitsriv89/11-typeahead-autocomplete-system/autocomplete"
)

const (
	// maxPrefixLen is the maximum prefix length indexed into Redis.
	// Longer prefixes have negligible traffic; capping keeps memory bounded.
	maxPrefixLen = 20
	// topK is the maximum number of items stored per prefix key in Redis.
	topK = 20
	// defaultSuggestLimit is returned when the caller passes limit=0.
	defaultSuggestLimit = 10
	// redisKeyTTL is the TTL for hot-prefix cache keys.
	redisKeyTTL = 24 * time.Hour
)

// Store holds DB and Redis clients and implements autocomplete.Store.
type Store struct {
	db  *sql.DB
	rdb *redis.Client
	log *zap.Logger

	statsMu sync.RWMutex
	stats   autocomplete.IndexStats
}

// New constructs a Store and verifies connectivity.
func New(db *sql.DB, rdb *redis.Client, log *zap.Logger) (*Store, error) {
	s := &Store{db: db, rdb: rdb, log: log}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("store: postgres ping: %w", err)
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("store: redis ping: %w", err)
	}
	return s, nil
}

// prefixKey returns the Redis sorted-set key for a given prefix and locale.
func prefixKey(prefix, locale string) string {
	return fmt.Sprintf("ac:pfx:%s:%s", locale, prefix)
}

// statsKey is the Redis key for cached IndexStats JSON.
const statsKey = "ac:stats"

// AddItem persists a new item to PostgreSQL and indexes all its prefixes into Redis.
func (s *Store) AddItem(ctx context.Context, item *autocomplete.SuggestItem) (int64, error) {
	const q = `
		INSERT INTO suggest_items (text, category, popularity, locale)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`
	row := s.db.QueryRowContext(ctx, q, item.Text, item.Category, item.Popularity, item.Locale)
	if err := row.Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return 0, fmt.Errorf("store: add item: %w", err)
	}
	if err := s.indexItem(ctx, item); err != nil {
		// Non-fatal: item is in PG; background rebuild will re-index.
		s.log.Warn("store: index item in redis failed", zap.Int64("id", item.ID), zap.Error(err))
	}
	return item.ID, nil
}

// GetItem fetches a single item by primary key.
func (s *Store) GetItem(ctx context.Context, id int64) (*autocomplete.SuggestItem, error) {
	const q = `SELECT id, text, category, popularity, locale, created_at, updated_at
		FROM suggest_items WHERE id = $1`
	row := s.db.QueryRowContext(ctx, q, id)
	item := &autocomplete.SuggestItem{}
	err := row.Scan(&item.ID, &item.Text, &item.Category, &item.Popularity,
		&item.Locale, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get item: %w", err)
	}
	return item, nil
}

// ListItems returns corpus items for a locale with pagination.
func (s *Store) ListItems(ctx context.Context, locale string, limit, offset int) ([]*autocomplete.SuggestItem, error) {
	const q = `SELECT id, text, category, popularity, locale, created_at, updated_at
		FROM suggest_items WHERE ($1 = '' OR locale = $1)
		ORDER BY popularity DESC LIMIT $2 OFFSET $3`
	rows, err := s.db.QueryContext(ctx, q, locale, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: list items: %w", err)
	}
	defer rows.Close()
	items := make([]*autocomplete.SuggestItem, 0, limit)
	for rows.Next() {
		item := &autocomplete.SuggestItem{}
		if err := rows.Scan(&item.ID, &item.Text, &item.Category, &item.Popularity,
			&item.Locale, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: list items scan: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeleteItem removes an item from PostgreSQL. The Redis keys expire naturally via TTL;
// a rebuild will also clear them. We don't eagerly remove from Redis to avoid O(prefixes) deletes.
func (s *Store) DeleteItem(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM suggest_items WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete item: %w", err)
	}
	return nil
}

// IncrementPopularity bumps the popularity of an item and updates the Redis score.
func (s *Store) IncrementPopularity(ctx context.Context, id int64, delta float64) error {
	const q = `UPDATE suggest_items SET popularity = popularity + $1, updated_at = now()
		WHERE id = $2 RETURNING text, locale, popularity`
	var text, locale string
	var newPop float64
	if err := s.db.QueryRowContext(ctx, q, delta, id).Scan(&text, &locale, &newPop); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("store: increment popularity: %w", err)
	}
	// Update score in Redis for all prefixes of this item.
	pipe := s.rdb.Pipeline()
	score := autocomplete.ScoreItem(newPop)
	member := fmt.Sprintf("%d\t%s\t%s", id, text, "")
	for _, pfx := range autocomplete.GeneratePrefixes(text, maxPrefixLen) {
		key := prefixKey(pfx, locale)
		pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
		pipe.Expire(ctx, key, redisKeyTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		s.log.Warn("store: update redis score failed", zap.Int64("id", id), zap.Error(err))
	}
	return nil
}

// Suggest returns top-K suggestions for the given prefix from Redis.
// Falls back to PostgreSQL LIKE query if Redis has no data for this prefix.
func (s *Store) Suggest(ctx context.Context, prefix, locale string, limit int) ([]*autocomplete.Suggestion, error) {
	if limit <= 0 {
		limit = defaultSuggestLimit
	}
	if limit > topK {
		limit = topK
	}
	key := prefixKey(prefix, locale)
	results, err := s.rdb.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err == nil && len(results) > 0 {
		return parseRedisResults(results), nil
	}
	// Redis miss: query Postgres and populate cache.
	return s.suggestFromPG(ctx, prefix, locale, limit)
}

// suggestFromPG performs a LIKE query and back-fills Redis.
func (s *Store) suggestFromPG(ctx context.Context, prefix, locale string, limit int) ([]*autocomplete.Suggestion, error) {
	const q = `SELECT id, text, category, popularity FROM suggest_items
		WHERE ($1 = '' OR locale = $1) AND lower(text) LIKE $2
		ORDER BY popularity DESC LIMIT $3`
	pattern := strings.ToLower(prefix) + "%"
	rows, err := s.db.QueryContext(ctx, q, locale, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("store: suggest pg fallback: %w", err)
	}
	defer rows.Close()

	suggestions := make([]*autocomplete.Suggestion, 0, limit)
	pipe := s.rdb.Pipeline()
	key := prefixKey(prefix, locale)

	for rows.Next() {
		var id int64
		var text, category string
		var pop float64
		if err := rows.Scan(&id, &text, &category, &pop); err != nil {
			return nil, fmt.Errorf("store: suggest pg scan: %w", err)
		}
		suggestions = append(suggestions, &autocomplete.Suggestion{
			Text:     text,
			Category: category,
			Score:    pop,
			ItemID:   id,
		})
		member := fmt.Sprintf("%d\t%s\t%s", id, text, category)
		pipe.ZAdd(ctx, key, redis.Z{Score: pop, Member: member})
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("store: suggest pg rows: %w", rows.Err())
	}
	pipe.Expire(ctx, key, redisKeyTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		s.log.Warn("store: backfill redis failed", zap.String("prefix", prefix), zap.Error(err))
	}
	return suggestions, nil
}

// parseRedisResults converts ZREVRANGEBYSCORE output into Suggestion slice.
func parseRedisResults(results []redis.Z) []*autocomplete.Suggestion {
	out := make([]*autocomplete.Suggestion, 0, len(results))
	for _, z := range results {
		member := fmt.Sprintf("%v", z.Member)
		parts := strings.SplitN(member, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		var id int64
		fmt.Sscanf(parts[0], "%d", &id)
		text := parts[1]
		category := ""
		if len(parts) == 3 {
			category = parts[2]
		}
		out = append(out, &autocomplete.Suggestion{
			ItemID:   id,
			Text:     text,
			Category: category,
			Score:    z.Score,
		})
	}
	return out
}

// indexItem adds all prefixes of item.Text into Redis sorted sets.
func (s *Store) indexItem(ctx context.Context, item *autocomplete.SuggestItem) error {
	score := autocomplete.ScoreItem(item.Popularity)
	member := fmt.Sprintf("%d\t%s\t%s", item.ID, item.Text, item.Category)
	pipe := s.rdb.Pipeline()
	for _, pfx := range autocomplete.GeneratePrefixes(item.Text, maxPrefixLen) {
		key := prefixKey(pfx, item.Locale)
		pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
		// Keep only top-K members per prefix to bound Redis memory.
		pipe.ZRemRangeByRank(ctx, key, 0, int64(-topK-1))
		pipe.Expire(ctx, key, redisKeyTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// RebuildIndex scans every item in PostgreSQL and re-indexes into Redis.
// It deletes existing prefix keys first to remove stale entries.
func (s *Store) RebuildIndex(ctx context.Context) (*autocomplete.IndexStats, error) {
	start := time.Now()
	s.log.Info("store: rebuilding index")

	// Delete all existing prefix keys.
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, "ac:pfx:*", 200).Result()
		if err != nil {
			return nil, fmt.Errorf("store: rebuild scan: %w", err)
		}
		if len(keys) > 0 {
			if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
				return nil, fmt.Errorf("store: rebuild del: %w", err)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	// Re-index all items in batches.
	const batchSize = 500
	var offset int
	var totalItems int64
	for {
		rows, err := s.db.QueryContext(ctx,
			`SELECT id, text, category, popularity, locale FROM suggest_items ORDER BY id LIMIT $1 OFFSET $2`,
			batchSize, offset)
		if err != nil {
			return nil, fmt.Errorf("store: rebuild query: %w", err)
		}
		var count int
		for rows.Next() {
			item := &autocomplete.SuggestItem{}
			if err := rows.Scan(&item.ID, &item.Text, &item.Category, &item.Popularity, &item.Locale); err != nil {
				rows.Close()
				return nil, fmt.Errorf("store: rebuild scan row: %w", err)
			}
			if err := s.indexItem(ctx, item); err != nil {
				s.log.Warn("store: rebuild index item failed", zap.Int64("id", item.ID), zap.Error(err))
			}
			count++
		}
		rows.Close()
		if rows.Err() != nil {
			return nil, fmt.Errorf("store: rebuild rows err: %w", rows.Err())
		}
		totalItems += int64(count)
		offset += count
		if count < batchSize {
			break
		}
	}

	// Count total prefix keys now in Redis.
	var prefixCount int64
	cursor = 0
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, "ac:pfx:*", 200).Result()
		if err != nil {
			break
		}
		prefixCount += int64(len(keys))
		cursor = next
		if cursor == 0 {
			break
		}
	}

	stats := &autocomplete.IndexStats{
		TotalItems:      totalItems,
		TotalPrefixes:   prefixCount,
		LastRebuildAt:   time.Now(),
		RebuildDuration: time.Since(start).Milliseconds(),
	}
	s.statsMu.Lock()
	s.stats = *stats
	s.statsMu.Unlock()

	// Cache stats in Redis.
	if b, err := json.Marshal(stats); err == nil {
		s.rdb.Set(ctx, statsKey, b, redisKeyTTL)
	}

	s.log.Info("store: rebuild complete",
		zap.Int64("items", totalItems),
		zap.Int64("prefixes", prefixCount),
		zap.Int64("duration_ms", stats.RebuildDuration))
	return stats, nil
}

// GetIndexStats returns cached stats; falls back to in-memory copy.
func (s *Store) GetIndexStats(ctx context.Context) (*autocomplete.IndexStats, error) {
	data, err := s.rdb.Get(ctx, statsKey).Bytes()
	if err == nil {
		stats := &autocomplete.IndexStats{}
		if err := json.Unmarshal(data, stats); err == nil {
			return stats, nil
		}
	}
	s.statsMu.RLock()
	stats := s.stats
	s.statsMu.RUnlock()
	return &stats, nil
}

// RecordQuery appends a query log entry to PostgreSQL.
func (s *Store) RecordQuery(ctx context.Context, log *autocomplete.QueryLog) error {
	const q = `INSERT INTO query_logs (prefix, selected_item_id, latency_ms, locale)
		VALUES ($1, $2, $3, $4)`
	_, err := s.db.ExecContext(ctx, q, log.Prefix, log.SelectedItemID, log.LatencyMS, log.Locale)
	if err != nil {
		return fmt.Errorf("store: record query: %w", err)
	}
	return nil
}
