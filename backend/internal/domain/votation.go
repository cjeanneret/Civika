package domain

type Votation struct {
	ID             string               `json:"id"`
	Title          string               `json:"title"`
	DateISO        string               `json:"dateIso"`
	Language       string               `json:"language"`
	Translations   []LocalizedText      `json:"translations,omitempty"`
	SourceMetadata SourceMetadata       `json:"sourceMetadata"`
	Intervenants   []IntervenantSummary `json:"intervenants,omitempty"`
	Questions      []Question           `json:"questions"`
}

type Question struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Options []Option `json:"options"`
}

type Option struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type Resultat struct {
	QuestionID string             `json:"questionId"`
	Values     map[string]float64 `json:"values"`
}

type ImpactScenario struct {
	VotationID      string               `json:"votationId"`
	Language        string               `json:"language"`
	Summary         string               `json:"summary"`
	Source          string               `json:"source"`
	SourceMetadata  SourceMetadata       `json:"sourceMetadata"`
	Intervenants    []IntervenantSummary `json:"intervenants,omitempty"`
	RelatedDocument string               `json:"relatedDocument,omitempty"`
}

type LocalizedText struct {
	Lang  string `json:"lang"`
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

type SourceMetadata struct {
	SourceSystem       string   `json:"sourceSystem"`
	SourceURI          string   `json:"sourceUri"`
	ExternalID         string   `json:"externalId"`
	SourceOrg          string   `json:"sourceOrg,omitempty"`
	LicenseURI         string   `json:"licenseUri,omitempty"`
	AvailableLanguages []string `json:"availableLanguages,omitempty"`
}

type IntervenantSummary struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Role      string `json:"role,omitempty"`
}
