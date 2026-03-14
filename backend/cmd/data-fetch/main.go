package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout    = 20 * time.Second
	defaultMaxResponseB   = 1_500_000
	defaultRequestAgent   = "civika-data-fetch/0.1"
	defaultRawDir         = "data/raw"
	defaultNormalizedDir  = "data/normalized"
	defaultCacheDir       = "data/fetch-cache/openparldata"
	defaultVotingsLimit   = 5
	defaultMaxDepth       = 3
	defaultMaxNodes       = 150
	defaultMinInterval    = 1 * time.Second
	defaultMaxRetries     = 3
	defaultBackoffMax     = 30 * time.Second
	defaultCacheTTL       = 72 * time.Hour
	defaultCacheRetention = 168 * time.Hour
	openParlVotingsURL    = "https://api.openparldata.ch/v1/votings"
)

const (
	envDataFetchVotingsLimit    = "DATA_FETCH_VOTINGS_LIMIT"
	envDataFetchMaxDepth        = "DATA_FETCH_MAX_DEPTH"
	envDataFetchMaxNodesPerVote = "DATA_FETCH_MAX_NODES_PER_VOTING"
	envDataFetchMinReqInterval  = "DATA_FETCH_MIN_REQUEST_INTERVAL"
	envDataFetchMaxRetries      = "DATA_FETCH_MAX_RETRIES"
	envDataFetchBackoffMax      = "DATA_FETCH_BACKOFF_MAX"
	envDataFetchCacheDir        = "DATA_FETCH_CACHE_DIR"
	envDataFetchCacheTTL        = "DATA_FETCH_CACHE_TTL"
	envDataFetchCacheRetention  = "DATA_FETCH_CACHE_RETENTION"
	envDataFetchForceRefresh    = "DATA_FETCH_FORCE_REFRESH"

	allowedOpenParlAPIHost = "api.openparldata.ch"
)

type opListMeta struct {
	Self string `json:"self"`
}

type opLinkSet struct {
	Affairs string `json:"affairs"`
	Votes   string `json:"votes"`
	Meeting string `json:"meeting"`
	Bodies  string `json:"bodies"`
}

type opVoting struct {
	ID             int               `json:"id"`
	URLAPI         string            `json:"url_api"`
	BodyKey        string            `json:"body_key"`
	ExternalID     string            `json:"external_id"`
	Date           string            `json:"date"`
	Title          map[string]string `json:"title"`
	AffairID       int               `json:"affair_id"`
	AffairTitle    map[string]string `json:"affair_title"`
	MeaningOfYes   map[string]string `json:"meaning_of_yes"`
	MeaningOfNo    map[string]string `json:"meaning_of_no"`
	Links          opLinkSet         `json:"links"`
	SelectionCause string            `json:"-"`
}

type opVotingsResponse struct {
	Meta opListMeta `json:"meta"`
	Data []opVoting `json:"data"`
}

type opAffairLinks struct {
	Contributors string `json:"contributors"`
	Docs         string `json:"docs"`
	Texts        string `json:"texts"`
}

type opAffair struct {
	ID        int               `json:"id"`
	URLAPI    string            `json:"url_api"`
	External  string            `json:"external_id"`
	Title     map[string]string `json:"title"`
	TitleLong map[string]string `json:"title_long"`
	TypeName  map[string]string `json:"type_name"`
	StateName map[string]string `json:"state_name"`
	Links     opAffairLinks     `json:"links"`
}

type opAffairsResponse struct {
	Meta opListMeta `json:"meta"`
	Data []opAffair `json:"data"`
}

type opContributor struct {
	Type           string            `json:"type"`
	Firstname      string            `json:"firstname"`
	Lastname       string            `json:"lastname"`
	Fullname       string            `json:"fullname"`
	Role           map[string]string `json:"role"`
	RoleHarmonized map[string]string `json:"role_harmonized"`
	Party          map[string]string `json:"party"`
}

type opContributorsResponse struct {
	Meta opListMeta      `json:"meta"`
	Data []opContributor `json:"data"`
}

type opDoc struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Text     string `json:"text"`
	Date     string `json:"date"`
	Language string `json:"language"`
	Format   string `json:"format"`
}

type opDocsResponse struct {
	Meta opListMeta `json:"meta"`
	Data []opDoc    `json:"data"`
}

type opText struct {
	Type map[string]string `json:"type"`
	Text map[string]string `json:"text"`
}

type opTextsResponse struct {
	Meta opListMeta `json:"meta"`
	Data []opText   `json:"data"`
}

type normalizedInitiant struct {
	Fullname string `json:"fullname"`
	Party    string `json:"party,omitempty"`
	Role     string `json:"role,omitempty"`
}

type normalizedDoc struct {
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	Date     string `json:"date,omitempty"`
	Language string `json:"language,omitempty"`
	Format   string `json:"format,omitempty"`
	Text     string `json:"text,omitempty"`
}

