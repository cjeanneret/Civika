package rag

import (
	"math"
	"sort"
)

type ChunkTokenStats struct {
	Min int
	Avg int
	P95 int
	Max int
}

type ChunkCountStat struct {
	Key   string
	Count int
}

type TopDocumentStat struct {
	DocumentID string
	Title      string
	SourcePath string
	ChunkCount int
}

type ChunkDebugStats struct {
	DocumentCount int
	ChunkCount    int
	Tokens        ChunkTokenStats
	ByLanguage    []ChunkCountStat
	BySource      []ChunkCountStat
	TopDocuments  []TopDocumentStat
}

func BuildChunkDebugStats(documents []Document, chunks []Chunk) ChunkDebugStats {
	stats := ChunkDebugStats{
		DocumentCount: len(documents),
		ChunkCount:    len(chunks),
		ByLanguage:    []ChunkCountStat{},
		BySource:      []ChunkCountStat{},
		TopDocuments:  []TopDocumentStat{},
	}
	if len(chunks) == 0 {
		return stats
	}

	tokens := make([]int, 0, len(chunks))
	byLanguage := make(map[string]int)
	bySource := make(map[string]int)
	topDocsMap := make(map[string]TopDocumentStat)
	tokenSum := 0

	for _, chunk := range chunks {
		tokens = append(tokens, chunk.TokenCount)
		tokenSum += chunk.TokenCount

		lang := chunk.Language
		if lang == "" {
			lang = "unknown"
		}
		byLanguage[lang]++

		source := chunk.Source.SourceSystem
		if source == "" {
			source = "unknown"
		}
		bySource[source]++

		docStat, ok := topDocsMap[chunk.DocumentID]
		if !ok {
			docStat = TopDocumentStat{
				DocumentID: chunk.DocumentID,
				Title:      chunk.Title,
				SourcePath: chunk.SourcePath,
				ChunkCount: 0,
			}
		}
		docStat.ChunkCount++
		topDocsMap[chunk.DocumentID] = docStat
	}

	sort.Ints(tokens)
	stats.Tokens.Min = tokens[0]
	stats.Tokens.Max = tokens[len(tokens)-1]
	stats.Tokens.Avg = int(math.Round(float64(tokenSum) / float64(len(tokens))))
	stats.Tokens.P95 = percentile95(tokens)
	stats.ByLanguage = toSortedCountStats(byLanguage)
	stats.BySource = toSortedCountStats(bySource)
	stats.TopDocuments = toSortedTopDocumentStats(topDocsMap)
	return stats
}

func percentile95(sortedAsc []int) int {
	if len(sortedAsc) == 0 {
		return 0
	}
	// Nearest-rank percentile with 1-indexed rank.
	rank := int(math.Ceil(0.95 * float64(len(sortedAsc))))
	if rank < 1 {
		rank = 1
	}
	index := rank - 1
	if index >= len(sortedAsc) {
		index = len(sortedAsc) - 1
	}
	return sortedAsc[index]
}

func toSortedCountStats(counts map[string]int) []ChunkCountStat {
	out := make([]ChunkCountStat, 0, len(counts))
	for key, count := range counts {
		out = append(out, ChunkCountStat{Key: key, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Key < out[j].Key
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func toSortedTopDocumentStats(m map[string]TopDocumentStat) []TopDocumentStat {
	out := make([]TopDocumentStat, 0, len(m))
	for _, value := range m {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ChunkCount == out[j].ChunkCount {
			return out[i].DocumentID < out[j].DocumentID
		}
		return out[i].ChunkCount > out[j].ChunkCount
	})
	return out
}
