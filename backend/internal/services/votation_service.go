package services

import (
	"context"
)

type VotationService interface {
	ListVotations(ctx context.Context, filters VotationFilters) (VotationListResult, error)
	GetVotationByID(ctx context.Context, id string, lang string) (VotationDetail, error)
	ListObjectsByVotation(ctx context.Context, votationID string, lang string) ([]ObjectSummary, error)
	GetObjectByID(ctx context.Context, objectID string, lang string) (ObjectDetail, error)
	ListObjectSources(ctx context.Context, objectID string) ([]ObjectSource, error)
	GetTaxonomies(ctx context.Context) (Taxonomies, error)
}

type QueryService interface {
	Query(ctx context.Context, input QAQueryInput) (QAQueryOutput, error)
}

type QACacheMetricsReader interface {
	MetricsSnapshot() QACacheMetricsSnapshot
}
