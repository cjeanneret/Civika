import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Civika",
  description: "PoC de visualisation des votations suisses",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="fr">
      <body>{children}</body>
    </html>
  );
}
