"use client";

import { X } from "lucide-react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { cancelRun } from "@/app/actions";

export function CancelButton({ runId }: { runId: string }) {
  const router = useRouter();
  return (
    <Button
      variant="ghost"
      size="xs"
      className="text-muted-foreground hover:text-danger"
      onClick={async () => {
        await cancelRun(runId);
        router.refresh();
      }}
    >
      <X className="size-3" />
      cancel
    </Button>
  );
}