type normalizedText struct {
	Type          string            `json:"type,omitempty"`
	Text          string            `json:"text,omitempty"`
	TypeLocalized map[string]string `json:"type_localized,omitempty"`
	TextLocalized map[string]string `json:"text_localized,omitempty"`
}

type normalizedFixture struct {
	SourceSystem       string               `json:"source_system"`
	SourceOrg          string               `json:"source_org"`
	FetchedAtUTC       string               `json:"fetched_at_utc"`
	SelectionStrategy  string               `json:"selection_strategy"`
	AvailableLanguages []string             `json:"available_languages,omitempty"`
	DisplayTitle       map[string]string    `json:"display_title,omitempty"`
	CommuneCode        string               `json:"commune_code,omitempty"`
	CommuneName        string               `json:"commune_name,omitempty"`
	Voting             opVoting             `json:"voting"`
	Affair             opAffair             `json:"affair"`
	Initiants          []normalizedInitiant `json:"initiants"`
	Arguments          map[string]string    `json:"arguments"`
	Docs               []normalizedDoc      `json:"docs"`
	Texts              []normalizedText     `json:"texts"`
}

type fetchConfig struct {
	VotingsLimit       int
	MaxDepth           int
	MaxNodesPerVoting  int
	MinRequestInterval time.Duration
	MaxRetries         int
	BackoffMax         time.Duration
	CacheDir           string
	CacheTTL           time.Duration
	CacheRetention     time.Duration
	ForceRefresh       bool
}

type crawlNode struct {
	URL   string
	Depth int
	Name  string
}

type cacheMeta struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	SHA256      string `json:"sha256"`
	FetchedAt   string `json:"fetched_at_utc"`
}

type fetcher struct {
	client             *http.Client
	maxResponseBytes   int64
	minRequestInterval time.Duration
	maxRetries         int
	backoffMax         time.Duration
	rng                *rand.Rand
	lastRequestAt      time.Time
	cacheDir           string
	cacheTTL           time.Duration
	forceRefresh       bool
}

