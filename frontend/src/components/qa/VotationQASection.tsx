"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ApiClientError, queryQA } from "@/lib/api";
import { QAComposer } from "@/components/qa/QAComposer";
import { QAHistoryItem, QAResponseStack } from "@/components/qa/QAResponseStack";
import type { LocaleCode } from "@/lib/i18n/config";
import type { Messages } from "@/lib/i18n/messages";
import type { QAQueryOutput } from "@/types/api";

type VotationQASectionProps = {
  votationId: string;
  locale: LocaleCode;
  messages: Messages;
};

function mapToHistoryItem(question: string, output: QAQueryOutput): QAHistoryItem {
  return {
    id: crypto.randomUUID(),
    question,
    answer: output.answer,
    createdAtIso: new Date().toISOString(),
    citations: output.citations ?? [],
  };
}

export function VotationQASection({ votationId, locale, messages }: VotationQASectionProps) {
  const [items, setItems] = useState<QAHistoryItem[]>([]);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [isLoadingAuto, setIsLoadingAuto] = useState(true);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const hasBootstrappedRef = useRef(false);

  const currentExpandedId = useMemo(() => {
    if (expandedId) {
      return expandedId;
    }
    return items.length > 0 ? items[items.length - 1].id : null;
  }, [expandedId, items]);

  const submitQuestion = useCallback(async (question: string) => {
    setErrorMessage(null);
    const output = await queryQA({
      question,
      language: locale,
      context: {
        votationId,
        objectId: "",
        canton: "",
      },
      client: {
        instance: "web-frontend",
        version: "0.1.0",
      },
    });

    const nextItem = mapToHistoryItem(question, output);
    setItems((previous) => [...previous, nextItem]);
    setExpandedId(nextItem.id);
  }, [locale, votationId]);

  useEffect(() => {
    if (hasBootstrappedRef.current) {
      return;
    }
    hasBootstrappedRef.current = true;
    const timer = setTimeout(() => {
      submitQuestion(messages.qaDefaultQuestion)
        .catch((error: unknown) => {
          if (error instanceof ApiClientError) {
            setErrorMessage(error.message);
            return;
          }
          setErrorMessage(messages.qaAutoLoadError);
        })
        .finally(() => setIsLoadingAuto(false));
    }, 0);
    return () => clearTimeout(timer);
  }, [messages.qaAutoLoadError, messages.qaDefaultQuestion, submitQuestion]);

  return (
    <section className="qa-section">
      <h2>{messages.qaTitle}</h2>
      <p className="muted">{messages.qaIntro}</p>

      <QAComposer
        messages={messages}
        disabled={isLoadingAuto}
        onSubmit={async (question) => {
          try {
            await submitQuestion(question);
          } catch (error: unknown) {
            if (error instanceof ApiClientError) {
              setErrorMessage(error.message);
              throw error;
            }
            setErrorMessage(messages.qaSubmitError);
            throw error;
          }
        }}
      />

      {isLoadingAuto ? <p className="muted">{messages.qaAutoLoading}</p> : null}
      {errorMessage ? <p className="error-text">{errorMessage}</p> : null}
      <QAResponseStack
        items={items}
        expandedId={currentExpandedId}
        locale={locale}
        messages={messages}
        onExpand={setExpandedId}
      />
    </section>
  );
}
