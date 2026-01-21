#!/bin/sh
set -eu

ES_URL="http://es-coord:9200"
INDEX="movies"

# 只有当 RESET_MOVIES=true/1/yes 才会 delete + 重建
RESET="${RESET_MOVIES:-false}"

echo "[es-data-ingest] ES_URL=$ES_URL INDEX=$INDEX RESET_MOVIES=$RESET"

# ----------------------------
# 0) wait for ES
# ----------------------------
echo "[es-data-ingest] waiting for ES..."
until curl -fsS "$ES_URL" >/dev/null 2>&1; do
  sleep 1
done
echo "[es-data-ingest] ES is up"

# ----------------------------
# 1) optional reset: delete index
# ----------------------------
case "$RESET" in
  true|1|yes|YES|TRUE)
    echo "[es-data-ingest] RESET enabled: deleting index $INDEX (ignore if missing)"
    curl -fsS -X DELETE "$ES_URL/$INDEX" >/dev/null 2>&1 || true
    ;;
  *)
    echo "[es-data-ingest] RESET disabled: keep existing index if any"
    ;;
esac

# ----------------------------
# 2) ensure index exists (create with 3 shards + 1 replica + mapping)
#    NOTE: number_of_shards is fixed at creation time
# ----------------------------
if curl -fsS -o /dev/null "$ES_URL/$INDEX"; then
  echo "[es-data-ingest] index exists: $INDEX"
else
  echo "[es-data-ingest] creating index $INDEX (3 shards, 1 replica) + mapping"
  curl -fsS -X PUT "$ES_URL/$INDEX" \
    -H "Content-Type: application/json" \
    -d '{
      "settings": {
        "number_of_shards": 3,
        "number_of_replicas": 1
      },
      "mappings": {
        "properties": {
          "text":    { "type": "text" },
          "suggest": { "type": "completion" }
        }
      }
    }' >/dev/null
fi

# ----------------------------
# 3) bulk upsert (fixed _id => re-run won't duplicate)
# ----------------------------
echo "[es-data-ingest] bulk upsert docs (fixed _id)"
curl -fsS -X POST "$ES_URL/$INDEX/_bulk" \
  -H "Content-Type: application/x-ndjson" \
  --data-binary @- >/dev/null <<'NDJSON'
{"index":{"_id":"1"}}
{"text":"Batman Begins","suggest":{"input":["batman begins","batman","begins"]}}
{"index":{"_id":"2"}}
{"text":"The Dark Knight","suggest":{"input":["the dark knight","dark knight","batman"]}}
{"index":{"_id":"3"}}
{"text":"The Dark Knight Rises","suggest":{"input":["the dark knight rises","dark knight rises","batman"]}}
{"index":{"_id":"4"}}
{"text":"Inception","suggest":{"input":["inception"]}}
{"index":{"_id":"5"}}
{"text":"Interstellar","suggest":{"input":["interstellar"]}}
{"index":{"_id":"6"}}
{"text":"Inside Out","suggest":{"input":["inside out","inside"]}}
{"index":{"_id":"7"}}
{"text":"Iron Man","suggest":{"input":["iron man","iron"]}}
{"index":{"_id":"8"}}
{"text":"Indiana Jones","suggest":{"input":["indiana jones","indiana","jones"]}}
{"index":{"_id":"9"}}
{"text":"Avengers: Endgame","suggest":{"input":["avengers endgame","endgame","avengers"]}}
{"index":{"_id":"10"}}
{"text":"Avatar","suggest":{"input":["avatar"]}}
NDJSON

# ----------------------------
# 4) refresh + show shards & count
# ----------------------------
curl -fsS -X POST "$ES_URL/$INDEX/_refresh" >/dev/null
echo "[es-data-ingest] shards:"
curl -fsS "$ES_URL/_cat/shards/$INDEX?v"
echo "[es-data-ingest] count:"
curl -fsS "$ES_URL/$INDEX/_count"
echo "\n[es-data-ingest] done"