func main() {
	if err := run(); err != nil {
		log.Printf("data-fetch error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseFetchConfig(os.Args[1:])
	if err != nil {
		return err
	}
	repoRoot, err := detectRepoRoot()
	if err != nil {
		return err
	}

	rawRoot := filepath.Join(repoRoot, defaultRawDir)
	normRoot := filepath.Join(repoRoot, defaultNormalizedDir)
	cacheRoot := filepath.Join(repoRoot, cfg.CacheDir)

	if err := os.MkdirAll(rawRoot, 0o755); err != nil {
		return fmt.Errorf("create raw root: %w", err)
	}
	if err := os.MkdirAll(normRoot, 0o755); err != nil {
		return fmt.Errorf("create normalized root: %w", err)
	}
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return fmt.Errorf("create cache root: %w", err)
	}
	if err := cleanupExpiredCache(cacheRoot, cfg.CacheRetention); err != nil {
		return fmt.Errorf("cleanup cache: %w", err)
	}

	client := &http.Client{Timeout: defaultHTTPTimeout}
	f := &fetcher{
		client:             client,
		maxResponseBytes:   defaultMaxResponseB,
		minRequestInterval: cfg.MinRequestInterval,
		maxRetries:         cfg.MaxRetries,
		backoffMax:         cfg.BackoffMax,
		rng:                rand.New(rand.NewSource(time.Now().UnixNano())),
		cacheDir:           cacheRoot,
		cacheTTL:           cfg.CacheTTL,
		forceRefresh:       cfg.ForceRefresh,
	}
	ctx := context.Background()

	listURL := fmt.Sprintf("%s?limit=%d&offset=0", openParlVotingsURL, cfg.VotingsLimit)
	fmt.Printf("Fetch votings list: %s\n", listURL)
	votingsBody, _, err := f.fetchURL(ctx, listURL)
	if err != nil {
		return fmt.Errorf("fetch votings list: %w", err)
	}
	listFileName := fmt.Sprintf("votings-limit%d.json", cfg.VotingsLimit)
	if err := writeFile(filepath.Join(rawRoot, "openparldata", listFileName), votingsBody); err != nil {
		return err
	}

	var votings opVotingsResponse
	if err := json.Unmarshal(votingsBody, &votings); err != nil {
		return fmt.Errorf("decode votings list: %w", err)
	}
	if len(votings.Data) == 0 {
		return errors.New("openparldata votings list is empty")
	}

	selected, selectionStrategy := selectVotings(votings.Data, time.Now().UTC(), cfg.VotingsLimit)
	fmt.Printf("Selection strategy: %s (%d votings)\n", selectionStrategy, len(selected))

	for idx, voting := range selected {
		if err := processVoting(ctx, f, rawRoot, normRoot, idx+1, voting, selectionStrategy, cfg); err != nil {
			return fmt.Errorf("process voting %d: %w", voting.ID, err)
		}
	}

	fmt.Printf("Data fetch termine (raw=%s, normalized=%s, cache=%s)\n", rawRoot, normRoot, cacheRoot)
	return nil
}

func parseFetchConfig(args []string) (fetchConfig, error) {
	defaultCfg := fetchConfig{
		VotingsLimit:       envIntAny([]string{envDataFetchVotingsLimit}, defaultVotingsLimit),
		MaxDepth:           envIntAny([]string{envDataFetchMaxDepth}, defaultMaxDepth),
		MaxNodesPerVoting:  envIntAny([]string{envDataFetchMaxNodesPerVote}, defaultMaxNodes),
		MinRequestInterval: envDurationAny([]string{envDataFetchMinReqInterval}, defaultMinInterval),
		MaxRetries:         envIntAny([]string{envDataFetchMaxRetries}, defaultMaxRetries),
		BackoffMax:         envDurationAny([]string{envDataFetchBackoffMax}, defaultBackoffMax),
		CacheDir:           envString(envDataFetchCacheDir, defaultCacheDir),
		CacheTTL:           envDurationAny([]string{envDataFetchCacheTTL}, defaultCacheTTL),
		CacheRetention:     envDurationAny([]string{envDataFetchCacheRetention}, defaultCacheRetention),
		ForceRefresh:       envBool(envDataFetchForceRefresh, false),
	}

	fs := flag.NewFlagSet("data-fetch", flag.ContinueOnError)
	votingsLimit := fs.Int("votings-limit", defaultCfg.VotingsLimit, "number of votings to fetch from listing")
	maxDepth := fs.Int("max-depth", defaultCfg.MaxDepth, "max recursive depth for link crawling")
	maxNodes := fs.Int("max-nodes-per-voting", defaultCfg.MaxNodesPerVoting, "safety cap for crawled nodes per voting")
	minInterval := fs.Duration("min-request-interval", defaultCfg.MinRequestInterval, "minimum interval between HTTP requests")
	maxRetries := fs.Int("max-retries", defaultCfg.MaxRetries, "max retries for 429/5xx requests")
	backoffMax := fs.Duration("backoff-max", defaultCfg.BackoffMax, "maximum backoff delay for retries")
	cacheDir := fs.String("cache-dir", defaultCfg.CacheDir, "relative cache directory from repo root")
	cacheTTL := fs.Duration("cache-ttl", defaultCfg.CacheTTL, "duration before cached content is refreshed")
	cacheRetention := fs.Duration("cache-retention", defaultCfg.CacheRetention, "duration to retain cached files before cleanup")
	forceRefresh := fs.Bool("force-refresh", defaultCfg.ForceRefresh, "force network refresh and overwrite cache")
	if err := fs.Parse(args); err != nil {
		return fetchConfig{}, err
	}

	cfg := fetchConfig{
		VotingsLimit:       *votingsLimit,
		MaxDepth:           *maxDepth,
		MaxNodesPerVoting:  *maxNodes,
		MinRequestInterval: *minInterval,
		MaxRetries:         *maxRetries,
		BackoffMax:         *backoffMax,
		CacheDir:           strings.TrimSpace(*cacheDir),
		CacheTTL:           *cacheTTL,
		CacheRetention:     *cacheRetention,
		ForceRefresh:       *forceRefresh,
	}
	if cfg.VotingsLimit <= 0 {
		return fetchConfig{}, errors.New("votings-limit must be > 0")
	}
	if cfg.MaxDepth < 0 {
		return fetchConfig{}, errors.New("max-depth must be >= 0")
	}
	if cfg.MaxNodesPerVoting <= 0 {
		return fetchConfig{}, errors.New("max-nodes-per-voting must be > 0")
	}
	if cfg.MinRequestInterval < 0 {
		return fetchConfig{}, errors.New("min-request-interval must be >= 0")
	}
	if cfg.MaxRetries < 0 {
		return fetchConfig{}, errors.New("max-retries must be >= 0")
	}
	if cfg.BackoffMax <= 0 {
		return fetchConfig{}, errors.New("backoff-max must be > 0")
	}
	if cfg.CacheDir == "" {
		return fetchConfig{}, errors.New("cache-dir must not be empty")
	}
	if cfg.CacheTTL < 0 {
		return fetchConfig{}, errors.New("cache-ttl must be >= 0")
	}
	if cfg.CacheRetention <= 0 {
		return fetchConfig{}, errors.New("cache-retention must be > 0")
	}
	return cfg, nil
}

func envIntAny(keys []string, fallback int) int {
	for _, key := range keys {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err == nil {
			return value
		}
	}
	return fallback
}

func envDurationAny(keys []string, fallback time.Duration) time.Duration {
	for _, key := range keys {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		value, err := time.ParseDuration(raw)
		if err == nil {
			return value
		}
	}
	return fallback
}

func envString(key, fallback string) string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	return raw
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func detectRepoRoot() (string, error) {
	if forcedRoot := strings.TrimSpace(os.Getenv("CIVIKA_REPO_ROOT")); forcedRoot != "" {
		if directoryExists(forcedRoot) {
			return forcedRoot, nil
		}
		return "", fmt.Errorf("CIVIKA_REPO_ROOT does not exist: %s", forcedRoot)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}

	candidates := []string{
		cwd,
		filepath.Dir(cwd),
	}
	if executablePath, execErr := os.Executable(); execErr == nil {
		execDir := filepath.Dir(executablePath)
		candidates = append(candidates, execDir, filepath.Dir(execDir))
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if fileExists(filepath.Join(candidate, "backend", "go.mod")) {
			return candidate, nil
		}
	}
	if fileExists(filepath.Join(cwd, "go.mod")) && filepath.Base(cwd) == "backend" {
		return filepath.Dir(cwd), nil
	}
	if directoryExists(cwd) {
		return cwd, nil
	}
	return "", errors.New("cannot detect repository root and cwd fallback unavailable")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func cleanupExpiredCache(cacheDir string, retention time.Duration) error {
	if retention <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-retention)
	return filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
		return nil
	})
}

