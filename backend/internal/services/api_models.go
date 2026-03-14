package services

import "time"

type VotationFilters struct {
	DateFrom *time.Time
	DateTo   *time.Time
	Level    string
	Canton   string
	Status   string
	Lang     string
	Limit    int
	Offset   int
}

type VotationListItem struct {
	ID            string            `json:"id"`
	DateISO       string            `json:"dateIso,omitempty"`
	Level         string            `json:"level,omitempty"`
	Canton        string            `json:"canton,omitempty"`
	CommuneCode   string            `json:"communeCode,omitempty"`
	CommuneName   string            `json:"communeName,omitempty"`
	Status        string            `json:"status,omitempty"`
	Language      string            `json:"language,omitempty"`
	Titles        map[string]string `json:"titles,omitempty"`
	DisplayTitles map[string]string `json:"displayTitles,omitempty"`
	Translation   *TranslationState `json:"translationStatus,omitempty"`
	ObjectIDs     []string          `json:"objectIds,omitempty"`
	SourceURLs    []string          `json:"sourceUrls,omitempty"`
}

type VotationListResult struct {
	Items  []VotationListItem `json:"items"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
	Total  int                `json:"total"`
}

type VotationDetail struct {
	ID            string            `json:"id"`
	DateISO       string            `json:"dateIso,omitempty"`
	Level         string            `json:"level,omitempty"`
	Canton        string            `json:"canton,omitempty"`
	CommuneCode   string            `json:"communeCode,omitempty"`
	CommuneName   string            `json:"communeName,omitempty"`
	Status        string            `json:"status,omitempty"`
	Language      string            `json:"language,omitempty"`
	Titles        map[string]string `json:"titles,omitempty"`
	DisplayTitles map[string]string `json:"displayTitles,omitempty"`
	Translation   *TranslationState `json:"translationStatus,omitempty"`
	ObjectIDs     []string          `json:"objectIds,omitempty"`
	SourceURLs    []string          `json:"sourceUrls,omitempty"`
}

type TranslationState struct {
	State             string `json:"state"`
	RequestedLanguage string `json:"requestedLanguage,omitempty"`
	FallbackLanguage  string `json:"fallbackLanguage,omitempty"`
	Message           string `json:"message,omitempty"`
}

type ObjectSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Type   string `json:"type,omitempty"`
	Theme  string `json:"theme,omitempty"`
	Slug   string `json:"slug,omitempty"`
	Status string `json:"status,omitempty"`
}

type ObjectDetail struct {
	ID            string            `json:"id"`
	VotationID    string            `json:"votationId,omitempty"`
	Language      string            `json:"language,omitempty"`
	Title         string            `json:"title"`
	Type          string            `json:"type,omitempty"`
	Theme         string            `json:"theme,omitempty"`
	Section       map[string]string `json:"sections,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	SourceSystems []string          `json:"sourceSystems,omitempty"`
}

type ObjectSource struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Origin string `json:"origin,omitempty"`
}

type Taxonomies struct {
	Levels      []string `json:"levels"`
	Cantons     []string `json:"cantons"`
	Statuses    []string `json:"statuses"`
	Languages   []string `json:"languages"`
	ObjectTypes []string `json:"objectTypes"`
	Themes      []string `json:"themes"`
}

type QAQueryInput struct {
	Question string         `json:"question"`
	Language string         `json:"language"`
	Context  QAQueryContext `json:"context"`
	Client   QAQueryClient  `json:"client"`
}

type QAQueryContext struct {
	VotationID string `json:"votationId"`
	ObjectID   string `json:"objectId"`
	Canton     string `json:"canton"`
}

type QAQueryClient struct {
	Instance string `json:"instance"`
	Version  string `json:"version"`
}

type Citation struct {
	SourceType string `json:"sourceType"`
	URL        string `json:"url"`
	Title      string `json:"title"`
}

type QAQueryOutput struct {
	Answer    string      `json:"answer"`
	Language  string      `json:"language"`
	Citations []Citation  `json:"citations"`
	Meta      QAQueryMeta `json:"meta"`
}

type QAQueryMeta struct {
	Confidence    float64  `json:"confidence"`
	UsedDocuments []string `json:"usedDocuments"`
}
