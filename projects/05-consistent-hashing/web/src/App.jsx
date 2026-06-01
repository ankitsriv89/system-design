import { useState, useEffect, useCallback, useRef } from 'react'
import RingCanvas from './RingCanvas.jsx'

const API = ''  // same origin; dev proxy handles it

const STEPS = [
  { id: 'intro',    label: '1. The Problem' },
  { id: 'ring',     label: '2. The Ring' },
  { id: 'vnodes',   label: '3. Virtual Nodes' },
  { id: 'weights',  label: '4. Weights' },
  { id: 'lookup',   label: '5. Key Lookup' },
  { id: 'rebalance',label: '6. Rebalance' },
]

const PRESETS = {
  empty: [],
  small: [
    { id: 'node-a', weight: 1 },
    { id: 'node-b', weight: 1 },
    { id: 'node-c', weight: 1 },
  ],
  weighted: [
    { id: 'small-1', weight: 1 },
    { id: 'small-2', weight: 1 },
    { id: 'large-1', weight: 3 },
  ],
  large: [
    { id: 'db-1', weight: 1 }, { id: 'db-2', weight: 1 }, { id: 'db-3', weight: 1 },
    { id: 'db-4', weight: 2 }, { id: 'db-5', weight: 2 },
  ],
}

const NODE_COLORS = [
  '#6366f1','#22d3ee','#f472b6','#4ade80','#fbbf24',
  '#f87171','#a78bfa','#34d399','#fb923c','#38bdf8',
]

