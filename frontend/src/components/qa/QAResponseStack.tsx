"use client";

import type { Citation } from "@/types/api";
import type { LocaleCode } from "@/lib/i18n/config";
import { toIntlLocale } from "@/lib/i18n/config";
import type { Messages } from "@/lib/i18n/messages";

export type QAHistoryItem = {
  id: string;
  question: string;
  answer: string;
  createdAtIso: string;
  citations: Citation[];
};

type QAResponseStackProps = {
  items: QAHistoryItem[];
  expandedId: string | null;
  locale: LocaleCode;
  messages: Messages;
  onExpand: (id: string) => void;
};

function buildSummary(answer: string): string {
  const normalized = answer.replace(/\s+/g, " ").trim();
  if (normalized.length <= 120) {
    return normalized;
  }
  return `${normalized.slice(0, 117)}...`;
}

function formatTime(iso: string, locale: LocaleCode): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return new Intl.DateTimeFormat(toIntlLocale(locale), {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

export function QAResponseStack({ items, expandedId, locale, messages, onExpand }: QAResponseStackProps) {
  if (items.length === 0) {
    return <p className="muted">{messages.qaNoAnswerYet}</p>;
  }

  return (
    <div className="qa-stack">
      {items.map((item) => {
        const isExpanded = expandedId === item.id;
        return (
          <section key={item.id} className={`qa-item ${isExpanded ? "expanded" : "collapsed"}`}>
            <button type="button" className="qa-item-header" onClick={() => onExpand(item.id)}>
              <span className="qa-item-time">{formatTime(item.createdAtIso, locale)}</span>
              <span className="qa-item-summary">{buildSummary(item.answer)}</span>
            </button>
            {isExpanded ? (
              <div className="qa-item-content">
                <p className="qa-question">
                  <strong>{messages.qaQuestionPrefix}:</strong> {item.question}
                </p>
                <p className="qa-answer">{item.answer}</p>
                {item.citations.length > 0 ? (
                  <ul className="qa-citations">
                    {item.citations.map((citation, index) => (
                      <li key={`${item.id}-citation-${index}`}>
                        <a href={citation.url} target="_blank" rel="noreferrer">
                          {citation.title || citation.url}
                        </a>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </div>
            ) : null}
          </section>
        );
      })}
    </div>
  );
}