func (f *fetcher) fetchURL(ctx context.Context, rawURL string) ([]byte, string, error) {
	if f == nil || f.client == nil {
		return nil, "", errors.New("nil fetcher client")
	}

	url := canonicalURL(rawURL)
	if !f.forceRefresh {
		body, contentType, ok, err := f.readCache(url)
		if err != nil {
			return nil, "", err
		}
		if ok {
			fmt.Printf("  -> cache hit: %s\n", url)
			return body, contentType, nil
		}
	}

	var lastErr error
	for attempt := 0; attempt <= f.maxRetries; attempt++ {
		if err := f.waitForNextRequest(ctx); err != nil {
			return nil, "", err
		}
		body, contentType, retryAfter, err := f.fetchURLOnce(ctx, url)
		if err == nil {
			if cacheErr := f.writeCache(url, contentType, body); cacheErr != nil {
				return nil, "", cacheErr
			}
			return body, contentType, nil
		}
		lastErr = err
		if !isRetryableError(err) || attempt == f.maxRetries {
			break
		}
		delay := retryDelay(attempt, retryAfter, f.backoffMax)
		if err := sleepCtx(ctx, delay); err != nil {
			return nil, "", err
		}
	}
	return nil, "", lastErr
}

func (f *fetcher) readCache(url string) ([]byte, string, bool, error) {
	if strings.TrimSpace(f.cacheDir) == "" {
		return nil, "", false, nil
	}
	metaPath, bodyPath := cachePaths(f.cacheDir, url)
	metaRaw, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", false, nil
		}
		return nil, "", false, fmt.Errorf("read cache meta: %w", err)
	}
	var meta cacheMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return nil, "", false, fmt.Errorf("decode cache meta: %w", err)
	}
	t, err := time.Parse(time.RFC3339, meta.FetchedAt)
	if err != nil {
		return nil, "", false, nil
	}
	if f.cacheTTL > 0 && time.Since(t) > f.cacheTTL {
		return nil, "", false, nil
	}
	body, err := os.ReadFile(bodyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", false, nil
		}
		return nil, "", false, fmt.Errorf("read cache body: %w", err)
	}
	if !validateSHA(body, meta.SHA256) {
		return nil, "", false, nil
	}
	return body, meta.ContentType, true, nil
}

func (f *fetcher) writeCache(url string, contentType string, body []byte) error {
	if strings.TrimSpace(f.cacheDir) == "" {
		return nil
	}
	metaPath, bodyPath := cachePaths(f.cacheDir, url)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	sum := sha256.Sum256(body)
	meta := cacheMeta{
		URL:         url,
		ContentType: strings.TrimSpace(contentType),
		SHA256:      hex.EncodeToString(sum[:]),
		FetchedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	metaRaw, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encode cache meta: %w", err)
	}
	if err := os.WriteFile(bodyPath, body, 0o644); err != nil {
		return fmt.Errorf("write cache body: %w", err)
	}
	if err := os.WriteFile(metaPath, metaRaw, 0o644); err != nil {
		return fmt.Errorf("write cache meta: %w", err)
	}
	return nil
}

func cachePaths(cacheDir string, url string) (metaPath string, bodyPath string) {
	key := sha256.Sum256([]byte(canonicalURL(url)))
	hexKey := hex.EncodeToString(key[:])
	prefix := filepath.Join(cacheDir, hexKey[:2], hexKey)
	return prefix + ".meta.json", prefix + ".body"
}

func canonicalURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	parsed, err := neturl.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	parsed.Fragment = ""
	return parsed.String()
}

func validateSHA(body []byte, expectedHex string) bool {
	expected := strings.TrimSpace(expectedHex)
	if expected == "" {
		return false
	}
	sum := sha256.Sum256(body)
	return strings.EqualFold(expected, hex.EncodeToString(sum[:]))
}

func (f *fetcher) fetchURLOnce(ctx context.Context, url string) ([]byte, string, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("User-Agent", defaultRequestAgent)
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.1")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, "", retryAfter, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	reader := io.LimitReader(resp.Body, f.maxResponseBytes+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", 0, err
	}
	if int64(len(body)) > f.maxResponseBytes {
		return nil, "", 0, fmt.Errorf("response exceeds %d bytes", f.maxResponseBytes)
	}
	return body, resp.Header.Get("Content-Type"), 0, nil
}

