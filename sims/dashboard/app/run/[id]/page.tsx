import { notFound } from "next/navigation";
import Link from "next/link";
import { runsDir } from "@/lib/paths";
import { loadRun } from "@/lib/runs";
import { JudgeBadge } from "@/components/JudgeBadge";
import { TranscriptView } from "@/components/TranscriptView";
import { DiffView } from "@/components/DiffView";

export const dynamic = "force-dynamic";

export default async function RunPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const run = loadRun(runsDir(), id);
  if (!run) notFound();
  const s = run.summary;
  return (
    <main className="mx-auto max-w-6xl p-8 font-mono text-sm">
      <Link className="text-blue-700 underline" href="/">← runs</Link>
      <h1 className="my-2 text-xl font-bold">{s.id}</h1>
      <div className="mb-4 flex flex-wrap gap-4 rounded bg-muted p-3">
        <span>scenario {s.scenario}</span><span>coder {s.coder}</span><span>model {s.model}</span><span>harness {s.harness}</span>
        <span>effort {s.effort}</span><span>turns {s.turns}</span><span>cost {s.cost}</span>
        <span>build <JudgeBadge value={s.build} /></span>
        <span>tests <JudgeBadge value={s.tests} /></span>
        <span>judge <JudgeBadge value={s.judge} /></span>
      </div>

      <h2 className="mb-1 mt-4 font-bold">judge verdict</h2>
      <pre className="overflow-auto rounded bg-muted p-2 text-xs whitespace-pre-wrap">{run.judgeLog || "—"}</pre>

      <details className="my-2 rounded border p-2">
        <summary className="cursor-pointer font-bold">build · tests{run.err ? " · stderr" : ""}</summary>
        <pre className="mt-2 overflow-auto whitespace-pre-wrap text-xs">{`build:\n${run.buildLog}\ntests:\n${run.testLog}${run.err ? `\nstderr:\n${run.err}` : ""}`}</pre>
      </details>

      <details className="my-2 rounded border p-2" open>
        <summary className="cursor-pointer font-bold">session transcript ({run.transcript.length} events)</summary>
        <div className="mt-2"><TranscriptView events={run.transcript} /></div>
      </details>

      <details className="my-2 rounded border p-2">
        <summary className="cursor-pointer font-bold">diff</summary>
        <div className="mt-2"><DiffView lines={run.diff} /></div>
      </details>
    </main>
  );
}
