SHELL := /bin/sh

TEST_ENV_FILE ?= .env.test
TEST_COMPOSE_FILE ?= docker-compose.test.yml
TEST_OLLAMA_MODEL ?= qwen3:4b
TEST_OLLAMA_EMBEDDING_MODEL ?= nomic-embed-text
TEST_OLLAMA_START_TIMEOUT ?= 30
TEST_OLLAMA_HEALTH_URL ?= http://127.0.0.1:11434/api/tags
PURGE_VOLUMES ?= 0

.PHONY: help env up down logs stack-smoke db-up init-db backend-test bootstrap data-fetch test-data-fetch rag-index rag-debug-chunks rag-query rag-debug-pipeline rag-integration-pipeline rag-ci test-ollama-check test-ollama-start test-ollama-pull test-stack-up test-db-init test-infra-up test-infra-logs test-infra-down test-infra-purge test-rag-shell test-api-smoke frontend-audit backend-vulncheck security-check check-go-image-consistency

help:
	@echo "Civika - cibles Makefile"
	@echo ""
	@echo "Configuration:"
	@echo "  make env                 # cree .env depuis .env.example si absent"
	@echo ""
	@echo "Stack Docker:"
	@echo "  make up                  # lance postgres + api + web"
	@echo "  make down                # arrete la stack"
	@echo "  make logs                # affiche les logs de la stack"
	@echo "  make stack-smoke         # verifie rapidement API + frontend"
	@echo "  make db-up               # lance uniquement postgres+pgvector"
	@echo "  make init-db             # initialise la DB (script idempotent)"
	@echo "  make bootstrap           # prepare env + db + init + tests backend"
	@echo ""
	@echo "Backend / RAG:"
	@echo "  make backend-test        # execute go test ./... dans backend"
	@echo "  make data-fetch          # telecharge les donnees OpenParlData via cache incremental"
	@echo "  make rag-index [CORPUS=data/normalized]"
	@echo "  make rag-debug-chunks [CORPUS=data/normalized] [SAMPLE=3]"
	@echo "  make rag-query Q='question'"
	@echo "  make rag-debug-pipeline [CORPUS=data/normalized] [SAMPLE=3]"
	@echo "  make rag-integration-pipeline [CORPUS=data/normalized] Q='question'"
	@echo "  make rag-ci [CORPUS=data/normalized] [SAMPLE=3] Q='question'"
	@echo ""
	@echo "Infra de tests complete (Ollama + Compose dedie):"
	@echo "  make test-ollama-check   # verifie la presence de la CLI ollama"
	@echo "  make test-ollama-start   # demarre/valide le service ollama"
	@echo "  make test-ollama-pull    # telecharge les modeles $(TEST_OLLAMA_MODEL) et $(TEST_OLLAMA_EMBEDDING_MODEL)"
	@echo "  make test-stack-up       # lance la stack de test docker compose"
	@echo "  make test-db-init        # initialise la DB de la stack test"
	@echo "  make test-infra-up       # enchaine ollama + stack test + init DB"
	@echo "  make test-infra-logs     # affiche les logs de la stack test"
	@echo "  make test-infra-down     # arrete la stack test (volumes conserves par defaut)"
	@echo "  make test-infra-purge    # arrete la stack test et supprime les volumes"
	@echo "  make test-data-fetch     # lance data-fetch dans rag_chunker"
	@echo "  make test-api-smoke      # smoke test HTTP API (demarre infra test par defaut)"
	@echo "  make test-rag-shell      # ouvre un shell dans le conteneur rag_chunker"
	@echo ""
	@echo "Qualite / coherence:"
	@echo "  make check-go-image-consistency # verifie coherence go.mod <-> image golang"
	@echo "  make frontend-audit             # execute npm audit --omit=dev dans frontend"
	@echo "  make backend-vulncheck          # execute govulncheck dans backend"
	@echo "  make security-check             # execute tous les controles de vulnerabilites"

env:
	@if [ ! -f .env ]; then cp .env.example .env; echo ".env cree depuis .env.example"; else echo ".env existe deja"; fi

up: env
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

stack-smoke:
	@command -v curl >/dev/null 2>&1 || (echo "Erreur: curl introuvable."; exit 1)
	@echo "==> Verifie API /health"
	@curl --max-time 8 -fsS http://127.0.0.1:8080/health >/dev/null
	@echo "==> Verifie frontend /"
	@curl --max-time 8 -fsS http://127.0.0.1:3000 >/dev/null
	@echo "OK: API et frontend repondent."

db-up: env
	docker compose up -d postgres

init-db: env
	bash scripts/init-db.sh

backend-test:
	cd backend && go test ./...

bootstrap: env db-up init-db backend-test
	@echo "Bootstrap termine."

