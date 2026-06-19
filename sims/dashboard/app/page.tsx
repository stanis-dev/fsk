import { listRuns } from "@/lib/runs";
import { listScenarios } from "@/lib/scenarios";
import { RunTable } from "@/components/RunTable";
import { RunMenu } from "@/components/RunMenu";
import { AutoRefresh } from "@/components/AutoRefresh";

export const dynamic = "force-dynamic";

export default function Home() {
  const runs = listRuns();
  const scenarios = listScenarios();
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
        <RunMenu scenarios={scenarios} />
      </header>
      <RunTable runs={runs} />
    </main>
  );
}
