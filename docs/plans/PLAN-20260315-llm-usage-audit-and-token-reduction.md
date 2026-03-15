# Audit usage LLM et reduction tokens

## Contexte

Civika utilise un pipeline RAG avec deux modes explicites:
- `RAG_MODE=local`: deterministe, sans appel reseau LLM.
- `RAG_MODE=llm`: embeddings, summarization QA et traduction via API LLM.

Ce document formalise:
- l'audit des surfaces LLM existantes,
- les limites deja en place pour contenir les couts,
- un plan de reduction des requetes/tokens selon best practices,
- un cadrage cache compatible privacy-first.

## Objectifs

- Cartographier ou et comment les appels LLM sont declenches.
- Lister les garde-fous existants (limites, timeouts, sanitization, rate limit).
- Identifier les ecarts priorites cout/tokens/privacy.
- Definir un backlog priorise, implemementable sans regression securite/privacy.

## Decisions principales

- Prioriser les optimisations "token governance" avant toute optimisation avancee.
- Appliquer une separation stricte entre:
  - cache persistant d'artefacts publics (autorise),
  - cache QA utilisateur durable brut (interdit).
- Garder la selection de mode RAG explicite (`local`/`llm`) sans fallback silencieux.
- Ne jamais stocker prompts/reponses QA utilisateur bruts en persistance serveur.
- Le cache de questions est autorise uniquement apres sanitation stricte et sans metadonnee personnelle.

## Arborescence cible

- `docs/plans/PLAN-20260315-llm-usage-audit-and-token-reduction.md` (ce document)

## Cartographie des usages LLM

### Appels LLM directs (runtime backend)

- `backend/internal/rag/embed.go`
  - `LLMEmbedder.EmbedTexts` -> `POST /v1/embeddings`
- `backend/internal/rag/query.go`
  - `LLMSummarizer.Summarize` -> `POST /v1/chat/completions`
- `backend/internal/rag/translate.go`
  - `LLMTranslator.Translate/translateOnce` -> `POST /v1/chat/completions`

### Appels LLM indirects (via services/endpoints)

- QA:
  - endpoint `POST /api/v1/qa/query`
  - flux: `handlers.go` -> `qa_service.go` -> `rag.QueryRAG` (embedding) + `rag.ExplainVotation` (summarization)
- Traduction votations:
  - endpoints `GET /api/v1/votations` et `GET /api/v1/votations/{id}`
  - flux: `sql_query_service.go` -> fallback traduction asynchrone -> `LLMTranslator`
- CLI RAG:
  - `backend/cmd/rag-cli/main.go` (`index`, `query`) en mode `RAG_MODE=llm`

### Cablage et configuration

- Validation mode: `backend/config/config.go` (`ValidateRAGMode`)
- Runtime API: `backend/cmd/civika-api/main.go`
- Runtime CLI: `backend/cmd/rag-cli/main.go`
- Variables principales: `.env.example`, `docs/advanced-usage.md`

## Limites existantes pour reduire la consommation

### Garde-fous applicatifs

- Rate limit QA:
  - `API_QA_RATE_LIMIT_QPS` (defaut `1`)
  - `API_QA_RATE_LIMIT_BURST` (defaut `3`)
  - middleware sur `POST /api/v1/qa/query`
- Validation longueur question:
  - rejet HTTP si `question > 2000` chars
  - sanitation service: troncature a `1200` chars + redaction email/telephone
- Limites prompt/input:
  - `LLM_MAX_PROMPT_CHARS` (defaut `4000`) sur summarization
  - `LLM_EMBEDDING_MAX_INPUT_CHARS` (defaut `4000`) sur embeddings
- Timeouts:
  - `LLM_TIMEOUT` (defaut `10s`)
  - `LLM_TRANSLATION_TIMEOUT` (defaut code `10s`, exemple env `120s`)
  - `LLM_EMBEDDING_TIMEOUT` (defaut `10s`)
- Taille de reponse lue:
  - `io.LimitReader(..., 2*1024*1024)` sur chat/embeddings/traduction
- Reutilisation existante:
  - reuse de traductions deja prêtes via hash source
  - skip indexation intelligente via fingerprint documents
