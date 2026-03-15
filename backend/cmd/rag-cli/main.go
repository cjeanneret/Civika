package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	workers := fs.Int("workers", 1, "number of indexing workers (1-8)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *workers < 1 || *workers > 8 {
		return fmt.Errorf("workers must be between %d and %d", 1, 8)
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
	indexRunID := newRAGIndexRunID()

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
	usageCollector := newIndexUsageCollector(store)
	ctx = rag.WithUsageScope(ctx, rag.UsageScope{
		Flow:  "rag_index",
		Mode:  cfg.RAG.Mode,
		RunID: indexRunID,
	})
	ctx = rag.WithUsageEmitter(ctx, usageCollector.Emit)

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

	var translator rag.Translator
	if cfg.RAG.Mode == "llm" {
		translator, err = buildTranslator(cfg)
		if err != nil {
			indexTask.Fail(err, map[string]any{"phase": "build_translator"})
			return err
		}
	}
	embedder, err := buildEmbedder(cfg)
	if err != nil {
		indexTask.Fail(err, map[string]any{"phase": "build_embedder"})
		return err
	}
	documentGroups := groupDocumentsByID(documentsToProcess)
	totals, err := processDocumentGroups(ctx, processDocumentGroupsInput{
		Cfg:            cfg,
		Store:          store,
		UsageCollector: usageCollector,
		Embedder:       embedder,
		Translator:     translator,
		ExistingState:  existingState,
		ChunkCfg:       chunkCfg,
		IndexRunID:     indexRunID,
		DocumentGroups: documentGroups,
		Workers:        *workers,
	})
	if err != nil {
		indexTask.Fail(err, map[string]any{"phase": "process_document"})
		return err
	}
	totalDuration := time.Since(indexTask.startedAt)
	processedDocumentCount := totals.ProcessedDocuments
	avgChunksPerDocument := 0.0
	documentsPerMinute := 0.0
	chunksPerSecond := 0.0
	upsertSharePercent := 0.0
	if processedDocumentCount > 0 {
		avgChunksPerDocument = float64(totals.TotalChunks) / float64(processedDocumentCount)
	}
	if totalDuration > 0 {
		documentsPerMinute = float64(processedDocumentCount) / totalDuration.Minutes()
		chunksPerSecond = float64(totals.TotalChunks) / totalDuration.Seconds()
		upsertSharePercent = (float64(totals.TotalUpsertDuration) / float64(totalDuration)) * 100.0
	}
	indexTask.Done(map[string]any{
		"documents":           len(documentsToProcess),
		"chunks":              totals.TotalChunks,
		"embedder":            embedder.Name(),
		"skipped_documents":   skipReport.SkippedDocuments,
		"grouped_documents":   skipReport.GroupedDocuments,
		"processed_documents": processedDocumentCount,
		"avg_chunks_per_doc":  fmt.Sprintf("%.2f", avgChunksPerDocument),
		"docs_per_min":        fmt.Sprintf("%.2f", documentsPerMinute),
		"chunks_per_sec":      fmt.Sprintf("%.2f", chunksPerSecond),
		"upsert_ms":           totals.TotalUpsertDuration.Milliseconds(),
		"embedding_ms":        totals.TotalEmbeddingDuration.Milliseconds(),
		"upsert_share_pct":    fmt.Sprintf("%.2f", upsertSharePercent),
	})

	fmt.Printf(
		"Indexation terminee: corpus=%s, %d document(s) traites, %d ignore(s), %d chunk(s), embedder=%s, duree=%s\n",
		selectedCorpus,
		len(documentsToProcess),
		skipReport.SkippedDocuments,
		totals.TotalChunks,
		embedder.Name(),
		totalDuration.Round(time.Second),
	)
	fmt.Printf(
		"Stats: chunks/doc=%.2f, docs/min=%.2f, chunks/s=%.2f, embed_ms=%d, upsert_ms=%d (%.2f%% du temps total)\n",
		avgChunksPerDocument,
		documentsPerMinute,
		chunksPerSecond,
		totals.TotalEmbeddingDuration.Milliseconds(),
		totals.TotalUpsertDuration.Milliseconds(),
		upsertSharePercent,
	)
	return nil
}

type indexStore interface {
	UpsertChunks(ctx context.Context, items []rag.EmbeddedChunk) error
	UpsertIndexDocumentMetrics(ctx context.Context, metric rag.UsageDocumentMetrics) error
}

type processDocumentGroupsInput struct {
	Cfg            config.Config
	Store          indexStore
	UsageCollector *indexUsageCollector
	Embedder       rag.Embedder
	Translator     rag.Translator
	ExistingState  map[string]rag.IndexDocumentState
	ChunkCfg       rag.ChunkConfig
	IndexRunID     string
	DocumentGroups [][]rag.Document
	Workers        int
}

