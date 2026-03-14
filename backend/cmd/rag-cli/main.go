package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"civika/backend/config"
	"civika/backend/internal/debuglog"
	"civika/backend/internal/rag"
)

func main() {
	if err := run(); err != nil {
		log.Printf("rag-cli error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: rag-cli <index|debug-chunks|query> [flags]")
	}

	cfg := config.LoadFromEnv()
	rag.SetSupportedLanguages(cfg.RAG.SupportedLanguages)
	debuglog.Configure(cfg.Debug.Enabled, cfg.Debug.LogPath)
	if err := cfg.ValidateRAGMode(); err != nil {
		return err
	}
	switch os.Args[1] {
	case "index":
		return runIndex(cfg, os.Args[2:])
	case "ingest":
		return runIndex(cfg, os.Args[2:])
	case "debug-chunks":
		return runDebugChunks(cfg, os.Args[2:])
	case "query":
		return runQuery(cfg, os.Args[2:])
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}

func runIndex(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	corpusPath := fs.String("corpus", "", "local corpus path (default: data/normalized)")
	maxFileBytes := fs.Int64("max-file-bytes", 2*1024*1024, "max file size in bytes")
	chunkSize := fs.Int("chunk-size", cfg.RAG.ChunkSizeTokens, "chunk size in tokens")
	overlap := fs.Float64("chunk-overlap", cfg.RAG.ChunkOverlapRatio, "chunk overlap ratio")
	if err := fs.Parse(args); err != nil {
		return err
	}
	selectedCorpus := *corpusPath
	if selectedCorpus == "" {
		selectedCorpus = defaultCorpusPath()
	}
	if !directoryExists(selectedCorpus) {
		return fmt.Errorf("corpus directory not found: %s", selectedCorpus)
	}
	indexTask := startIndexTask("index_total", map[string]any{
		"mode":   cfg.RAG.Mode,
		"corpus": selectedCorpus,
	})

	ctx, cancel := indexContextForMode(cfg.RAG.Mode, cfg.RAG.IndexTimeout)
	defer cancel()

	ingestTask := startIndexTask("ingest_corpus", map[string]any{
		"max_file_bytes": *maxFileBytes,
	})
	documents, err := rag.IngestCorpus(ctx, selectedCorpus, rag.IngestConfig{
		MaxFileBytes: *maxFileBytes,
	})
	if err != nil {
		ingestTask.Fail(err, nil)
		indexTask.Fail(err, map[string]any{"phase": "ingest_corpus"})
		return fmt.Errorf("ingest corpus: %w", err)
	}
	documents = rag.PrepareDocumentsForIndex(documents)
	ingestTask.Done(map[string]any{"documents": len(documents)})

	store, err := buildStore(cfg)
	if err != nil {
		indexTask.Fail(err, map[string]any{"phase": "build_store"})
		return err
	}
	defer store.Close()

	initSchemaTask := startIndexTask("init_schema", nil)
	if err := store.InitSchema(ctx); err != nil {
		initSchemaTask.Fail(err, nil)
		indexTask.Fail(err, map[string]any{"phase": "init_schema"})
		return fmt.Errorf("init schema: %w", err)
	}
	initSchemaTask.Done(nil)

	documentIDs := uniqueDocumentIDs(documents)
	stateTask := startIndexTask("load_index_state", map[string]any{
		"documents": len(documentIDs),
	})
	existingState, err := store.LoadIndexState(ctx, documentIDs)
	if err != nil {
		stateTask.Fail(err, nil)
		indexTask.Fail(err, map[string]any{"phase": "load_index_state"})
		return fmt.Errorf("load existing index state: %w", err)
	}
	stateTask.Done(map[string]any{
		"documents_with_state": len(existingState),
	})

	chunkCfg := rag.ChunkConfig{
		ChunkSizeTokens: *chunkSize,
		OverlapRatio:    *overlap,
	}
	documentsToProcess, skipReport := rag.FilterDocumentsForIndex(documents, cfg.RAG.SupportedLanguages, cfg.RAG.Mode, chunkCfg, existingState)
	if skipReport.SkippedDocuments > 0 {
		log.Printf("rag-cli index skip_summary grouped_documents=%d skipped_documents=%d processed_documents=%d", skipReport.GroupedDocuments, skipReport.SkippedDocuments, skipReport.ProcessedDocs)
	}
	if len(documentsToProcess) == 0 {
		indexTask.Done(map[string]any{
			"documents":         len(documents),
			"skipped_documents": skipReport.SkippedDocuments,
			"chunks":            0,
		})
		fmt.Printf("Indexation terminee (skip intelligent): corpus=%s, documents_groupes=%d, ignores=%d, traites=0\n", selectedCorpus, skipReport.GroupedDocuments, skipReport.SkippedDocuments)
		return nil
	}

	if cfg.RAG.Mode == "llm" {
		translator, err := buildTranslator(cfg)
		if err != nil {
			indexTask.Fail(err, map[string]any{"phase": "build_translator"})
			return err
		}
		translationTask := startIndexTask("ensure_translations", map[string]any{
			"provider":     translator.Name(),
			"documents":    len(documentsToProcess),
			"skipped_docs": skipReport.SkippedDocuments,
			"grouped_docs": skipReport.GroupedDocuments,
		})
		err = runWithHeartbeat(ctx, 30*time.Second, "rag-cli: traduction en cours... (toujours actif)", func() error {
			var translationErr error
			documentsToProcess, translationErr = rag.EnsureMissingTranslationsWithOptions(
				ctx,
				documentsToProcess,
				cfg.RAG.SupportedLanguages,
				cfg.RAG.DefaultLanguage,
				translator,
				rag.EnsureMissingTranslationsOptions{
					ExistingByDocument: existingState,
				},
			)
			return translationErr
		})
		if err != nil {
			translationTask.Fail(err, nil)
			indexTask.Fail(err, map[string]any{"phase": "ensure_translations"})
			return fmt.Errorf("ensure translations: %w", err)
		}
		translationTask.Done(map[string]any{
			"documents":           len(documentsToProcess),
			"supported_languages": len(cfg.RAG.SupportedLanguages),
		})
	}
	embedder, err := buildEmbedder(cfg)
	if err != nil {
		indexTask.Fail(err, map[string]any{"phase": "build_embedder"})
		return err
	}
	documentGroups := groupDocumentsByID(documentsToProcess)
	totalChunks := 0
	totalEmbeddingDuration := time.Duration(0)
	totalUpsertDuration := time.Duration(0)
	for i, documentGroup := range documentGroups {
		docID := documentGroup[0].ID
		documentTask := startIndexTask("process_document", map[string]any{
			"document_id":       docID,
			"progress_position": i + 1,
			"progress_total":    len(documentGroups),
		})
		chunkTask := startIndexTask("chunk_document", map[string]any{
			"document_id":       docID,
			"chunk_size_tokens": chunkCfg.ChunkSizeTokens,
			"overlap_ratio":     fmt.Sprintf("%.4f", chunkCfg.OverlapRatio),
			"translations":      len(documentGroup),
		})
		chunks, chunkErr := rag.ChunkDocuments(documentGroup, chunkCfg)
		if chunkErr != nil {
			chunkTask.Fail(chunkErr, nil)
			documentTask.Fail(chunkErr, map[string]any{"phase": "chunk_document"})
			indexTask.Fail(chunkErr, map[string]any{"phase": "chunk_document", "document_id": docID})
			return fmt.Errorf("chunk document %s: %w", docID, chunkErr)
		}
		chunkTask.Done(map[string]any{
			"document_id": docID,
			"chunks":      len(chunks),
		})

		texts := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			texts = append(texts, chunk.Text)
		}
		embeddingTask := startIndexTask("embed_document_chunks", map[string]any{
			"document_id": docID,
			"chunks":      len(chunks),
			"embedder":    embedder.Name(),
		})
		var vectors [][]float32
		embeddingStartedAt := time.Now()
		err = runWithHeartbeat(ctx, 30*time.Second, "rag-cli: embeddings en cours... (toujours actif)", func() error {
			var embedErr error
			vectors, embedErr = embedder.EmbedTexts(ctx, texts)
			return embedErr
		})
		totalEmbeddingDuration += time.Since(embeddingStartedAt)
		if err != nil {
			embeddingTask.Fail(err, nil)
			documentTask.Fail(err, map[string]any{"phase": "embed_document_chunks"})
			indexTask.Fail(err, map[string]any{"phase": "embed_document_chunks", "document_id": docID})
			return fmt.Errorf("embed document chunks %s: %w", docID, err)
		}
		embeddingTask.Done(map[string]any{
			"document_id": docID,
			"vectors":     len(vectors),
		})
		if len(vectors) != len(chunks) {
			inconsistentErr := errors.New("embedder returned inconsistent number of vectors")
			documentTask.Fail(inconsistentErr, map[string]any{"phase": "embed_document_chunks", "chunks": len(chunks), "vectors": len(vectors)})
			indexTask.Fail(inconsistentErr, map[string]any{"phase": "embed_document_chunks", "document_id": docID, "chunks": len(chunks), "vectors": len(vectors)})
			return inconsistentErr
		}

		embedded := make([]rag.EmbeddedChunk, 0, len(chunks))
		for idx, chunk := range chunks {
			embedded = append(embedded, rag.EmbeddedChunk{
				Chunk:  chunk,
				Vector: vectors[idx],
			})
		}
		upsertTask := startIndexTask("upsert_document_chunks", map[string]any{
			"document_id": docID,
			"chunks":      len(embedded),
		})
		upsertStartedAt := time.Now()
		if err := store.UpsertChunks(ctx, embedded); err != nil {
			upsertTask.Fail(err, nil)
			documentTask.Fail(err, map[string]any{"phase": "upsert_document_chunks"})
			indexTask.Fail(err, map[string]any{"phase": "upsert_document_chunks", "document_id": docID})
			return fmt.Errorf("upsert document chunks %s: %w", docID, err)
		}
		totalUpsertDuration += time.Since(upsertStartedAt)
		upsertTask.Done(nil)
		totalChunks += len(chunks)
		documentTask.Done(map[string]any{
			"document_id": docID,
			"chunks":      len(chunks),
		})
	}
	totalDuration := time.Since(indexTask.startedAt)
	processedDocumentCount := len(documentGroups)
	avgChunksPerDocument := 0.0
	documentsPerMinute := 0.0
	chunksPerSecond := 0.0
	upsertSharePercent := 0.0
	if processedDocumentCount > 0 {
		avgChunksPerDocument = float64(totalChunks) / float64(processedDocumentCount)
	}
	if totalDuration > 0 {
		documentsPerMinute = float64(processedDocumentCount) / totalDuration.Minutes()
		chunksPerSecond = float64(totalChunks) / totalDuration.Seconds()
		upsertSharePercent = (float64(totalUpsertDuration) / float64(totalDuration)) * 100.0
	}
	indexTask.Done(map[string]any{
		"documents":           len(documentsToProcess),
		"chunks":              totalChunks,
		"embedder":            embedder.Name(),
		"skipped_documents":   skipReport.SkippedDocuments,
		"grouped_documents":   skipReport.GroupedDocuments,
		"processed_documents": skipReport.ProcessedDocs,
		"avg_chunks_per_doc":  fmt.Sprintf("%.2f", avgChunksPerDocument),
		"docs_per_min":        fmt.Sprintf("%.2f", documentsPerMinute),
		"chunks_per_sec":      fmt.Sprintf("%.2f", chunksPerSecond),
		"upsert_ms":           totalUpsertDuration.Milliseconds(),
		"embedding_ms":        totalEmbeddingDuration.Milliseconds(),
		"upsert_share_pct":    fmt.Sprintf("%.2f", upsertSharePercent),
	})

	fmt.Printf(
		"Indexation terminee: corpus=%s, %d document(s) traites, %d ignore(s), %d chunk(s), embedder=%s, duree=%s\n",
		selectedCorpus,
		len(documentsToProcess),
		skipReport.SkippedDocuments,
		totalChunks,
		embedder.Name(),
		totalDuration.Round(time.Second),
	)
	fmt.Printf(
		"Stats: chunks/doc=%.2f, docs/min=%.2f, chunks/s=%.2f, embed_ms=%d, upsert_ms=%d (%.2f%% du temps total)\n",
		avgChunksPerDocument,
		documentsPerMinute,
		chunksPerSecond,
		totalEmbeddingDuration.Milliseconds(),
		totalUpsertDuration.Milliseconds(),
		upsertSharePercent,
	)
	return nil
}

