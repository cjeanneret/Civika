import type { LocaleCode } from "@/lib/i18n/config";
import { toIntlLocale } from "@/lib/i18n/config";
import type { Messages } from "@/lib/i18n/messages";
import type { VotationDetail, VotationListItem } from "@/types/api";

type VotationLike = VotationListItem | VotationDetail;

export function pickBestTitle(item: VotationLike, messages: Messages): string {
  const preferred = item.displayTitles?.[item.language ?? ""] ?? "";
  if (preferred.trim() !== "") {
    return withTranslationPendingTag(preferred, item, messages);
  }

  const titles = item.displayTitles ?? item.titles;
  if (!titles) {
    return messages.titleUnavailable;
  }
  const raw = titles.fr ?? titles.de ?? titles.it ?? titles.rm ?? titles.en ?? Object.values(titles)[0] ?? messages.titleUnavailable;
  return withTranslationPendingTag(raw, item, messages);
}

function withTranslationPendingTag(title: string, item: VotationLike, messages: Messages): string {
  if (item.translationStatus?.state !== "pending") {
    return title;
  }
  return `${title} (${messages.translationInProgress})`;
}

export function formatDate(dateIso: string | undefined, locale: LocaleCode, messages: Messages): string {
  if (!dateIso) {
    return messages.dateUnknown;
  }
  const date = new Date(dateIso);
  if (Number.isNaN(date.getTime())) {
    return messages.dateUnknown;
  }
  return new Intl.DateTimeFormat(toIntlLocale(locale), {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  }).format(date);
}

export function formatLevel(level: string | undefined, messages: Messages): string {
  switch (level) {
    case "federal":
      return messages.levelFederal;
    case "cantonal":
      return messages.levelCantonal;
    case "communal":
      return messages.levelCommunal;
    default:
      return messages.levelUnknown;
  }
}

export function formatStatus(status: string | undefined, messages: Messages): string {
  switch (status) {
    case "past":
      return messages.statusPast;
    case "upcoming":
      return messages.statusUpcoming;
    default:
      return messages.statusUnknown;
  }
}

export function buildLocationLabel(item: VotationLike, messages: Messages): string {
  if (item.level === "federal") {
    return messages.locationFederal;
  }
  if (item.level === "communal") {
    const parts = [item.communeName, item.canton].filter((value): value is string => Boolean(value && value.trim() !== ""));
    if (parts.length > 0) {
      return parts.join(", ");
    }
  }
  if (item.level === "cantonal" && item.canton) {
    return item.canton;
  }
  return messages.locationUnknown;
}