type processDocumentResult struct {
	DocumentID        string
	ChunkCount        int
	EmbeddingDuration time.Duration
	UpsertDuration    time.Duration
}

type processDocumentTotals struct {
	ProcessedDocuments     int
	TotalChunks            int
	TotalEmbeddingDuration time.Duration
	TotalUpsertDuration    time.Duration
}

func processDocumentGroups(ctx context.Context, input processDocumentGroupsInput) (processDocumentTotals, error) {
	if input.Workers <= 0 {
		return processDocumentTotals{}, errors.New("workers must be > 0")
	}
	if len(input.DocumentGroups) == 0 {
		return processDocumentTotals{}, nil
	}
	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type documentJob struct {
		position int
		group    []rag.Document
	}
	jobs := make(chan documentJob)
	results := make(chan processDocumentResult, len(input.DocumentGroups))

	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	workerCount := input.Workers
	if workerCount > len(input.DocumentGroups) {
		workerCount = len(input.DocumentGroups)
	}
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if jobCtx.Err() != nil {
					return
				}
				result, err := processDocumentGroup(processDocumentGroupInput{
					Cfg:              input.Cfg,
					Ctx:              jobCtx,
					Store:            input.Store,
					UsageCollector:   input.UsageCollector,
					Embedder:         input.Embedder,
					Translator:       input.Translator,
					ExistingState:    input.ExistingState,
					ChunkCfg:         input.ChunkCfg,
					IndexRunID:       input.IndexRunID,
					DocumentGroup:    job.group,
					ProgressPosition: job.position,
					ProgressTotal:    len(input.DocumentGroups),
				})
				if err != nil {
					errOnce.Do(func() {
						firstErr = err
						cancel()
					})
					return
				}
				results <- result
			}
		}()
	}

publishLoop:
	for idx, group := range input.DocumentGroups {
		select {
		case <-jobCtx.Done():
			break publishLoop
		case jobs <- documentJob{
			position: idx + 1,
			group:    group,
		}:
		}
	}
	close(jobs)
	wg.Wait()
	close(results)
	if firstErr != nil {
		return processDocumentTotals{}, firstErr
	}

	totals := processDocumentTotals{}
	for result := range results {
		totals.ProcessedDocuments++
		totals.TotalChunks += result.ChunkCount
		totals.TotalEmbeddingDuration += result.EmbeddingDuration
		totals.TotalUpsertDuration += result.UpsertDuration
	}
	return totals, nil
}

type processDocumentGroupInput struct {
	Cfg              config.Config
	Ctx              context.Context
	Store            indexStore
	UsageCollector   *indexUsageCollector
	Embedder         rag.Embedder
	Translator       rag.Translator
	ExistingState    map[string]rag.IndexDocumentState
	ChunkCfg         rag.ChunkConfig
	IndexRunID       string
	DocumentGroup    []rag.Document
	ProgressPosition int
	ProgressTotal    int
}

