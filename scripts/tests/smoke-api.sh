#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"

API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:8080}"
START_INFRA="${START_INFRA:-1}"
PREPARE_RAG_DATA="${PREPARE_RAG_DATA:-1}"
WAIT_TIMEOUT_SECONDS="${WAIT_TIMEOUT_SECONDS:-120}"
QA_TIMEOUT_SECONDS="${QA_TIMEOUT_SECONDS:-120}"
QA_QUESTION="${QA_QUESTION:-Quels sont les enjeux principaux de cette votation ?}"
TEST_ENV_FILE="${TEST_ENV_FILE:-.env.test}"
TEST_COMPOSE_FILE="${TEST_COMPOSE_FILE:-docker-compose.test.yml}"

echo "==> Civika API smoke test"
echo "Repo: $REPO_ROOT"
echo "API:  $API_BASE_URL"

pretty_print_json() {
  payload="$1"
  printf "%s" "$payload" | python3 -m json.tool
}

if [ "$START_INFRA" = "1" ]; then
  echo "==> Demarrage de l'infra de test (make test-infra-up)"
  make -C "$REPO_ROOT" test-infra-up
else
  echo "==> START_INFRA=0, infra non redemarree"
fi

if [ "$PREPARE_RAG_DATA" = "1" ]; then
  echo "==> Preparation donnees RAG (data-fetch + index)"
  docker compose --env-file "$REPO_ROOT/$TEST_ENV_FILE" -f "$REPO_ROOT/$TEST_COMPOSE_FILE" exec -T rag_chunker sh -lc "/app/data-fetch && /app/rag-cli index"
else
  echo "==> PREPARE_RAG_DATA=0, preparation RAG ignoree"
fi

echo "==> Attente de l'API /health"
attempt=0
max_attempts=$((WAIT_TIMEOUT_SECONDS / 2))
while [ "$attempt" -lt "$max_attempts" ]; do
  if curl --max-time 3 -fsS "$API_BASE_URL/health" >/dev/null 2>&1; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done
if [ "$attempt" -ge "$max_attempts" ]; then
  echo "Erreur: API indisponible apres ${WAIT_TIMEOUT_SECONDS}s" >&2
  exit 1
fi

echo "==> GET /health"
HEALTH_JSON="$(curl --max-time 8 -fsS "$API_BASE_URL/health")"
printf "%s" "$HEALTH_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("status")=="ok", "health status != ok"'
pretty_print_json "$HEALTH_JSON"

echo "==> GET /info"
INFO_JSON="$(curl --max-time 8 -fsS "$API_BASE_URL/info")"
printf "%s" "$INFO_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "ragMode" in d, "missing ragMode"; assert "features" in d, "missing features"'
pretty_print_json "$INFO_JSON"

echo "==> GET /api/v1/votations"
VOTATIONS_JSON="$(curl --max-time 8 -fsS "$API_BASE_URL/api/v1/votations?limit=5&offset=0&lang=fr")"
printf "%s" "$VOTATIONS_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d.get("items", []), list), "missing items array"'
pretty_print_json "$VOTATIONS_JSON"
printf "%s" "$VOTATIONS_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items", []); levels=sorted({x.get("level","") for x in items if x.get("level")}); statuses=sorted({x.get("status","") for x in items if x.get("status")}); print(f"Stats votations: items={len(items)} total={d.get('\''total'\'',0)} levels={levels} statuses={statuses}")'
VOTATION_ID="$(printf "%s" "$VOTATIONS_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items", []); print(items[0].get("id","") if items else "")')"

echo "==> GET /api/v1/taxonomies"
TAXO_JSON="$(curl --max-time 8 -fsS "$API_BASE_URL/api/v1/taxonomies")"
printf "%s" "$TAXO_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "levels" in d, "missing levels"; assert "languages" in d, "missing languages"'
pretty_print_json "$TAXO_JSON"
printf "%s" "$TAXO_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(f"Stats taxonomies: levels={len(d.get('\''levels'\'',[]))} cantons={len(d.get('\''cantons'\'',[]))} statuses={len(d.get('\''statuses'\'',[]))} languages={len(d.get('\''languages'\'',[]))} objectTypes={len(d.get('\''objectTypes'\'',[]))} themes={len(d.get('\''themes'\'',[]))}")'