- Observabilite:
  - metriques `ai_usage_events`, `ai_usage_daily_agg`, `rag_index_document_metrics`
  - endpoint `GET /api/v1/metrics/ai-usage`

### Ecarts et faiblesses identifies

- Pas de borne explicite de sortie LLM (`max_tokens`) dans payload chat.
- Controle majoritairement en caracteres, pas en tokens modele.
- Retry traduction:
  - present et configure en CLI (`LLM_TRANSLATION_MAX_RETRIES`),
  - non cable dans runtime API (`buildTranslatorRuntime`) -> perte de robustesse.
- Pas de budget hard/soft de consommation par periode.
- Pas de cache QA privacy-safe explicite.
- Log debug traduction peut inclure `response_preview` (risque exposition contenu).

## Flux technique cible (reduction tokens/requetes)

```mermaid
flowchart TD
  clientQA[ClientQA] --> qaEndpoint[POST_api_v1_qa_query]
  qaEndpoint --> qaSanitize[InputValidationAndSanitize]
  qaSanitize --> qaBudget[TokenBudgetGuard]
  qaBudget --> qaCache[EphemeralQACacheMemory]
  qaCache -->|miss| ragRetrieve[RAGRetrieveTopK]
  ragRetrieve --> promptBudget[PromptBudgetBuilder]
  promptBudget --> llmSummarize[LLMSummarizerWithMaxTokens]
  llmSummarize --> usageMetrics[AIUsageMetrics]
  llmSummarize --> qaCache

  votationsEndpoint[GET_votations] --> translationQueue[TranslationJobs]
  translationQueue --> translationDedup[DedupAndInFlightGuard]
  translationDedup --> llmTranslate[LLMTranslatorWithOutputCap]
  llmTranslate --> translationStore[PublicCorpusTranslations]
  llmTranslate --> usageMetrics
```

## Propositions best practices (priorisees)

## Phase 1 - Quick wins (faible risque, impact rapide)

1. Ajouter une borne de sortie LLM explicite
- Ajouter config:
  - `LLM_MAX_OUTPUT_TOKENS_SUMMARIZATION`
  - `LLM_MAX_OUTPUT_TOKENS_TRANSLATION`
- Injecter ces bornes dans payloads chat (`max_tokens` ou equivalent provider).
- Benefice: reduction immediate des sorties longues et du cout.

2. Renforcer les contraintes de prompt de sortie
- Summarization: instruction explicite "1 a 2 phrases, max N mots, style factuel".
- Traduction: reaffirmer "texte seul, sans commentaire" + cap output tokens.
- Benefice: reduction output tokens et variance.

3. Budget de contexte par requete QA
- Limiter le contexte injecte par:
  - top-k dynamique selon budget tokens,
  - troncature snippets par source,
  - arret des ajouts quand budget atteint.
- Benefice: baisse input tokens avec impact faible sur precision si calibrage.

4. Corriger le cablage retries traduction cote API
- Passer `TranslationMaxRetries` a `LLMTranslatorConfig` dans `backend/cmd/civika-api/main.go`.
- Benefice: moins d'echecs "bruit", reduction des relances manuelles couteuses.

5. Supprimer `response_preview` des logs debug traduction
- Conserver uniquement metadonnees techniques (taille, status, duree, code erreur).
- Benefice: alignement privacy et reduction risque fuite contenu.

## Phase 2 - Reduction structurelle (moyen terme)

1. Token accounting pre-appel
- Estimer tokens avant appel (approximation conservative).
- Refuser/adapter requete si depassement budget input.

2. Budget journalier soft/hard
- Seuil soft: alerte + degradation controlee.
- Seuil hard: blocage explicite des appels LLM (pas de fallback implicite).
- Pilotage via config et metriques existantes.

3. Cache QA a deux niveaux (privacy-safe)
- Niveau L1 exact cache:
  - match exact sur hash de question sanitizee + contexte.
  - faible risque de faux positifs, hit rapide.
- Niveau L2 semantic cache:
  - recherche vectorielle sur question sanitizee.
  - seuil de similarite obligatoire + garde-fous de contexte.
