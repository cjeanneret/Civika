import type {
  ApiError,
  HealthResponse,
  QAQueryInput,
  QAQueryOutput,
  VotationDetail,
  VotationListResult,
} from "@/types/api";
import type { LocaleCode } from "@/lib/i18n/config";

function resolveBaseUrl(): string {
  if (typeof window === "undefined") {
    return process.env.INTERNAL_API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
  }
  return process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
}
const DEFAULT_TIMEOUT_MS = 30_000;

type RequestOptions = {
  method?: "GET" | "POST";
  body?: string;
  timeoutMs?: number;
};

export class ApiClientError extends Error {
  code: string;

  constructor(message: string, code = "request_failed") {
    super(message);
    this.name = "ApiClientError";
    this.code = code;
  }
}

async function requestJSON<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const baseUrl = resolveBaseUrl();
  const timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const headers: Record<string, string> = {};
    if (options.body) {
      headers["Content-Type"] = "application/json";
    }

    const response = await fetch(`${baseUrl}${path}`, {
      method: options.method ?? "GET",
      body: options.body,
      cache: "no-store",
      headers,
      signal: controller.signal,
    });

    if (!response.ok) {
      const fallbackMessage = "Le service est indisponible pour le moment.";
      let apiError: ApiError | null = null;
      try {
        apiError = (await response.json()) as ApiError;
      } catch {
        apiError = null;
      }

      throw new ApiClientError(apiError?.message ?? fallbackMessage, apiError?.code ?? "http_error");
    }

    return (await response.json()) as T;
  } catch (error) {
    if (error instanceof ApiClientError) {
      throw error;
    }
    if (error instanceof Error && error.name === "AbortError") {
      throw new ApiClientError("Le service a pris trop de temps a repondre.", "timeout");
    }
    throw new ApiClientError("Le service est indisponible pour le moment.");
  } finally {
    clearTimeout(timeoutId);
  }
}

export async function getHealth(): Promise<HealthResponse> {
  try {
    return await requestJSON<HealthResponse>("/health");
  } catch {
    return { status: "unavailable" };
  }
}

export async function getVotations(limit = 20, offset = 0, lang: LocaleCode = "fr"): Promise<VotationListResult> {
  const query = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
    lang,
  });
  return requestJSON<VotationListResult>(`/api/v1/votations?${query.toString()}`);
}

export async function getVotationById(id: string, lang: LocaleCode = "fr"): Promise<VotationDetail> {
  const query = new URLSearchParams({ lang });
  return requestJSON<VotationDetail>(`/api/v1/votations/${encodeURIComponent(id)}?${query.toString()}`);
}

export async function queryQA(input: QAQueryInput): Promise<QAQueryOutput> {
  return requestJSON<QAQueryOutput>("/api/v1/qa/query", {
    method: "POST",
    body: JSON.stringify(input),
    timeoutMs: 120_000,
  });
}