export default function App() {
  const [step, setStep] = useState('intro')
  const [ringID] = useState('demo')
  const [nodes, setNodes] = useState([])
  const [vnodes, setVnodes] = useState([])
  const [stats, setStats] = useState(null)
  const [simDist, setSimDist] = useState(null)
  const [lookupKey, setLookupKey] = useState('')
  const [lookupResult, setLookupResult] = useState(null)
  const [highlightKey, setHighlightKey] = useState(null)
  const [newNodeID, setNewNodeID] = useState('')
  const [newNodeWeight, setNewNodeWeight] = useState(1)
  const [logs, setLogs] = useState([])
  const [lastMovement, setLastMovement] = useState(null)
  const [animating, setAnimating] = useState(false)
  const [replicaKey, setReplicaKey] = useState('')
  const [replicaResult, setReplicaResult] = useState(null)
  const ringCreated = useRef(false)
  const colorMap = useRef({})

  function colorFor(id) {
    if (!colorMap.current[id]) {
      const idx = Object.keys(colorMap.current).length
      colorMap.current[id] = NODE_COLORS[idx % NODE_COLORS.length]
    }
    return colorMap.current[id]
  }

  function addLog(msg, type = 'info') {
    const ts = new Date().toLocaleTimeString('en', { hour12: false })
    setLogs(prev => [{ ts, msg, type }, ...prev].slice(0, 30))
  }

  // --- API helpers ---
  async function apiFetch(path, opts = {}) {
    const res = await fetch(API + path, {
      headers: { 'Content-Type': 'application/json' },
      ...opts,
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(err.error ?? res.statusText)
    }
    return res.status === 204 ? null : res.json()
  }

  const refreshVnodes = useCallback(async () => {
    try {
      const data = await apiFetch(`/v1/rings/${ringID}/stats`)
      setStats(data)
      const vdata = await apiFetch(`/v1/rings/${ringID}/simulate?keys=10000`)
      setSimDist(vdata.distribution)
      // get vnodes for ring canvas
      const nodeList = await apiFetch(`/v1/rings/${ringID}/stats`)
      // fetch vnode positions via a dedicated call if needed; approximate from stats
      setStats(nodeList)
    } catch (_) {}
  }, [ringID])

  // Fetch raw vnode positions from backend
  const refreshVnodePositions = useCallback(async () => {
    try {
      // The /rings/{ring}/vnodes endpoint returns raw vnode list
      const res = await fetch(API + `/v1/rings/${ringID}/vnodes`)
      if (res.ok) {
        const data = await res.json()
        setVnodes(data.vnodes ?? [])
      }
    } catch (_) {}
  }, [ringID])

  // Ensure ring exists on mount
  useEffect(() => {
    if (ringCreated.current) return
    ringCreated.current = true
    ;(async () => {
      try {
        await apiFetch(`/v1/rings`, {
          method: 'POST',
          body: JSON.stringify({ id: ringID, replicas: 150 }),
        })
        addLog(`Ring "${ringID}" created (150 vnodes/weight)`, 'success')
      } catch (e) {
        if (!e.message.includes('already exists')) addLog(e.message, 'warn')
      }
    })()
  }, [ringID])

  async function doAddNode(id, weight) {
    if (!id) return
    setAnimating(true)
    try {
      const data = await apiFetch(`/v1/rings/${ringID}/nodes`, {
        method: 'POST',
        body: JSON.stringify({ id, weight: Number(weight) }),
      })
      setNodes(prev => [...prev.filter(n => n.id !== id), { id, weight: Number(weight) }])
      if (data.KeyMovement) setLastMovement(data.KeyMovement)
      addLog(`+ ${id} (weight=${weight}) → ${data.VNodeCount} vnodes, stddev=${data.StdDev?.toFixed(4)}`, 'success')
      await refreshVnodes()
      await refreshVnodePositions()
    } catch (e) {
      addLog(e.message, 'warn')
    } finally {
      setAnimating(false)
    }
  }

  async function doRemoveNode(id) {
    setAnimating(true)
    try {
      const data = await apiFetch(`/v1/rings/${ringID}/nodes/${id}`, { method: 'DELETE' })
      setNodes(prev => prev.filter(n => n.id !== id))
      if (data.KeyMovement) setLastMovement(data.KeyMovement)
      addLog(`- ${id} removed → ${data.VNodeCount} vnodes, moved ${data.KeyMovement?.MovedPct?.toFixed(1)}% keys`, 'warn')
      await refreshVnodes()
      await refreshVnodePositions()
    } catch (e) {
      addLog(e.message, 'warn')
    } finally {
      setAnimating(false)
    }
  }

  async function doLookup() {
    if (!lookupKey) return
    try {
      const data = await apiFetch(`/v1/rings/${ringID}/keys/${encodeURIComponent(lookupKey)}/owner`)
      setLookupResult(data)
      setHighlightKey(lookupKey)
      addLog(`lookup("${lookupKey}") → ${data.owner}`, 'success')
    } catch (e) {
      addLog(e.message, 'warn')
    }
  }

  async function doReplicaLookup() {
    if (!replicaKey) return
    try {
      const data = await apiFetch(`/v1/rings/${ringID}/keys/${encodeURIComponent(replicaKey)}/replicas?n=3`)
      setReplicaResult(data)
      addLog(`replicas("${replicaKey}") → [${data.replicas?.join(', ')}]`, 'success')
    } catch (e) {
      addLog(e.message, 'warn')
    }
  }

  async function loadPreset(name) {
    // Remove all current nodes
    for (const n of nodes) {
      await apiFetch(`/v1/rings/${ringID}/nodes/${n.id}`, { method: 'DELETE' }).catch(() => {})
    }
    setNodes([])
    setLastMovement(null)
    setLookupResult(null)
    setHighlightKey(null)
    setSimDist(null)
    for (const n of (PRESETS[name] ?? [])) {
      await doAddNode(n.id, n.weight)
    }
  }

  const totalSimKeys = simDist ? Object.values(simDist).reduce((a, b) => a + b, 0) : 0

  // --- Step content ---
  function renderStep() {
    switch (step) {
      case 'intro': return <StepIntro />
      case 'ring':  return <StepRing nodes={nodes} onPreset={loadPreset} />
      case 'vnodes': return <StepVnodes stats={stats} simDist={simDist} nodes={nodes} colorFor={colorFor} totalSimKeys={totalSimKeys} />
      case 'weights': return <StepWeights simDist={simDist} nodes={nodes} colorFor={colorFor} totalSimKeys={totalSimKeys} onPreset={loadPreset} />
      case 'lookup': return (
        <StepLookup
          lookupKey={lookupKey} setLookupKey={setLookupKey}
          lookupResult={lookupResult} onLookup={doLookup}
          replicaKey={replicaKey} setReplicaKey={setReplicaKey}
          replicaResult={replicaResult} onReplica={doReplicaLookup}
          colorFor={colorFor} nodes={nodes}
        />
      )
      case 'rebalance': return (
        <StepRebalance
          lastMovement={lastMovement} simDist={simDist} nodes={nodes}
          colorFor={colorFor} totalSimKeys={totalSimKeys} stats={stats}
          onPreset={loadPreset}
        />
      )
      default: return null
    }
  }

  return (
    <div className="app">
      <header className="header">
        <div>
          <h1><span>Consistent</span> Hashing Explorer</h1>
        </div>
        <span className="badge">port 8084</span>
        <span className="badge" style={{ background: 'color-mix(in srgb, #4ade80 15%, transparent)', borderColor: 'color-mix(in srgb, #4ade80 30%, transparent)', color: '#4ade80' }}>
          SHA-256 ring
        </span>
      </header>

      <div className="main">
        {/* Sidebar */}
        <aside className="sidebar">
          {/* Node controls */}
          <div className="card">
            <div className="card-title">Add Node</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.6rem' }}>
              <div className="form-row">
                <div className="form-group">
                  <label>Node ID</label>
                  <input
                    placeholder="e.g. node-a"
                    value={newNodeID}
                    onChange={e => setNewNodeID(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && doAddNode(newNodeID, newNodeWeight)}
                  />
                </div>
                <div className="form-group">
                  <label>Weight</label>
                  <input
                    type="number" min={1} max={10}
                    value={newNodeWeight}
                    onChange={e => setNewNodeWeight(Number(e.target.value))}
                  />
                </div>
              </div>
              <button
                className="btn btn-primary"
                onClick={() => { doAddNode(newNodeID, newNodeWeight); setNewNodeID('') }}
                disabled={!newNodeID || animating}
              >
                {animating ? '⟳ Adding…' : '+ Add Node'}
              </button>
            </div>
          </div>

          {/* Presets */}
          <div className="card">
            <div className="card-title">Presets</div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.4rem' }}>
              {Object.keys(PRESETS).map(p => (
                <button key={p} className="btn btn-ghost" style={{ fontSize: '0.75rem', padding: '0.35rem 0.7rem' }}
                  onClick={() => loadPreset(p)}>
                  {p}
                </button>
              ))}
            </div>
          </div>

          {/* Node list */}
          <div className="card" style={{ flex: 1 }}>
            <div className="card-title">Nodes ({nodes.length})</div>
            {nodes.length === 0
              ? <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>No nodes — add one above or pick a preset.</p>
              : (
                <div className="node-list">
                  {nodes.map(n => (
                    <div key={n.id} className="node-pill">
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                        <div className="node-dot" style={{ background: colorFor(n.id) }} />
                        <span className="node-id">{n.id}</span>
                        <span className="node-meta">w={n.weight}</span>
                      </div>
                      <button className="btn btn-danger" style={{ padding: '0.2rem 0.5rem', fontSize: '0.7rem' }}
                        onClick={() => doRemoveNode(n.id)}>
                        ✕
                      </button>
                    </div>
                  ))}
                </div>
              )
            }
          </div>

          {/* Stats quick view */}
          {stats && (
            <div className="card">
              <div className="card-title">Ring Stats</div>
              <div className="stat-row"><span>Version</span><span className="stat-val">{stats.Version}</span></div>
              <div className="stat-row"><span>Physical nodes</span><span className="stat-val">{stats.NodeCount}</span></div>
              <div className="stat-row"><span>Virtual nodes</span><span className="stat-val">{stats.VNodeCount}</span></div>
              <div className="stat-row"><span>Std dev</span><span className="stat-val">{stats.StdDev?.toFixed(5)}</span></div>
            </div>
          )}

          {/* Log feed */}
          <div className="card">
            <div className="card-title">Event Log</div>
            <div className="log-feed">
              {logs.length === 0
                ? <div className="log-entry info"><span className="msg">Waiting for events…</span></div>
                : logs.map((l, i) => (
                  <div key={i} className={`log-entry ${l.type}`}>
                    <span className="ts">{l.ts}</span>
                    <span className="msg">{l.msg}</span>
                  </div>
                ))
              }
            </div>
          </div>
        </aside>

        {/* Main content */}
        <main className="content">
          {/* Step nav */}
          <div className="steps">
            {STEPS.map(s => (
              <button key={s.id} className={`step-btn ${step === s.id ? 'active' : ''}`}
                onClick={() => setStep(s.id)}>
                {s.label}
              </button>
            ))}
          </div>

          {/* Ring visualisation — always visible */}
          <div className="ring-wrap">
            <RingCanvas nodes={nodes} highlightKey={highlightKey} vnodeData={vnodes} animating={animating} />
          </div>

          {/* Tutorial step */}
          {renderStep()}
        </main>
      </div>
    </div>
  )
}

// ─── Step components ────────────────────────────────────────────────────────

function StepIntro() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      <div>
        <div className="section-title">The Thundering Herd Problem</div>
        <div className="section-subtitle">Why simple modulo hashing breaks at scale</div>
      </div>

      <div className="concept-box">
        <h3>Modulo hashing</h3>
        With <em>N</em> cache nodes, the naive approach assigns each key to:
        <div className="concept-formula">node = hash(key) % N</div>
        Works great — until you add or remove a node. When N changes from 3 to 4,
        approximately <strong>75% of keys</strong> remap to different nodes.
        Every client gets a cache miss simultaneously and hammers the database. This is the <strong>thundering herd</strong>.
      </div>

      <div className="two-col">
        <ModuloDemo n={3} n2={4} />
        <div className="card">
          <div className="card-title">Consistent Hashing</div>
          <p style={{ fontSize: '0.85rem', lineHeight: 1.8, color: 'var(--text-dim)' }}>
            Maps both <strong>nodes</strong> and <strong>keys</strong> onto a circular ring <code>[0, 2³²)</code>.<br /><br />
            A key's owner is the first node clockwise from its position.
            When a node is added or removed, only <code>K/N</code> keys move — not the whole dataset.
          </p>
          <div className="concept-formula">
            keys_moved ≈ K / N<br />
            (3→4 nodes: ~25% move, not ~75%)
          </div>
        </div>
      </div>

      <div className="three-col">
        {[
          { icon: '🗂', title: 'Distributed Caches', text: 'Memcached libketama: 160 vnodes/server, MD5 hash.' },
          { icon: '🪨', title: 'Databases', text: 'Cassandra: 256 vnodes/node, token-aware client routing.' },
          { icon: '🔀', title: 'Load Balancers', text: 'Nginx/Envoy: ring hash upstream for session affinity.' },
        ].map(c => (
          <div key={c.title} className="card" style={{ textAlign: 'center' }}>
            <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>{c.icon}</div>
            <div style={{ fontWeight: 600, marginBottom: '0.4rem' }}>{c.title}</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-dim)' }}>{c.text}</div>
          </div>
        ))}
      </div>
    </div>
  )
}

