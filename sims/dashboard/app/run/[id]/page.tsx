import type { ReactNode } from "react";
import { notFound } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, ChevronRight } from "lucide-react";
import { runsDir } from "@/lib/paths";
import { loadRun } from "@/lib/runs";
import { JudgeBadge } from "@/components/JudgeBadge";
import { TranscriptView } from "@/components/TranscriptView";
import { DiffView } from "@/components/DiffView";
import { TelemetryView } from "@/components/TelemetryView";
import { cn } from "@/lib/utils";
import type { Check, CriterionVerdict } from "@/lib/types";

export const dynamic = "force-dynamic";

const LABEL = "text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground";

function Meta({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="min-w-0">
      <dt className={LABEL}>{label}</dt>
      <dd className={cn("mt-1 truncate text-sm", mono && "font-mono")}>{value}</dd>
    </div>
  );
}

const CRIT: Record<CriterionVerdict, { dot: string; text: string }> = {
  MET: { dot: "bg-success", text: "text-success" },
  UNMET: { dot: "bg-danger", text: "text-danger" },
  CANNOT_ASSESS: { dot: "bg-warning", text: "text-warning" },
};

function CritVerdict({ value }: { value: CriterionVerdict }) {
  const c = CRIT[value];
  return (
    <span className={cn("inline-flex items-center gap-1.5 text-xs font-medium", c.text)}>
      <span className={cn("size-1.5 rounded-full", c.dot)} aria-hidden />
      {value.toLowerCase().replace("_", " ")}
    </span>
  );
}

function Disclosure({ title, open, children }: { title: string; open?: boolean; children: ReactNode }) {
  return (
    <details open={open} className="group rounded-lg border border-border">
      <summary
        className={cn(
          "flex cursor-pointer list-none items-center gap-2 px-4 py-3 transition-colors select-none hover:text-foreground [&::-webkit-details-marker]:hidden",
          LABEL,
        )}
      >
        <ChevronRight className="size-3.5 shrink-0 transition-transform duration-200 group-open:rotate-90" />
        {title}
      </summary>
      <div className="border-t border-border p-4">{children}</div>
    </details>
  );
}

export default async function RunPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const run = loadRun(runsDir(), id);
  if (!run) notFound();
  const s = run.summary;
  return (
    <main className="mx-auto w-full max-w-6xl px-8 py-12">
      <Link
        href="/"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft className="size-3.5" />
        runs
      </Link>
      <h1 className="mt-3 font-mono text-2xl font-semibold tracking-tight">{s.id}</h1>

      <dl className="mt-6 grid grid-cols-2 gap-x-8 gap-y-4 border-t border-border pt-6 sm:grid-cols-4 lg:grid-cols-7">
        <Meta label="scenario" value={s.scenario} />
        <Meta label="coder" value={s.coder} />
        <Meta label="model" value={s.model} mono />
        <Meta label="harness" value={s.harness} />
        <Meta label="effort" value={s.effort} />
        <Meta label="turns" value={s.turns} mono />
        <Meta label="cost" value={s.cost} mono />
      </dl>
      <div className="mt-5 flex flex-wrap items-center gap-x-8 gap-y-3 border-b border-border pb-6">
        {(["build", "tests", "judge"] as const).map((key) => (
          <div key={key} className="flex items-center gap-2">
            <span className={LABEL}>{key}</span>
            <JudgeBadge value={s[key] as Check} />
          </div>
        ))}
      </div>

      <section className="mt-8">
        <h2 className={cn(LABEL, "mb-2")}>judge verdict</h2>
        <pre className="overflow-auto rounded-lg border border-border bg-muted/40 p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap">
          {run.judgeLog || "-"}
        </pre>
      </section>

      {run.judgeReport?.checks && (
        <section className="mt-8">
          <h2 className={cn(LABEL, "mb-3")}>
            checks · {run.judgeReport.checks.passed ? "all passed" : "failed"}
          </h2>
          <ul className="space-y-2">
            {run.judgeReport.checks.results.map((result) => (
              <li key={result.id} className="rounded-lg border border-border p-4">
                <div className="flex items-center gap-3">
                  <span
                    className={cn(
                      "inline-flex items-center gap-1.5 text-xs font-medium",
                      result.pass ? "text-success" : "text-danger",
                    )}
                  >
                    <span
                      className={cn("size-1.5 rounded-full", result.pass ? "bg-success" : "bg-danger")}
                      aria-hidden
                    />
                    {result.pass ? "pass" : "fail"}
                  </span>
                  <span className="font-mono text-sm font-medium">{result.id}</span>
                </div>
                {result.detail && (
                  <p className="mt-2 text-sm text-muted-foreground">{result.detail}</p>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      {run.judgeReport?.expectations && (
        <section className="mt-8">
          <h2 className={cn(LABEL, "mb-3")}>expectations · {run.judgeReport.expectations.model}</h2>
          <ul className="space-y-2">
            {run.judgeReport.expectations.criteria.map((criterion) => (
              <li key={criterion.id} className="rounded-lg border border-border p-4">
                <div className="flex items-center gap-3">
                  <CritVerdict value={criterion.verdict} />
                  <span className="font-mono text-sm font-medium">{criterion.id}</span>
                </div>
                {criterion.reasoning && (
                  <p className="mt-2 text-sm text-muted-foreground">{criterion.reasoning}</p>
                )}
                {criterion.evidence_quote && (
                  <pre className="mt-2 overflow-auto rounded-md border border-border bg-muted/40 p-2 font-mono text-xs whitespace-pre-wrap">
                    {criterion.evidence_quote}
                  </pre>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="mt-8 space-y-3">
        <Disclosure title={`build · tests${run.err ? " · stderr" : ""}`}>
          <pre className="overflow-auto font-mono text-xs leading-relaxed whitespace-pre-wrap">
            {`build:\n${run.buildLog}\ntests:\n${run.testLog}${run.err ? `\nstderr:\n${run.err}` : ""}`}
          </pre>
        </Disclosure>

        <Disclosure title={`session transcript · ${run.transcript.length} events`} open>
          <TranscriptView events={run.transcript} />
        </Disclosure>

        <Disclosure title={`MCP telemetry · ${run.telemetry.total} calls`} open>
          <TelemetryView summary={run.telemetry} />
        </Disclosure>

        <Disclosure title="diff">
          <DiffView lines={run.diff} />
        </Disclosure>
      </div>
    </main>
  );
}
