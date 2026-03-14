export type HealthResponse = {
  status: string;
};

export type ApiError = {
  code: string;
  message: string;
  requestId?: string;
};

export type VotationListItem = {
  id: string;
  dateIso?: string;
  level?: string;
  canton?: string;
  communeCode?: string;
  communeName?: string;
  status?: string;
  language?: string;
  titles?: Record<string, string>;
  displayTitles?: Record<string, string>;
  translationStatus?: TranslationStatus;
  objectIds?: string[];
  sourceUrls?: string[];
};

export type VotationListResult = {
  items: VotationListItem[];
  limit: number;
  offset: number;
  total: number;
};

export type VotationDetail = {
  id: string;
  dateIso?: string;
  level?: string;
  canton?: string;
  communeCode?: string;
  communeName?: string;
  status?: string;
  language?: string;
  titles?: Record<string, string>;
  displayTitles?: Record<string, string>;
  translationStatus?: TranslationStatus;
  objectIds?: string[];
  sourceUrls?: string[];
};

export type TranslationStatus = {
  state: string;
  requestedLanguage?: string;
  fallbackLanguage?: string;
  message?: string;
};

export type Citation = {
  sourceType: string;
  url: string;
  title: string;
};

export type QAQueryInput = {
  question: string;
  language: string;
  context: {
    votationId: string;
    objectId: string;
    canton: string;
  };
  client: {
    instance: string;
    version: string;
  };
};

export type QAQueryOutput = {
  answer: string;
  language: string;
  citations: Citation[];
  meta: {
    confidence: number;
    usedDocuments: string[];
  };
};