func (f *fetcher) waitForNextRequest(ctx context.Context) error {
	if f.minRequestInterval <= 0 {
		f.lastRequestAt = time.Now()
		return nil
	}
	if f.rng == nil {
		f.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	now := time.Now()
	waitFor := f.minRequestInterval - now.Sub(f.lastRequestAt)
	if waitFor > 0 {
		jitter := time.Duration(f.rng.Intn(151)+100) * time.Millisecond
		if err := sleepCtx(ctx, waitFor+jitter); err != nil {
			return err
		}
	}
	f.lastRequestAt = time.Now()
	return nil
}

func retryDelay(attempt int, retryAfter, backoffMax time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > backoffMax {
			return backoffMax
		}
		return retryAfter
	}
	base := 1 * time.Second
	delay := base << attempt
	if delay > backoffMax {
		return backoffMax
	}
	return delay
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "HTTP status: 429") ||
		strings.Contains(msg, "HTTP status: 500") ||
		strings.Contains(msg, "HTTP status: 502") ||
		strings.Contains(msg, "HTTP status: 503") ||
		strings.Contains(msg, "HTTP status: 504")
}

func parseRetryAfter(raw string) time.Duration {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(s); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(s); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func selectVotings(votings []opVoting, now time.Time, maxCount int) ([]opVoting, string) {
	futures := make([]opVoting, 0, len(votings))
	for _, v := range votings {
		if isFutureVoting(v.Date, now) {
			v.SelectionCause = "future"
			futures = append(futures, v)
		}
	}
	if len(futures) > 0 {
		if len(futures) > maxCount {
			futures = futures[:maxCount]
		}
		return futures, "future"
	}

	fallback := make([]opVoting, 0, len(votings))
	for i, v := range votings {
		v.SelectionCause = "fallback_past"
		fallback = append(fallback, v)
		if i+1 >= maxCount {
			break
		}
	}
	return fallback, "fallback_past"
}

func processVoting(ctx context.Context, f *fetcher, rawRoot, normRoot string, rank int, voting opVoting, selectionStrategy string, cfg fetchConfig) error {
	baseDir := filepath.Join(rawRoot, "openparldata", fmt.Sprintf("%02d-voting-%d", rank, voting.ID))
	normalizedPath := filepath.Join(normRoot, "openparldata", fmt.Sprintf("%02d-voting-%d.json", rank, voting.ID))

	seeds := buildVotingSeedURLs(voting)
	crawledBodies, err := crawlVoting(ctx, f, baseDir, voting.ID, seeds, cfg)
	if err != nil {
		return err
	}

	affairsURL := strings.TrimSpace(voting.Links.Affairs)
	if affairsURL == "" {
		affairsURL = fmt.Sprintf("%s/%d/affairs", openParlVotingsURL, voting.ID)
	}
	affairsBody, err := bodyFromCacheOrFetch(ctx, f, crawledBodies, affairsURL)
	if err != nil {
		return fmt.Errorf("fetch affairs: %w", err)
	}
	if err := writeFile(filepath.Join(baseDir, "voting-affairs.json"), affairsBody); err != nil {
		return err
	}

	var affairs opAffairsResponse
	if err := json.Unmarshal(affairsBody, &affairs); err != nil {
		return fmt.Errorf("decode affairs: %w", err)
	}
	if len(affairs.Data) == 0 {
		return errors.New("no affair linked to voting")
	}
	affair := affairs.Data[0]

	contributorsURL := strings.TrimSpace(affair.Links.Contributors)
	if contributorsURL == "" {
		contributorsURL = fmt.Sprintf("https://api.openparldata.ch/v1/affairs/%d/contributors", affair.ID)
	}
	contribBody, err := bodyFromCacheOrFetch(ctx, f, crawledBodies, contributorsURL)
	if err != nil {
		return fmt.Errorf("fetch contributors: %w", err)
	}
	if err := writeFile(filepath.Join(baseDir, "affair-contributors.json"), contribBody); err != nil {
		return err
	}
	var contributors opContributorsResponse
	if err := json.Unmarshal(contribBody, &contributors); err != nil {
		return fmt.Errorf("decode contributors: %w", err)
	}

	docsURL := strings.TrimSpace(affair.Links.Docs)
	if docsURL == "" {
		docsURL = fmt.Sprintf("https://api.openparldata.ch/v1/affairs/%d/docs", affair.ID)
	}
	docsBody, err := bodyFromCacheOrFetch(ctx, f, crawledBodies, docsURL)
	if err != nil {
		return fmt.Errorf("fetch docs: %w", err)
	}
	if err := writeFile(filepath.Join(baseDir, "affair-docs.json"), docsBody); err != nil {
		return err
	}
	var docs opDocsResponse
	if err := json.Unmarshal(docsBody, &docs); err != nil {
		return fmt.Errorf("decode docs: %w", err)
	}

	textsURL := strings.TrimSpace(affair.Links.Texts)
	if textsURL == "" {
		textsURL = fmt.Sprintf("https://api.openparldata.ch/v1/affairs/%d/texts", affair.ID)
	}
	textsBody, err := bodyFromCacheOrFetch(ctx, f, crawledBodies, textsURL)
	if err != nil {
		return fmt.Errorf("fetch texts: %w", err)
	}
	if err := writeFile(filepath.Join(baseDir, "affair-texts.json"), textsBody); err != nil {
		return err
	}
	var texts opTextsResponse
	if err := json.Unmarshal(textsBody, &texts); err != nil {
		return fmt.Errorf("decode texts: %w", err)
	}

	normalized := buildNormalizedFixture(voting, affair, contributors.Data, docs.Data, texts.Data, selectionStrategy)
	normalizedJSON, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode normalized: %w", err)
	}
	if err := writeFile(normalizedPath, normalizedJSON); err != nil {
		return err
	}
	fmt.Printf("  -> raw: %s\n", baseDir)
	fmt.Printf("  -> normalized: %s\n", normalizedPath)
	return nil
}

