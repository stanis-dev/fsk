"use client";

import { useState } from "react";
import { X } from "lucide-react";
import { cancelRun } from "@/lib/api";

export function CancelButton({ runId }: { runId: string }) {
  const [error, setError] = useState<string | null>(null);
  return (
    <button
      type="button"
      className="text-muted-foreground hover:text-danger"
      onClick={async () => {
        try {
          setError(null);
          await cancelRun(runId);
        } catch (e) {
          setError(e instanceof Error ? e.message : String(e));
        }
      }}
      title={error ?? undefined}
    >
      <X className="size-3" />
      {error ? "cancel failed" : "cancel"}
    </button>
  );
}
