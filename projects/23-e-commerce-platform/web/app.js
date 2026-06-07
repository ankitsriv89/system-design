'use strict';

const BASE = '';

// ── Helpers ──────────────────────────────────────────────────────────────────

function log(msg, type = '') {
  const box = document.getElementById('log');
  const el = document.createElement('div');
  el.className = 'log-entry ' + type;
  el.textContent = new Date().toISOString().slice(11, 23) + ' ' + msg;
  box.prepend(el);
  while (box.children.length > 80) box.lastChild.remove();
}

async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body !== undefined) opts.body = JSON.stringify(body);
  try {
    const res = await fetch(BASE + path, opts);
    const text = await res.text();
    const data = text ? JSON.parse(text) : null;
    if (!res.ok) throw Object.assign(new Error(data?.error || res.statusText), { status: res.status });
    return data;
  } catch (e) {
    log('ERR ' + path + ': ' + e.message, 'err');
    throw e;
  }
}

function esc(s) {
  const d = document.createElement('div');
  d.textContent = String(s ?? '');
  return d.innerHTML;
}

function statusBadge(s) {
  return `<span class="status-badge status-${esc(s)}">${esc(s)}</span>`;
}

// ── Status dot ───────────────────────────────────────────────────────────────

async function checkHealth() {
  const dot = document.getElementById('statusDot');
  try {
    await fetch(BASE + '/actuator/health');
    dot.className = 'status-dot ok';
  } catch {
    dot.className = 'status-dot err';
  }
}

// ── Saga flow ─────────────────────────────────────────────────────────────────

const SAGA_STEPS = [
  'PENDING', 'INVENTORY_RESERVED', 'PAYMENT_AUTHORIZED', 'CONFIRMED',
  'INVENTORY_FAILED', 'PAYMENT_FAILED', 'CANCELLED'
];

function highlightSagaStep(status) {
  SAGA_STEPS.forEach(s => {
    const el = document.getElementById('step-' + s);
    if (el) el.classList.toggle('active', s === status);
  });
}

// ── Catalog ──────────────────────────────────────────────────────────────────

async function loadProducts() {
  const products = await api('GET', '/v1/products');
  renderProducts(products);
  log('Loaded ' + products.length + ' products', 'ok');
}

async function searchProducts() {
  const q = document.getElementById('searchQuery').value.trim();
  if (!q) { loadProducts(); return; }
  const products = await api('GET', '/v1/products?q=' + encodeURIComponent(q));
  renderProducts(products);
  log('Search "' + q + '" → ' + products.length + ' results');
}

function renderProducts(products) {
  const list = document.getElementById('productList');
  if (!products.length) { list.innerHTML = '<div style="color:var(--muted);font-size:11px">No products</div>'; return; }
  list.innerHTML = products.map(p => {
    const stockClass = p.stock === 0 ? 'prod-stock-zero' : p.stock < 5 ? 'prod-stock-low' : 'prod-stock-ok';
    return `<div class="product-card" onclick="copyProductId('${esc(p.id)}')">
      <div class="prod-name">${esc(p.name)}</div>
      <div class="prod-meta">SKU: ${esc(p.sku)} · ${esc(p.category || '—')}</div>
      <div style="display:flex;justify-content:space-between;margin-top:4px">
        <span class="prod-price">$${esc(Number(p.price).toFixed(2))}</span>
        <span class="${stockClass}">stock: ${esc(p.stock)}</span>
      </div>
      <div style="color:var(--muted);font-size:10px;margin-top:2px">${esc(p.id)}</div>
    </div>`;
  }).join('');
}

function copyProductId(id) {
  document.getElementById('addProductId').value = id;
  log('Selected product ' + id, 'info');
}

async function createProduct() {
  const sku   = document.getElementById('newSku').value.trim();
  const name  = document.getElementById('newName').value.trim();
  const price = parseFloat(document.getElementById('newPrice').value);
  const stock = parseInt(document.getElementById('newStock').value, 10);
  const category = document.getElementById('newCategory').value.trim();

  if (!sku || !name) { log('SKU and name are required', 'err'); return; }

  const p = await api('POST', '/v1/products', { sku, name, price, stock, category });
  log('Created product ' + p.id + ' (' + p.sku + ')', 'ok');
  loadProducts();
}

// ── Cart ─────────────────────────────────────────────────────────────────────

async function viewCart() {
  const userId = document.getElementById('cartUserId').value.trim();
  const cart = await api('GET', '/v1/cart/' + userId);
  renderCart(cart);
  log('Cart for ' + userId + ': ' + cart.items.length + ' items  total=$' + Number(cart.total || 0).toFixed(2));
}

async function addToCart() {
  const userId    = document.getElementById('cartUserId').value.trim();
  const productId = document.getElementById('addProductId').value.trim();
  const qty       = parseInt(document.getElementById('addQty').value, 10);

  if (!productId) { log('Enter a product ID', 'err'); return; }

  const cart = await api('POST', '/v1/cart/' + userId + '/items', { productId, quantity: qty });
  renderCart(cart);
  log('Added ' + qty + '× ' + productId + ' to cart', 'ok');
}

