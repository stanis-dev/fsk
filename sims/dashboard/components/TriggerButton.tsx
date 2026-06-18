"use client";

import { Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { triggerRun } from "@/app/actions";

export function TriggerButton() {
  return (
    <form action={triggerRun}>
      <Button type="submit" variant="outline" size="sm">
        <Play className="size-3.5" />
        trigger run
      </Button>
    </form>
  );
}
