/**
 * odyssey-scene.js — Three.js 3D scene for Interstellar Odyssey.
 *
 * Exports:
 *   sceneReady  — Promise that resolves when the scene is initialised
 *   setDestination(idx) — cross-fade background + reposition ship
 *   playWarp()          — warp jump animation (allowed)
 *   playDeny()          — denied flash + shake
 */

import * as THREE from 'three';

// ── Destination background images (local WebP, served by Go) ──────────────
const BG_IMAGES = [
  '/assets/bg/earth.webp',
  '/assets/bg/moon.webp',
  '/assets/bg/mars.webp',
  '/assets/bg/asteroid-belt.webp',
  '/assets/bg/jupiter.webp',
  '/assets/bg/saturn.webp',
  '/assets/bg/uranus.webp',
  '/assets/bg/neptune.webp',
  '/assets/bg/pluto.webp',
  '/assets/bg/alpha-centauri.webp',
  '/assets/bg/sgr-a.webp',
];

// ── Module state ──────────────────────────────────────────────────────────
let renderer, scene, camera, ship, engineLight;
let stars;
let warpLines = [];
let currentDestIdx = 0;

const clock = new THREE.Clock();

// ── Warp flash + deny flash DOM elements ─────────────────────────────────
function addFlashDivs() {
  ['warpFlash', 'denyFlash'].forEach(id => {
    if (!document.getElementById(id)) {
      const d = document.createElement('div');
      d.id = id;
      document.body.appendChild(d);
    }
  });
}

// ── Scene init ────────────────────────────────────────────────────────────
function initScene() {
  const canvas = document.getElementById('c');
  renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: false });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.setSize(window.innerWidth, window.innerHeight);
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.1;

  scene = new THREE.Scene();

  camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 2000);
  camera.position.set(0, 1.8, 6);
  camera.lookAt(0, 0, 0);

  // Ambient + directional light (simulating distant star)
  scene.add(new THREE.AmbientLight(0x112244, 1.8));
  const sun = new THREE.DirectionalLight(0xfff5e0, 2.5);
  sun.position.set(5, 8, 4);
  scene.add(sun);

  // Engine glow point light (cyan, attached to ship position)
  engineLight = new THREE.PointLight(0x61f3ff, 0, 4);
  engineLight.position.set(0, 0, 0.6);
  scene.add(engineLight);

  buildBackground();
  buildStars();
  buildWarpLines();

  window.addEventListener('resize', onResize);
  animate();
}

// ── Background — CSS div, always fills viewport perfectly ─────────────────
let bgDiv;
function buildBackground() {
  bgDiv = document.createElement('div');
  bgDiv.id = 'sceneBg';
  bgDiv.style.cssText = `
    position:fixed;inset:0;z-index:0;
    background-image:url(${BG_IMAGES[0]});
    background-size:cover;background-position:center;
    transition:opacity 0.4s ease;
  `;
  document.body.insertBefore(bgDiv, document.body.firstChild);

  // Make Three.js canvas transparent so bg div shows through
  renderer.setClearColor(0x000000, 0);
}

// ── Star field (particle system) ─────────────────────────────────────────
function buildStars() {
  const count = 3500;
  const geo = new THREE.BufferGeometry();
  const pos = new Float32Array(count * 3);
  const size = new Float32Array(count);

  for (let i = 0; i < count; i++) {
    pos[i * 3]     = (Math.random() - 0.5) * 80;
    pos[i * 3 + 1] = (Math.random() - 0.5) * 60;
    pos[i * 3 + 2] = (Math.random() - 0.5) * 40 - 5;
    size[i] = Math.random() * 1.8 + 0.4;
  }
  geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
  geo.setAttribute('size', new THREE.BufferAttribute(size, 1));

  const mat = new THREE.PointsMaterial({
    color: 0xffffff,
    size: 0.06,
    sizeAttenuation: true,
    transparent: true,
    opacity: 0.85,
  });
  stars = new THREE.Points(geo, mat);
  scene.add(stars);
}

