export const SUPPORTED_LOCALES = ["fr", "de", "it", "rm", "en"] as const;

export type LocaleCode = (typeof SUPPORTED_LOCALES)[number];

export const DEFAULT_LOCALE: LocaleCode = "fr";

const localeSet = new Set<string>(SUPPORTED_LOCALES);

export function isSupportedLocale(value: string): value is LocaleCode {
  return localeSet.has(value);
}

export function normalizeLocale(value: string | undefined): LocaleCode {
  if (!value) {
    return DEFAULT_LOCALE;
  }
  const raw = value.trim().toLowerCase();
  return isSupportedLocale(raw) ? raw : DEFAULT_LOCALE;
}

export function toIntlLocale(locale: LocaleCode): string {
  switch (locale) {
    case "de":
      return "de-CH";
    case "it":
      return "it-CH";
    case "rm":
      return "rm-CH";
    case "en":
      return "en-CH";
    case "fr":
    default:
      return "fr-CH";
  }
}
