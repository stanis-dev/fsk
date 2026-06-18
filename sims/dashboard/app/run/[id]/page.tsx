import { notFound } from "next/navigation";
import Link from "next/link";
import { runsDir } from "@/lib/paths";
import { loadRun } from "@/lib/runs";
import { JudgeBadge } from "@/components/JudgeBadge";
import { TranscriptView } from "@/components/TranscriptView";
import { DiffView } from "@/components/DiffView";
import { TelemetryView } from "@/components/TelemetryView";

export const dynamic = "force-dynamic";

function chipClass(verdict: string): string {
  const base = "rounded px-1.5 py-0.5 text-xs font-bold ";
  if (verdict === "MET") return base + "bg-green-100 text-green-800";
  if (verdict === "UNMET") return base + "bg-red-100 text-red-800";
  return base + "bg-yellow-100 text-yellow-800"; // CANNOT_ASSESS
}

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

      {run.judgeReport?.rubric && (
        <section className="mt-4">
          <h2 className="mb-1 font-bold">
            rubric ({run.judgeReport.rubric.model}) —{" "}
            <span className={run.judgeReport.verdict === "conformant" ? "text-green-700" : "text-red-700"}>
              {run.judgeReport.verdict}
            </span>
          </h2>
          <ul className="space-y-2">
            {run.judgeReport.rubric.criteria.map((criterion) => (
              <li key={criterion.id} className="rounded border p-2">
                <div className="flex items-center gap-2">
                  <span className={chipClass(criterion.verdict)}>{criterion.verdict}</span>
                  <span className="font-bold">{criterion.id}</span>
                </div>
                {criterion.reasoning && <p className="mt-1 text-xs">{criterion.reasoning}</p>}
                {criterion.evidence_quote && (
                  <pre className="mt-1 overflow-auto rounded bg-muted p-1 text-xs whitespace-pre-wrap">{criterion.evidence_quote}</pre>
                )}
                {criterion.cite && <p className="mt-1 text-xs text-muted-foreground">cite: {criterion.cite}</p>}
              </li>
            ))}
          </ul>
        </section>
      )}

      <details className="my-2 rounded border p-2">
        <summary className="cursor-pointer font-bold">build · tests{run.err ? " · stderr" : ""}</summary>
        <pre className="mt-2 overflow-auto whitespace-pre-wrap text-xs">{`build:\n${run.buildLog}\ntests:\n${run.testLog}${run.err ? `\nstderr:\n${run.err}` : ""}`}</pre>
      </details>

      <details className="my-2 rounded border p-2" open>
        <summary className="cursor-pointer font-bold">session transcript ({run.transcript.length} events)</summary>
        <div className="mt-2"><TranscriptView events={run.transcript} /></div>
      </details>

      <details className="my-2 rounded border p-2" open>
        <summary className="cursor-pointer font-bold">MCP telemetry ({run.telemetry.total} calls)</summary>
        <div className="mt-2"><TelemetryView summary={run.telemetry} /></div>
      </details>

      <details className="my-2 rounded border p-2">
        <summary className="cursor-pointer font-bold">diff</summary>
        <div className="mt-2"><DiffView lines={run.diff} /></div>
      </details>
    </main>
  );
}