- Strategie de non-serving (cache miss force):
  - similarite sous seuil,
  - contexte incompatible (langue, votation/object, mode, modeles, versions),
  - entree detectee comme sensible non sanitisee.

4. Batching embeddings borne
- Introduire `LLM_EMBEDDING_MAX_BATCH_ITEMS`.
- Decouper les gros lots d'indexation.
- Benefice: requetes plus stables et moins de retries couteux.

## Design cache semantique (detail)

### Flux de resolution

```mermaid
flowchart TD
  qaRequest[QARequest] --> sanitizeInput[SanitizeInput]
  sanitizeInput --> exactLookup[ExactCacheLookup]
  exactLookup -->|hit| returnCached[ReturnCachedAnswer]
  exactLookup -->|miss| semanticLookup[SemanticCacheLookup]
  semanticLookup --> thresholdGate[SimilarityThresholdGate]
  thresholdGate -->|pass| contextGate[ContextCompatibilityGate]
  contextGate -->|pass| returnSemantic[ReturnSemanticCachedAnswer]
  thresholdGate -->|fail| llmPath[LLMQueryPath]
  contextGate -->|fail| llmPath
  llmPath --> storeCache[StoreCacheEntry]
  storeCache --> returnFresh[ReturnFreshAnswer]
```

### Regles de matching

- L1 exact cache:
  - cle derivee: `hash(questionSanitized + contextKey + cacheVersion)`.
  - `contextKey` inclut au minimum:
    - `lang`,
    - `votationId` / `objectId` (si presents),
    - `RAG_MODE`,
    - `embeddingModel`,
    - `generationModel`,
    - `promptVersion`,
    - `embeddingDimensions`,
    - `indexVersion`.
- L2 semantic cache:
  - calcul embedding sur `questionSanitized`,
  - recherche top-1 (ou top-k court) dans table vectorielle dediee,
  - score minimal configurable (ex: 0.90 par defaut initial),
  - verification contextuelle stricte avant serving.

### Regles de non-serving

- Toujours bypass cache si:
  - score < seuil,
  - mismatch de contexte ou version,
  - entree contenant des traces potentiellement personnelles non sanitisees,
  - entree trop courte/ambigue (ex: bruit ou demande non interpretable).
- En cas de bypass:
  - executer le chemin LLM standard,
  - stocker uniquement si l'entree sanitisee est conforme.

### Modele de donnees cible

- Table logique `qa_semantic_cache` (nom final a confirmer):
  - `cache_id` (pk technique),
  - `question_sanitized` (texte nettoye),
  - `question_hash` (hash non reversible),
  - `question_embedding` (`vector(dim)`),
  - `answer_text` (reponse QA),
  - `answer_citations_hash` (controle de coherence),
  - `lang`, `votation_id`, `object_id`,
  - `rag_mode`, `embedding_model`, `generation_model`,
  - `prompt_version`, `embedding_dimensions`, `index_version`,
  - `created_at`, `last_hit_at`, `expires_at`, `hit_count`.
- Index:
  - index vectoriel sur `question_embedding`,
  - index btree sur `(question_hash, lang, rag_mode, prompt_version, index_version)`,
  - index sur `expires_at` pour purge.

### Politique TTL, retention et purge

- L1 exact cache:
  - TTL court (ex: 10 min),
  - evictions agressives (taille max en memoire).
- L2 semantic cache:
  - TTL moyen (ex: 24h) avec purge reguliere.
- Invalidation forcee sur changement de:
  - `RAG_MODE`,
  - modeles LLM/embedding,
  - `RAG_EMBEDDING_DIMENSIONS`,
  - `promptVersion`,
  - `indexVersion`.

### Observabilite autorisee (sans contenu sensible)

- Metriques:
  - hit/miss L1 et L2,
  - score moyen de similarite sur hits L2,
  - latence lookup cache,
  - taux de bypass pour mismatch contexte.
- Logs autorises:
  - IDs techniques, status, score, durees.
- Logs interdits:
  - prompts complets, reponses brutes non necessaires, IP/user-agent, PII.

## Phase 3 - Optimisations avancees (optionnelles)

1. Cache semantique QA prudent
- Seulement si signal qualite suffisant et sans stockage durable de contenu utilisateur.
- Requiert garde-fous stricts anti-mauvaise reponse contextuelle.

