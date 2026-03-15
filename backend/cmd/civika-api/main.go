package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"civika/backend/config"
	"civika/backend/internal/debuglog"
	httpapi "civika/backend/internal/http"
	"civika/backend/internal/rag"
	"civika/backend/internal/services"
)

func main() {
	cfg := config.LoadFromEnv()
	rag.SetSupportedLanguages(cfg.RAG.SupportedLanguages)
	debuglog.Configure(cfg.Debug.Enabled, cfg.Debug.LogPath)
	if err := cfg.ValidateRAGMode(); err != nil {
		log.Fatalf("invalid RAG configuration: %v", err)
	}
	debugLogPath := cfg.Debug.LogPath
	if debugLogPath == "" {
		debugLogPath = "/tmp/debug-2055fd.log"
	}

	db, err := services.OpenPostgresDB(cfg.Postgres)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	store, err := rag.NewPostgresVectorStore(rag.PostgresVectorStoreConfig{
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
	if err != nil {
		log.Fatalf("build vector store: %v", err)
	}
	defer store.Close()

	if err := store.InitSchema(context.Background()); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	embedder, summarizer, err := buildRAGRuntime(cfg)
	if err != nil {
		log.Fatalf("build rag runtime: %v", err)
	}
	translator, err := buildTranslatorRuntime(cfg)
	if err != nil {
		log.Fatalf("build translator runtime: %v", err)
	}

	votationService := services.NewSQLQueryServiceWithTranslations(db, services.TranslationRuntimeConfig{
		Translator:         translator,
		SupportedLanguages: cfg.RAG.SupportedLanguages,
		DefaultLanguage:    cfg.RAG.DefaultLanguage,
		FallbackLanguage:   cfg.RAG.FallbackLanguage,
	})
	qaCache := services.NewQACache(cfg.QACache)
	qaService := services.NewQAService(store, embedder, summarizer, cfg.RAG.TopK, store, cfg.RAG.Mode, qaCache)

	srv := &http.Server{
		Addr: cfg.APIAddress(),
		Handler: httpapi.NewRouter(cfg, httpapi.RouterDependencies{
			VotationService: votationService,
			QAService:       qaService,
			UsageMetrics:    store,
			QACacheMetrics:  qaCache,
			APIVersion:      "v1",
			RAGMode:         cfg.RAG.Mode,
		}),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	log.Printf(
		"civika-api listening on %s debugLogEnabled=%t debugLogPath=%s",
		cfg.APIAddress(),
		cfg.Debug.Enabled,
		debugLogPath,
	)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func buildRAGRuntime(cfg config.Config) (rag.Embedder, rag.Summarizer, error) {
	switch cfg.RAG.Mode {
	case "local":
		return rag.NewDeterministicEmbedder(cfg.RAG.EmbeddingDimensions), rag.NewDeterministicSummarizer(), nil
	case "llm":
		embedder, err := rag.NewLLMEmbedder(rag.LLMEmbedderConfig{
			Enabled:       cfg.LLMEmbedding.Enabled,
			BaseURL:       cfg.LLMEmbedding.BaseURL,
			APIKey:        cfg.LLMEmbedding.APIKey,
			ModelName:     cfg.LLMEmbedding.ModelName,
			Timeout:       cfg.LLMEmbedding.Timeout,
			MaxInputChars: cfg.LLMEmbedding.MaxInputChars,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create llm embedder: %w", err)
		}
		summarizer, err := rag.NewLLMSummarizer(rag.LLMSummarizerConfig{
			Enabled:         cfg.LLM.Enabled,
			BaseURL:         cfg.LLM.BaseURL,
			APIKey:          cfg.LLM.APIKey,
			ModelName:       cfg.LLM.ModelName,
			Timeout:         cfg.LLM.Timeout,
			MaxPromptChars:  cfg.LLM.MaxPromptChars,
			MaxOutputTokens: cfg.LLM.MaxOutputTokens,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create llm summarizer: %w", err)
		}
		return embedder, summarizer, nil
	default:
		return nil, nil, fmt.Errorf("invalid RAG mode %q", cfg.RAG.Mode)
	}
}

func buildTranslatorRuntime(cfg config.Config) (rag.Translator, error) {
	if cfg.RAG.Mode != "llm" {
		return nil, nil
	}
	translator, err := rag.NewLLMTranslator(rag.LLMTranslatorConfig{
		Enabled:         cfg.LLM.Enabled,
		BaseURL:         cfg.LLM.BaseURL,
		APIKey:          cfg.LLM.APIKey,
		ModelName:       cfg.LLM.ModelName,
		Timeout:         cfg.LLM.TranslationTimeout,
		MaxInputChars:   cfg.LLM.MaxPromptChars,
		MaxRetries:      cfg.LLM.TranslationMaxRetries,
		MaxOutputTokens: cfg.LLM.TranslationMaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("create llm translator: %w", err)
	}
	return translator, nil
}