func buildVotingSeedURLs(voting opVoting) []crawlNode {
	seeds := make([]crawlNode, 0, 5)
	push := func(rawURL, name string) {
		u := strings.TrimSpace(rawURL)
		if u == "" {
			return
		}
		seeds = append(seeds, crawlNode{URL: u, Depth: 0, Name: name})
	}
	push(voting.URLAPI, "voting")
	push(voting.Links.Affairs, "affairs")
	push(voting.Links.Votes, "votes")
	push(voting.Links.Meeting, "meeting")
	push(voting.Links.Bodies, "bodies")
	if strings.TrimSpace(voting.Links.Affairs) == "" {
		seeds = append(seeds, crawlNode{
			URL:   fmt.Sprintf("%s/%d/affairs", openParlVotingsURL, voting.ID),
			Depth: 0,
			Name:  "affairs-fallback",
		})
	}
	return seeds
}

func crawlVoting(ctx context.Context, f *fetcher, baseDir string, votingID int, seeds []crawlNode, cfg fetchConfig) (map[string][]byte, error) {
	queue := make([]crawlNode, 0, len(seeds))
	queue = append(queue, seeds...)
	seen := make(map[string]struct{})
	bodies := make(map[string][]byte)
	nodeCount := 0

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if node.Depth > cfg.MaxDepth {
			continue
		}
		if !isAllowedOpenParlAPIURL(node.URL) {
			continue
		}
		if _, ok := seen[node.URL]; ok {
			continue
		}
		seen[node.URL] = struct{}{}
		nodeCount++
		if nodeCount > cfg.MaxNodesPerVoting {
			break
		}

		body, _, err := f.fetchURL(ctx, node.URL)
		if err != nil {
			return nil, fmt.Errorf("fetch node depth=%d url=%s: %w", node.Depth, node.URL, err)
		}
		bodies[node.URL] = body
		rawPath := filepath.Join(baseDir, fmt.Sprintf("d%d-%03d-%s.json", node.Depth, nodeCount, sanitizeURLToName(node.Name, node.URL)))
		if err := writeFile(rawPath, body); err != nil {
			return nil, err
		}

		children := extractChildURLs(body)
		for _, child := range children {
			if !isAllowedOpenParlAPIURL(child) {
				continue
			}
			queue = append(queue, crawlNode{URL: child, Depth: node.Depth + 1, Name: "link"})
		}
	}
	fmt.Printf("  -> crawled voting=%d nodes=%d depth<=%d\n", votingID, nodeCount, cfg.MaxDepth)
	return bodies, nil
}

func bodyFromCacheOrFetch(ctx context.Context, f *fetcher, cache map[string][]byte, url string) ([]byte, error) {
	if body, ok := cache[url]; ok {
		return body, nil
	}
	if !isAllowedOpenParlAPIURL(url) {
		return nil, fmt.Errorf("URL outside allowed scope: %s", url)
	}
	body, _, err := f.fetchURL(ctx, url)
	if err != nil {
		return nil, err
	}
	cache[url] = body
	return body, nil
}

func extractChildURLs(body []byte) []string {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	var walk func(node any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			for key, value := range typed {
				if strings.EqualFold(key, "links") {
					appendStringURLs(value, seen, &out)
				}
				if strings.EqualFold(key, "meta") {
					if metaObj, ok := value.(map[string]any); ok {
						if nextPage, ok := metaObj["next_page"].(string); ok {
							appendURL(nextPage, seen, &out)
						}
					}
				}
				walk(value)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(payload)
	return out
}

func appendStringURLs(node any, seen map[string]struct{}, out *[]string) {
	switch typed := node.(type) {
	case map[string]any:
		for _, value := range typed {
			appendStringURLs(value, seen, out)
		}
	case []any:
		for _, item := range typed {
			appendStringURLs(item, seen, out)
		}
	case string:
		appendURL(typed, seen, out)
	}
}

func appendURL(candidate string, seen map[string]struct{}, out *[]string) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return
	}
	if _, ok := seen[candidate]; ok {
		return
	}
	seen[candidate] = struct{}{}
	*out = append(*out, candidate)
}

