package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCachePathsStable(t *testing.T) {
	dir := t.TempDir()
	url := "https://api.openparldata.ch/v1/votings?limit=5&offset=0"
	meta1, body1 := cachePaths(dir, url)
	meta2, body2 := cachePaths(dir, url)
	if meta1 != meta2 || body1 != body2 {
		t.Fatalf("cache paths must be stable for same URL")
	}
	if filepath.Ext(meta1) != ".json" {
		t.Fatalf("expected .json meta file, got %s", meta1)
	}
}

func TestWriteAndReadCache(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{
		cacheDir:     dir,
		cacheTTL:     10 * time.Minute,
		forceRefresh: false,
	}
	url := "https://api.openparldata.ch/v1/votings/1/affairs"
	body := []byte(`{"ok":true}`)

	if err := f.writeCache(url, "application/json", body); err != nil {
		t.Fatalf("writeCache failed: %v", err)
	}
	gotBody, gotType, ok, err := f.readCache(url)
	if err != nil {
		t.Fatalf("readCache failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected cache hit")
	}
	if string(gotBody) != string(body) {
		t.Fatalf("unexpected body: %s", string(gotBody))
	}
	if gotType != "application/json" {
		t.Fatalf("unexpected content-type: %s", gotType)
	}
}

func TestReadCacheExpired(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{
		cacheDir:     dir,
		cacheTTL:     time.Minute,
		forceRefresh: false,
	}
	url := "https://api.openparldata.ch/v1/votings/1"
	if err := f.writeCache(url, "application/json", []byte(`{"id":1}`)); err != nil {
		t.Fatalf("writeCache failed: %v", err)
	}
	metaPath, _ := cachePaths(dir, url)
	metaRaw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta failed: %v", err)
	}
	var meta cacheMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatalf("unmarshal meta failed: %v", err)
	}
	meta.FetchedAt = time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	updated, _ := json.Marshal(meta)
	if err := os.WriteFile(metaPath, updated, 0o644); err != nil {
		t.Fatalf("rewrite meta failed: %v", err)
	}
	_, _, ok, err := f.readCache(url)
	if err != nil {
		t.Fatalf("readCache failed: %v", err)
	}
	if ok {
		t.Fatalf("expected cache miss due to TTL expiry")
	}
}

func TestExtractChildURLsIncludesNextPageAndLinks(t *testing.T) {
	payload := []byte(`{
		"meta": {"next_page": "https://api.openparldata.ch/v1/votings?page=2"},
		"data": [{
			"links": {
				"affairs": "https://api.openparldata.ch/v1/votings/1/affairs",
				"votes": "https://api.openparldata.ch/v1/votings/1/votes"
			}
		}]
	}`)
	urls := extractChildURLs(payload)
	if len(urls) < 3 {
		t.Fatalf("expected at least 3 child URLs, got %d", len(urls))
	}
}

func TestBodyFromCacheOrFetchUsesInMemoryCacheFirst(t *testing.T) {
	cached := map[string][]byte{
		"https://api.openparldata.ch/v1/votings/1/affairs": []byte(`{"ok":1}`),
	}
	body, err := bodyFromCacheOrFetch(context.Background(), &fetcher{}, cached, "https://api.openparldata.ch/v1/votings/1/affairs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"ok":1}` {
		t.Fatalf("unexpected cached body: %s", string(body))
	}
}

func TestDeriveBodyLocationHintUsesMatchingBodyKey(t *testing.T) {
	crawled := map[string][]byte{
		"https://api.openparldata.ch/v1/votings/101451/bodies": []byte(`{
			"data": [
				{"body_key":"351","canton_key":"BE","name":{"de":"Bern","fr":"Berne"}}
			]
		}`),
	}
	hint := deriveBodyLocationHint(crawled, opVoting{ID: 101451, BodyKey: "351"})
	if hint.Canton != "BE" {
		t.Fatalf("expected canton BE, got %q", hint.Canton)
	}
	if hint.CommuneCode != "351" {
		t.Fatalf("expected commune code 351, got %q", hint.CommuneCode)
	}
	if hint.CommuneName != "Berne" {
		t.Fatalf("expected commune name Berne, got %q", hint.CommuneName)
	}
}

