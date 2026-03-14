import Link from "next/link";
import type { LocaleCode } from "@/lib/i18n/config";
import type { Messages } from "@/lib/i18n/messages";
import { buildLocationLabel, formatDate, formatLevel, formatStatus, pickBestTitle } from "@/lib/votations";
import type { VotationListItem } from "@/types/api";

type VotationListProps = {
  items: VotationListItem[];
  locale: LocaleCode;
  messages: Messages;
};

export function VotationList({ items, locale, messages }: VotationListProps) {
  if (items.length === 0) {
    return <p className="muted">{messages.noVotations}</p>;
  }

  return (
    <ul className="votation-list">
      {items.map((item) => (
        <li key={item.id} className="votation-card">
          <Link href={`/${locale}/votations/${encodeURIComponent(item.id)}`} className="votation-card-link">
            <span className="votation-card-date">{formatDate(item.dateIso, locale, messages)}</span>
            <span className="votation-card-title">{pickBestTitle(item, messages)}</span>
            <span className="votation-card-meta">
              {formatLevel(item.level, messages)} - {formatStatus(item.status, messages)} - {messages.locationLabel}:{" "}
              {buildLocationLabel(item, messages)}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}
