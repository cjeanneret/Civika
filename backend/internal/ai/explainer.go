package ai

import "context"

type ImpactExplainer interface {
    ExplainImpact(ctx context.Context, input ExplainInput) (ExplainOutput, error)
}
