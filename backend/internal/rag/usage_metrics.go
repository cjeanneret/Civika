package rag

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	UsageListMinLimit     = 1
	UsageListDefaultLimit = 100
	UsageListMaxLimit     = 1000

	usageListInitialAllocCap = 128
)

type UsageScope struct {
	Flow       string
	Mode       string
	RunID      string
	RequestID  string
	DocumentID string
}

type UsageEvent struct {
	EventID      string
	CreatedAtUTC time.Time
	Flow         string
	Operation    string
	Mode         string
	ProviderName string
	ModelName    string
	RunID        string
	RequestID    string
	DocumentID   string
	SourceLang   string
	TargetLang   string
	InputChars   int
	OutputChars  int
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	UsageSource  string
	Status       string
	DurationMS   int64
	ErrorCode    string
}

type UsageDocumentMetrics struct {
	RunID                   string
	DocumentID              string
	SourceLang              string
	SourceContentChars      int
	TitleChars              int
	TranslationsAttempted   int
	TranslationsSucceeded   int
	ChunksCount             int
	ChunksTokensSum         int
	EmbeddingCalls          int
	EmbeddingInputCharsSum  int
	EmbeddingInputTokensSum int
	EmbeddingTotalTokensSum int
	LLMInputTokensSum       int
	LLMOutputTokensSum      int
	LLMTotalTokensSum       int
	Status                  string
	IndexedAtUTC            time.Time
}

type UsageDailyAggregate struct {
	Day             string `json:"day"`
	Flow            string `json:"flow"`
	Operation       string `json:"operation"`
	Mode            string `json:"mode"`
	ModelName       string `json:"modelName"`
	ProviderName    string `json:"providerName"`
	EventsCount     int64  `json:"eventsCount"`
	SuccessCount    int64  `json:"successCount"`
	ErrorCount      int64  `json:"errorCount"`
	InputCharsSum   int64  `json:"inputCharsSum"`
	OutputCharsSum  int64  `json:"outputCharsSum"`
	InputTokensSum  int64  `json:"inputTokensSum"`
	OutputTokensSum int64  `json:"outputTokensSum"`
	TotalTokensSum  int64  `json:"totalTokensSum"`
	DurationMSSum   int64  `json:"durationMsSum"`
}