if [ -n "$VOTATION_ID" ]; then
  echo "==> GET /api/v1/votations/$VOTATION_ID"
  VOTATION_DETAIL_JSON="$(curl --max-time 8 -fsS "$API_BASE_URL/api/v1/votations/$VOTATION_ID")"
  printf "%s" "$VOTATION_DETAIL_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("id"), "missing id"'
  pretty_print_json "$VOTATION_DETAIL_JSON"
  printf "%s" "$VOTATION_DETAIL_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(f"Stats votation detail: objectIds={len(d.get('\''objectIds'\'',[]))} sourceUrls={len(d.get('\''sourceUrls'\'',[]))} titles={len(d.get('\''titles'\'',{}))}")'

  echo "==> GET /api/v1/votations/$VOTATION_ID/objects"
  OBJECTS_JSON="$(curl --max-time 8 -fsS "$API_BASE_URL/api/v1/votations/$VOTATION_ID/objects?lang=fr")"
  printf "%s" "$OBJECTS_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d.get("items", []), list), "missing object items array"'
  pretty_print_json "$OBJECTS_JSON"
  printf "%s" "$OBJECTS_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items", []); types=sorted({x.get("type","") for x in items if x.get("type")}); print(f"Stats objects: items={len(items)} distinct_types={types}")'
  OBJECT_ID="$(printf "%s" "$OBJECTS_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items", []); print(items[0].get("id","") if items else "")')"

  if [ -n "$OBJECT_ID" ]; then
    echo "==> GET /api/v1/objects/$OBJECT_ID"
    OBJECT_DETAIL_JSON="$(curl --max-time 8 -fsS "$API_BASE_URL/api/v1/objects/$OBJECT_ID?lang=fr")"
    printf "%s" "$OBJECT_DETAIL_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d.get("id"), "missing object id"'
    pretty_print_json "$OBJECT_DETAIL_JSON"
    printf "%s" "$OBJECT_DETAIL_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(f"Stats object detail: tags={len(d.get('\''tags'\'',[]))} sections={len(d.get('\''sections'\'',{}))} sourceSystems={len(d.get('\''sourceSystems'\'',[]))}")'

    echo "==> GET /api/v1/objects/$OBJECT_ID/sources"
    SOURCES_JSON="$(curl --max-time 8 -fsS "$API_BASE_URL/api/v1/objects/$OBJECT_ID/sources")"
    printf "%s" "$SOURCES_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert isinstance(d.get("items", []), list), "missing sources items array"'
    pretty_print_json "$SOURCES_JSON"
    printf "%s" "$SOURCES_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items", []); official=sum(1 for x in items if x.get("type")=="official"); print(f"Stats sources: items={len(items)} official={official}")'
  fi
fi

echo "==> POST /api/v1/qa/query"
QA_PAYLOAD="$(cat <<EOF
{
  "question": "$QA_QUESTION",
  "language": "fr",
  "context": {
    "votationId": "$VOTATION_ID",
    "objectId": "",
    "canton": "GE"
  },
  "client": {
    "instance": "smoke-script",
    "version": "1.0.0"
  }
}
EOF
)"
QA_JSON="$(curl --max-time "$QA_TIMEOUT_SECONDS" -fsS -X POST "$API_BASE_URL/api/v1/qa/query" -H "Content-Type: application/json" -d "$QA_PAYLOAD")"
printf "%s" "$QA_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert "answer" in d, "missing answer"; assert "meta" in d, "missing meta"'
pretty_print_json "$QA_JSON"
printf "%s" "$QA_JSON" | python3 -c 'import json,sys; d=json.load(sys.stdin); answer=d.get("answer",""); meta=d.get("meta",{}); citations=d.get("citations",[]); print(f"Stats qa: answer_chars={len(answer)} citations={len(citations)} used_documents={len(meta.get('\''usedDocuments'\'',[]))} confidence={meta.get('\''confidence'\'')}")'

echo "Smoke test API termine avec succes."