// ── Warp lines (streaks during jump) ─────────────────────────────────────
function buildWarpLines() {
  const mat = new THREE.LineBasicMaterial({ color: 0x61f3ff, transparent: true, opacity: 0 });
  for (let i = 0; i < 60; i++) {
    const angle = Math.random() * Math.PI * 2;
    const radius = Math.random() * 3 + 0.5;
    const len = Math.random() * 8 + 4;
    const points = [
      new THREE.Vector3(Math.cos(angle) * radius, Math.sin(angle) * radius, 0),
      new THREE.Vector3(Math.cos(angle) * radius * 0.1, Math.sin(angle) * radius * 0.1, len),
    ];
    const geo = new THREE.BufferGeometry().setFromPoints(points);
    const line = new THREE.Line(geo, mat.clone());
    line.visible = false;
    scene.add(line);
    warpLines.push(line);
  }
}

// ── Ship ──────────────────────────────────────────────────────────────────
let shipLoadedResolve;
// shipReady resolves when the ship mesh is in the scene (used internally by loadShip)
new Promise(r => { shipLoadedResolve = r; });

function loadShip() {
  ship = buildProceduralShip();
  scene.add(ship);
  shipLoadedResolve();
}

function buildProceduralShip() {
  const group = new THREE.Group();
  group.name = 'ship_loaded';

  // Main bus (hexagonal prism body)
  const bodyGeo = new THREE.CylinderGeometry(0.28, 0.28, 0.18, 6);
  const bodyMat = new THREE.MeshStandardMaterial({ color: 0xc8d8e8, metalness: 0.8, roughness: 0.3 });
  const body = new THREE.Mesh(bodyGeo, bodyMat);
  body.rotation.x = Math.PI / 2;
  group.add(body);

  // High-gain dish (torus + plane)
  const dishRim = new THREE.Mesh(
    new THREE.TorusGeometry(0.35, 0.02, 8, 32),
    new THREE.MeshStandardMaterial({ color: 0xe8f4ff, metalness: 0.9, roughness: 0.2 })
  );
  dishRim.position.y = 0.32;
  dishRim.rotation.x = Math.PI / 2;
  group.add(dishRim);

  const dishFace = new THREE.Mesh(
    new THREE.CircleGeometry(0.33, 32),
    new THREE.MeshStandardMaterial({ color: 0xbbd0ee, metalness: 0.6, roughness: 0.5, side: THREE.DoubleSide })
  );
  dishFace.position.y = 0.31;
  dishFace.rotation.x = -Math.PI / 2;
  group.add(dishFace);

  // RTG booms (2 side booms)
  const boomMat = new THREE.MeshStandardMaterial({ color: 0x778899, metalness: 0.7, roughness: 0.4 });
  const rtgMat = new THREE.MeshStandardMaterial({ color: 0x445566, metalness: 0.5, roughness: 0.5 });

  [-1, 1].forEach(side => {
    const boom = new THREE.Mesh(new THREE.CylinderGeometry(0.015, 0.015, 0.9), boomMat);
    boom.position.set(side * 0.55, 0, 0.1);
    boom.rotation.z = Math.PI / 2;
    group.add(boom);

    const rtg = new THREE.Mesh(new THREE.BoxGeometry(0.06, 0.06, 0.22), rtgMat);
    rtg.position.set(side * 1.02, 0, 0.1);
    group.add(rtg);
  });

  // Science boom
  const sciBoom = new THREE.Mesh(new THREE.CylinderGeometry(0.01, 0.01, 0.7), boomMat);
  sciBoom.position.set(0, 0, -0.44);
  group.add(sciBoom);

  // Engine nozzle glow
  const nozzleGeo = new THREE.ConeGeometry(0.06, 0.18, 12);
  const nozzleMat = new THREE.MeshStandardMaterial({
    color: 0x61f3ff, emissive: 0x61f3ff, emissiveIntensity: 1.2,
    transparent: true, opacity: 0.8
  });
  const nozzle = new THREE.Mesh(nozzleGeo, nozzleMat);
  nozzle.position.set(0, 0, 0.26);
  nozzle.rotation.x = Math.PI / 2;
  group.add(nozzle);

  group.rotation.x = 0.12;
  return group;
}

