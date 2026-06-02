# Code Flow — 07 API Gateway

## Full Call Graph (main → storage)

```mermaid
flowchart TD
    main["main()"]
    pg["store.NewPG(dsn)"]
    redis["store.NewRedis(addr)"]
    router["&gateway.Router{}"]
    gwNew["gateway.New(cfg, pg, pg, redis, pg, router)"]
    reload["gw.ReloadRoutes(ctx)\n→ pg.ListRoutes() → router.Reload()"]
    reloader["go routeReloader(ctx, gw, 30s)"]
    apiNew["api.New(gw, pg, pg, redis, log, metrics, idGen)"]
    adminSrv["http.Server :8089\nhandler.RegisterAdmin(mux)"]
    proxySrv["http.Server :8088\nhandler.RegisterProxy(mux)"]

    main --> pg
    main --> redis
    main --> router
    main --> gwNew
    gwNew --> reload
    main --> reloader
    main --> apiNew
    apiNew --> adminSrv
    apiNew --> proxySrv
```

## Operation: Proxy Request

```mermaid
flowchart TD
    ProxyHTTP["handler.proxy(w, r)"]
    GenID["idGen() → requestID"]
    Eval["gw.Evaluate(ctx, requestID, r)"]
    RouterMatch["router.Match(r.URL.Path)"]
    AuthCheck{route.AuthRequired?}
    ExtractToken["extractBearerToken(r)"]
    AuthDB["pg.Authenticate(ctx, rawKey)\n= SELECT WHERE hashed_key=sha256(rawKey)"]
    ScopeCheck{hasScope?}
    QuotaCheck{key.QuotaPerMin > 0?}
    RateLimit["redis.Allow(ctx, keyID, quota)\n= ZREMRANGEBYSCORE + ZCARD + ZADD + EXPIRE"]
    LogDecision["pg.Record(ctx, decision)"]
    BuildURL["buildUpstreamURL(route, r)"]
    ReverseProxy["httputil.ReverseProxy.ServeHTTP\n(shared bufPool)"]
    Metrics["metrics.RequestTotal.Inc()\nmetrics.RequestDuration.Observe()"]

    ProxyHTTP --> GenID
    GenID --> Eval
    Eval --> RouterMatch
    RouterMatch -->|no match| Return404["return ErrNotFound → 404"]
    RouterMatch -->|match| AuthCheck
    AuthCheck -->|yes| ExtractToken
    ExtractToken -->|empty| Return401["return ErrUnauthorized → 401"]
    ExtractToken -->|token| AuthDB
    AuthDB -->|not found| Return401
    AuthDB -->|found| ScopeCheck
    ScopeCheck -->|fail| Return403["return ErrForbidden → 403"]
    ScopeCheck -->|pass| QuotaCheck
    QuotaCheck -->|yes| RateLimit
    RateLimit -->|blocked| Return429["return ErrRateLimited → 429"]
    RateLimit -->|allowed| LogDecision
    QuotaCheck -->|no| LogDecision
    AuthCheck -->|no| BuildURL
    LogDecision --> BuildURL
    BuildURL --> ReverseProxy
    ReverseProxy --> Metrics
```

## Operation: Upsert Route (admin)

```mermaid
flowchart TD
    UpsertHTTP["handler.upsertRoute(w, r)"]
    ParseBody["json.Decode(body)"]
    ValidateURL["url.ParseRequestURI(upstream)"]
    PGUpsert["pg.UpsertRoute(ctx, route)\n= INSERT ... ON CONFLICT DO UPDATE"]
    ReloadRoutes["gw.ReloadRoutes(ctx)\n→ pg.ListRoutes() → router.Reload()"]
    Return200["200 OK — route JSON"]

    UpsertHTTP --> ParseBody
    ParseBody -->|invalid| Return400["400 Bad Request"]
    ParseBody -->|valid| ValidateURL
    ValidateURL -->|invalid| Return400
    ValidateURL -->|valid| PGUpsert
    PGUpsert --> ReloadRoutes
    ReloadRoutes --> Return200
```

## Operation: Rate Limit Check (Redis sliding window)

```mermaid
flowchart TD
    Allow["redis.Allow(ctx, keyID, limitPerMin)"]
    Now["now = time.Now()"]
    WindowStart["windowStart = now - 1 minute"]
    Pipeline["Redis PIPELINE"]
    ZRem["ZREMRANGEBYSCORE rl:keyID 0 windowStart.UnixNano()\n(remove expired timestamps)"]
    ZCard["ZCARD rl:keyID\n(count in current window)"]
    ZAdd["ZADD rl:keyID now.UnixNano() now.UnixNano()\n(record this request)"]
    Expire["EXPIRE rl:keyID 2m\n(reclaim idle keys)"]
    Compare{"count < limitPerMin?"}
    Allowed["return true (allowed)"]
    Blocked["return false (rate limited)"]

    Allow --> Now --> WindowStart --> Pipeline
    Pipeline --> ZRem --> ZCard --> ZAdd --> Expire
    Expire -->|exec pipeline| Compare
    Compare -->|yes| Allowed
    Compare -->|no| Blocked
```

## Call Graph Summary

```mermaid
graph LR
    main --> store_NewPG
    main --> store_NewRedis
    main --> gateway_New
    main --> api_New
    api_Handler_proxy --> gateway_Evaluate
    gateway_Evaluate --> gateway_Router_Match
    gateway_Evaluate --> store_PGStore_Authenticate
    gateway_Evaluate --> store_RedisLimiter_Allow
    gateway_Evaluate --> store_PGStore_Record
    api_Handler_upsertRoute --> store_PGStore_UpsertRoute
    api_Handler_upsertRoute --> gateway_ReloadRoutes
    gateway_ReloadRoutes --> store_PGStore_ListRoutes
    gateway_ReloadRoutes --> gateway_Router_Reload
```

## Why each call is made

| Call | Reason |
|---|---|
| `router.Match()` | O(n) linear scan over active routes — avoids network round-trip; routes fit in L1/L2 cache |
| `pg.Authenticate()` | Raw key values are never stored; SHA-256 hash is the DB key, so a compromised DB does not expose credentials |
| `redis.Allow()` — pipeline | Four Redis commands in one round-trip; ZREMRANGEBYSCORE + ZCARD is the sliding-window algorithm; ZADD records the current request |
| `pg.Record()` — after forward | Decision logged after the upstream responds so the status code is known; context cancellation means a slow DB write cannot delay the client response |
| `gw.ReloadRoutes()` after admin upsert | Immediate consistency: operators expect route changes to take effect within the same HTTP response, not on the next 30-second tick |
