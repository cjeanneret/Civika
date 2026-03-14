"use client";

import { useEffect } from "react";

type PendingTranslationRefresherProps = {
  enabled: boolean;
  intervalMs?: number;
};

export function PendingTranslationRefresher({ enabled, intervalMs = 12000 }: PendingTranslationRefresherProps) {
  useEffect(() => {
    if (!enabled) {
      return;
    }
    const id = window.setTimeout(() => {
      window.location.reload();
    }, intervalMs);
    return () => window.clearTimeout(id);
  }, [enabled, intervalMs]);

  return null;
}