type UsageEventRow struct {
	EventID      string `json:"eventId"`
	CreatedAtUTC string `json:"createdAt"`
	Flow         string `json:"flow"`
	Operation    string `json:"operation"`
	Mode         string `json:"mode"`
	ProviderName string `json:"providerName"`
	ModelName    string `json:"modelName"`
	RunID        string `json:"runId"`
	RequestID    string `json:"requestId"`
	DocumentID   string `json:"documentId"`
	SourceLang   string `json:"sourceLang"`
	TargetLang   string `json:"targetLang"`
	InputChars   int    `json:"inputChars"`
	OutputChars  int    `json:"outputChars"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	TotalTokens  int    `json:"totalTokens"`
	UsageSource  string `json:"usageSource"`
	Status       string `json:"status"`
	DurationMS   int64  `json:"durationMs"`
	ErrorCode    string `json:"errorCode"`
}

type UsageListFilter struct {
	FromUTC   *time.Time
	ToUTC     *time.Time
	Flow      string
	Operation string
	Mode      string
	Limit     int
	Offset    int
}

type UsageMetricsReader interface {
	ListUsageEvents(ctx context.Context, filter UsageListFilter) ([]UsageEventRow, error)
	ListUsageDailyAggregates(ctx context.Context, filter UsageListFilter) ([]UsageDailyAggregate, error)
}

type UsageMetricsWriter interface {
	RecordUsageEvent(ctx context.Context, event UsageEvent) error
	UpsertIndexDocumentMetrics(ctx context.Context, metric UsageDocumentMetrics) error
}

type usageScopeKey struct{}
type usageEmitterKey struct{}

type usageEmitter func(context.Context, UsageEvent)

func WithUsageScope(ctx context.Context, scope UsageScope) context.Context {
	return context.WithValue(ctx, usageScopeKey{}, scope)
}

func WithUsageDocumentID(ctx context.Context, documentID string) context.Context {
	scope := usageScopeFromContext(ctx)
	scope.DocumentID = strings.TrimSpace(documentID)
	return WithUsageScope(ctx, scope)
}

func usageScopeFromContext(ctx context.Context) UsageScope {
	scope, _ := ctx.Value(usageScopeKey{}).(UsageScope)
	return scope
}

func WithUsageEmitter(ctx context.Context, emit func(context.Context, UsageEvent)) context.Context {
	if emit == nil {
		return ctx
	}
	return context.WithValue(ctx, usageEmitterKey{}, usageEmitter(emit))
}

func emitUsageEvent(ctx context.Context, event UsageEvent) {
	emit, _ := ctx.Value(usageEmitterKey{}).(usageEmitter)
	if emit == nil {
		return
	}
	scope := usageScopeFromContext(ctx)
	if strings.TrimSpace(event.Flow) == "" {
		event.Flow = scope.Flow
	}
	if strings.TrimSpace(event.Mode) == "" {
		event.Mode = scope.Mode
	}
	if strings.TrimSpace(event.RunID) == "" {
		event.RunID = scope.RunID
	}
	if strings.TrimSpace(event.RequestID) == "" {
		event.RequestID = scope.RequestID
	}
	if strings.TrimSpace(event.DocumentID) == "" {
		event.DocumentID = scope.DocumentID
	}
	if event.CreatedAtUTC.IsZero() {
		event.CreatedAtUTC = time.Now().UTC()
	}
	if strings.TrimSpace(event.UsageSource) == "" {
		event.UsageSource = "unknown"
	}
	if strings.TrimSpace(event.Status) == "" {
		event.Status = "success"
	}
	emit(ctx, event)
}

func ParseUsageFromResponseBody(raw []byte) (UsageEvent, error) {
	var payload struct {
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return UsageEvent{}, err
	}
	if len(payload.Usage) == 0 {
		return UsageEvent{UsageSource: "unknown"}, nil
	}
	inputTokens := intFromAny(payload.Usage["prompt_tokens"])
	if inputTokens == 0 {
		inputTokens = intFromAny(payload.Usage["input_tokens"])
	}
	outputTokens := intFromAny(payload.Usage["completion_tokens"])
	if outputTokens == 0 {
		outputTokens = intFromAny(payload.Usage["output_tokens"])
	}
	totalTokens := intFromAny(payload.Usage["total_tokens"])
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}
	source := "provider"
	if inputTokens == 0 && outputTokens == 0 && totalTokens == 0 {
		source = "unknown"
	}
	return UsageEvent{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
		UsageSource:  source,
	}, nil
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func generateUsageEventID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	return "evt_" + hex.EncodeToString(buf)
}

func sanitizeUsageEvent(event UsageEvent) (UsageEvent, error) {
	normalized := event
	if normalized.CreatedAtUTC.IsZero() {
		normalized.CreatedAtUTC = time.Now().UTC()
	}
	normalized.Flow = strings.TrimSpace(normalized.Flow)
	normalized.Operation = strings.TrimSpace(normalized.Operation)
	normalized.Mode = strings.TrimSpace(normalized.Mode)
	normalized.ProviderName = strings.TrimSpace(normalized.ProviderName)
	normalized.ModelName = strings.TrimSpace(normalized.ModelName)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.RequestID = strings.TrimSpace(normalized.RequestID)
	normalized.DocumentID = strings.TrimSpace(normalized.DocumentID)
	normalized.SourceLang = strings.TrimSpace(normalized.SourceLang)
	normalized.TargetLang = strings.TrimSpace(normalized.TargetLang)
	normalized.UsageSource = strings.TrimSpace(normalized.UsageSource)
	normalized.Status = strings.TrimSpace(normalized.Status)
	normalized.ErrorCode = strings.TrimSpace(normalized.ErrorCode)
	if normalized.EventID == "" {
		normalized.EventID = generateUsageEventID()
	}
	if normalized.Flow == "" || normalized.Operation == "" || normalized.Mode == "" {
		return UsageEvent{}, errors.New("usage event requires flow, operation and mode")
	}
	if normalized.ProviderName == "" {
		normalized.ProviderName = "unknown"
	}
	if normalized.ModelName == "" {
		normalized.ModelName = "unknown"
	}
	if normalized.Status == "" {
		normalized.Status = "success"
	}
	if normalized.UsageSource == "" {
		normalized.UsageSource = "unknown"
	}
	if normalized.InputChars < 0 {
		normalized.InputChars = 0
	}
	if normalized.OutputChars < 0 {
		normalized.OutputChars = 0
	}
	if normalized.InputTokens < 0 {
		normalized.InputTokens = 0
	}
	if normalized.OutputTokens < 0 {
		normalized.OutputTokens = 0
	}
	if normalized.TotalTokens < 0 {
		normalized.TotalTokens = 0
	}
	if normalized.TotalTokens == 0 {
		normalized.TotalTokens = normalized.InputTokens + normalized.OutputTokens
	}
	if normalized.DurationMS < 0 {
		normalized.DurationMS = 0
	}
	return normalized, nil
}

func (s *PostgresVectorStore) initUsageSchema(ctx context.Context) error {
	createEvents := `
CREATE TABLE IF NOT EXISTS ai_usage_events (
	event_id TEXT PRIMARY KEY,
	created_at TIMESTAMPTZ NOT NULL,
	flow TEXT NOT NULL,
	operation TEXT NOT NULL,
	mode TEXT NOT NULL,
	provider_name TEXT NOT NULL,
	model_name TEXT NOT NULL,
	run_id TEXT NOT NULL DEFAULT '',
	request_id TEXT NOT NULL DEFAULT '',
	document_id TEXT NOT NULL DEFAULT '',
	source_lang TEXT NOT NULL DEFAULT '',
	target_lang TEXT NOT NULL DEFAULT '',
	input_chars INT NOT NULL DEFAULT 0,
	output_chars INT NOT NULL DEFAULT 0,
	input_tokens INT NOT NULL DEFAULT 0,
	output_tokens INT NOT NULL DEFAULT 0,
	total_tokens INT NOT NULL DEFAULT 0,
	usage_source TEXT NOT NULL DEFAULT 'unknown',
	status TEXT NOT NULL,
	duration_ms BIGINT NOT NULL DEFAULT 0,
	error_code TEXT NOT NULL DEFAULT ''
)`
	if _, err := s.db.ExecContext(ctx, createEvents); err != nil {
		return fmt.Errorf("create ai_usage_events table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS ai_usage_events_created_at_idx ON ai_usage_events (created_at DESC)`); err != nil {
		return fmt.Errorf("create ai_usage_events created_at index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS ai_usage_events_flow_op_idx ON ai_usage_events (flow, operation, mode, created_at DESC)`); err != nil {
		return fmt.Errorf("create ai_usage_events flow/op index: %w", err)
	}

	createDaily := `
CREATE TABLE IF NOT EXISTS ai_usage_daily_agg (
	day DATE NOT NULL,
	flow TEXT NOT NULL,
	operation TEXT NOT NULL,
	mode TEXT NOT NULL,
	model_name TEXT NOT NULL,
	provider_name TEXT NOT NULL,
	events_count BIGINT NOT NULL DEFAULT 0,
	success_count BIGINT NOT NULL DEFAULT 0,
	error_count BIGINT NOT NULL DEFAULT 0,
	input_chars_sum BIGINT NOT NULL DEFAULT 0,
	output_chars_sum BIGINT NOT NULL DEFAULT 0,
	input_tokens_sum BIGINT NOT NULL DEFAULT 0,
	output_tokens_sum BIGINT NOT NULL DEFAULT 0,
	total_tokens_sum BIGINT NOT NULL DEFAULT 0,
	duration_ms_sum BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (day, flow, operation, mode, model_name, provider_name)
)`
	if _, err := s.db.ExecContext(ctx, createDaily); err != nil {
		return fmt.Errorf("create ai_usage_daily_agg table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS ai_usage_daily_agg_day_idx ON ai_usage_daily_agg (day DESC)`); err != nil {
		return fmt.Errorf("create ai_usage_daily_agg day index: %w", err)
	}

	createDocMetrics := `
CREATE TABLE IF NOT EXISTS rag_index_document_metrics (
	run_id TEXT NOT NULL,
	document_id TEXT NOT NULL,
	source_lang TEXT NOT NULL DEFAULT '',
	source_content_chars INT NOT NULL DEFAULT 0,
	title_chars INT NOT NULL DEFAULT 0,
	translations_attempted INT NOT NULL DEFAULT 0,
	translations_succeeded INT NOT NULL DEFAULT 0,
	chunks_count INT NOT NULL DEFAULT 0,
	chunks_tokens_sum INT NOT NULL DEFAULT 0,
	embedding_calls INT NOT NULL DEFAULT 0,
	embedding_input_chars_sum INT NOT NULL DEFAULT 0,
	embedding_input_tokens_sum INT NOT NULL DEFAULT 0,
	embedding_total_tokens_sum INT NOT NULL DEFAULT 0,
	llm_input_tokens_sum INT NOT NULL DEFAULT 0,
	llm_output_tokens_sum INT NOT NULL DEFAULT 0,
	llm_total_tokens_sum INT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'unknown',
	indexed_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (run_id, document_id)
)`
	if _, err := s.db.ExecContext(ctx, createDocMetrics); err != nil {
		return fmt.Errorf("create rag_index_document_metrics table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS rag_index_document_metrics_indexed_at_idx ON rag_index_document_metrics (indexed_at DESC)`); err != nil {
		return fmt.Errorf("create rag_index_document_metrics indexed_at index: %w", err)
	}
	return nil
}

func (s *PostgresVectorStore) RecordUsageEvent(ctx context.Context, event UsageEvent) error {
	normalized, err := sanitizeUsageEvent(event)
	if err != nil {
		return err
	}
	insertEvent := `
INSERT INTO ai_usage_events (
	event_id, created_at, flow, operation, mode, provider_name, model_name,
	run_id, request_id, document_id, source_lang, target_lang,
	input_chars, output_chars, input_tokens, output_tokens, total_tokens,
	usage_source, status, duration_ms, error_code
) VALUES (
	$1, $2, $3, $4, $5, $6, $7,
	$8, $9, $10, $11, $12,
	$13, $14, $15, $16, $17,
	$18, $19, $20, $21
)`
	upsertDaily := `
INSERT INTO ai_usage_daily_agg (
	day, flow, operation, mode, model_name, provider_name,
	events_count, success_count, error_count,
	input_chars_sum, output_chars_sum, input_tokens_sum, output_tokens_sum, total_tokens_sum, duration_ms_sum
) VALUES (
	$1, $2, $3, $4, $5, $6,
	1, $7, $8,
	$9, $10, $11, $12, $13, $14
)
ON CONFLICT (day, flow, operation, mode, model_name, provider_name)
DO UPDATE SET
	events_count = ai_usage_daily_agg.events_count + 1,
	success_count = ai_usage_daily_agg.success_count + EXCLUDED.success_count,
	error_count = ai_usage_daily_agg.error_count + EXCLUDED.error_count,
	input_chars_sum = ai_usage_daily_agg.input_chars_sum + EXCLUDED.input_chars_sum,
	output_chars_sum = ai_usage_daily_agg.output_chars_sum + EXCLUDED.output_chars_sum,
	input_tokens_sum = ai_usage_daily_agg.input_tokens_sum + EXCLUDED.input_tokens_sum,
	output_tokens_sum = ai_usage_daily_agg.output_tokens_sum + EXCLUDED.output_tokens_sum,
	total_tokens_sum = ai_usage_daily_agg.total_tokens_sum + EXCLUDED.total_tokens_sum,
	duration_ms_sum = ai_usage_daily_agg.duration_ms_sum + EXCLUDED.duration_ms_sum
`
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin usage tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(
		ctx,
		insertEvent,
		normalized.EventID,
		normalized.CreatedAtUTC,
		normalized.Flow,
		normalized.Operation,
		normalized.Mode,
		normalized.ProviderName,
		normalized.ModelName,
		normalized.RunID,
		normalized.RequestID,
		normalized.DocumentID,
		normalized.SourceLang,
		normalized.TargetLang,
		normalized.InputChars,
		normalized.OutputChars,
		normalized.InputTokens,
		normalized.OutputTokens,
		normalized.TotalTokens,
		normalized.UsageSource,
		normalized.Status,
		normalized.DurationMS,
		normalized.ErrorCode,
	); err != nil {
		return fmt.Errorf("insert usage event: %w", err)
	}
	successCount := int64(0)
	errorCount := int64(0)
	if normalized.Status == "success" {
		successCount = 1
	} else {
		errorCount = 1
	}
	day := normalized.CreatedAtUTC.UTC().Format("2006-01-02")
	if _, err := tx.ExecContext(
		ctx,
		upsertDaily,
		day,
		normalized.Flow,
		normalized.Operation,
		normalized.Mode,
		normalized.ModelName,
		normalized.ProviderName,
		successCount,
		errorCount,
		normalized.InputChars,
		normalized.OutputChars,
		normalized.InputTokens,
		normalized.OutputTokens,
		normalized.TotalTokens,
		normalized.DurationMS,
	); err != nil {
		return fmt.Errorf("upsert daily usage: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit usage tx: %w", err)
	}
	return nil
}

func (s *PostgresVectorStore) UpsertIndexDocumentMetrics(ctx context.Context, metric UsageDocumentMetrics) error {
	runID := strings.TrimSpace(metric.RunID)
	documentID := strings.TrimSpace(metric.DocumentID)
	if runID == "" || documentID == "" {
		return errors.New("document metrics require run_id and document_id")
	}
	indexedAt := metric.IndexedAtUTC
	if indexedAt.IsZero() {
		indexedAt = time.Now().UTC()
	}
	status := strings.TrimSpace(metric.Status)
	if status == "" {
		status = "unknown"
	}
	query := `
INSERT INTO rag_index_document_metrics (
	run_id, document_id, source_lang, source_content_chars, title_chars,
	translations_attempted, translations_succeeded, chunks_count, chunks_tokens_sum,
	embedding_calls, embedding_input_chars_sum, embedding_input_tokens_sum, embedding_total_tokens_sum,
	llm_input_tokens_sum, llm_output_tokens_sum, llm_total_tokens_sum, status, indexed_at
) VALUES (
	$1, $2, $3, $4, $5,
	$6, $7, $8, $9,
	$10, $11, $12, $13,
	$14, $15, $16, $17, $18
)
ON CONFLICT (run_id, document_id)
DO UPDATE SET
	source_lang = EXCLUDED.source_lang,
	source_content_chars = EXCLUDED.source_content_chars,
	title_chars = EXCLUDED.title_chars,
	translations_attempted = EXCLUDED.translations_attempted,
	translations_succeeded = EXCLUDED.translations_succeeded,
	chunks_count = EXCLUDED.chunks_count,
	chunks_tokens_sum = EXCLUDED.chunks_tokens_sum,
	embedding_calls = EXCLUDED.embedding_calls,
	embedding_input_chars_sum = EXCLUDED.embedding_input_chars_sum,
	embedding_input_tokens_sum = EXCLUDED.embedding_input_tokens_sum,
	embedding_total_tokens_sum = EXCLUDED.embedding_total_tokens_sum,
	llm_input_tokens_sum = EXCLUDED.llm_input_tokens_sum,
	llm_output_tokens_sum = EXCLUDED.llm_output_tokens_sum,
	llm_total_tokens_sum = EXCLUDED.llm_total_tokens_sum,
	status = EXCLUDED.status,
	indexed_at = EXCLUDED.indexed_at
`
	_, err := s.db.ExecContext(
		ctx,
		query,
		runID,
		documentID,
		strings.TrimSpace(metric.SourceLang),
		maxInt(metric.SourceContentChars, 0),
		maxInt(metric.TitleChars, 0),
		maxInt(metric.TranslationsAttempted, 0),
		maxInt(metric.TranslationsSucceeded, 0),
		maxInt(metric.ChunksCount, 0),
		maxInt(metric.ChunksTokensSum, 0),
		maxInt(metric.EmbeddingCalls, 0),
		maxInt(metric.EmbeddingInputCharsSum, 0),
		maxInt(metric.EmbeddingInputTokensSum, 0),
		maxInt(metric.EmbeddingTotalTokensSum, 0),
		maxInt(metric.LLMInputTokensSum, 0),
		maxInt(metric.LLMOutputTokensSum, 0),
		maxInt(metric.LLMTotalTokensSum, 0),
		status,
		indexedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert index document metrics: %w", err)
	}
	return nil
}

func (s *PostgresVectorStore) ListUsageEvents(ctx context.Context, filter UsageListFilter) ([]UsageEventRow, error) {
	limit, offset := normalizeUsagePagination(filter)
	conditions := []string{"1=1"}
	args := []any{}
	push := func(sqlExpr string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s $%d", sqlExpr, len(args)))
	}
	if filter.FromUTC != nil {
		push("created_at >=", *filter.FromUTC)
	}
	if filter.ToUTC != nil {
		push("created_at <=", *filter.ToUTC)
	}
	if strings.TrimSpace(filter.Flow) != "" {
		push("flow =", strings.TrimSpace(filter.Flow))
	}
	if strings.TrimSpace(filter.Operation) != "" {
		push("operation =", strings.TrimSpace(filter.Operation))
	}
	if strings.TrimSpace(filter.Mode) != "" {
		push("mode =", strings.TrimSpace(filter.Mode))
	}
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
SELECT event_id, created_at, flow, operation, mode, provider_name, model_name,
	run_id, request_id, document_id, source_lang, target_lang,
	input_chars, output_chars, input_tokens, output_tokens, total_tokens,
	usage_source, status, duration_ms, error_code
FROM ai_usage_events
WHERE %s
ORDER BY created_at DESC, event_id DESC
LIMIT $%d OFFSET $%d
`, strings.Join(conditions, " AND "), len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list usage events: %w", err)
	}
	defer rows.Close()
	// Keep allocation cap internal and bounded to avoid user-driven pre-allocation sizes.
	out := make([]UsageEventRow, 0, usageListInitialAllocCap)
	for rows.Next() {
		var item UsageEventRow
		var createdAt time.Time
		if scanErr := rows.Scan(
			&item.EventID,
			&createdAt,
			&item.Flow,
			&item.Operation,
			&item.Mode,
			&item.ProviderName,
			&item.ModelName,
			&item.RunID,
			&item.RequestID,
			&item.DocumentID,
			&item.SourceLang,
			&item.TargetLang,
			&item.InputChars,
			&item.OutputChars,
			&item.InputTokens,
			&item.OutputTokens,
			&item.TotalTokens,
			&item.UsageSource,
			&item.Status,
			&item.DurationMS,
			&item.ErrorCode,
		); scanErr != nil {
			return nil, fmt.Errorf("scan usage event: %w", scanErr)
		}
		item.CreatedAtUTC = createdAt.UTC().Format(time.RFC3339)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage events: %w", err)
	}
	return out, nil
}

func (s *PostgresVectorStore) ListUsageDailyAggregates(ctx context.Context, filter UsageListFilter) ([]UsageDailyAggregate, error) {
	limit, offset := normalizeUsagePagination(filter)
	conditions := []string{"1=1"}
	args := []any{}
	push := func(sqlExpr string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s $%d", sqlExpr, len(args)))
	}
	if filter.FromUTC != nil {
		push("day >=", filter.FromUTC.UTC().Format("2006-01-02"))
	}
	if filter.ToUTC != nil {
		push("day <=", filter.ToUTC.UTC().Format("2006-01-02"))
	}
	if strings.TrimSpace(filter.Flow) != "" {
		push("flow =", strings.TrimSpace(filter.Flow))
	}
	if strings.TrimSpace(filter.Operation) != "" {
		push("operation =", strings.TrimSpace(filter.Operation))
	}
	if strings.TrimSpace(filter.Mode) != "" {
		push("mode =", strings.TrimSpace(filter.Mode))
	}
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
SELECT day, flow, operation, mode, model_name, provider_name,
	events_count, success_count, error_count,
	input_chars_sum, output_chars_sum, input_tokens_sum, output_tokens_sum, total_tokens_sum, duration_ms_sum
FROM ai_usage_daily_agg
WHERE %s
ORDER BY day DESC, flow, operation, mode
LIMIT $%d OFFSET $%d
`, strings.Join(conditions, " AND "), len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list daily usage aggregates: %w", err)
	}
	defer rows.Close()
	// Keep allocation cap internal and bounded to avoid user-driven pre-allocation sizes.
	out := make([]UsageDailyAggregate, 0, usageListInitialAllocCap)
	for rows.Next() {
		var item UsageDailyAggregate
		var day time.Time
		if scanErr := rows.Scan(
			&day,
			&item.Flow,
			&item.Operation,
			&item.Mode,
			&item.ModelName,
			&item.ProviderName,
			&item.EventsCount,
			&item.SuccessCount,
			&item.ErrorCount,
			&item.InputCharsSum,
			&item.OutputCharsSum,
			&item.InputTokensSum,
			&item.OutputTokensSum,
			&item.TotalTokensSum,
			&item.DurationMSSum,
		); scanErr != nil {
			return nil, fmt.Errorf("scan daily usage aggregate: %w", scanErr)
		}
		item.Day = day.UTC().Format("2006-01-02")
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily usage aggregates: %w", err)
	}
	return out, nil
}

func normalizeUsagePagination(filter UsageListFilter) (int, int) {
	limit := filter.Limit
	if limit < UsageListMinLimit {
		limit = UsageListDefaultLimit
	}
	if limit > UsageListMaxLimit {
		limit = UsageListMaxLimit
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func maxInt(value int, minValue int) int {
	if value < minValue {
		return minValue
	}
	return value
}

func asSQLNullString(v string) sql.NullString {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}