func isAllowedOpenParlAPIURL(raw string) bool {
	parsed, err := neturl.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}
	return strings.EqualFold(parsed.Host, allowedOpenParlAPIHost)
}

func sanitizeURLToName(nameHint, rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err == nil {
		pathPart := strings.Trim(parsed.Path, "/")
		pathPart = strings.ReplaceAll(pathPart, "/", "-")
		pathPart = strings.ReplaceAll(pathPart, ".", "-")
		pathPart = strings.ReplaceAll(pathPart, "_", "-")
		if pathPart != "" {
			nameHint = firstNonEmpty(nameHint, pathPart)
		}
	}
	value := strings.ToLower(strings.TrimSpace(nameHint))
	if value == "" {
		value = "node"
	}
	repl := strings.NewReplacer(" ", "-", "/", "-", ":", "-", "?", "-", "&", "-", "=", "-", ".", "-", "_", "-")
	value = repl.Replace(value)
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "-")
}

func writeFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func buildNormalizedFixture(voting opVoting, affair opAffair, contributors []opContributor, docs []opDoc, texts []opText, selectionStrategy string) normalizedFixture {
	initiants := selectInitiants(contributors)
	args := extractArguments(docs, texts)
	displayTitle := deriveDisplayTitleMap(voting, affair)
	communeCode := firstNonEmpty(
		strings.TrimSpace(lookupVotingString(voting, "commune_code")),
		strings.TrimSpace(lookupVotingString(voting, "municipality_code")),
		strings.TrimSpace(lookupVotingString(voting, "gemeinde_code")),
		strings.TrimSpace(lookupVotingString(voting, "comune_code")),
	)
	communeName := firstNonEmpty(
		strings.TrimSpace(lookupVotingString(voting, "commune_name")),
		strings.TrimSpace(lookupVotingString(voting, "municipality_name")),
		strings.TrimSpace(lookupVotingString(voting, "gemeinde_name")),
		strings.TrimSpace(lookupVotingString(voting, "comune_name")),
	)

	outDocs := make([]normalizedDoc, 0, len(docs))
	for _, d := range docs {
		outDocs = append(outDocs, normalizedDoc{
			Name:     strings.TrimSpace(d.Name),
			URL:      strings.TrimSpace(d.URL),
			Date:     strings.TrimSpace(d.Date),
			Language: strings.TrimSpace(d.Language),
			Format:   strings.TrimSpace(d.Format),
			Text:     compactText(d.Text, 4000),
		})
	}

	outTexts := make([]normalizedText, 0, len(texts))
	for _, t := range texts {
		outTexts = append(outTexts, normalizedText{
			Type:          localizedFirst(t.Type),
			Text:          compactText(localizedFirst(t.Text), 2000),
			TypeLocalized: cloneLocalizedMap(t.Type),
			TextLocalized: cloneLocalizedMap(t.Text),
		})
	}
	sort.Slice(outTexts, func(i, j int) bool { return outTexts[i].Type < outTexts[j].Type })
	availableLanguages := collectAvailableLanguages(voting, affair, outDocs, texts)

	return normalizedFixture{
		SourceSystem:       "openparldata",
		SourceOrg:          "OpenParlData.ch",
		FetchedAtUTC:       time.Now().UTC().Format(time.RFC3339),
		SelectionStrategy:  selectionStrategy,
		AvailableLanguages: availableLanguages,
		DisplayTitle:       displayTitle,
		CommuneCode:        communeCode,
		CommuneName:        communeName,
		Voting:             voting,
		Affair:             affair,
		Initiants:          initiants,
		Arguments:          args,
		Docs:               outDocs,
		Texts:              outTexts,
	}
}