type indexTaskScope struct {
	taskName  string
	startedAt time.Time
}

func startIndexTask(taskName string, fields map[string]any) indexTaskScope {
	scope := indexTaskScope{
		taskName:  taskName,
		startedAt: time.Now(),
	}
	logIndexTaskEvent("start", taskName, fields)
	return scope
}

func (s indexTaskScope) Done(fields map[string]any) {
	withDuration := mergeTaskFields(fields, map[string]any{
		"duration_ms": time.Since(s.startedAt).Milliseconds(),
	})
	logIndexTaskEvent("done", s.taskName, withDuration)
}

func (s indexTaskScope) Fail(err error, fields map[string]any) {
	withFailure := mergeTaskFields(fields, map[string]any{
		"duration_ms": time.Since(s.startedAt).Milliseconds(),
		"error":       err,
	})
	logIndexTaskEvent("failed", s.taskName, withFailure)
}

func logIndexTaskEvent(taskStatus string, taskName string, fields map[string]any) {
	log.Printf("rag-cli index task=%s task_name=%s%s", taskStatus, taskName, formatIndexTaskFields(fields))
}

func formatIndexTaskFields(fields map[string]any) string {
	if len(fields) == 0 {
		return ""
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		value := fields[key]
		if value == nil {
			continue
		}
		b.WriteString(" ")
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(strings.ReplaceAll(fmt.Sprint(value), " ", "_"))
	}
	return b.String()
}