func TestDeriveBodyLocationHintIgnoresNonBodyPayloads(t *testing.T) {
	crawled := map[string][]byte{
		"https://api.openparldata.ch/v1/votings?limit=1": []byte(`{
			"data": [{"id": 101451, "title": {"de": "Dummy"}}]
		}`),
		"https://api.openparldata.ch/v1/votings/101451/bodies": []byte(`{
			"data": [
				{"body_key":"351","canton_key":"BE","name":{"de":"Bern","fr":"Berne"}}
			]
		}`),
	}
	hint := deriveBodyLocationHint(crawled, opVoting{ID: 101451, BodyKey: "351"})
	if hint.Canton != "BE" {
		t.Fatalf("expected canton BE, got %q", hint.Canton)
	}
	if hint.CommuneCode != "351" {
		t.Fatalf("expected commune code 351, got %q", hint.CommuneCode)
	}
	if hint.CommuneName != "Berne" {
		t.Fatalf("expected commune name Berne, got %q", hint.CommuneName)
	}
}

func TestDeriveBodyLocationHintPrefersRichMatchOverVotingBodyKey(t *testing.T) {
	crawled := map[string][]byte{
		"https://api.openparldata.ch/v1/votings/101451": []byte(`{
			"data": [{"body_key":"351"}]
		}`),
		"https://api.openparldata.ch/v1/votings/101451/bodies": []byte(`{
			"data": [
				{"body_key":"351","canton_key":"BE","name":{"de":"Bern","fr":"Berne"}}
			]
		}`),
	}
	hint := deriveBodyLocationHint(crawled, opVoting{ID: 101451, BodyKey: "351"})
	if hint.Canton != "BE" {
		t.Fatalf("expected canton BE from /bodies match, got %q", hint.Canton)
	}
	if hint.CommuneName != "Berne" {
		t.Fatalf("expected commune name Berne from /bodies match, got %q", hint.CommuneName)
	}
}

func TestDeriveBodyLocationHintPrefersBodiesEndpoint(t *testing.T) {
	crawled := map[string][]byte{
		"https://api.openparldata.ch/v1/votings/101451": []byte(`{
			"data": [
				{"body_key":"351","canton_key":"SR","name":{"de":"Stadtrat (SR)"}}
			]
		}`),
		"https://api.openparldata.ch/v1/votings/101451/bodies": []byte(`{
			"data": [
				{"body_key":"351","canton_key":"BE","name":{"de":"Bern","fr":"Berne"}}
			]
		}`),
	}
	hint := deriveBodyLocationHint(crawled, opVoting{ID: 101451, BodyKey: "351"})
	if hint.Canton != "BE" {
		t.Fatalf("expected canton BE from /bodies endpoint, got %q", hint.Canton)
	}
	if hint.CommuneName != "Berne" {
		t.Fatalf("expected commune name Berne from /bodies endpoint, got %q", hint.CommuneName)
	}
}

func TestCollectAvailableLanguagesIgnoresMeaningMaps(t *testing.T) {
	voting := opVoting{
		MeaningOfYes: map[string]string{"fr": "Nein"},
		MeaningOfNo:  map[string]string{"it": "No"},
		Title:        map[string]string{},
		AffairTitle:  map[string]string{},
	}
	affair := opAffair{
		Title: map[string]string{"de": "Volksschule Wankdorf"},
	}
	docs := []normalizedDoc{
		{Language: "de"},
	}

	langs := collectAvailableLanguages(voting, affair, docs, nil)
	if len(langs) != 1 || langs[0] != "de" {
		t.Fatalf("expected only de available, got %v", langs)
	}
}
