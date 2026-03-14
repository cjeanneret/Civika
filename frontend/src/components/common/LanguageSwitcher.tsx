import Link from "next/link";
import { SUPPORTED_LOCALES, type LocaleCode } from "@/lib/i18n/config";
import { getMessages } from "@/lib/i18n/messages";

type LanguageSwitcherProps = {
  currentLocale: LocaleCode;
  pathWithoutLocale: string;
};

export function LanguageSwitcher({ currentLocale, pathWithoutLocale }: LanguageSwitcherProps) {
  const normalizedPath = pathWithoutLocale.startsWith("/") ? pathWithoutLocale : `/${pathWithoutLocale}`;
  return (
    <nav className="language-switcher" aria-label="Language switcher">
      {SUPPORTED_LOCALES.map((locale) => {
        const href = `/${locale}${normalizedPath}`;
        const isActive = locale === currentLocale;
        return (
          <Link key={locale} href={href} className={`language-link${isActive ? " active" : ""}`}>
            {getMessages(locale).localeName}
          </Link>
        );
      })}
    </nav>
  );
}
