#!/usr/bin/env bash
# Load test: concurrent producers and consumers.
# Requires: curl, bc
set -euo pipefail

BASE="${MQ_BASE_URL:-http://localhost:8094}"
TOPIC="${LOAD_TOPIC:-load-test}"
PRODUCERS="${PRODUCERS:-4}"
MSGS_PER_PRODUCER="${MSGS_PER_PRODUCER:-250}"
CONSUMERS="${CONSUMERS:-2}"

echo "==> Setting up load-test topic..."
curl -sf -X POST "$BASE/v1/topics" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$TOPIC\",\"partitions\":4,\"retention_hours\":1}" > /dev/null || true

echo "==> Starting $PRODUCERS producers × $MSGS_PER_PRODUCER msgs each..."
START=$(date +%s%N)

produce() {
  local id=$1
  for i in $(seq 1 $MSGS_PER_PRODUCER); do
    curl -sf -X POST "$BASE/v1/topics/$TOPIC/messages" \
      -H "Content-Type: application/json" \
      -d "{\"key\":\"producer-$id\",\"payload\":\"{\\\"seq\\\":$i,\\\"p\\\":$id}\"}" > /dev/null
  done
  echo " producer-$id done"
}

for i in $(seq 1 $PRODUCERS); do
  produce $i &
done
wait

END_PRODUCE=$(date +%s%N)
ELAPSED_PRODUCE=$(echo "scale=3; ($END_PRODUCE - $START) / 1000000000" | bc)
TOTAL=$((PRODUCERS * MSGS_PER_PRODUCER))
THROUGHPUT=$(echo "scale=0; $TOTAL / $ELAPSED_PRODUCE" | bc)
echo "==> Published $TOTAL msgs in ${ELAPSED_PRODUCE}s (~${THROUGHPUT} msg/s)"

echo ""
echo "==> Starting $CONSUMERS consumers..."
START_POLL=$(date +%s%N)
ACKED=0

consume() {
  local group="load-group-$1"
  local count=0
  while true; do
    resp=$(curl -sf -X POST "$BASE/v1/topics/$TOPIC/messages:poll" \
      -H "Content-Type: application/json" \
      -d "{\"consumer_group\":\"$group\",\"partition\":-1,\"max_messages\":20,\"visibility_timeout_seconds\":30}")
    n=$(echo "$resp" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('count',0))")
    if [ "$n" = "0" ]; then break; fi
    # Ack all
    echo "$resp" | python3 -c "
import json,sys,subprocess
d=json.load(sys.stdin)
for m in d.get('messages',[]):
    subprocess.run(['curl','-sf','-X','POST','$BASE/v1/messages/'+m['id']+':ack',
                    '-H','Content-Type: application/json',
                    '-d','{\"consumer_group\":\"$group\"}'],capture_output=True)
" 2>/dev/null || true
    count=$((count + n))
  done
  echo " consumer-$1 acked $count"
}

for i in $(seq 1 $CONSUMERS); do
  consume $i &
done
wait

END_POLL=$(date +%s%N)
ELAPSED_POLL=$(echo "scale=3; ($END_POLL - $START_POLL) / 1000000000" | bc)
echo "==> Poll+ack cycle in ${ELAPSED_POLL}s"

echo ""
echo "==> Final stats:"
curl -sf "$BASE/v1/stats" | python3 -m json.tool