function ModuloDemo({ n, n2 }) {
  const keys = ['user:42', 'order:7', 'cart:99', 'session:1', 'product:5', 'cache:x']
  function assign(key, nodes) { return `node-${(simpleHash(key) % nodes)}`}
  function simpleHash(s) { let h = 5381; for (let c of s) h = ((h << 5) + h) ^ c.charCodeAt(0); return Math.abs(h) }

  return (
    <div className="card">
      <div className="card-title">Modulo hashing: {n} → {n2} nodes</div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem' }}>
        {keys.map(k => {
          const before = assign(k, n)
          const after = assign(k, n2)
          const moved = before !== after
          return (
            <div key={k} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.78rem' }}>
              <code style={{ color: 'var(--text-dim)', width: 90 }}>{k}</code>
              <span style={{ fontFamily: 'var(--mono)', color: '#6366f1' }}>{before}</span>
              <span style={{ color: 'var(--text-muted)' }}>→</span>
              <span style={{ fontFamily: 'var(--mono)', color: moved ? '#f87171' : '#4ade80' }}>{after}</span>
              {moved && <span style={{ color: '#f87171', fontSize: '0.7rem' }}>moved</span>}
            </div>
          )
        })}
        <div style={{ marginTop: '0.75rem', padding: '0.5rem', background: 'color-mix(in srgb, #f87171 10%, transparent)', borderRadius: 6, fontSize: '0.8rem', color: '#f87171' }}>
          {keys.filter(k => assign(k, n) !== assign(k, n2)).length}/{keys.length} keys moved = {Math.round(keys.filter(k => assign(k, n) !== assign(k, n2)).length / keys.length * 100)}%
        </div>
      </div>
    </div>
  )
}

