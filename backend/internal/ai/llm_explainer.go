package ai

import (
	"context"
	"errors"
)

type LLMImpactExplainer struct {
	BaseURL        string
	APIKey         string
	ModelName      string
	MaxPromptChars int
}

func NewLLMImpactExplainer(baseURL, apiKey, modelName string, maxPromptChars int) *LLMImpactExplainer {
	return &LLMImpactExplainer{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		ModelName:      modelName,
		MaxPromptChars: maxPromptChars,
	}
}

func (l *LLMImpactExplainer) ExplainImpact(ctx context.Context, input ExplainInput) (ExplainOutput, error) {
	_ = ctx
	_ = input

	if l.BaseURL == "" || l.ModelName == "" {
		return ExplainOutput{}, errors.New("llm explainer not configured")
	}

	return ExplainOutput{}, errors.New("llm explainer not implemented")
}
