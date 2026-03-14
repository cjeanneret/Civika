-- Idempotent initialization for Civika RAG on PostgreSQL + pgvector.
-- This script is safe to re-run.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS documents (
  id TEXT PRIMARY KEY,
  source_system TEXT NOT NULL,
  source_uri TEXT NOT NULL,
  external_id TEXT NOT NULL,
  votation_id TEXT NOT NULL DEFAULT '',
  object_id TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL DEFAULT '',
  canton TEXT NOT NULL DEFAULT '',
  commune_code TEXT NOT NULL DEFAULT '',
  commune_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  object_type TEXT NOT NULL DEFAULT '',
  object_theme TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  vote_date TIMESTAMPTZ NULL,
  source_org TEXT NOT NULL,
  content_type TEXT NOT NULL,
  license_uri TEXT NOT NULL DEFAULT '',
  fetched_at_utc TIMESTAMPTZ NOT NULL,
  issued_at TIMESTAMPTZ NULL,
  modified_at TIMESTAMPTZ NULL,
  source_metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX IF NOT EXISTS documents_source_external_uidx
  ON documents (source_system, external_id);

CREATE INDEX IF NOT EXISTS documents_votation_id_idx
  ON documents (votation_id);

CREATE INDEX IF NOT EXISTS documents_object_id_idx
  ON documents (object_id);

CREATE INDEX IF NOT EXISTS documents_filters_idx
  ON documents (vote_date, level, canton, commune_code, status);

CREATE TABLE IF NOT EXISTS document_translations (
  id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  lang TEXT NOT NULL,
  title TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  content_normalized TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX IF NOT EXISTS document_translations_document_lang_uidx
  ON document_translations (document_id, lang);

CREATE TABLE IF NOT EXISTS intervenants (
  id TEXT PRIMARY KEY,
  first_name TEXT NOT NULL,
  last_name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX IF NOT EXISTS intervenants_identity_uidx
  ON intervenants (first_name, last_name, role);

CREATE TABLE IF NOT EXISTS document_intervenants (
  id TEXT PRIMARY KEY,
  document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  intervenant_id TEXT NOT NULL REFERENCES intervenants(id) ON DELETE CASCADE,
  relation_type TEXT NOT NULL DEFAULT 'mentioned',
  order_index INT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS document_intervenants_unique_link_uidx
  ON document_intervenants (document_id, intervenant_id, relation_type);

CREATE INDEX IF NOT EXISTS document_intervenants_document_idx
  ON document_intervenants (document_id);

CREATE TABLE IF NOT EXISTS rag_chunks (
  id TEXT PRIMARY KEY,
  doc_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  translation_id TEXT NULL REFERENCES document_translations(id) ON DELETE SET NULL,
  votation_id TEXT NOT NULL DEFAULT '',
  object_id TEXT NOT NULL DEFAULT '',
  level TEXT NOT NULL DEFAULT '',
  canton TEXT NOT NULL DEFAULT '',
  commune_code TEXT NOT NULL DEFAULT '',
  commune_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  vote_date TIMESTAMPTZ NULL,
  lang TEXT NOT NULL,
  source_path TEXT NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  token_count INT NOT NULL,
  chunk_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  embedding vector(__RAG_EMBEDDING_DIMENSIONS__) NOT NULL
);

CREATE INDEX IF NOT EXISTS rag_chunks_embedding_idx
  ON rag_chunks
  USING hnsw (embedding vector_cosine_ops);

CREATE INDEX IF NOT EXISTS rag_chunks_doc_lang_idx
  ON rag_chunks (doc_id, lang);

CREATE INDEX IF NOT EXISTS rag_chunks_filters_idx
  ON rag_chunks (vote_date, level, canton, commune_code, status, votation_id, object_id);
