"use client";

import { useState } from "react";
import { X } from "lucide-react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { cancelRun } from "@/app/actions";

export function CancelButton({ runId }: { runId: string }) {
  const router = useRouter();
  const [failed, setFailed] = useState(false);
  return (
    <Button
      variant="ghost"
      size="xs"
      className="text-muted-foreground hover:text-danger"
      onClick={async () => {
        try {
          setFailed(false);
          await cancelRun(runId);
          router.refresh();
        } catch {
          setFailed(true);
        }
      }}
    >
      <X className="size-3" />
      {failed ? "cancel failed" : "cancel"}
    </Button>
  );
}