func deriveDisplayTitleMap(voting opVoting, affair opAffair) map[string]string {
	out := map[string]string{}
	for _, lang := range []string{"fr", "de", "it", "rm", "en"} {
		value := strings.TrimSpace(voting.AffairTitle[lang])
		if value == "" {
			value = strings.TrimSpace(affair.Title[lang])
		}
		if value == "" {
			value = strings.TrimSpace(voting.Title[lang])
		}
		if value != "" {
			out[lang] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func lookupVotingString(voting opVoting, key string) string {
	raw, err := json.Marshal(voting)
	if err != nil {
		return ""
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	value, _ := payload[key].(string)
	return value
}

func collectAvailableLanguages(voting opVoting, affair opAffair, docs []normalizedDoc, texts []opText) []string {
	seen := map[string]struct{}{}
	var out []string
	push := func(raw string) {
		candidate := strings.ToLower(strings.TrimSpace(raw))
		switch candidate {
		case "de", "fr", "it", "rm", "en":
		default:
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	pushMap := func(values map[string]string) {
		for key, value := range values {
			if strings.TrimSpace(value) == "" {
				continue
			}
			push(key)
		}
	}
	pushMap(voting.Title)
	pushMap(voting.AffairTitle)
	pushMap(voting.MeaningOfYes)
	pushMap(voting.MeaningOfNo)
	pushMap(affair.Title)
	pushMap(affair.TitleLong)
	pushMap(affair.TypeName)
	pushMap(affair.StateName)
	for _, doc := range docs {
		push(doc.Language)
	}
	for _, text := range texts {
		for key, value := range text.Text {
			if strings.TrimSpace(value) == "" {
				continue
			}
			push(key)
		}
	}
	if len(out) == 0 {
		out = append(out, "fr")
	}
	sort.Strings(out)
	return out
}

func cloneLocalizedMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out[strings.TrimSpace(key)] = trimmed
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func selectInitiants(contributors []opContributor) []normalizedInitiant {
	filtered := make([]normalizedInitiant, 0)
	fallbackPersons := make([]normalizedInitiant, 0)
	for _, c := range contributors {
		fullname := strings.TrimSpace(c.Fullname)
		if fullname == "" {
			fullname = strings.TrimSpace(strings.TrimSpace(c.Firstname) + " " + strings.TrimSpace(c.Lastname))
		}
		if fullname == "" {
			continue
		}
		item := normalizedInitiant{
			Fullname: fullname,
			Party:    localizedFirst(c.Party),
			Role:     firstNonEmpty(localizedFirst(c.RoleHarmonized), localizedFirst(c.Role)),
		}
		if strings.EqualFold(strings.TrimSpace(c.Type), "person") {
			fallbackPersons = append(fallbackPersons, item)
		}
		if isInitiantRole(item.Role) {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	if len(fallbackPersons) > 0 {
		return fallbackPersons
	}
	return filtered
}

func extractArguments(docs []opDoc, texts []opText) map[string]string {
	proChunks := make([]string, 0)
	contraChunks := make([]string, 0)
	genericChunks := make([]string, 0)
	for _, d := range docs {
		chunk := compactText(d.Text, 1800)
		if chunk == "" {
			continue
		}
		switch scoreArgumentSide(chunk) {
		case "pro":
			proChunks = append(proChunks, chunk)
		case "contra":
			contraChunks = append(contraChunks, chunk)
		default:
			genericChunks = append(genericChunks, chunk)
		}
	}
	for _, t := range texts {
		chunk := compactText(localizedFirst(t.Text), 1200)
		if chunk == "" {
			continue
		}
		genericChunks = append(genericChunks, chunk)
	}
	if len(proChunks) == 0 && len(genericChunks) > 0 {
		proChunks = append(proChunks, genericChunks[0])
	}
	if len(contraChunks) == 0 && len(genericChunks) > 1 {
		contraChunks = append(contraChunks, genericChunks[1])
	}
	pro := "indisponible"
	if len(proChunks) > 0 {
		pro = compactText(strings.Join(proChunks, "\n\n"), 2400)
	}
	contra := "indisponible"
	if len(contraChunks) > 0 {
		contra = compactText(strings.Join(contraChunks, "\n\n"), 2400)
	}
	return map[string]string{"pro": pro, "contra": contra}
}

func scoreArgumentSide(input string) string {
	s := strings.ToLower(input)
	proTokens := []string{"begrundung", "fuer", "dafur", "pro", "soutien", "favorable", "unterstutzt"}
	contraTokens := []string{"ablehnung", "dagegen", "contra", "contre", "defavorable", "refus", "oppose"}
	proScore := 0
	for _, tok := range proTokens {
		if strings.Contains(s, tok) {
			proScore++
		}
	}
	contraScore := 0
	for _, tok := range contraTokens {
		if strings.Contains(s, tok) {
			contraScore++
		}
	}
	if proScore == 0 && contraScore == 0 {
		return "unknown"
	}
	if proScore >= contraScore {
		return "pro"
	}
	return "contra"
}

func isInitiantRole(role string) bool {
	s := strings.ToLower(strings.TrimSpace(role))
	if s == "" {
		return false
	}
	tokens := []string{"auteur", "urheber", "initiant", "motionnaire", "postulant"}
	for _, tok := range tokens {
		if strings.Contains(s, tok) {
			return true
		}
	}
	return false
}

func isFutureVoting(dateValue string, now time.Time) bool {
	t, err := parseISODate(dateValue)
	if err != nil {
		return false
	}
	return t.After(now)
}

func parseISODate(value string) (time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, errors.New("empty date")
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported date format %q", v)
}

func localizedFirst(input map[string]string) string {
	for _, key := range []string{"fr", "de", "it", "rm", "en"} {
		if val := strings.TrimSpace(input[key]); val != "" {
			return val
		}
	}
	for _, val := range input {
		val = strings.TrimSpace(val)
		if val != "" {
			return val
		}
	}
	return ""
}

func compactText(input string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	out := strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
	if len(out) <= maxLen {
		return out
	}
	return strings.TrimSpace(out[:maxLen]) + "..."
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
