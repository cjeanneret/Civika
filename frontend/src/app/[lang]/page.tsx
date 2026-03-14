import { VotationList } from "@/components/votations/VotationList";
import { PendingTranslationRefresher } from "@/components/common/PendingTranslationRefresher";
import { LanguageSwitcher } from "@/components/common/LanguageSwitcher";
import { getVotations } from "@/lib/api";
import { normalizeLocale } from "@/lib/i18n/config";
import { getMessages } from "@/lib/i18n/messages";
import type { VotationListItem } from "@/types/api";

type LocalizedHomePageProps = {
  params: Promise<{
    lang: string;
  }>;
};

export default async function LocalizedHomePage({ params }: LocalizedHomePageProps) {
  const { lang } = await params;
  const locale = normalizeLocale(lang);
  const messages = getMessages(locale);

  let items: VotationListItem[] = [];
  let errorMessage: string | null = null;

  try {
    const result = await getVotations(20, 0, locale);
    items = [...result.items].sort((a, b) => {
      const aTime = a.dateIso ? new Date(a.dateIso).getTime() : 0;
      const bTime = b.dateIso ? new Date(b.dateIso).getTime() : 0;
      return bTime - aTime;
    });
  } catch {
    errorMessage = messages.loadingVotationsError;
  }

  return (
    <main className="container">
      <PendingTranslationRefresher enabled={items.some((item) => item.translationStatus?.state === "pending")} />
      <LanguageSwitcher currentLocale={locale} pathWithoutLocale="/" />
      <header className="page-header">
        <h1>Civika</h1>
        <p className="muted">{messages.latestVotations}</p>
      </header>
      {errorMessage ? (
        <p className="error-text">{errorMessage}</p>
      ) : (
        <VotationList items={items} locale={locale} messages={messages} />
      )}
    </main>
  );
}
