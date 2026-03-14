"use client";

import { FormEvent, useMemo, useState } from "react";
import type { Messages } from "@/lib/i18n/messages";

type QAComposerProps = {
  disabled?: boolean;
  maxLength?: number;
  messages: Messages;
  onSubmit: (question: string) => Promise<void>;
};

export function QAComposer({ disabled = false, maxLength = 2000, messages, onSubmit }: QAComposerProps) {
  const [question, setQuestion] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const canSubmit = useMemo(() => {
    const trimmed = question.trim();
    return !disabled && !isSubmitting && trimmed.length > 0 && trimmed.length <= maxLength;
  }, [disabled, isSubmitting, maxLength, question]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = question.trim();
    if (trimmed.length === 0) {
      setErrorMessage(messages.qaQuestionEmpty);
      return;
    }
    if (trimmed.length > maxLength) {
      setErrorMessage(messages.qaQuestionTooLong.replace("{max}", String(maxLength)));
      return;
    }

    setErrorMessage(null);
    setIsSubmitting(true);
    try {
      await onSubmit(trimmed);
      setQuestion("");
    } catch {
      setErrorMessage(messages.qaSubmitError);
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <form className="qa-composer" onSubmit={handleSubmit}>
      <label htmlFor="qa-question" className="qa-label">
        {messages.qaQuestionLabel}
      </label>
      <textarea
        id="qa-question"
        className="qa-textarea"
        rows={4}
        maxLength={maxLength}
        disabled={disabled || isSubmitting}
        value={question}
        onChange={(event) => setQuestion(event.target.value)}
        placeholder={messages.qaQuestionPlaceholder}
      />
      <div className="qa-composer-footer">
        <span className="muted">
          {question.trim().length}/{maxLength}
        </span>
        <button type="submit" className="btn" disabled={!canSubmit}>
          {isSubmitting ? messages.qaSending : messages.qaSend}
        </button>
      </div>
      {errorMessage ? <p className="error-text">{errorMessage}</p> : null}
    </form>
  );
}