// ── Animation loop ────────────────────────────────────────────────────────
let shakeIntensity = 0;

function animate() {
  requestAnimationFrame(animate);
  const t = clock.getElapsedTime();

  // Gentle ship bob + slow rotation
  if (ship) {
    ship.position.y = Math.sin(t * 0.6) * 0.06;
    ship.rotation.y = Math.sin(t * 0.2) * 0.12;
    // engine light pulse
    engineLight.intensity = 0.6 + Math.sin(t * 3) * 0.25;
    engineLight.position.copy(ship.position).z += 0.5;
  }

  // Star parallax drift
  if (stars) {
    stars.rotation.y = t * 0.005;
    stars.rotation.x = t * 0.002;
  }

  // Camera shake (deny effect)
  if (shakeIntensity > 0.001) {
    camera.position.x = (Math.random() - 0.5) * shakeIntensity;
    camera.position.y = 1.8 + (Math.random() - 0.5) * shakeIntensity;
    shakeIntensity *= 0.82;
  } else {
    camera.position.x = 0;
    camera.position.y = 1.8;
    shakeIntensity = 0;
  }

  renderer.render(scene, camera);
}

// ── Background cross-fade via CSS ─────────────────────────────────────────
function crossFadeBackground(idx) {
  if (!bgDiv) return;
  bgDiv.style.opacity = '0';
  setTimeout(() => {
    bgDiv.style.backgroundImage = `url(${BG_IMAGES[Math.min(idx, BG_IMAGES.length - 1)]})`;
    bgDiv.style.opacity = '1';
  }, 400);
}

// ── Warp jump animation ───────────────────────────────────────────────────
export function playWarp() {
  // Show warp lines
  warpLines.forEach(l => {
    l.visible = true;
    l.material.opacity = 0;
  });

  const start = performance.now();
  const dur = 1200;

  const tick = (now) => {
    const p = (now - start) / dur;
    if (p > 1) {
      warpLines.forEach(l => { l.visible = false; });
      return;
    }
    // Lines fade in then out
    const opacity = p < 0.5 ? p * 2 : (1 - p) * 2;
    warpLines.forEach(l => { l.material.opacity = opacity * 0.85; });

    // Ship accelerates into the screen
    if (ship) ship.position.z = -p * 3;

    requestAnimationFrame(tick);

    if (p > 0.6 && ship) {
      ship.position.z += 0.15; // snap back
    }
  };
  requestAnimationFrame(tick);

  // Flash
  const wf = document.getElementById('warpFlash');
  if (wf) {
    wf.classList.add('active');
    setTimeout(() => wf.classList.remove('active'), 300);
  }
}

// ── Deny effect ───────────────────────────────────────────────────────────
export function playDeny() {
  shakeIntensity = 0.25;
  engineLight.color.set(0xff3355);
  setTimeout(() => engineLight.color.set(0x61f3ff), 600);

  const df = document.getElementById('denyFlash');
  if (df) {
    df.classList.add('active');
    setTimeout(() => df.classList.remove('active'), 400);
  }
}

// ── Set destination ───────────────────────────────────────────────────────
export function setDestination(idx) {
  if (idx === currentDestIdx) return;
  currentDestIdx = idx;
  crossFadeBackground(idx);
}

// ── Resize ────────────────────────────────────────────────────────────────
function onResize() {
  camera.aspect = window.innerWidth / window.innerHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(window.innerWidth, window.innerHeight);
}

// ── Exports + init ────────────────────────────────────────────────────────
export const sceneReady = new Promise(resolve => {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => { addFlashDivs(); initScene(); loadShip(); resolve(); });
  } else {
    addFlashDivs(); initScene(); loadShip(); resolve();
  }
});
