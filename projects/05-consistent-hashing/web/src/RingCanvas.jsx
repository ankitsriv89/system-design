import { useEffect, useRef, useState } from 'react'

// 20 visually distinct hues mapped to node IDs deterministically
const NODE_COLORS = [
  '#6366f1','#22d3ee','#f472b6','#4ade80','#fbbf24',
  '#f87171','#a78bfa','#34d399','#fb923c','#38bdf8',
  '#e879f9','#84cc16','#f43f5e','#2dd4bf','#facc15',
]

function colorForNode(_id, index) {
  return NODE_COLORS[index % NODE_COLORS.length]
}

// Hash a string to [0, 2^32) — mirrors the Go SHA256 approach in JS
async function hashToPosition(s) {
  const enc = new TextEncoder()
  const buf = await crypto.subtle.digest('SHA-256', enc.encode(s))
  const view = new DataView(buf)
  return view.getUint32(0)
}

const MAX32 = 4294967296

function posToAngle(pos) {
  return (pos / MAX32) * 2 * Math.PI - Math.PI / 2
}

function polarToXY(cx, cy, r, angle) {
  return {
    x: cx + r * Math.cos(angle),
    y: cy + r * Math.sin(angle),
  }
}

export default function RingCanvas({ nodes, highlightKey, vnodeData }) {
  const SIZE = 500
  const CX = SIZE / 2
  const CY = SIZE / 2
  const R_RING = 190
  const R_NODE = 170

  const [nodeColors, setNodeColors] = useState({})
  const [vnodePositions, setVnodePositions] = useState([])
  const [keyPos, setKeyPos] = useState(null)
  const [keyOwner, setKeyOwner] = useState(null)
  const prevNodesRef = useRef([])

  // Assign colours to nodes
  useEffect(() => {
    setNodeColors(prev => {
      const next = { ...prev }
      nodes.forEach((n, i) => {
        if (!next[n.id]) next[n.id] = colorForNode(n.id, i)
      })
      return next
    })
  }, [nodes])

  // Compute vnode arc positions from vnodeData (array of {Position, NodeID})
  useEffect(() => {
    if (!vnodeData || vnodeData.length === 0) {
      setVnodePositions([])
      return
    }
    const pts = vnodeData.map(v => ({
      angle: posToAngle(v.Position),
      nodeID: v.NodeID,
      position: v.Position,
    }))
    setVnodePositions(pts)
  }, [vnodeData])

  // Compute key position
  useEffect(() => {
    if (!highlightKey || nodes.length === 0 || vnodeData.length === 0) {
      setKeyPos(null)
      setKeyOwner(null)
      return
    }
    let cancelled = false
    ;(async () => {
      const h = await hashToPosition(highlightKey)
      if (cancelled) return
      const angle = posToAngle(h)
      setKeyPos({ angle, hash: h })
      // find successor
      const sorted = [...vnodeData].sort((a, b) => a.Position - b.Position)
      let owner = sorted.find(v => v.Position >= h)
      if (!owner) owner = sorted[0]
      if (!cancelled) setKeyOwner(owner?.NodeID ?? null)
    })()
    return () => { cancelled = true }
  }, [highlightKey, vnodeData, nodes])

  useEffect(() => { prevNodesRef.current = nodes }, [nodes])

  // Build arc segments per node (for ring colouring)
  const arcSegments = []
  if (vnodePositions.length > 0 && Object.keys(nodeColors).length > 0) {
    const sorted = [...vnodePositions].sort((a, b) => a.angle - b.angle)
    sorted.forEach((v, i) => {
      const next = sorted[(i + 1) % sorted.length]
      const startAngle = v.angle
      let endAngle = next.angle
      if (endAngle <= startAngle) endAngle += 2 * Math.PI
      const midAngle = (startAngle + endAngle) / 2
      arcSegments.push({ startAngle, endAngle, nodeID: v.nodeID, midAngle })
    })
  }

  function describeArc(cx, cy, r, startAngle, endAngle) {
    const s = polarToXY(cx, cy, r, startAngle)
    const e = polarToXY(cx, cy, r, endAngle)
    const largeArc = endAngle - startAngle > Math.PI ? 1 : 0
    return `M ${s.x} ${s.y} A ${r} ${r} 0 ${largeArc} 1 ${e.x} ${e.y}`
  }

  // Compute average position per physical node for label placement
  const nodeLabels = nodes.map(n => {
    const pts = vnodePositions.filter(v => v.nodeID === n.id)
    if (pts.length === 0) return null
    // circular mean
    let sx = 0, sy = 0
    pts.forEach(p => { sx += Math.cos(p.angle); sy += Math.sin(p.angle) })
    const meanAngle = Math.atan2(sy / pts.length, sx / pts.length)
    const xy = polarToXY(CX, CY, R_NODE, meanAngle)
    return { id: n.id, x: xy.x, y: xy.y, angle: meanAngle, color: nodeColors[n.id] }
  }).filter(Boolean)

  return (
    <svg
      width={SIZE}
      height={SIZE}
      viewBox={`0 0 ${SIZE} ${SIZE}`}
      style={{ maxWidth: '100%', height: 'auto' }}
    >
      <defs>
        <radialGradient id="ringGrad" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stopColor="#1a1e2a" />
          <stop offset="100%" stopColor="#0d0f14" />
        </radialGradient>
        <filter id="glow">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
        </filter>
        <filter id="glow-strong">
          <feGaussianBlur stdDeviation="5" result="blur" />
          <feMerge><feMergeNode in="blur" /><feMergeNode in="SourceGraphic" /></feMerge>
        </filter>
      </defs>

      {/* Background */}
      <circle cx={CX} cy={CY} r={SIZE / 2 - 2} fill="url(#ringGrad)" />

      {/* Coloured arc segments */}
      {arcSegments.map((seg, i) => (
        <path
          key={i}
          d={describeArc(CX, CY, R_RING, seg.startAngle, seg.endAngle)}
          stroke={nodeColors[seg.nodeID] ?? '#334155'}
          strokeWidth={8}
          fill="none"
          strokeOpacity={0.55}
        />
      ))}

      {/* Base ring track */}
      <circle
        cx={CX} cy={CY} r={R_RING}
        fill="none"
        stroke="#252a38"
        strokeWidth={2}
        strokeDasharray="6 4"
      />

      {/* Clock markers at 0, π/2, π, 3π/2 */}
      {[0, 0.25, 0.5, 0.75].map((frac, i) => {
        const a = frac * 2 * Math.PI - Math.PI / 2
        const outer = polarToXY(CX, CY, R_RING + 14, a)
        const inner = polarToXY(CX, CY, R_RING - 14, a)
        const label = polarToXY(CX, CY, R_RING + 28, a)
        const labels = ['0', '2³⁰', '2³¹', '3·2³⁰']
        return (
          <g key={i}>
            <line x1={inner.x} y1={inner.y} x2={outer.x} y2={outer.y} stroke="#334155" strokeWidth={1} />
            <text x={label.x} y={label.y} textAnchor="middle" dominantBaseline="middle"
              fontSize={9} fill="#475569" fontFamily="JetBrains Mono, monospace">{labels[i]}</text>
          </g>
        )
      })}

      {/* Virtual node dots */}
      {vnodePositions.map((v, i) => {
        const xy = polarToXY(CX, CY, R_RING, v.angle)
        return (
          <circle
            key={i}
            cx={xy.x} cy={xy.y} r={3}
            fill={nodeColors[v.nodeID] ?? '#475569'}
            opacity={0.7}
          />
        )
      })}

      {/* Key position on ring */}
      {keyPos && (() => {
        const xy = polarToXY(CX, CY, R_RING, keyPos.angle)
        const ownerColor = nodeColors[keyOwner] ?? '#fff'
        // line from centre to ring
        const inner = polarToXY(CX, CY, 40, keyPos.angle)
        return (
          <g filter="url(#glow-strong)">
            <line x1={inner.x} y1={inner.y} x2={xy.x} y2={xy.y}
              stroke={ownerColor} strokeWidth={1.5} strokeDasharray="4 3" strokeOpacity={0.6} />
            <circle cx={xy.x} cy={xy.y} r={8} fill={ownerColor} opacity={0.25} />
            <circle cx={xy.x} cy={xy.y} r={5} fill={ownerColor} />
            <text x={xy.x} y={xy.y - 14} textAnchor="middle"
              fontSize={10} fill={ownerColor} fontFamily="JetBrains Mono, monospace"
              fontWeight="600">key</text>
          </g>
        )
      })()}

      {/* Arrow from key to owner node */}
      {keyPos && keyOwner && (() => {
        const keyXY = polarToXY(CX, CY, R_RING, keyPos.angle)
        const ownerLabel = nodeLabels.find(l => l.id === keyOwner)
        if (!ownerLabel) return null
        const ownerColor = nodeColors[keyOwner] ?? '#fff'
        return (
          <line
            x1={keyXY.x} y1={keyXY.y}
            x2={ownerLabel.x} y2={ownerLabel.y}
            stroke={ownerColor} strokeWidth={1.5}
            strokeDasharray="5 3"
            strokeOpacity={0.5}
            markerEnd="none"
          />
        )
      })()}

      {/* Node labels (physical) */}
      {nodeLabels.map(n => {
        const r = R_NODE + 28
        const xy = polarToXY(CX, CY, r, n.angle)
        const isOwner = keyOwner === n.id
        return (
          <g key={n.id} filter={isOwner ? 'url(#glow)' : undefined}>
            {/* connector from ring to label */}
            {(() => {
              const ringXY = polarToXY(CX, CY, R_RING, n.angle)
              const labelXY = polarToXY(CX, CY, r - 18, n.angle)
              return (
                <line x1={ringXY.x} y1={ringXY.y} x2={labelXY.x} y2={labelXY.y}
                  stroke={n.color} strokeWidth={1} strokeOpacity={0.3} />
              )
            })()}
            <rect
              x={xy.x - 28} y={xy.y - 12}
              width={56} height={24}
              rx={6}
              fill={isOwner
                ? n.color
                : `color-mix(in srgb, ${n.color} 20%, #13161e)`}
              stroke={n.color}
              strokeWidth={isOwner ? 2 : 1}
              strokeOpacity={isOwner ? 1 : 0.5}
            />
            <text
              x={xy.x} y={xy.y}
              textAnchor="middle"
              dominantBaseline="middle"
              fontSize={10}
              fontWeight={isOwner ? '700' : '500'}
              fill={isOwner ? '#0d0f14' : n.color}
              fontFamily="JetBrains Mono, monospace"
            >
              {n.id.length > 7 ? n.id.slice(0, 7) + '…' : n.id}
            </text>
          </g>
        )
      })}

      {/* Centre text */}
      <text x={CX} y={CY - 10} textAnchor="middle" fontSize={13} fill="#475569" fontFamily="Inter, sans-serif" fontWeight="600">
        Hash Ring
      </text>
      <text x={CX} y={CY + 8} textAnchor="middle" fontSize={10} fill="#334155" fontFamily="JetBrains Mono, monospace">
        {`[0, 2³²)`}
      </text>
      {nodes.length === 0 && (
        <text x={CX} y={CY + 28} textAnchor="middle" fontSize={10} fill="#475569">
          Add a node →
        </text>
      )}
    </svg>
  )
}
