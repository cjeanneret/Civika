package ai

import "context"

type MockImpactExplainer struct{}

func NewMockImpactExplainer() *MockImpactExplainer {
	return &MockImpactExplainer{}
}

func (m *MockImpactExplainer) ExplainImpact(ctx context.Context, input ExplainInput) (ExplainOutput, error) {
	_ = ctx
	_ = input
	return ExplainOutput{
		Summary: "Scenario deterministe placeholder.",
		Source:  "mock",
	}, nil
}
