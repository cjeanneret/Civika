import Image from "next/image";
import Link from "next/link";
import type { LocaleCode } from "@/lib/i18n/config";

type SiteBrandProps = {
  locale: LocaleCode;
  asHeading?: boolean;
  compact?: boolean;
};

export function SiteBrand({ locale, asHeading = false, compact = false }: SiteBrandProps) {
  const title = asHeading ? <h1 className="site-brand-title">Civika</h1> : <span className="site-brand-title">Civika</span>;

  return (
    <div className={`site-brand${compact ? " site-brand-compact" : ""}`}>
      <Link href={`/${locale}`} className="site-brand-link" aria-label="Civika">
        <span className="site-brand-logo-wrap">
          <Image src="/icon.png" alt="" width={40} height={40} aria-hidden priority />
        </span>
        {title}
      </Link>
    </div>
  );
}
