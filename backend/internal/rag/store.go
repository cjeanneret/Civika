package rag

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pgvector/pgvector-go"
)

type EmbeddedChunk struct {
	Chunk  Chunk
	Vector []float32
}

type SearchHit struct {
	Chunk Chunk
	Score float64
}

type VectorStore interface {
	InitSchema(ctx context.Context) error
	UpsertChunks(ctx context.Context, items []EmbeddedChunk) error
	SearchSimilar(ctx context.Context, queryVector []float32, topK int) ([]SearchHit, error)
}

type PostgresVectorStoreConfig struct {
	Host               string
	Port               string
	User               string
	Password           string
	DBName             string
	SSLMode            string
	TableName          string
	EmbeddingDimension int
	MaxOpenConns       int
	MaxIdleConns       int
}

type PostgresVectorStore struct {
	db        *sql.DB
	tableName string
	dimension int
}

func NewPostgresVectorStore(cfg PostgresVectorStoreConfig) (*PostgresVectorStore, error) {
	if strings.TrimSpace(cfg.Host) == "" ||
		strings.TrimSpace(cfg.Port) == "" ||
		strings.TrimSpace(cfg.User) == "" ||
		strings.TrimSpace(cfg.DBName) == "" {
		return nil, errors.New("postgres config is incomplete")
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	if cfg.TableName == "" {
		cfg.TableName = "rag_chunks"
	}
	if cfg.EmbeddingDimension <= 0 {
		return nil, errors.New("embedding dimension must be positive")
	}
	if !isSafeSQLIdentifier(cfg.TableName) {
		return nil, errors.New("table name contains invalid characters")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.SSLMode,
	)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	return &PostgresVectorStore{
		db:        db,
		tableName: cfg.TableName,
		dimension: cfg.EmbeddingDimension,
	}, nil
}

func (s *PostgresVectorStore) Close() error {
	return s.db.Close()
}

func (s *PostgresVectorStore) InitSchema(ctx context.Context) error {
	createExtension := `CREATE EXTENSION IF NOT EXISTS vector`
	if _, err := s.db.ExecContext(ctx, createExtension); err != nil {
		return fmt.Errorf("create vector extension: %w", err)
	}

	createDocuments := `
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
	index_complete BOOLEAN NOT NULL DEFAULT false,
	indexed_chunk_count INT NOT NULL DEFAULT 0,
	index_completed_at TIMESTAMPTZ NULL,
	source_metadata JSONB NOT NULL DEFAULT '{}'::jsonb
)`
	if _, err := s.db.ExecContext(ctx, createDocuments); err != nil {
		return fmt.Errorf("create documents table: %w", err)
	}
	alterDocumentColumns := []string{
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS votation_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS object_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS level TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS canton TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS commune_code TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS commune_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS object_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS object_theme TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS vote_date TIMESTAMPTZ NULL`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS index_complete BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS indexed_chunk_count INT NOT NULL DEFAULT 0`,
		`ALTER TABLE documents ADD COLUMN IF NOT EXISTS index_completed_at TIMESTAMPTZ NULL`,
	}
	for _, statement := range alterDocumentColumns {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("alter documents table: %w", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS documents_source_external_uidx ON documents (source_system, external_id)`); err != nil {
		return fmt.Errorf("create documents unique index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS documents_votation_id_idx ON documents (votation_id)`); err != nil {
		return fmt.Errorf("create documents votation index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS documents_object_id_idx ON documents (object_id)`); err != nil {
		return fmt.Errorf("create documents object index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS documents_filters_idx ON documents (vote_date, level, canton, commune_code, status)`); err != nil {
		return fmt.Errorf("create documents filter index: %w", err)
	}

	createTranslations := `
CREATE TABLE IF NOT EXISTS document_translations (
	id TEXT PRIMARY KEY,
	document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
	lang TEXT NOT NULL,
	title TEXT NOT NULL,
	summary TEXT NOT NULL DEFAULT '',
	content_normalized TEXT NOT NULL,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb
)`
	if _, err := s.db.ExecContext(ctx, createTranslations); err != nil {
		return fmt.Errorf("create document_translations table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS document_translations_document_lang_uidx ON document_translations (document_id, lang)`); err != nil {
		return fmt.Errorf("create document_translations unique index: %w", err)
	}

	createIntervenants := `
CREATE TABLE IF NOT EXISTS intervenants (
	id TEXT PRIMARY KEY,
	first_name TEXT NOT NULL,
	last_name TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT '',
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb
)`
	if _, err := s.db.ExecContext(ctx, createIntervenants); err != nil {
		return fmt.Errorf("create intervenants table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS intervenants_identity_uidx ON intervenants (first_name, last_name, role)`); err != nil {
		return fmt.Errorf("create intervenants unique index: %w", err)
	}

	createDocumentIntervenants := `
CREATE TABLE IF NOT EXISTS document_intervenants (
	id TEXT PRIMARY KEY,
	document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
	intervenant_id TEXT NOT NULL REFERENCES intervenants(id) ON DELETE CASCADE,
	relation_type TEXT NOT NULL DEFAULT 'mentioned',
	order_index INT NOT NULL DEFAULT 0
)`
	if _, err := s.db.ExecContext(ctx, createDocumentIntervenants); err != nil {
		return fmt.Errorf("create document_intervenants table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS document_intervenants_unique_link_uidx ON document_intervenants (document_id, intervenant_id, relation_type)`); err != nil {
		return fmt.Errorf("create document_intervenants unique index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS document_intervenants_document_idx ON document_intervenants (document_id)`); err != nil {
		return fmt.Errorf("create document_intervenants document index: %w", err)
	}

	createChunks := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
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
	embedding vector(%d) NOT NULL
)`, s.tableName, s.dimension)
	if _, err := s.db.ExecContext(ctx, createChunks); err != nil {
		return fmt.Errorf("create rag table: %w", err)
	}
	alterChunkColumns := []string{
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS votation_id TEXT NOT NULL DEFAULT ''`, s.tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS object_id TEXT NOT NULL DEFAULT ''`, s.tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS level TEXT NOT NULL DEFAULT ''`, s.tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS canton TEXT NOT NULL DEFAULT ''`, s.tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS commune_code TEXT NOT NULL DEFAULT ''`, s.tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS commune_name TEXT NOT NULL DEFAULT ''`, s.tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT ''`, s.tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT ''`, s.tableName),
		fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS vote_date TIMESTAMPTZ NULL`, s.tableName),
	}
	for _, statement := range alterChunkColumns {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("alter rag table: %w", err)
		}
	}
	createChunkIndex := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_embedding_idx ON %s USING hnsw (embedding vector_cosine_ops)`, s.tableName, s.tableName)
	if _, err := s.db.ExecContext(ctx, createChunkIndex); err != nil {
		return fmt.Errorf("create rag vector index: %w", err)
	}
	createChunkDocLangIndex := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_doc_lang_idx ON %s (doc_id, lang)`, s.tableName, s.tableName)
	if _, err := s.db.ExecContext(ctx, createChunkDocLangIndex); err != nil {
		return fmt.Errorf("create rag doc/lang index: %w", err)
	}
	createChunkFilterIndex := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_filters_idx ON %s (vote_date, level, canton, commune_code, status, votation_id, object_id)`, s.tableName, s.tableName)
	if _, err := s.db.ExecContext(ctx, createChunkFilterIndex); err != nil {
		return fmt.Errorf("create rag filter index: %w", err)
	}
	return nil
}

func (s *PostgresVectorStore) UpsertChunks(ctx context.Context, items []EmbeddedChunk) error {
	if len(items) == 0 {
		return errors.New("embedded chunks are required")
	}
	itemsByDocument := groupEmbeddedChunksByDocument(items)
	for _, documentItems := range itemsByDocument {
		if err := s.upsertDocumentChunks(ctx, documentItems); err != nil {
			return err
		}
	}
	return nil
}

func groupEmbeddedChunksByDocument(items []EmbeddedChunk) [][]EmbeddedChunk {
	grouped := map[string][]EmbeddedChunk{}
	orderedDocumentIDs := make([]string, 0, len(items))
	for _, item := range items {
		docID := strings.TrimSpace(item.Chunk.DocumentID)
		if docID == "" {
			docID = "__missing_document_id__"
		}
		if _, exists := grouped[docID]; !exists {
			orderedDocumentIDs = append(orderedDocumentIDs, docID)
		}
		grouped[docID] = append(grouped[docID], item)
	}
	result := make([][]EmbeddedChunk, 0, len(orderedDocumentIDs))
	for _, docID := range orderedDocumentIDs {
		result = append(result, grouped[docID])
	}
	return result
}

func (s *PostgresVectorStore) upsertDocumentChunks(ctx context.Context, items []EmbeddedChunk) error {
	if len(items) == 0 {
		return nil
	}
	documentID := strings.TrimSpace(items[0].Chunk.DocumentID)
	if documentID == "" {
		return errors.New("document id is required")
	}
	for _, item := range items {
		if strings.TrimSpace(item.Chunk.DocumentID) != documentID {
			return fmt.Errorf("embedded chunks contain multiple document ids in one transaction: %q vs %q", documentID, item.Chunk.DocumentID)
		}
	}

	upsertDocumentQuery := `
INSERT INTO documents (id, source_system, source_uri, external_id, votation_id, object_id, level, canton, commune_code, commune_name, status, object_type, object_theme, source_type, vote_date, source_org, content_type, license_uri, fetched_at_utc, issued_at, modified_at, index_complete, indexed_chunk_count, index_completed_at, source_metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
ON CONFLICT (id)
DO UPDATE SET
	source_system = EXCLUDED.source_system,
	source_uri = EXCLUDED.source_uri,
	external_id = EXCLUDED.external_id,
	votation_id = EXCLUDED.votation_id,
	object_id = EXCLUDED.object_id,
	level = EXCLUDED.level,
	canton = EXCLUDED.canton,
	commune_code = EXCLUDED.commune_code,
	commune_name = EXCLUDED.commune_name,
	status = EXCLUDED.status,
	object_type = EXCLUDED.object_type,
	object_theme = EXCLUDED.object_theme,
	source_type = EXCLUDED.source_type,
	vote_date = EXCLUDED.vote_date,
	source_org = EXCLUDED.source_org,
	content_type = EXCLUDED.content_type,
	license_uri = EXCLUDED.license_uri,
	fetched_at_utc = EXCLUDED.fetched_at_utc,
	issued_at = EXCLUDED.issued_at,
	modified_at = EXCLUDED.modified_at,
	index_complete = EXCLUDED.index_complete,
	indexed_chunk_count = EXCLUDED.indexed_chunk_count,
	index_completed_at = EXCLUDED.index_completed_at,
	source_metadata = EXCLUDED.source_metadata
`

	upsertTranslationQuery := `
INSERT INTO document_translations (id, document_id, lang, title, summary, content_normalized, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (document_id, lang)
DO UPDATE SET
	id = EXCLUDED.id,
	document_id = EXCLUDED.document_id,
	lang = EXCLUDED.lang,
	title = EXCLUDED.title,
	summary = EXCLUDED.summary,
	content_normalized = EXCLUDED.content_normalized,
	metadata = EXCLUDED.metadata
`

	upsertIntervenantQuery := `
INSERT INTO intervenants (id, first_name, last_name, role, metadata)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id)
DO UPDATE SET
	first_name = EXCLUDED.first_name,
	last_name = EXCLUDED.last_name,
	role = EXCLUDED.role,
	metadata = EXCLUDED.metadata
`

	upsertDocumentIntervenantQuery := `
INSERT INTO document_intervenants (id, document_id, intervenant_id, relation_type, order_index)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id)
DO UPDATE SET
	document_id = EXCLUDED.document_id,
	intervenant_id = EXCLUDED.intervenant_id,
	relation_type = EXCLUDED.relation_type,
	order_index = EXCLUDED.order_index
`

	upsertChunkQuery := fmt.Sprintf(`
INSERT INTO %s (id, doc_id, translation_id, votation_id, object_id, level, canton, commune_code, commune_name, status, source_type, vote_date, lang, source_path, title, content, token_count, chunk_metadata, embedding)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
ON CONFLICT (id)
DO UPDATE SET
	doc_id = EXCLUDED.doc_id,
	translation_id = EXCLUDED.translation_id,
	votation_id = EXCLUDED.votation_id,
	object_id = EXCLUDED.object_id,
	level = EXCLUDED.level,
	canton = EXCLUDED.canton,
	commune_code = EXCLUDED.commune_code,
	commune_name = EXCLUDED.commune_name,
	status = EXCLUDED.status,
	source_type = EXCLUDED.source_type,
	vote_date = EXCLUDED.vote_date,
	lang = EXCLUDED.lang,
	source_path = EXCLUDED.source_path,
	title = EXCLUDED.title,
	content = EXCLUDED.content,
	token_count = EXCLUDED.token_count,
	chunk_metadata = EXCLUDED.chunk_metadata,
	embedding = EXCLUDED.embedding
`, s.tableName)
	deleteDocumentChunksQuery := fmt.Sprintf(`DELETE FROM %s WHERE doc_id = $1`, s.tableName)
	deleteDocumentIntervenantsQuery := `DELETE FROM document_intervenants WHERE document_id = $1`
	setDocumentCompleteQuery := `
UPDATE documents
SET index_complete = $2,
	indexed_chunk_count = $3,
	index_completed_at = $4
WHERE id = $1
`

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, deleteDocumentIntervenantsQuery, documentID); err != nil {
		return fmt.Errorf("clear document intervenants %s: %w", documentID, err)
	}
	if _, err := tx.ExecContext(ctx, deleteDocumentChunksQuery, documentID); err != nil {
		return fmt.Errorf("clear document chunks %s: %w", documentID, err)
	}

	for _, item := range items {
		if len(item.Vector) != s.dimension {
			return fmt.Errorf("invalid vector dimension for chunk %s", item.Chunk.ID)
		}

		sourceMetadataJSON := marshalSanitizedMetadata(item.Chunk.Source.Extra)
		fetchedAt := item.Chunk.Source.FetchedAtUTC
		if fetchedAt.IsZero() {
			fetchedAt = time.Now().UTC()
		}
		filterData := extractChunkFilterData(item.Chunk)
		if _, err := tx.ExecContext(
			ctx,
			upsertDocumentQuery,
			item.Chunk.DocumentID,
			normalizeNonEmpty(item.Chunk.Source.SourceSystem, "local_fixture"),
			normalizeNonEmpty(item.Chunk.Source.SourceURI, item.Chunk.SourcePath),
			normalizeNonEmpty(item.Chunk.Source.ExternalID, item.Chunk.DocumentID),
			filterData.VotationID,
			filterData.ObjectID,
			filterData.Level,
			filterData.Canton,
			filterData.CommuneCode,
			filterData.CommuneName,
			filterData.Status,
			filterData.ObjectType,
			filterData.ObjectTheme,
			filterData.SourceType,
			filterData.VoteDate,
			item.Chunk.Source.SourceOrg,
			normalizeNonEmpty(item.Chunk.Source.ContentType, "text/plain"),
			item.Chunk.Source.LicenseURI,
			fetchedAt,
			item.Chunk.Source.IssuedAt,
			item.Chunk.Source.ModifiedAt,
			false,
			0,
			nil,
			sourceMetadataJSON,
		); err != nil {
			return fmt.Errorf("upsert document %s: %w", item.Chunk.DocumentID, err)
		}

		translationID := normalizeNonEmpty(item.Chunk.TranslationID, fmt.Sprintf("%s:%s", item.Chunk.DocumentID, normalizeNonEmpty(item.Chunk.Language, "fr")))
		translationMetadata := map[string]any{
			"available_languages": item.Chunk.Source.AvailableLanguages,
		}
		for _, key := range []string{
			"translation_status",
			"translation_provider",
			"translation_source_lang",
			"translation_source_hash",
			"translation_updated_at",
			"index_content_hash",
			"index_source_fingerprint",
			"index_source_fingerprint_confidence",
		} {
			if value, ok := item.Chunk.Metadata[key]; ok {
				translationMetadata[key] = value
			}
		}
		if displayTitle, ok := item.Chunk.Metadata["display_title"]; ok {
			translationMetadata["display_title"] = displayTitle
		}
		if _, err := tx.ExecContext(
			ctx,
			upsertTranslationQuery,
			translationID,
			item.Chunk.DocumentID,
			normalizeNonEmpty(item.Chunk.Language, "fr"),
			normalizeNonEmpty(item.Chunk.Title, "Sans titre"),
			"",
			item.Chunk.Text,
			marshalSanitizedMetadata(translationMetadata),
		); err != nil {
			return fmt.Errorf("upsert translation %s: %w", translationID, err)
		}

		for idx, intervenant := range item.Chunk.Intervenants {
			if strings.TrimSpace(intervenant.FirstName) == "" || strings.TrimSpace(intervenant.LastName) == "" {
				continue
			}
			intervenantID := buildIntervenantID(intervenant.FirstName, intervenant.LastName, intervenant.Role)
			if _, err := tx.ExecContext(
				ctx,
				upsertIntervenantQuery,
				intervenantID,
				intervenant.FirstName,
				intervenant.LastName,
				intervenant.Role,
				[]byte("{}"),
			); err != nil {
				return fmt.Errorf("upsert intervenant %s: %w", intervenantID, err)
			}
			linkID := fmt.Sprintf("%s:%s:%d", item.Chunk.DocumentID, intervenantID, idx)
			if _, err := tx.ExecContext(
				ctx,
				upsertDocumentIntervenantQuery,
				linkID,
				item.Chunk.DocumentID,
				intervenantID,
				"mentioned",
				idx,
			); err != nil {
				return fmt.Errorf("upsert document/intervenant link %s: %w", linkID, err)
			}
		}

		chunkMetadata := map[string]any{
			"source_system":       item.Chunk.Source.SourceSystem,
			"source_uri":          item.Chunk.Source.SourceURI,
			"external_id":         item.Chunk.Source.ExternalID,
			"source_org":          item.Chunk.Source.SourceOrg,
			"available_languages": item.Chunk.Source.AvailableLanguages,
			"intervenants":        item.Chunk.Intervenants,
			"extra":               item.Chunk.Metadata,
		}
		chunkMetadataJSON := marshalSanitizedMetadata(chunkMetadata)
		_, err := tx.ExecContext(
			ctx,
			upsertChunkQuery,
			item.Chunk.ID,
			item.Chunk.DocumentID,
			translationID,
			filterData.VotationID,
			filterData.ObjectID,
			filterData.Level,
			filterData.Canton,
			filterData.CommuneCode,
			filterData.CommuneName,
			filterData.Status,
			filterData.SourceType,
			filterData.VoteDate,
			normalizeNonEmpty(item.Chunk.Language, "fr"),
			item.Chunk.SourcePath,
			item.Chunk.Title,
			item.Chunk.Text,
			item.Chunk.TokenCount,
			chunkMetadataJSON,
			pgvector.NewVector(item.Vector),
		)
		if err != nil {
			return fmt.Errorf("upsert chunk %s: %w", item.Chunk.ID, err)
		}
	}
	if _, err := tx.ExecContext(
		ctx,
		setDocumentCompleteQuery,
		documentID,
		true,
		len(items),
		time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("mark document complete %s: %w", documentID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (s *PostgresVectorStore) LoadIndexState(ctx context.Context, documentIDs []string) (map[string]IndexDocumentState, error) {
	state := map[string]IndexDocumentState{}
	ids := uniqueNonEmptyStrings(documentIDs)
	if len(ids) == 0 {
		return state, nil
	}

	docPlaceholders, docArgs := buildSQLPlaceholders(ids, 1)
	documentsQuery := fmt.Sprintf(
		`SELECT id, COALESCE(source_metadata->>'index_source_fingerprint', ''), COALESCE(source_metadata->>'index_source_fingerprint_confidence', ''), COALESCE(index_complete, false), COALESCE(indexed_chunk_count, 0) FROM documents WHERE id IN (%s)`,
		docPlaceholders,
	)
	docRows, err := s.db.QueryContext(ctx, documentsQuery, docArgs...)
	if err != nil {
		return nil, fmt.Errorf("load existing documents state: %w", err)
	}
	defer docRows.Close()
	for docRows.Next() {
		var docID, fingerprint, confidence string
		var indexComplete bool
		var indexedChunkCount int
		if scanErr := docRows.Scan(&docID, &fingerprint, &confidence, &indexComplete, &indexedChunkCount); scanErr != nil {
			return nil, fmt.Errorf("scan existing documents state: %w", scanErr)
		}
		state[docID] = IndexDocumentState{
			DocumentID:                  docID,
			SourceFingerprint:           fingerprint,
			SourceFingerprintConfidence: confidence,
			IndexComplete:               indexComplete,
			IndexedChunkCount:           indexedChunkCount,
			Translations:                map[string]IndexTranslationState{},
		}
	}
	if err := docRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing documents state: %w", err)
	}

	translationPlaceholders, translationArgs := buildSQLPlaceholders(ids, 1)
	translationsQuery := fmt.Sprintf(
		`SELECT document_id, id, lang, title, content_normalized, COALESCE(metadata->>'translation_status', ''), COALESCE(metadata->>'translation_provider', ''), COALESCE(metadata->>'translation_source_hash', ''), COALESCE(metadata->>'index_content_hash', '') FROM document_translations WHERE document_id IN (%s)`,
		translationPlaceholders,
	)
	translationRows, err := s.db.QueryContext(ctx, translationsQuery, translationArgs...)
	if err != nil {
		return nil, fmt.Errorf("load existing translations state: %w", err)
	}
	defer translationRows.Close()
	for translationRows.Next() {
		var (
			documentID    string
			translationID string
			lang          string
			title         string
			content       string
			status        string
			provider      string
			sourceHash    string
			contentHash   string
		)
		if scanErr := translationRows.Scan(&documentID, &translationID, &lang, &title, &content, &status, &provider, &sourceHash, &contentHash); scanErr != nil {
			return nil, fmt.Errorf("scan existing translations state: %w", scanErr)
		}
		docState, exists := state[documentID]
		if !exists {
			docState = IndexDocumentState{
				DocumentID:   documentID,
				Translations: map[string]IndexTranslationState{},
			}
		}
		if docState.Translations == nil {
			docState.Translations = map[string]IndexTranslationState{}
		}
		docState.Translations[lang] = IndexTranslationState{
			TranslationID: translationID,
			Lang:          lang,
			Title:         title,
			Content:       content,
			Status:        status,
			Provider:      provider,
			SourceHash:    sourceHash,
			ContentHash:   contentHash,
		}
		state[documentID] = docState
	}
	if err := translationRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing translations state: %w", err)
	}
	return state, nil
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func buildSQLPlaceholders(values []string, startIndex int) (string, []any) {
	placeholders := make([]string, 0, len(values))
	args := make([]any, 0, len(values))
	for i, value := range values {
		placeholders = append(placeholders, "$"+strconv.Itoa(startIndex+i))
		args = append(args, value)
	}
	return strings.Join(placeholders, ", "), args
}

func (s *PostgresVectorStore) SearchSimilar(ctx context.Context, queryVector []float32, topK int) ([]SearchHit, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("query vector is required")
	}
	if len(queryVector) != s.dimension {
		return nil, errors.New("query vector has invalid dimension")
	}
	if topK <= 0 {
		topK = 5
	}

	query := fmt.Sprintf(`
SELECT id, doc_id, translation_id, lang, source_path, title, content, token_count, chunk_metadata, 1 - (embedding <=> $1) AS score
FROM %s
ORDER BY embedding <=> $1
LIMIT $2
`, s.tableName)
	rows, err := s.db.QueryContext(ctx, query, pgvector.NewVector(queryVector), topK)
	if err != nil {
		return nil, fmt.Errorf("search similar: %w", err)
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var hit SearchHit
		var translationID sql.NullString
		var language string
		var rawMetadata []byte
		if err := rows.Scan(
			&hit.Chunk.ID,
			&hit.Chunk.DocumentID,
			&translationID,
			&language,
			&hit.Chunk.SourcePath,
			&hit.Chunk.Title,
			&hit.Chunk.Text,
			&hit.Chunk.TokenCount,
			&rawMetadata,
			&hit.Score,
		); err != nil {
			return nil, fmt.Errorf("scan search row: %w", err)
		}
		hit.Chunk.Language = language
		if translationID.Valid {
			hit.Chunk.TranslationID = translationID.String
		}
		metadata := map[string]any{}
		if len(rawMetadata) > 0 {
			_ = json.Unmarshal(rawMetadata, &metadata)
		}
		hit.Chunk.Metadata = metadata
		hit.Chunk.Source = decodeSourceMetadata(metadata)
		hit.Chunk.Intervenants = decodeIntervenants(metadata)
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search rows: %w", err)
	}
	return hits, nil
}

func isSafeSQLIdentifier(v string) bool {
	return regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(v)
}

func buildIntervenantID(firstName, lastName, role string) string {
	base := strings.ToLower(strings.TrimSpace(firstName + "_" + lastName + "_" + role))
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_")
	return "intv:" + replacer.Replace(base)
}

func normalizeNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func decodeSourceMetadata(metadata map[string]any) SourceMetadata {
	source := SourceMetadata{
		SourceSystem: toString(metadata["source_system"]),
		SourceURI:    toString(metadata["source_uri"]),
		ExternalID:   toString(metadata["external_id"]),
		SourceOrg:    toString(metadata["source_org"]),
	}
	if langs, ok := metadata["available_languages"].([]any); ok {
		source.AvailableLanguages = extractLanguages(langs)
	}
	return source
}

func decodeIntervenants(metadata map[string]any) []Intervenant {
	raw, ok := metadata["intervenants"].([]any)
	if !ok {
		return nil
	}
	out := make([]Intervenant, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		firstName := toString(row["first_name"])
		lastName := toString(row["last_name"])
		if firstName == "" || lastName == "" {
			continue
		}
		out = append(out, Intervenant{
			FirstName: firstName,
			LastName:  lastName,
			Role:      toString(row["role"]),
		})
	}
	return out
}

type chunkFilterData struct {
	VotationID  string
	ObjectID    string
	Level       string
	Canton      string
	CommuneCode string
	CommuneName string
	Status      string
	ObjectType  string
	ObjectTheme string
	SourceType  string
	VoteDate    *time.Time
}

func extractChunkFilterData(chunk Chunk) chunkFilterData {
	getMetadataString := func(key string) string {
		if chunk.Metadata == nil {
			return ""
		}
		return strings.TrimSpace(toString(chunk.Metadata[key]))
	}

	out := chunkFilterData{
		VotationID:  normalizeNonEmpty(getMetadataString("votation_id"), chunk.Source.ExternalID),
		ObjectID:    getMetadataString("object_id"),
		Level:       strings.ToLower(getMetadataString("level")),
		Canton:      strings.ToUpper(getMetadataString("canton")),
		CommuneCode: strings.TrimSpace(getMetadataString("commune_code")),
		CommuneName: strings.TrimSpace(getMetadataString("commune_name")),
		Status:      strings.ToLower(getMetadataString("status")),
		ObjectType:  getMetadataString("object_type"),
		ObjectTheme: getMetadataString("object_theme"),
		SourceType:  strings.ToLower(normalizeNonEmpty(getMetadataString("source_type"), "official")),
	}
	if rawVoteDate := getMetadataString("vote_date"); rawVoteDate != "" {
		if parsed, ok := parseTime(rawVoteDate); ok {
			out.VoteDate = &parsed
		}
	}
	if out.Level == "" {
		out.Level = "cantonal"
	}
	if out.Status == "" {
		if out.VoteDate != nil && out.VoteDate.After(time.Now().UTC()) {
			out.Status = "upcoming"
		} else {
			out.Status = "past"
		}
	}
	return out
}