function StepRing({ nodes, onPreset }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      <div>
        <div className="section-title">The Hash Ring</div>
        <div className="section-subtitle">Placing nodes and keys on a circle</div>
      </div>
      <div className="concept-box">
        <h3>How it works</h3>
        Hash each node's ID to get its position on the ring <code>[0, 2³²)</code>.
        To find a key's owner: hash the key, then walk <strong>clockwise</strong> until you hit a node.
        <div className="concept-formula">
          owner(key) = first node where node.pos ≥ hash(key)  [wrap at 2³²]
        </div>
        Lookup is O(log N) binary search on the sorted vnode list.
      </div>
      <div className="two-col">
        <div className="card">
          <div className="card-title">Try it: load "small" preset</div>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-dim)', marginBottom: '0.75rem' }}>
            3 nodes get placed on the ring. Notice how the coloured arcs show which node owns each segment of the ring.
          </p>
          <button className="btn btn-primary" onClick={() => onPreset('small')}>Load 3-node preset</button>
        </div>
        <div className="card">
          <div className="card-title">Scale events</div>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-dim)' }}>
            Add a node on the left — watch the arc for the new node appear.
            Only the adjacent arc shrinks. All other arcs stay unchanged.
            That's consistent hashing: minimal disruption on topology changes.
          </p>
          {nodes.length > 0 && (
            <div style={{ marginTop: '0.75rem', padding: '0.5rem', background: 'color-mix(in srgb, #4ade80 10%, transparent)', borderRadius: 6, fontSize: '0.8rem', color: '#4ade80' }}>
              Ring has {nodes.length} node{nodes.length !== 1 ? 's' : ''}. Try adding or removing one.
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function StepVnodes({ stats, simDist, nodes, colorFor, totalSimKeys }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      <div>
        <div className="section-title">Virtual Nodes</div>
        <div className="section-subtitle">Fixing the load imbalance of a naïve ring</div>
      </div>
      <div className="concept-box">
        <h3>The imbalance problem</h3>
        With 3 physical nodes at 3 random positions, arcs are unequal — one node might own 50% of keys while another owns 15%.
        <div className="concept-formula">
          vnode_i = hash("{`node-id#i`}") for i in [0, replicas × weight)
        </div>
        With 150 vnodes per node, the law of large numbers makes arc lengths converge toward 1/N each.
        Std dev of arc lengths approaches 0 = perfect balance.
      </div>

      {stats && (
        <div className="three-col">
          <div className="card" style={{ textAlign: 'center' }}>
            <div style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--accent)', fontFamily: 'var(--mono)' }}>{stats.VNodeCount}</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-dim)' }}>virtual nodes</div>
          </div>
          <div className="card" style={{ textAlign: 'center' }}>
            <div style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--accent2)', fontFamily: 'var(--mono)' }}>{stats.StdDev?.toFixed(4)}</div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-dim)' }}>arc std dev (lower = better)</div>
          </div>
          <div className="card" style={{ textAlign: 'center' }}>
            <div style={{ fontSize: '2rem', fontWeight: 700, color: 'var(--green)', fontFamily: 'var(--mono)' }}>
              {nodes.length > 0 ? (100 / nodes.length).toFixed(1) + '%' : '—'}
            </div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-dim)' }}>ideal share per node</div>
          </div>
        </div>
      )}

      {simDist && totalSimKeys > 0 && (
        <div className="card">
          <div className="card-title">Key distribution (10,000 simulated keys)</div>
          <div className="bar-chart">
            {Object.entries(simDist).sort((a, b) => b[1] - a[1]).map(([id, count]) => {
              const pct = (count / totalSimKeys * 100)
              const ideal = 100 / Object.keys(simDist).length
              const delta = pct - ideal
              return (
                <div key={id} className="bar-row">
                  <div className="bar-label" style={{ color: colorFor(id) }}>{id}</div>
                  <div className="bar-track">
                    <div className="bar-fill" style={{ width: `${pct}%`, background: colorFor(id) }} />
                  </div>
                  <span className="bar-pct">{pct.toFixed(1)}%</span>
                  <span style={{ fontSize: '0.72rem', color: delta > 5 ? '#f87171' : delta < -5 ? '#fbbf24' : '#4ade80', width: 50 }}>
                    {delta > 0 ? '+' : ''}{delta.toFixed(1)}%
                  </span>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

function StepWeights({ simDist, nodes, colorFor, totalSimKeys, onPreset }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      <div>
        <div className="section-title">Weighted Placement</div>
        <div className="section-subtitle">Heterogeneous hardware handled automatically</div>
      </div>
      <div className="concept-box">
        <h3>Not all nodes are equal</h3>
        A node with 32 GB RAM should own more keys than one with 8 GB.
        Weight controls how many vnodes a node gets:
        <div className="concept-formula">
          vnodes(node) = weight × replicas<br />
          fraction_owned ≈ weight / Σ(all weights)
        </div>
        Cassandra calls this the <em>token range</em>; DynamoDB calls it <em>partition capacity</em>.
      </div>

      <div className="two-col">
        <div className="card">
          <div className="card-title">Load "weighted" preset</div>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-dim)', marginBottom: '0.75rem' }}>
            2 nodes at weight=1 + 1 node at weight=3.<br />
            The heavy node should own ~60% of keys.
          </p>
          <button className="btn btn-primary" onClick={() => onPreset('weighted')}>Load weighted preset</button>
        </div>

        {simDist && totalSimKeys > 0 && (
          <div className="card">
            <div className="card-title">Actual distribution</div>
            <div className="bar-chart">
              {nodes.map(n => {
                const count = simDist[n.id] ?? 0
                const pct = count / totalSimKeys * 100
                const totalWeight = nodes.reduce((s, x) => s + x.weight, 0)
                const expected = (n.weight / totalWeight) * 100
                return (
                  <div key={n.id} className="bar-row">
                    <div className="bar-label" style={{ color: colorFor(n.id) }}>{n.id}</div>
                    <div className="bar-track">
                      <div className="bar-fill" style={{ width: `${pct}%`, background: colorFor(n.id) }} />
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', width: 80, fontSize: '0.72rem' }}>
                      <span style={{ color: 'var(--text-dim)' }}>{pct.toFixed(1)}% actual</span>
                      <span style={{ color: 'var(--text-muted)' }}>{expected.toFixed(1)}% expected</span>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function StepLookup({ lookupKey, setLookupKey, lookupResult, onLookup,
                       replicaKey, setReplicaKey, replicaResult, onReplica,
                       colorFor, nodes }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      <div>
        <div className="section-title">Key Lookup & Replication</div>
        <div className="section-subtitle">Finding a key's owner in O(log N)</div>
      </div>
      <div className="concept-box">
        <h3>Binary search on sorted vnodes</h3>
        The ring stores vnodes in a sorted slice. Lookup is:
        <div className="concept-formula">
          idx = sort.Search(len(vnodes), v.Position ≥ hash(key))<br />
          if idx == len(vnodes): idx = 0  // wrap around
        </div>
        For replication: walk clockwise, collect first R <em>distinct physical nodes</em>.
      </div>
      <div className="two-col">
        <div className="card">
          <div className="card-title">Single owner lookup</div>
          <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.75rem' }}>
            <input placeholder="any key string…" value={lookupKey}
              onChange={e => setLookupKey(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && onLookup()} />
            <button className="btn btn-primary" onClick={onLookup} disabled={!lookupKey || nodes.length === 0}>
              Lookup
            </button>
          </div>
          {lookupResult && (
            <div className="lookup-result">
              <div><span style={{ color: 'var(--text-dim)' }}>key: </span><span style={{ color: 'var(--accent2)' }}>{lookupResult.key}</span></div>
              <div><span style={{ color: 'var(--text-dim)' }}>owner: </span><span className="owner-highlight" style={{ color: colorFor(lookupResult.owner) }}>{lookupResult.owner}</span></div>
              <div><span style={{ color: 'var(--text-dim)' }}>ring version: </span><span style={{ color: 'var(--text-dim)' }}>{lookupResult.version}</span></div>
            </div>
          )}
          {nodes.length === 0 && <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Add nodes first.</p>}
        </div>
        <div className="card">
          <div className="card-title">Replica lookup (R=3)</div>
          <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '0.75rem' }}>
            <input placeholder="any key string…" value={replicaKey}
              onChange={e => setReplicaKey(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && onReplica()} />
            <button className="btn btn-primary" onClick={onReplica} disabled={!replicaKey || nodes.length < 2}>
              Replicate
            </button>
          </div>
          {replicaResult && (
            <div className="lookup-result">
              <div style={{ marginBottom: '0.5rem' }}><span style={{ color: 'var(--text-dim)' }}>key: </span><span style={{ color: 'var(--accent2)' }}>{replicaResult.key}</span></div>
              {replicaResult.replicas?.map((r, i) => (
                <div key={r}>
                  <span style={{ color: 'var(--text-dim)' }}>{i === 0 ? 'primary: ' : `replica-${i}: `}</span>
                  <span style={{ color: colorFor(r), fontWeight: i === 0 ? 600 : 400 }}>{r}</span>
                </div>
              ))}
            </div>
          )}
          {nodes.length < 2 && <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Add at least 2 nodes.</p>}
        </div>
      </div>
    </div>
  )
}

function StepRebalance({ lastMovement, simDist, nodes, colorFor, totalSimKeys, stats, onPreset }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      <div>
        <div className="section-title">Rebalance Analysis</div>
        <div className="section-subtitle">How much data moves when topology changes?</div>
      </div>
      <div className="concept-box">
        <h3>The 1/N guarantee</h3>
        When adding the <em>N</em>th node to a ring with <em>K</em> keys, only ~K/N keys need to move.
        All other keys remain on their current node — no thundering herd.
        <div className="concept-formula">
          expected_movement = 1/N = {nodes.length > 0 ? (100 / nodes.length).toFixed(1) : '?'}%
        </div>
      </div>

      {!lastMovement && (
        <div className="card">
          <div className="card-title">Try it</div>
          <p style={{ fontSize: '0.85rem', color: 'var(--text-dim)', marginBottom: '0.75rem' }}>
            Load the "small" preset, then add or remove a node on the left to see the rebalance stats here.
          </p>
          <button className="btn btn-primary" onClick={() => onPreset('small')}>Load 3-node preset</button>
        </div>
      )}

      {lastMovement && (
        <div className="two-col">
          <div className="card">
            <div className="card-title">Last topology change</div>
            <div className="stat-row">
              <span>Sample keys</span>
              <span className="stat-val">{lastMovement.TotalKeys?.toLocaleString()}</span>
            </div>
            <div className="stat-row">
              <span>Keys moved</span>
              <span className="stat-val" style={{ color: lastMovement.MovedPct > 50 ? '#f87171' : '#4ade80' }}>
                {lastMovement.MovedKeys?.toLocaleString()}
              </span>
            </div>
            <div className="stat-row">
              <span>% moved</span>
              <span className="stat-val">
                <span className={`movement-badge ${lastMovement.MovedPct <= 35 ? 'movement-good' : 'movement-warn'}`}>
                  {lastMovement.MovedPct?.toFixed(1)}%
                </span>
              </span>
            </div>
            <div className="stat-row">
              <span>Ring std dev</span>
              <span className="stat-val">{stats?.StdDev?.toFixed(5)}</span>
            </div>
          </div>
          <div className="card">
            <div className="card-title">Current distribution</div>
            {simDist && totalSimKeys > 0
              ? (
                <div className="bar-chart">
                  {Object.entries(simDist).sort((a, b) => b[1] - a[1]).map(([id, count]) => (
                    <div key={id} className="bar-row">
                      <div className="bar-label" style={{ color: colorFor(id) }}>{id}</div>
                      <div className="bar-track">
                        <div className="bar-fill" style={{ width: `${count / totalSimKeys * 100}%`, background: colorFor(id) }} />
                      </div>
                      <span className="bar-pct">{(count / totalSimKeys * 100).toFixed(1)}%</span>
                    </div>
                  ))}
                </div>
              )
              : <p style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>No data yet.</p>
            }
          </div>
        </div>
      )}

      <div className="card">
        <div className="card-title">Real-world numbers</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.3rem' }}>
          {[
            ['Redis Cluster', '16,384 slots', 'Fixed slot ring; key moves = slots/old_N per added node'],
            ['Cassandra', '256 vnodes/node', 'Token-aware drivers avoid coordinator hops'],
            ['Memcached libketama', '160 vnodes/server', 'MD5 hash; used by every major CDN'],
            ['DynamoDB', 'Managed partitions', 'Partition splits auto-balance; you only see throughput'],
          ].map(([sys, config, note]) => (
            <div key={sys} style={{ padding: '0.5rem', borderBottom: '1px solid var(--border)', fontSize: '0.82rem' }}>
              <span style={{ fontWeight: 600, color: 'var(--accent2)' }}>{sys}</span>
              <span style={{ color: 'var(--text-dim)', margin: '0 0.5rem' }}>·</span>
              <span style={{ fontFamily: 'var(--mono)', fontSize: '0.78rem' }}>{config}</span>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.75rem', marginTop: '0.2rem' }}>{note}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