2. Routing modele par complexite
- Modele plus petit par defaut, plus grand sur cas complexes.
- A implementer seulement avec metriques de qualite etablies.

## Regles cache privacy-first (obligatoires)

### Autorise

- Persistant:
  - traductions de corpus public officiel,
  - embeddings de corpus public,
  - questions QA sanitisees et dediees au cache semantique, sans identifiant personnel,
  - metadonnees techniques anonymes d'usage.
- Ephemere memoire:
  - reponses QA transitoires non reliees a une identite.

### Interdit

- Stockage durable de:
  - question utilisateur brute non sanitisee,
  - prompt complet,
  - reponse QA brute accompagnee de metadonnees personnelles,
  - IP, user-agent correlable, identifiant utilisateur.
- Tables de profilage utilisateur (`users`, `profiles`, `sessions`, `histories`, `preferences`).

### Schema de cle et invalidation

- Cle cache = hash(normalize(inputSanitized) + contextKey + cacheVersion)
- `contextKey` inclut au minimum `lang`, `votationId/objectId`, `mode`, `provider`, `model`, `promptVersion`, `embeddingDimensions`, `indexVersion`.
- Invalidation obligatoire sur changement de:
  - `RAG_MODE`,
  - modele LLM / fournisseur,
  - `RAG_EMBEDDING_DIMENSIONS`,
  - version de template prompt.

### TTL et retention

- Cache QA memoire: TTL court + eviction LRU + cap taille stricte.
- Cache persistant artefacts publics: retention documentee + purge periodique.

### Logging conforme

- Autorise:
  - hit/miss, latence, tailles, statut, code erreur, identifiants techniques non personnels.
- Interdit:
  - prompts complets, sorties brutes, tokens API, donnees personnelles, IP durable.

## Modifications de fichiers prevues (plan d'execution technique)

- `backend/config/config.go`
  - nouvelles variables output token caps, budget et limites batching.
- `.env.example`
  - exposition des nouvelles variables LLM de gouvernance.
- `docs/advanced-usage.md`
  - documentation du mode budget/caps/cache et procedure d'invalidation.
- `backend/internal/rag/query.go`
  - max output tokens + prompt constraints + budget contexte.
- `backend/internal/rag/translate.go`
  - max output tokens + suppression preview en logs.
- `backend/internal/rag/embed.go`
  - support batch borne (si phase 2 activee).
- `backend/cmd/civika-api/main.go`
  - cablage retries traduction + nouvelles configs.
- (phase 2) composants cache QA:
  - `backend/internal/services/qa_cache.go` (L1 exact cache memoire),
  - `backend/internal/services/qa_semantic_cache.go` (L2 semantic cache),
  - `backend/internal/rag/cache_store.go` (acces persistant cache, ou equivalent).
- `scripts/sql/init_pgvector.sql`
  - table/index vectoriel du cache semantique QA.
- `backend/config/config.go` + `.env.example` (+ `.env.test`)
  - seuil similarite, TTL, capacites cache, version cache.

## Checklist de verification post-generation

- [ ] Audit complete des points d'appel LLM valide (API, services, CLI).
- [ ] Limites existantes documentees avec valeurs effectives.
- [ ] Ecarts prioritaires traces avec impact cout/privacy.
- [ ] Regles cache privacy-first formalisees (autorise/interdit/invalidation).
- [ ] Backlog priorise en phases (quick wins -> moyen terme -> optionnel).
- [ ] Aucun conflit avec regles `privacy.mdc` et `project.mdc`.
- [ ] Procedure de verification securite/logging definie.
- [ ] Documentation des futures variables de config prevue.
- [ ] Tests de similarite L2: cas positifs et faux positifs.
- [ ] Tests d'invalidation: changement modele/prompt/index/mode.
- [ ] Tests privacy: aucune IP/PII en stockage et logs.

## Contraintes securite impactees

- Respect strict du mode explicite `local|llm` sans fallback silencieux.
- Aucun secret dans le code, configuration via env uniquement.
- Logs techniques minimaux, sans contenu sensible.
- Aucune persistance de donnees utilisateur identifiables.
