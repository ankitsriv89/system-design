#!/usr/bin/env bash
# Seed the e-commerce platform with sample products.
set -euo pipefail

BASE="${ECOMMERCE_BASE_URL:-http://localhost:8104}"

echo "==> Seeding products..."

create_product() {
  curl -sf -X POST "$BASE/v1/products" \
    -H "Content-Type: application/json" \
    -d "$1" | python3 -c "import json,sys; p=json.load(sys.stdin); print('  created:', p['id'], p['sku'])"
}

create_product '{"sku":"BOOK-001","name":"The Pragmatic Programmer","description":"From journeyman to master.","price":42.99,"stock":50,"category":"books"}'
create_product '{"sku":"BOOK-002","name":"Designing Data-Intensive Applications","description":"The big ideas behind reliable, scalable, and maintainable systems.","price":54.99,"stock":30,"category":"books"}'
create_product '{"sku":"BOOK-003","name":"Clean Code","description":"A handbook of agile software craftsmanship.","price":38.99,"stock":5,"category":"books"}'
create_product '{"sku":"ELEC-001","name":"Mechanical Keyboard","description":"TKL layout, brown switches.","price":129.99,"stock":20,"category":"electronics"}'
create_product '{"sku":"ELEC-002","name":"USB-C Hub","description":"7-in-1 multiport adapter.","price":49.99,"stock":100,"category":"electronics"}'
create_product '{"sku":"ELEC-003","name":"Monitor Stand","description":"Adjustable aluminium riser.","price":79.99,"stock":0,"category":"electronics"}'

echo ""
echo "==> Done. Products seeded."
