#!/usr/bin/env bash
# Seed the typeahead corpus with sample items.
set -euo pipefail

BASE="${BASE_URL:-http://localhost:8092}"

seed() {
  local text="$1" category="$2" popularity="$3" locale="${4:-en}"
  curl -sf -X POST "$BASE/v1/corpus/items" \
    -H "Content-Type: application/json" \
    -d "{\"text\":\"$text\",\"category\":\"$category\",\"popularity\":$popularity,\"locale\":\"$locale\"}" > /dev/null
  echo "  seeded: $text"
}

echo "Seeding corpus at $BASE..."

seed "golang"               "language"       950
seed "google search"        "product"        990
seed "goroutine"            "concept"        800
seed "grpc"                 "protocol"       850
seed "graphql"              "protocol"       870
seed "redis sorted sets"    "data-structure" 820
seed "redis cluster"        "infra"          780
seed "redis streams"        "data-structure" 750
seed "typeahead"            "concept"        760
seed "trie data structure"  "concept"        710
seed "postgresql full text" "database"       690
seed "prometheus metrics"   "observability"  680
seed "docker compose"       "infra"          900
seed "elasticsearch"        "search"         880
seed "kafka streams"        "messaging"      830
seed "kubernetes pod"       "infra"          860
seed "system design"        "concept"        930
seed "consistent hashing"   "concept"        720
seed "rate limiting"        "concept"        740
seed "load balancer"        "infra"          800
seed "cdn edge"             "infra"          770
seed "bloom filter"         "data-structure" 650
seed "lru cache"            "data-structure" 790
seed "distributed tracing"  "observability"  700
seed "opentelemetry"        "observability"  660

echo "Triggering index rebuild..."
curl -sf -X POST "$BASE/v1/admin/rebuild-index" | jq .
echo "Done."