data-fetch:
	mkdir -p data/raw data/normalized data/fetch-cache
	cd backend && go run ./cmd/data-fetch

rag-index:
	@if [ -n "$(CORPUS)" ]; then \
		cd backend && go run ./cmd/rag-cli index --corpus "$(CORPUS)"; \
	else \
		cd backend && go run ./cmd/rag-cli index; \
	fi

rag-debug-chunks:
	@if [ -n "$(CORPUS)" ] && [ -n "$(SAMPLE)" ]; then \
		cd backend && go run ./cmd/rag-cli debug-chunks --corpus "$(CORPUS)" --sample "$(SAMPLE)"; \
	elif [ -n "$(CORPUS)" ]; then \
		cd backend && go run ./cmd/rag-cli debug-chunks --corpus "$(CORPUS)"; \
	elif [ -n "$(SAMPLE)" ]; then \
		cd backend && go run ./cmd/rag-cli debug-chunks --sample "$(SAMPLE)"; \
	else \
		cd backend && go run ./cmd/rag-cli debug-chunks; \
	fi

rag-query:
	@test -n "$(Q)" || (echo "Erreur: utilisez Q='votre question'"; exit 1)
	cd backend && go run ./cmd/rag-cli query --q "$(Q)"

rag-debug-pipeline: data-fetch rag-debug-chunks
	@echo "Pipeline RAG niveau 1 termine (data-fetch + debug chunks)."

rag-integration-pipeline:
	@test -n "$(Q)" || (echo "Erreur: utilisez Q='votre question'"; exit 1)
	$(MAKE) db-up
	$(MAKE) init-db
	$(MAKE) rag-index CORPUS="$(CORPUS)"
	$(MAKE) rag-query Q="$(Q)"
	@echo "Pipeline RAG niveau 2 termine (DB + index + query)."

rag-ci:
	$(MAKE) rag-debug-pipeline CORPUS="$(CORPUS)" SAMPLE="$(SAMPLE)"
	$(MAKE) rag-integration-pipeline CORPUS="$(CORPUS)" Q="$(Q)"
	@echo "Pipeline RAG complet termine (niveaux 1 + 2)."

test-ollama-check:
	@command -v ollama >/dev/null 2>&1 || (echo "Erreur: CLI ollama introuvable. Installez Ollama puis relancez."; exit 1)
	@command -v curl >/dev/null 2>&1 || (echo "Erreur: curl introuvable. Installez curl puis relancez."; exit 1)

test-ollama-start: test-ollama-check
	@set -e; \
	if curl --max-time 2 -fsS "$(TEST_OLLAMA_HEALTH_URL)" >/dev/null 2>&1; then \
		echo "Ollama est deja disponible."; \
	else \
		echo "Demarrage du service Ollama..."; \
		nohup ollama serve >/tmp/civika-ollama.log 2>&1 || true; \
		i=0; \
		until curl --max-time 2 -fsS "$(TEST_OLLAMA_HEALTH_URL)" >/dev/null 2>&1; do \
			i=$$((i + 1)); \
			if [ $$i -ge "$(TEST_OLLAMA_START_TIMEOUT)" ]; then \
				echo "Erreur: Ollama indisponible apres attente. Consultez /tmp/civika-ollama.log"; \
				exit 1; \
			fi; \
			sleep 1; \
		done; \
		echo "Ollama demarre et accessible."; \
	fi

test-ollama-pull: test-ollama-start
	ollama pull "$(TEST_OLLAMA_MODEL)"
	ollama pull "$(TEST_OLLAMA_EMBEDDING_MODEL)"

test-stack-up:
	@test -f "$(TEST_ENV_FILE)" || (echo "Erreur: fichier $(TEST_ENV_FILE) introuvable."; exit 1)
	@test -f "$(TEST_COMPOSE_FILE)" || (echo "Erreur: fichier $(TEST_COMPOSE_FILE) introuvable."; exit 1)
	docker compose --env-file "$(TEST_ENV_FILE)" -f "$(TEST_COMPOSE_FILE)" up --build -d

test-db-init:
	@test -f "$(TEST_ENV_FILE)" || (echo "Erreur: fichier $(TEST_ENV_FILE) introuvable."; exit 1)
	@test -f "$(TEST_COMPOSE_FILE)" || (echo "Erreur: fichier $(TEST_COMPOSE_FILE) introuvable."; exit 1)
	ENV_FILE="$(TEST_ENV_FILE)" COMPOSE_FILE="$(TEST_COMPOSE_FILE)" bash scripts/init-db.sh

test-infra-up: test-ollama-pull test-stack-up test-db-init
	@echo "Infra de tests complete demarree."

