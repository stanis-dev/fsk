import { listRuns } from "@/lib/runs";
import { RunTable } from "@/components/RunTable";
import { TriggerButton } from "@/components/TriggerButton";
import { AutoRefresh } from "@/components/AutoRefresh";

// Parity with the Go dashboard's 10s refresh; a later plan replaces this with SWR.
export const dynamic = "force-dynamic";

export default function Home() {
  const runs = listRuns();
  return (
    <main className="mx-auto w-full max-w-6xl px-8 py-12">
      <AutoRefresh />
      <header className="mb-8 flex items-end justify-between gap-4 border-b border-border pb-5">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">fiskaly eval runs</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            agentic coding eval workbench · {runs.length} runs
          </p>
        </div>
        <TriggerButton />
      </header>
      <RunTable runs={runs} />
    </main>
  );
}
