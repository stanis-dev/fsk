"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { getScenario } from "@/lib/api";
import { ScenarioEditor } from "@/components/ScenarioEditor";
import type { ScenarioDetail } from "@/lib/types";

export default function ScenarioEditPage() {
  const { id } = useParams<{ id: string }>();
  const [detail, setDetail] = useState<ScenarioDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getScenario(id)
      .then(setDetail)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : String(e)));
  }, [id]);

  return (
    <main className="mx-auto w-full max-w-3xl px-8 py-12">
      <Link href="/scenarios" className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground">
        <ArrowLeft className="size-3.5" />
        scenarios
      </Link>
      {error && <p className="mt-6 text-sm text-muted-foreground">{error}</p>}
      {!error && !detail && <p className="mt-6 text-sm text-muted-foreground">loading…</p>}
      {detail && (
        <>
          <h1 className="mt-3 mb-8 font-mono text-2xl font-semibold tracking-tight">{detail.config.id}</h1>
          <ScenarioEditor detail={detail} />
        </>
      )}
    </main>
  );
}
