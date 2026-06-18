import { notFound } from "next/navigation";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { loadScenario } from "@/lib/scenarios";
import { ScenarioEditor } from "@/components/ScenarioEditor";

export const dynamic = "force-dynamic";

export default async function ScenarioEditPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const detail = loadScenario(id);
  if (!detail) notFound();
  return (
    <main className="mx-auto w-full max-w-3xl px-8 py-12">
      <Link href="/scenarios" className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground">
        <ArrowLeft className="size-3.5" />
        scenarios
      </Link>
      <h1 className="mt-3 mb-8 font-mono text-2xl font-semibold tracking-tight">{detail.config.id}</h1>
      <ScenarioEditor detail={detail} />
    </main>
  );
}