func mergeTaskFields(base map[string]any, additional map[string]any) map[string]any {
	if len(base) == 0 && len(additional) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range additional {
		out[key] = value
	}
	return out
}

func runWithHeartbeat(ctx context.Context, interval time.Duration, heartbeatMessage string, operation func() error) error {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Print(heartbeatMessage)
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}()
	err := operation()
	close(done)
	return err
}

func indexContextForMode(mode string, configuredTimeout time.Duration) (context.Context, context.CancelFunc) {
	if configuredTimeout > 0 {
		return context.WithTimeout(context.Background(), configuredTimeout)
	}
	if mode == "local" {
		// En local, on garde un garde-fou court pour eviter les executions bloquees.
		return context.WithTimeout(context.Background(), 2*time.Minute)
	}
	// En mode LLM, pas de deadline globale implicite; les timeouts de requete
	// (traduction/embeddings) restent responsables de la protection reseau.
	return context.WithCancel(context.Background())
}

func runQuery(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	question := fs.String("q", "", "query text")
	topK := fs.Int("top-k", cfg.RAG.TopK, "number of retrieved chunks")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *question == "" {
		return errors.New("query requires --q")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	embedder, err := buildEmbedder(cfg)
	if err != nil {
		return err
	}
	store, err := buildStore(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	hits, err := rag.QueryRAG(ctx, store, embedder, *question, *topK)
	if err != nil {
		return fmt.Errorf("query rag: %w", err)
	}
	if len(hits) == 0 {
		fmt.Println("Aucun document retrouve.")
		return nil
	}

	fmt.Println("Documents retrouves:")
	for i, hit := range hits {
		fmt.Printf("%d. score=%.4f source=%s title=%s lang=%s\n", i+1, hit.Score, hit.Chunk.SourcePath, hit.Chunk.Title, hit.Chunk.Language)
	}

	summarizer, err := buildSummarizer(cfg)
	if err != nil {
		return err
	}
	summary, err := rag.ExplainVotation(ctx, summarizer, *question, hits)
	if err != nil {
		return fmt.Errorf("summarize results: %w", err)
	}
	fmt.Println("\nResume:")
	fmt.Println(summary)
	return nil
}

func runDebugChunks(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("debug-chunks", flag.ContinueOnError)
	corpusPath := fs.String("corpus", "", "local corpus path (default: data/normalized)")
	maxFileBytes := fs.Int64("max-file-bytes", 2*1024*1024, "max file size in bytes")
	chunkSize := fs.Int("chunk-size", cfg.RAG.ChunkSizeTokens, "chunk size in tokens")
	overlap := fs.Float64("chunk-overlap", cfg.RAG.ChunkOverlapRatio, "chunk overlap ratio")
	sample := fs.Int("sample", 3, "number of sample chunks to print")
	topDocs := fs.Int("top-docs", 5, "number of top documents by chunk count")
	if err := fs.Parse(args); err != nil {
		return err
	}

	selectedCorpus := *corpusPath
	if selectedCorpus == "" {
		selectedCorpus = defaultCorpusPath()
	}
	if !directoryExists(selectedCorpus) {
		return fmt.Errorf("corpus directory not found: %s", selectedCorpus)
	}
	if *sample < 0 {
		return errors.New("sample must be >= 0")
	}
	if *topDocs <= 0 {
		return errors.New("top-docs must be > 0")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	documents, err := rag.IngestCorpus(ctx, selectedCorpus, rag.IngestConfig{
		MaxFileBytes: *maxFileBytes,
	})
	if err != nil {
		return fmt.Errorf("ingest corpus: %w", err)
	}
	chunks, err := rag.ChunkDocuments(documents, rag.ChunkConfig{
		ChunkSizeTokens: *chunkSize,
		OverlapRatio:    *overlap,
	})
	if err != nil {
		return fmt.Errorf("chunk documents: %w", err)
	}
	stats := rag.BuildChunkDebugStats(documents, chunks)

	fmt.Printf("Debug chunking (sans DB): corpus=%s\n", selectedCorpus)
	fmt.Printf("- documents: %d\n", stats.DocumentCount)
	fmt.Printf("- chunks: %d\n", stats.ChunkCount)
	fmt.Printf("- tokens/chunk: min=%d avg=%d p95=%d max=%d\n", stats.Tokens.Min, stats.Tokens.Avg, stats.Tokens.P95, stats.Tokens.Max)

	if len(stats.ByLanguage) > 0 {
		fmt.Println("- distribution langues:")
		for _, item := range stats.ByLanguage {
			fmt.Printf("  - %s: %d\n", item.Key, item.Count)
		}
	}
	if len(stats.BySource) > 0 {
		fmt.Println("- distribution source_system:")
		for _, item := range stats.BySource {
			fmt.Printf("  - %s: %d\n", item.Key, item.Count)
		}
	}
	if len(stats.TopDocuments) > 0 {
		fmt.Println("- top documents (par nombre de chunks):")
		limit := *topDocs
		if limit > len(stats.TopDocuments) {
			limit = len(stats.TopDocuments)
		}
		for i := 0; i < limit; i++ {
			item := stats.TopDocuments[i]
			fmt.Printf("  %d) chunks=%d doc=%s title=%s source=%s\n", i+1, item.ChunkCount, item.DocumentID, item.Title, item.SourcePath)
		}
	}

	if *sample > 0 && len(chunks) > 0 {
		limit := *sample
		if limit > len(chunks) {
			limit = len(chunks)
		}
		fmt.Printf("- echantillons de chunks (n=%d):\n", limit)
		for i := 0; i < limit; i++ {
			chunk := chunks[i]
			snippet := chunk.Text
			if len(snippet) > 220 {
				snippet = snippet[:220] + "..."
			}
			fmt.Printf("  %d) id=%s lang=%s tokens=%d title=%s\n", i+1, chunk.ID, chunk.Language, chunk.TokenCount, chunk.Title)
			fmt.Printf("     %s\n", snippet)
		}
	}

	return nil
}

func buildEmbedder(cfg config.Config) (rag.Embedder, error) {
	if cfg.RAG.Mode == "llm" {
		return rag.NewLLMEmbedder(rag.LLMEmbedderConfig{
			Enabled:       cfg.LLMEmbedding.Enabled,
			BaseURL:       cfg.LLMEmbedding.BaseURL,
			APIKey:        cfg.LLMEmbedding.APIKey,
			ModelName:     cfg.LLMEmbedding.ModelName,
			Timeout:       cfg.LLMEmbedding.Timeout,
			MaxInputChars: cfg.LLMEmbedding.MaxInputChars,
		})
	}
	return rag.NewDeterministicEmbedder(cfg.RAG.EmbeddingDimensions), nil
}

func buildSummarizer(cfg config.Config) (rag.Summarizer, error) {
	if cfg.RAG.Mode == "llm" {
		return rag.NewLLMSummarizer(rag.LLMSummarizerConfig{
			Enabled:        cfg.LLM.Enabled,
			BaseURL:        cfg.LLM.BaseURL,
			APIKey:         cfg.LLM.APIKey,
			ModelName:      cfg.LLM.ModelName,
			Timeout:        cfg.LLM.Timeout,
			MaxPromptChars: cfg.LLM.MaxPromptChars,
		})
	}
	return rag.NewDeterministicSummarizer(), nil
}

func buildStore(cfg config.Config) (*rag.PostgresVectorStore, error) {
	return rag.NewPostgresVectorStore(rag.PostgresVectorStoreConfig{
		Host:               cfg.Postgres.Host,
		Port:               cfg.Postgres.Port,
		User:               cfg.Postgres.User,
		Password:           cfg.Postgres.Password,
		DBName:             cfg.Postgres.DBName,
		SSLMode:            cfg.Postgres.SSLMode,
		TableName:          cfg.RAG.TableName,
		EmbeddingDimension: cfg.RAG.EmbeddingDimensions,
		MaxOpenConns:       cfg.Postgres.MaxOpenConns,
		MaxIdleConns:       cfg.Postgres.MaxIdleConns,
	})
}

func buildTranslator(cfg config.Config) (rag.Translator, error) {
	if cfg.RAG.Mode != "llm" {
		return rag.NewDisabledTranslator(), nil
	}
	translator, err := rag.NewLLMTranslator(rag.LLMTranslatorConfig{
		Enabled:       cfg.LLM.Enabled,
		BaseURL:       cfg.LLM.BaseURL,
		APIKey:        cfg.LLM.APIKey,
		ModelName:     cfg.LLM.ModelName,
		Timeout:       cfg.LLM.TranslationTimeout,
		MaxInputChars: cfg.LLM.MaxPromptChars,
		MaxRetries:    cfg.LLM.TranslationMaxRetries,
	})
	if err != nil {
		return nil, fmt.Errorf("create llm translator: %w", err)
	}
	return translator, nil
}

func defaultCorpusPath() string {
	candidates := []string{
		filepath.Join("data", "normalized"),
		filepath.Join("..", "data", "normalized"),
	}
	for _, candidate := range candidates {
		if directoryExists(candidate) {
			return candidate
		}
	}
	return candidates[0]
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func uniqueDocumentIDs(documents []rag.Document) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(documents))
	for _, doc := range documents {
		if doc.ID == "" {
			continue
		}
		if _, exists := seen[doc.ID]; exists {
			continue
		}
		seen[doc.ID] = struct{}{}
		out = append(out, doc.ID)
	}
	return out
}

func groupDocumentsByID(documents []rag.Document) [][]rag.Document {
	grouped := map[string][]rag.Document{}
	ids := make([]string, 0, len(documents))
	for _, doc := range documents {
		if strings.TrimSpace(doc.ID) == "" {
			continue
		}
		if _, exists := grouped[doc.ID]; !exists {
			ids = append(ids, doc.ID)
		}
		grouped[doc.ID] = append(grouped[doc.ID], doc)
	}
	sort.Strings(ids)
	out := make([][]rag.Document, 0, len(ids))
	for _, id := range ids {
		out = append(out, grouped[id])
	}
	return out
}
