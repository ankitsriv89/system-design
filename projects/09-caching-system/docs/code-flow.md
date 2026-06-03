# Code Flow — 09 Caching System

## Full call graph from main()

```mermaid
flowchart TD
    main["main()"]
    aofOpen["store.Open(aofPath)"]
    metricsNew["metrics.New(nil)"]
    cacheNew["cache.New(Config{...})"]
    replay["aof.Replay()"]
    apiNew["api.New(cache, aof, met, log)"]
    register["handler.Register(router)"]
    listenServe["http.Server.ListenAndServe"]
    sweeper["go cache.sweeper(ctx, 30s)"]
    sigWait["<-quit signal"]
    shutdown["srv.Shutdown(ctx)"]

    main --> aofOpen
    main --> metricsNew
    main --> cacheNew
    cacheNew --> sweeper
    main --> replay
    replay -->|"load surviving entries"| cacheSet["cache.Set(key,val,ttl)"]
    main --> apiNew
    main --> register
    main --> listenServe
    main --> sigWait --> shutdown
```

## SET operation

```mermaid
flowchart TD
    putHTTP["PUT /v1/cache/{key}"]
    parseBody["json.Decode(body)"]
    cacheSet["cache.Set(key, value, ttl)"]
    evictCheck{"memBytes + size > maxBytes?"}
    evictOne["evictOne(reason=capacity)"]
    lruPush["lruList.PushFront(key)"] 
    lfuPush["heap.Push(lfuHeap, item)"]
    updateMem["memBytes += size"]
    aofAppend["aof.AppendSet(key,val,ttl,expiresAt)"]
    promUpdate["met.Sets.Inc(); met.MemoryBytes.Set(...)"]
    respond["201 Created {entry}"]

    putHTTP --> parseBody --> cacheSet
    cacheSet --> evictCheck
    evictCheck -->|yes| evictOne --> evictCheck
    evictCheck -->|no| lruPush
    evictCheck -->|no LFU| lfuPush
    lruPush --> updateMem
    lfuPush --> updateMem
    updateMem --> aofAppend --> promUpdate --> respond
```

## GET operation

```mermaid
flowchart TD
    getHTTP["GET /v1/cache/{key}"]
    cacheLookup["cache.Get(key)"]
    exists{"key in map?"}
    expiredCheck{"entry.expired()?"}
    evictTTL["evictKey(key, 'ttl')"]
    updateLRU["lruList.MoveToFront(elem)"]
    updateLFU["heap.Fix(lfuHeap, idx)"]
    hitResp["200 {key,value,expires_at,access_count}"]
    missResp["404 {error}"]
    incrHits["met.Hits.Inc()"]
    incrMisses["met.Misses.Inc()"]

    getHTTP --> cacheLookup --> exists
    exists -->|no| incrMisses --> missResp
    exists -->|yes| expiredCheck
    expiredCheck -->|yes| evictTTL --> incrMisses --> missResp
    expiredCheck -->|no| updateLRU --> incrHits --> hitResp
    expiredCheck -->|no LFU| updateLFU --> incrHits --> hitResp
```

## Singleflight (GetOrLoad) — stampede protection

```mermaid
flowchart TD
    getOrLoad["cache.GetOrLoad(key, ttl, loader)"]
    firstGet["cache.Get(key)"]
    sfDo["sf.Do(key, func)"]
    secondGet["cache.Get(key) inside Do"]
    callLoader["loader()"]
    setResult["cache.Set(key, val, ttl)"]
    returnVal["return val, fromCache, nil"]

    getOrLoad --> firstGet
    firstGet -->|hit| returnVal
    firstGet -->|miss| sfDo
    sfDo --> secondGet
    secondGet -->|hit| returnVal
    secondGet -->|miss| callLoader --> setResult --> returnVal

    style sfDo fill:#bc8cff,color:#000
    style callLoader fill:#f0883e,color:#000
```

Why `singleflight`: without it, N concurrent misses on the same key each independently call the loader — the backend sees N queries. With `singleflight`, only one goroutine calls the loader; the others block and receive the same result when it returns.

## LRU eviction order

```mermaid
flowchart LR
    Front["HEAD\n(most recent)"]
    K1["key-A"]
    K2["key-B"]
    K3["key-C"]
    Back["TAIL\n(eviction victim)"]

    Front --> K1 --> K2 --> K3 --> Back
```

On `Get("key-A")`: key-A moves to HEAD. On capacity eviction: TAIL element is removed.

## LFU eviction order (min-heap)

The min-heap orders entries by `access_count` (ascending). Tie-breaking uses `last_access` (oldest first). `heap.Fix` is called after every access increment to maintain the invariant. `heap.Pop` removes the minimum for eviction.

## Call graph summary

```mermaid
graph LR
    main --> cache.New
    main --> store.Open
    main --> api.New
    api.New --> cache.Set
    api.New --> cache.Get
    api.New --> cache.Delete
    api.New --> cache.Flush
    api.New --> cache.Stats
    api.New --> cache.Entries
    api.New --> store.AppendSet
    api.New --> store.AppendDelete
    api.New --> store.AppendFlush
    cache.New --> sweeper
    sweeper --> cache.sweepExpired
    cache.Set --> evictOne
    evictOne --> evictKey
    cache.Get --> evictKey
```
