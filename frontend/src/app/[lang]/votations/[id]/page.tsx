import Link from "next/link";
import { VotationQASection } from "@/components/qa/VotationQASection";
import { PendingTranslationRefresher } from "@/components/common/PendingTranslationRefresher";
import { LanguageSwitcher } from "@/components/common/LanguageSwitcher";
import { getVotationById } from "@/lib/api";
import { normalizeLocale } from "@/lib/i18n/config";
import { getMessages } from "@/lib/i18n/messages";
import { buildLocationLabel, formatDate, formatLevel, formatStatus, pickBestTitle } from "@/lib/votations";
import type { VotationDetail } from "@/types/api";

type LocalizedVotationPageProps = {
  params: Promise<{
    lang: string;
    id: string;
  }>;
};

export default async function LocalizedVotationPage({ params }: LocalizedVotationPageProps) {
  const { lang, id } = await params;
  const locale = normalizeLocale(lang);
  const messages = getMessages(locale);

  let votation: VotationDetail | null = null;
  let errorMessage: string | null = null;

  try {
    votation = await getVotationById(id, locale);
  } catch {
    errorMessage = messages.loadingVotationError;
  }

  if (errorMessage || !votation) {
    return (
      <main className="container">
        <LanguageSwitcher currentLocale={locale} pathWithoutLocale={`/votations/${encodeURIComponent(id)}`} />
        <Link href={`/${locale}`} className="back-link">
          {messages.backToList}
        </Link>
        <p className="error-text">{errorMessage ?? messages.votationNotFound}</p>
      </main>
    );
  }

  return (
    <main className="container">
      <PendingTranslationRefresher enabled={votation.translationStatus?.state === "pending"} />
      <LanguageSwitcher currentLocale={locale} pathWithoutLocale={`/votations/${encodeURIComponent(id)}`} />
      <Link href={`/${locale}`} className="back-link">
        {messages.backToList}
      </Link>
      <header className="page-header">
        <h1>{pickBestTitle(votation, messages)}</h1>
        <p className="muted">
          {formatDate(votation.dateIso, locale, messages)} - {formatLevel(votation.level, messages)} -{" "}
          {formatStatus(votation.status, messages)}
        </p>
        <p className="muted">
          {messages.locationLabel}: {buildLocationLabel(votation, messages)}
        </p>
      </header>
      <VotationQASection votationId={votation.id} locale={locale} messages={messages} />
    </main>
  );
}
