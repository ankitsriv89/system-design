# Code Flow — Web Crawler (Project 12)

## Full Call Graph from main()

```mermaid
flowchart TD
    main["main()"] --> storeDB["store.NewDB(dsn)"]
    main --> storeCache["store.NewCache(addr,pass,db)"]
    main --> sigCtx["signal.NotifyContext(SIGTERM)"]
    main --> workers["for i < NUM_WORKERS: go worker.New(i).Run(ctx)"]
    main --> apiHandler["api.NewHandler(db,cache,log)"]
    main --> httpServe["http.Server.ListenAndServe(:8093)"]

    workers --> workerRun["Worker.Run(ctx)"]
    workerRun --> claimURLs["db.ClaimURLs(ctx, 5)\nSELECT FOR UPDATE SKIP LOCKED"]
    claimURLs --> processEntry["Worker.processEntry(entry)"]

    processEntry --> urlHash["crawler.URLHash(url)"]
    urlHash --> isSeen["cache.IsSeen(ctx, hash)\nREDIS SISMEMBER"]
    isSeen -->|seen| markSkipped["db.MarkURLDone(skipped)"]
    isSeen -->|unseen| getRobots["Worker.getRobots(host)"]

    getRobots --> cacheGetRobots["cache.GetRobots(host)\nREDIS GET robots:{host}"]
    cacheGetRobots -->|cache hit| robotsRule["use cached RobotsRule"]
    cacheGetRobots -->|miss| fetchRobots["fetcher.Fetch(/robots.txt)"]
    fetchRobots --> parseRobots["crawler.ParseRobotsTxt(body, ua)"]
    parseRobots --> cacheSetRobots["cache.SetRobots(24h)"]

    robotsRule --> isAllowed["crawler.IsAllowed(path, rule)"]
    isAllowed -->|disallowed| markSkipped2["db.MarkURLDone(skipped)"]
    isAllowed -->|allowed| sleepDelay["sleepCtx(crawl-delay)"]

    sleepDelay --> httpFetch["fetcher.Fetch(url)\nHTTP GET 15s timeout 2MB cap"]
    httpFetch -->|error| upsertErr["db.UpsertPageFetch(error)\ndb.MarkURLDone(failed, 5m)"]
    httpFetch -->|ok| contentHash["crawler.ContentHash(body)"]
    contentHash --> upsertFetch["db.UpsertPageFetch(PageFetch)"]
    upsertFetch --> markSeen["cache.MarkSeen(hash)\nREDIS SADD"]
    markSeen --> markDone["db.MarkURLDone(done, 24h)"]
    markDone --> extractLinks["crawler.ExtractLinks(body, baseURL)\nparse <a href>"]
    extractLinks --> enqueueLoop["for each link: db.EnqueueURL\nON CONFLICT DO NOTHING"]
```

## Operation: Submit Crawl Job (POST /v1/crawl-jobs)

```mermaid
flowchart TD
    req["HTTP POST /v1/crawl-jobs"] --> decode["json.Decode(body)"]
    decode --> validate["validate seed_url, max_depth"]
    validate --> normalize["crawler.NormalizeURL(seed_url)\nlower-case scheme+host, strip fragment"]
    normalize --> createJob["db.CreateJob(ctx, norm, depth)\nINSERT crawl_jobs"]
    createJob --> enqueueURL["db.EnqueueURL(norm, host, priority=10)\nINSERT url_frontier ON CONFLICT DO NOTHING"]
    enqueueURL --> updateStatus["db.UpdateJobStatus(running)"]
    updateStatus --> respond["201 {id, seed_url, status:running}"]
```

Why priority=10 for seeds: seeds are the entry points chosen by the operator and should be processed before organically discovered links (priority=1).

## Operation: Worker Claim (SELECT FOR UPDATE SKIP LOCKED)

```mermaid
flowchart TD
    claim["db.ClaimURLs(ctx, 5)"] --> pg["PostgreSQL\nUPDATE url_frontier SET status='fetching'\nWHERE id IN (\n  SELECT id ... WHERE status='pending'\n  AND next_fetch_at <= NOW()\n  ORDER BY priority DESC, next_fetch_at ASC\n  LIMIT 5 FOR UPDATE SKIP LOCKED\n)\nRETURNING ..."]
    pg --> entries["[]URLEntry"]
```

`SKIP LOCKED` is the key: rows being processed by other workers are invisible, giving lock-free fan-out across the worker pool without a message broker.

## Operation: robots.txt Caching

```mermaid
flowchart LR
    worker["Worker"] --> redisGet["REDIS GET robots:{host}"]
    redisGet -->|hit JSON| deserialize["json.Unmarshal → RobotsRule"]
    redisGet -->|nil| fetchLive["HTTP GET /robots.txt"]
    fetchLive --> parse["crawler.ParseRobotsTxt\nscans User-agent:, Disallow:, Crawl-delay:"]
    parse --> redisSet["REDIS SET robots:{host} 24h TTL"]
    redisSet --> rule["RobotsRule{Disallowed, CrawlDelay}"]
    deserialize --> rule
```

## Call Graph Summary

```mermaid
graph LR
    main --> api.Handler
    main --> worker.Worker
    api.Handler --> store.DB
    api.Handler --> store.Cache
    api.Handler --> crawler.NormalizeURL
    worker.Worker --> store.DB
    worker.Worker --> store.Cache
    worker.Worker --> crawler.HTTPFetcher
    worker.Worker --> crawler.URLHash
    worker.Worker --> crawler.ContentHash
    worker.Worker --> crawler.ParseRobotsTxt
    worker.Worker --> crawler.IsAllowed
    worker.Worker --> crawler.ExtractLinks
    store.DB --> lib/pq
    store.Cache --> redis/go-redis
    api.Handler --> promhttp
```