func processDocumentGroup(input processDocumentGroupInput) (processDocumentResult, error) {
	if len(input.DocumentGroup) == 0 {
		return processDocumentResult{}, errors.New("document group is empty")
	}
	docID := strings.TrimSpace(input.DocumentGroup[0].ID)
	if docID == "" {
		return processDocumentResult{}, errors.New("document id is required")
	}

	documentTask := startIndexTask("process_document", map[string]any{
		"document_id":       docID,
		"progress_position": input.ProgressPosition,
		"progress_total":    input.ProgressTotal,
	})

	documentGroup := input.DocumentGroup
	if input.Cfg.RAG.Mode == "llm" {
		if input.Translator == nil {
			err := errors.New("translator is required in llm mode")
			documentTask.Fail(err, map[string]any{"phase": "ensure_translations"})
			return processDocumentResult{}, err
		}
		translationTask := startIndexTask("ensure_document_translations", map[string]any{
			"document_id": docID,
			"provider":    input.Translator.Name(),
		})
		var err error
		err = runWithHeartbeat(input.Ctx, 30*time.Second, "rag-cli: traduction en cours... (toujours actif)", func() error {
			var translationErr error
			documentGroup, translationErr = rag.EnsureMissingTranslationsWithOptions(
				input.Ctx,
				documentGroup,
				input.Cfg.RAG.SupportedLanguages,
				input.Cfg.RAG.DefaultLanguage,
				input.Translator,
				rag.EnsureMissingTranslationsOptions{
					ExistingByDocument: map[string]rag.IndexDocumentState{
						docID: input.ExistingState[docID],
					},
				},
			)
			return translationErr
		})
		if err != nil {
			translationTask.Fail(err, nil)
			documentTask.Fail(err, map[string]any{"phase": "ensure_translations"})
			return processDocumentResult{}, fmt.Errorf("ensure translations %s: %w", docID, err)
		}
		translationTask.Done(map[string]any{
			"document_id":  docID,
			"translations": len(documentGroup),
			"target_langs": len(input.Cfg.RAG.SupportedLanguages),
		})
	}

	chunkTask := startIndexTask("chunk_document", map[string]any{
		"document_id":       docID,
		"chunk_size_tokens": input.ChunkCfg.ChunkSizeTokens,
		"overlap_ratio":     fmt.Sprintf("%.4f", input.ChunkCfg.OverlapRatio),
		"translations":      len(documentGroup),
	})
	chunks, err := rag.ChunkDocuments(documentGroup, input.ChunkCfg)
	if err != nil {
		chunkTask.Fail(err, nil)
		documentTask.Fail(err, map[string]any{"phase": "chunk_document"})
		return processDocumentResult{}, fmt.Errorf("chunk document %s: %w", docID, err)
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
		"embedder":    input.Embedder.Name(),
	})
	var vectors [][]float32
	embeddingStartedAt := time.Now()
	documentCtx := rag.WithUsageDocumentID(input.Ctx, docID)
	err = runWithHeartbeat(input.Ctx, 30*time.Second, "rag-cli: embeddings en cours... (toujours actif)", func() error {
		var embedErr error
		vectors, embedErr = input.Embedder.EmbedTexts(documentCtx, texts)
		return embedErr
	})
	embeddingDuration := time.Since(embeddingStartedAt)
	if err != nil {
		embeddingTask.Fail(err, nil)
		documentTask.Fail(err, map[string]any{"phase": "embed_document_chunks"})
		return processDocumentResult{}, fmt.Errorf("embed document chunks %s: %w", docID, err)
	}
	embeddingTask.Done(map[string]any{
		"document_id": docID,
		"vectors":     len(vectors),
	})
	if len(vectors) != len(chunks) {
		inconsistentErr := errors.New("embedder returned inconsistent number of vectors")
		documentTask.Fail(inconsistentErr, map[string]any{"phase": "embed_document_chunks", "chunks": len(chunks), "vectors": len(vectors)})
		return processDocumentResult{}, inconsistentErr
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
	if err := input.Store.UpsertChunks(input.Ctx, embedded); err != nil {
		upsertTask.Fail(err, nil)
		documentTask.Fail(err, map[string]any{"phase": "upsert_document_chunks"})
		return processDocumentResult{}, fmt.Errorf("upsert document chunks %s: %w", docID, err)
	}
	upsertDuration := time.Since(upsertStartedAt)
	upsertTask.Done(nil)

	collectorSnapshot := indexDocumentUsageSnapshot{}
	if input.UsageCollector != nil {
		collectorSnapshot = input.UsageCollector.SnapshotDocument(docID)
	}
	chunkTokenSum := sumChunkTokens(chunks)
	sourceDoc := pickSourceDocumentForMetrics(documentGroup, input.Cfg.RAG.DefaultLanguage)
	if err := input.Store.UpsertIndexDocumentMetrics(input.Ctx, rag.UsageDocumentMetrics{
		RunID:                   input.IndexRunID,
		DocumentID:              docID,
		SourceLang:              sourceDoc.Language,
		SourceContentChars:      len(sourceDoc.Content),
		TitleChars:              len(sourceDoc.Title),
		TranslationsAttempted:   collectorSnapshot.TranslationsAttempted,
		TranslationsSucceeded:   collectorSnapshot.TranslationsSucceeded,
		ChunksCount:             len(chunks),
		ChunksTokensSum:         chunkTokenSum,
		EmbeddingCalls:          collectorSnapshot.EmbeddingCalls,
		EmbeddingInputCharsSum:  collectorSnapshot.EmbeddingInputCharsSum,
		EmbeddingInputTokensSum: collectorSnapshot.EmbeddingInputTokensSum,
		EmbeddingTotalTokensSum: collectorSnapshot.EmbeddingTotalTokensSum,
		LLMInputTokensSum:       collectorSnapshot.LLMInputTokensSum,
		LLMOutputTokensSum:      collectorSnapshot.LLMOutputTokensSum,
		LLMTotalTokensSum:       collectorSnapshot.LLMTotalTokensSum,
		Status:                  "success",
		IndexedAtUTC:            time.Now().UTC(),
	}); err != nil {
		documentTask.Fail(err, map[string]any{"phase": "upsert_document_metrics"})
		return processDocumentResult{}, fmt.Errorf("upsert document metrics %s: %w", docID, err)
	}
	documentTask.Done(map[string]any{
		"document_id": docID,
		"chunks":      len(chunks),
	})
	return processDocumentResult{
		DocumentID:        docID,
		ChunkCount:        len(chunks),
		EmbeddingDuration: embeddingDuration,
		UpsertDuration:    upsertDuration,
	}, nil
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

type indexDocumentUsageSnapshot struct {
	TranslationsAttempted   int
	TranslationsSucceeded   int
	EmbeddingCalls          int
	EmbeddingInputCharsSum  int
	EmbeddingInputTokensSum int
	EmbeddingTotalTokensSum int
	LLMInputTokensSum       int
	LLMOutputTokensSum      int
	LLMTotalTokensSum       int
}

type indexDocumentUsageAccumulator struct {
	embeddingCalls          int
	embeddingInputCharsSum  int
	embeddingInputTokensSum int
	embeddingTotalTokensSum int
	llmInputTokensSum       int
	llmOutputTokensSum      int
	llmTotalTokensSum       int
	attemptedTargets        map[string]struct{}
	succeededTargets        map[string]struct{}
}

type indexUsageCollector struct {
	mu         sync.Mutex
	writer     rag.UsageMetricsWriter
	byDocument map[string]*indexDocumentUsageAccumulator
}

func newIndexUsageCollector(writer rag.UsageMetricsWriter) *indexUsageCollector {
	return &indexUsageCollector{
		writer:     writer,
		byDocument: map[string]*indexDocumentUsageAccumulator{},
	}
}

func (c *indexUsageCollector) Emit(ctx context.Context, event rag.UsageEvent) {
	if c.writer != nil {
		if err := c.writer.RecordUsageEvent(ctx, event); err != nil {
			log.Printf("rag-cli usage metrics write error: %v", err)
		}
	}
	docID := strings.TrimSpace(event.DocumentID)
	if docID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	acc, ok := c.byDocument[docID]
	if !ok {
		acc = &indexDocumentUsageAccumulator{
			attemptedTargets: map[string]struct{}{},
			succeededTargets: map[string]struct{}{},
		}
		c.byDocument[docID] = acc
	}
	switch event.Operation {
	case "embedding":
		acc.embeddingCalls++
		acc.embeddingInputCharsSum += maxInt(event.InputChars, 0)
		acc.embeddingInputTokensSum += maxInt(event.InputTokens, 0)
		acc.embeddingTotalTokensSum += maxInt(event.TotalTokens, 0)
	case "translation":
		acc.llmInputTokensSum += maxInt(event.InputTokens, 0)
		acc.llmOutputTokensSum += maxInt(event.OutputTokens, 0)
		acc.llmTotalTokensSum += maxInt(event.TotalTokens, 0)
		target := strings.TrimSpace(event.TargetLang)
		source := strings.TrimSpace(event.SourceLang)
		if target != "" && target != source {
			acc.attemptedTargets[target] = struct{}{}
			if strings.TrimSpace(event.Status) == "success" {
				acc.succeededTargets[target] = struct{}{}
			}
		}
	}
}

func (c *indexUsageCollector) SnapshotDocument(documentID string) indexDocumentUsageSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	acc, ok := c.byDocument[strings.TrimSpace(documentID)]
	if !ok {
		return indexDocumentUsageSnapshot{}
	}
	return indexDocumentUsageSnapshot{
		TranslationsAttempted:   len(acc.attemptedTargets),
		TranslationsSucceeded:   len(acc.succeededTargets),
		EmbeddingCalls:          acc.embeddingCalls,
		EmbeddingInputCharsSum:  acc.embeddingInputCharsSum,
		EmbeddingInputTokensSum: acc.embeddingInputTokensSum,
		EmbeddingTotalTokensSum: acc.embeddingTotalTokensSum,
		LLMInputTokensSum:       acc.llmInputTokensSum,
		LLMOutputTokensSum:      acc.llmOutputTokensSum,
		LLMTotalTokensSum:       acc.llmTotalTokensSum,
	}
}

func pickSourceDocumentForMetrics(documents []rag.Document, defaultLang string) rag.Document {
	normalizedDefault := strings.ToLower(strings.TrimSpace(defaultLang))
	if normalizedDefault != "" {
		for _, doc := range documents {
			if strings.ToLower(strings.TrimSpace(doc.Language)) == normalizedDefault {
				return doc
			}
		}
	}
	for _, doc := range documents {
		if strings.ToLower(strings.TrimSpace(doc.Language)) == "fr" {
			return doc
		}
	}
	if len(documents) > 0 {
		return documents[0]
	}
	return rag.Document{}
}

func sumChunkTokens(chunks []rag.Chunk) int {
	total := 0
	for _, chunk := range chunks {
		total += maxInt(chunk.TokenCount, 0)
	}
	return total
}

func newRAGIndexRunID() string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("rag-index-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("rag-index-%d-%s", time.Now().UTC().Unix(), hex.EncodeToString(raw))
}

func maxInt(value int, fallback int) int {
	if value < fallback {
		return fallback
	}
	return value
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