function renderCart(cart) {
  const el = document.getElementById('cartDisplay');
  if (!cart.items || !cart.items.length) {
    el.innerHTML = '<div style="color:var(--muted);font-size:11px">Cart is empty</div>';
    return;
  }
  el.innerHTML = cart.items.map(i =>
    `<div class="cart-item">
      <span>${esc(i.name)} ×${esc(i.quantity)}</span>
      <span style="color:var(--success)">$${esc(Number(i.lineTotal || i.unitPrice * i.quantity).toFixed(2))}</span>
    </div>`
  ).join('') +
    `<div class="cart-item" style="border-top:1px solid var(--border);margin-top:4px;font-weight:600">
      <span>Total</span><span style="color:var(--success)">$${esc(Number(cart.total || 0).toFixed(2))}</span>
    </div>`;
}

// ── Checkout ──────────────────────────────────────────────────────────────────

async function checkout() {
  const userId = document.getElementById('checkoutUserId').value.trim();
  const key    = 'idem-' + Date.now();
  const result = document.getElementById('checkoutResult');

  try {
    const order = await api('POST', '/v1/orders', { userId, idempotencyKey: key });
    highlightSagaStep(order.status);
    result.innerHTML = `<div style="color:var(--success)">Order created: ${esc(order.id)}</div>
      <div>${statusBadge(order.status)} · $${esc(Number(order.total).toFixed(2))}</div>`;
    log('Order ' + order.id + ' → ' + order.status, 'ok');

    // Poll for saga completion.
    pollOrderStatus(order.id);
  } catch (e) {
    result.innerHTML = `<div style="color:var(--error)">${esc(e.message)}</div>`;
  }
}

async function pollOrderStatus(orderId) {
  for (let i = 0; i < 10; i++) {
    await new Promise(r => setTimeout(r, 1000));
    try {
      const order = await api('GET', '/v1/orders/' + orderId);
      highlightSagaStep(order.status);
      log('Order ' + orderId + ' → ' + order.status, 'info');
      if (['CONFIRMED', 'PAYMENT_FAILED', 'INVENTORY_FAILED', 'CANCELLED'].includes(order.status)) {
        loadOrders();
        break;
      }
    } catch { break; }
  }
}

// ── Orders ────────────────────────────────────────────────────────────────────

async function loadOrders() {
  const userId = document.getElementById('ordersUserId').value.trim();
  const orders = await api('GET', '/v1/orders?userId=' + encodeURIComponent(userId));
  const list = document.getElementById('orderList');
  if (!orders.length) { list.innerHTML = '<div style="color:var(--muted);font-size:11px">No orders</div>'; return; }
  list.innerHTML = orders.map(o =>
    `<div class="order-card">
      <div class="order-id">${esc(o.id)}</div>
      <div style="display:flex;justify-content:space-between;margin-top:4px">
        ${statusBadge(o.status)}
        <span style="color:var(--success)">$${esc(Number(o.total).toFixed(2))}</span>
      </div>
    </div>`
  ).join('');
}

// ── Admin ─────────────────────────────────────────────────────────────────────

async function loadAllOrders() {
  const orders = await api('GET', '/v1/admin/orders');
  const list = document.getElementById('adminOrderList');
  if (!orders.length) { list.innerHTML = '<div style="color:var(--muted);font-size:11px">No orders</div>'; return; }
  list.innerHTML = orders.map(o =>
    `<div class="order-card">
      <div class="order-id">${esc(o.id.slice(0,18))}…</div>
      <div style="display:flex;justify-content:space-between;align-items:center;margin-top:4px;gap:4px">
        ${statusBadge(o.status)}
        <span style="color:var(--muted);font-size:10px">${esc(o.userId)}</span>
        <span style="color:var(--success)">$${esc(Number(o.total).toFixed(2))}</span>
        ${o.status === 'CONFIRMED'
          ? `<button class="btn btn-warning" style="padding:2px 6px;font-size:10px" onclick="shipOrder('${esc(o.id)}')">Ship</button>`
          : ''}
        ${o.status === 'SHIPPED'
          ? `<button class="btn btn-success" style="padding:2px 6px;font-size:10px" onclick="deliverOrder('${esc(o.id)}')">Deliver</button>`
          : ''}
      </div>
    </div>`
  ).join('');
  log('Loaded ' + orders.length + ' orders (admin)');
}

async function shipOrder(id) {
  await api('POST', '/v1/admin/orders/' + id + '/ship');
  log('Shipped order ' + id, 'ok');
  loadAllOrders();
}

async function deliverOrder(id) {
  await api('POST', '/v1/admin/orders/' + id + '/deliver');
  log('Delivered order ' + id, 'ok');
  loadAllOrders();
}

// ── Init ──────────────────────────────────────────────────────────────────────

checkHealth();
loadProducts();
setInterval(checkHealth, 30000);
