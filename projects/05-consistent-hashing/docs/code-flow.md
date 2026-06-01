# Code Flow — Consistent Hashing Service

## Startup flow

```mermaid
flowchart TD
    main["main()"]
    logger["zap.NewProduction()"]
    store["store.New()"]
    handler["api.New(store, log)"]
    router["mux.Router — register routes"]
    metrics_init["metrics/init() — prometheus.MustRegister"]
    srv["http.Server{Addr:8084}"]
    listen["srv.ListenAndServe()"]
    signal["os.Signal listener"]
    shutdown["srv.Shutdown(ctx 10s)"]

    main --> logger
    main --> store
    main --> handler
    handler --> router
    main --> metrics_init
    main --> srv
    srv --> listen
    listen -->|SIGTERM/SIGINT| signal
    signal --> shutdown
```

`main()` is wiring-only: it constructs dependencies, starts the server, and blocks on signal.

---

## Add Node operation

```mermaid
flowchart TD
    req["POST /v1/rings/{id}/nodes"]
    decode["json.Decode body → {id, weight, address}"]
    get_ring["store.GetRing(ringID)"]
    add_node["ring.AddNode(Node)"]
    gen_vnodes["for i in 0..weight×replicas:<br/>pos = SHA-256(nodeID+'#'+i)[0:4]<br/>append VNode{pos, nodeID}"]
    sort_vnodes["sort.Slice(vnodes, by Position)"]
    arc_lengths["arcLengths() — sum arc fraction per nodeID"]
    stddev["stdDev(vals) — √variance of arc fractions"]
    key_movement["estimate KeyMovement<br/>on 10k synthetic keys<br/>(before vs after arc distribution)"]
    version["version.Add(1)"]
    metrics["update Prometheus gauges"]
    resp["writeJSON 201 Stats"]

    req --> decode --> get_ring --> add_node
    add_node --> gen_vnodes --> sort_vnodes
    sort_vnodes --> arc_lengths --> stddev
    stddev --> key_movement --> version
    version --> metrics --> resp
```

**Why SHA-256 per vnode?** Uniform distribution across the 32-bit space requires a high-quality hash. MD5 is faster but SHA-256 avoids clustering on similar node names (e.g. `node-1`, `node-2`).

**Why re-sort after every AddNode?** The slice stays small (typically <5000 vnodes for a 30-node cluster). Re-sorting is O(V log V) ≈ 65K comparisons — negligible. An insertion sort would be O(V) but more complex code for no practical gain.

---

## Key Lookup operation

```mermaid
flowchart TD
    req["GET /v1/rings/{id}/keys/{key}/owner"]
    t0["t0 = time.Now()"]
    get_ring["store.GetRing(ringID)"]
    rlock["ring.mu.RLock()"]
    hash["h = SHA-256(key)[0:4] → uint32"]
    bsearch["sort.Search(len(vnodes),<br/>v.Position ≥ h)"]
    wrap["if idx == len → idx = 0 (wrap)"]
    owner["return vnodes[idx].NodeID"]
    runlock["ring.mu.RUnlock()"]
    observe["LookupDuration.Observe(elapsed)"]
    resp["writeJSON 200 {owner, version}"]

    req --> t0 --> get_ring --> rlock --> hash --> bsearch --> wrap --> owner --> runlock --> observe --> resp
```

**Why RLock not Lock?** Lookups are read-only. Many goroutines can hold RLock simultaneously. Only AddNode/RemoveNode acquire the exclusive Lock, so concurrent reads are never blocked by each other.

---

## Remove Node operation

```mermaid
flowchart TD
    req["DELETE /v1/rings/{id}/nodes/{node}"]
    get_ring["store.GetRing(ringID)"]
    lock["ring.mu.Lock()"]
    before["arcLengths() — snapshot before"]
    filter["filter vnodes: keep only NodeID != id"]
    delete["delete(ring.nodes, id)"]
    after["arcLengths() — snapshot after"]
    movement["estimate KeyMovement"]
    version["version.Add(1)"]
    resp["writeJSON 200 Stats"]

    req --> get_ring --> lock --> before --> filter --> delete --> after --> movement --> version --> resp
```

**Filter rather than mark-deleted** because the ring is small and a linear scan is O(V) with zero allocations (reuse the same slice backing array via `[:0]`).

---

## Simulation operation

```mermaid
flowchart TD
    req["GET /v1/rings/{id}/simulate?keys=N"]
    get_ring["store.GetRing(ringID)"]
    rlock["ring.mu.RLock()"]
    loop["for i in 0..N:<br/>key = 'sim-key-i'<br/>owner = lookupLocked(key)<br/>dist[owner]++"]
    runlock["ring.mu.RUnlock()"]
    resp["writeJSON 200 {distribution}"]

    req --> get_ring --> rlock --> loop --> runlock --> resp
```

`lookupLocked` is the same binary search as `Lookup` but without acquiring the mutex again — avoiding a deadlock since the caller already holds RLock.

---

## Call graph summary

```mermaid
graph LR
    main --> api.New
    main --> store.New
    api.Handler --> store.GetRing
    api.Handler --> ring.AddNode
    api.Handler --> ring.RemoveNode
    api.Handler --> ring.Lookup
    api.Handler --> ring.LookupN
    api.Handler --> ring.Stats
    api.Handler --> ring.SimulateKeys
    api.Handler --> ring.VNodes
    ring.AddNode --> hashPosition
    ring.Lookup --> keyHash
    ring.Stats --> arcLengths
    ring.Stats --> stdDev
```
