package ai

type ExplainInput struct {
	VotationID string
	QuestionID string
	Context    string
	Language   string
}

type ExplainOutput struct {
	Summary      string            `json:"summary"`
	Source       string            `json:"source"`
	Language     string            `json:"language"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Intervenants []Intervenant     `json:"intervenants,omitempty"`
}

type Intervenant struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      string `json:"role,omitempty"`
}