test-infra-logs:
	@test -f "$(TEST_ENV_FILE)" || (echo "Erreur: fichier $(TEST_ENV_FILE) introuvable."; exit 1)
	@test -f "$(TEST_COMPOSE_FILE)" || (echo "Erreur: fichier $(TEST_COMPOSE_FILE) introuvable."; exit 1)
	docker compose --env-file "$(TEST_ENV_FILE)" -f "$(TEST_COMPOSE_FILE)" logs -f

test-infra-down:
	@test -f "$(TEST_ENV_FILE)" || (echo "Erreur: fichier $(TEST_ENV_FILE) introuvable."; exit 1)
	@test -f "$(TEST_COMPOSE_FILE)" || (echo "Erreur: fichier $(TEST_COMPOSE_FILE) introuvable."; exit 1)
	@if [ "$(PURGE_VOLUMES)" = "1" ]; then \
		echo "Arret infra test + purge des volumes..."; \
		docker compose --env-file "$(TEST_ENV_FILE)" -f "$(TEST_COMPOSE_FILE)" down --remove-orphans --volumes; \
		docker compose --env-file "$(TEST_ENV_FILE)" -f "$(TEST_COMPOSE_FILE)" rm -fsv || true; \
	else \
		echo "Arret infra test sans purge volumes (PURGE_VOLUMES=1 pour purger)."; \
		docker compose --env-file "$(TEST_ENV_FILE)" -f "$(TEST_COMPOSE_FILE)" down --remove-orphans; \
		docker compose --env-file "$(TEST_ENV_FILE)" -f "$(TEST_COMPOSE_FILE)" rm -fs || true; \
	fi

test-infra-purge:
	$(MAKE) test-infra-down PURGE_VOLUMES=1

test-rag-shell:
	@test -f "$(TEST_ENV_FILE)" || (echo "Erreur: fichier $(TEST_ENV_FILE) introuvable."; exit 1)
	@test -f "$(TEST_COMPOSE_FILE)" || (echo "Erreur: fichier $(TEST_COMPOSE_FILE) introuvable."; exit 1)
	docker compose --env-file "$(TEST_ENV_FILE)" -f "$(TEST_COMPOSE_FILE)" exec rag_chunker sh

test-data-fetch:
	@test -f "$(TEST_ENV_FILE)" || (echo "Erreur: fichier $(TEST_ENV_FILE) introuvable."; exit 1)
	@test -f "$(TEST_COMPOSE_FILE)" || (echo "Erreur: fichier $(TEST_COMPOSE_FILE) introuvable."; exit 1)
	docker compose --env-file "$(TEST_ENV_FILE)" -f "$(TEST_COMPOSE_FILE)" exec -T rag_chunker sh -lc "/app/data-fetch"

test-api-smoke:
	@command -v python3 >/dev/null 2>&1 || (echo "Erreur: python3 introuvable."; exit 1)
	@command -v curl >/dev/null 2>&1 || (echo "Erreur: curl introuvable."; exit 1)
	@set -e; \
	chmod +x scripts/tests/smoke-api.sh; \
	cleanup() { \
		echo "==> Nettoyage automatique: arret de l'infra de test"; \
		$(MAKE) test-infra-down || true; \
	}; \
	trap cleanup EXIT INT TERM; \
	./scripts/tests/smoke-api.sh

frontend-audit:
	cd frontend && npm audit --omit=dev

backend-vulncheck:
	cd backend && go run golang.org/x/vuln/cmd/govulncheck@latest ./...

security-check: frontend-audit backend-vulncheck
	@echo "Controles de vulnerabilites termines."

check-go-image-consistency:
	@set -e; \
	expected="$$(awk '/^go /{print $$2; exit}' backend/go.mod | sed -E 's/^([0-9]+\.[0-9]+).*/\1/')"; \
	if [ -z "$$expected" ]; then \
		echo "Erreur: version Go introuvable dans backend/go.mod"; \
		exit 1; \
	fi; \
	versions="$$(awk '/^FROM golang:/{sub(/^FROM golang:/, ""); print $$1}' backend/Dockerfile)"; \
	if [ -z "$$versions" ]; then \
		echo "Erreur: aucune image golang trouvee dans backend/Dockerfile"; \
		exit 1; \
	fi; \
	for v in $$versions; do \
		actual="$$(printf '%s' "$$v" | sed -E 's/^([0-9]+\.[0-9]+).*/\1/')"; \
		if [ "$$actual" != "$$expected" ]; then \
			echo "Erreur: incoherence image Go (Dockerfile=$$v, go.mod=$$expected.x)"; \
			exit 1; \
		fi; \
	done; \
	echo "OK: image(s) golang coherente(s) avec backend/go.mod ($$(awk '/^go /{print $$2; exit}' backend/go.mod))."
