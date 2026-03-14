import { redirect } from "next/navigation";

type LegacyVotationPageProps = {
  params: Promise<{
    id: string;
  }>;
};

export default async function LegacyVotationPage({ params }: LegacyVotationPageProps) {
  const { id } = await params;
  redirect(`/fr/votations/${encodeURIComponent(id)}`);
}
