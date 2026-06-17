import { listRuns } from "@/lib/runs";
import { RunTable } from "@/components/RunTable";
import { TriggerButton } from "@/components/TriggerButton";
import { AutoRefresh } from "@/components/AutoRefresh";

// Parity with the Go dashboard's 10s refresh; a later plan replaces this with SWR.
export const dynamic = "force-dynamic";

export default function Home() {
  const runs = listRuns();
  return (
    <main className="mx-auto max-w-5xl p-8">
      <AutoRefresh />
      <div className="mb-4 flex items-center gap-4">
        <h1 className="text-xl font-bold">fiskaly eval runs</h1>
        <TriggerButton />
      </div>
      <RunTable runs={runs} />
    </main>
  );
}
