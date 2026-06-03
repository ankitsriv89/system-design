// Package worker implements the async crawl loop: claim URLs, enforce politeness,
// fetch pages, extract links, and re-enqueue discoveries.
package worker

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ankitsriv89/12-web-crawler/crawler"
	"github.com/ankitsriv89/12-web-crawler/metrics"
	"github.com/ankitsriv89/12-web-crawler/store"
)

const (
	userAgent      = "GoWebCrawler/1.0"
	robotsTTL      = 24 * time.Hour
	defaultDelay   = 1 * time.Second
	recrawlAfter   = 24 * time.Hour
	maxLinksPerPage = 50
)

// Worker runs the crawl loop until its context is cancelled.
type Worker struct {
	db      *store.DB
	cache   *store.Cache
	fetcher *crawler.HTTPFetcher
	log     *zap.Logger
	id      int
}

// New constructs a Worker.
func New(id int, db *store.DB, cache *store.Cache, log *zap.Logger) *Worker {
	return &Worker{
		db:      db,
		cache:   cache,
		fetcher: crawler.NewHTTPFetcher(userAgent),
		log:     log.With(zap.Int("worker_id", id)),
		id:      id,
	}
}

// Run runs the crawl loop until ctx is done.
func (w *Worker) Run(ctx context.Context) {
	w.log.Info("worker started")
	for {
		select {
		case <-ctx.Done():
			w.log.Info("worker stopped")
			return
		default:
		}

		entries, err := w.db.ClaimURLs(ctx, 5)
		if err != nil {
			w.log.Error("claim urls failed", zap.Error(err))
			sleepCtx(ctx, 2*time.Second)
			continue
		}
		if len(entries) == 0 {
			sleepCtx(ctx, 500*time.Millisecond)
			continue
		}

		for _, entry := range entries {
			if ctx.Err() != nil {
				return
			}
			w.processEntry(ctx, entry)
		}
	}
}

func (w *Worker) processEntry(ctx context.Context, entry crawler.URLEntry) {
	log := w.log.With(zap.String("url", entry.URL), zap.String("host", entry.Host))

	// Dedupe check via Redis seen-set.
	hash := crawler.URLHash(entry.URL)
	seen, err := w.cache.IsSeen(ctx, hash)
	if err != nil {
		log.Warn("dedupe check error", zap.Error(err))
	}
	if seen {
		metrics.DedupeHits.Inc()
		_ = w.db.MarkURLDone(ctx, entry.ID, crawler.StatusSkipped, recrawlAfter)
		return
	}

	// robots.txt enforcement.
	rule, err := w.getRobots(ctx, entry.Host)
	if err != nil {
		log.Warn("robots fetch error", zap.Error(err))
	}
	parsed, _ := url.Parse(entry.URL)
	if parsed != nil && !crawler.IsAllowed(parsed.Path, rule) {
		metrics.RobotsHits.WithLabelValues("disallowed").Inc()
		log.Debug("robots disallowed")
		_ = w.db.MarkURLDone(ctx, entry.ID, crawler.StatusSkipped, recrawlAfter)
		return
	}

	// Per-host politeness delay.
	delay := defaultDelay
	if rule != nil && rule.CrawlDelay > 0 {
		delay = rule.CrawlDelay
	}
	sleepCtx(ctx, delay)

	// Fetch.
	start := time.Now()
	result := w.fetcher.Fetch(entry.URL)
	elapsed := time.Since(start)
	metrics.FetchDuration.Observe(elapsed.Seconds())

	pf := crawler.PageFetch{
		URLHash:   hash,
		URL:       entry.URL,
		FetchedAt: time.Now(),
	}
	if result.Error != nil {
		pf.Error = result.Error.Error()
		pf.StatusCode = 0
		metrics.URLsFetched.WithLabelValues("error").Inc()
		log.Warn("fetch error", zap.Error(result.Error))
		_ = w.db.UpsertPageFetch(ctx, pf)
		_ = w.db.MarkURLDone(ctx, entry.ID, crawler.StatusFailed, 5*time.Minute)
		return
	}

	pf.StatusCode = result.StatusCode
	pf.BodySize = len(result.Body)
	pf.ContentHash = crawler.ContentHash(result.Body)
	metrics.URLsFetched.WithLabelValues(fmt.Sprintf("%d", result.StatusCode/100)+"xx").Inc()

	_ = w.db.UpsertPageFetch(ctx, pf)
	_ = w.cache.MarkSeen(ctx, hash)
	_ = w.db.MarkURLDone(ctx, entry.ID, crawler.StatusDone, recrawlAfter)

	log.Info("fetched",
		zap.Int("status", result.StatusCode),
		zap.Int("bytes", len(result.Body)),
		zap.Duration("elapsed", elapsed),
	)

	// Extract and enqueue links (only for HTML responses).
	if result.StatusCode == 200 && strings.Contains(result.ContentType, "text/html") {
		baseURL, _ := url.Parse(result.FinalURL)
		links := crawler.ExtractLinks(result.Body, baseURL)
		metrics.LinksExtracted.Add(float64(len(links)))
		enqueued := 0
		for _, link := range links {
			if enqueued >= maxLinksPerPage {
				break
			}
			lurl, err := url.Parse(link)
			if err != nil {
				continue
			}
			lhash := crawler.URLHash(link)
			alreadySeen, _ := w.cache.IsSeen(ctx, lhash)
			if alreadySeen {
				continue
			}
			if err := w.db.EnqueueURL(ctx, link, lurl.Host, 1, 0); err == nil {
				metrics.URLsEnqueued.Inc()
				enqueued++
			}
		}
		log.Debug("links enqueued", zap.Int("count", enqueued))
	}
}

// getRobots fetches robots.txt from cache or live, caching the result.
func (w *Worker) getRobots(ctx context.Context, host string) (*crawler.RobotsRule, error) {
	rule, err := w.cache.GetRobots(ctx, host)
	if err == nil && rule != nil {
		metrics.RobotsHits.WithLabelValues("hit").Inc()
		return rule, nil
	}
	metrics.RobotsHits.WithLabelValues("miss").Inc()

	robotsURL := "https://" + host + "/robots.txt"
	result := w.fetcher.Fetch(robotsURL)
	rule = &crawler.RobotsRule{
		Host:      host,
		UserAgent: userAgent,
		FetchedAt: time.Now(),
	}
	if result.Error == nil && result.StatusCode == 200 {
		rule.Disallowed, rule.CrawlDelay = crawler.ParseRobotsTxt(result.Body, userAgent)
	}
	_ = w.cache.SetRobots(ctx, host, rule, robotsTTL)
	return rule, nil
}

// sleepCtx sleeps for d or until ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
