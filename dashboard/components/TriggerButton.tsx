"use client";

import { Button } from "@/components/ui/button";
import { triggerRun } from "@/app/actions";

export function TriggerButton() {
  return (
    <form action={triggerRun}>
      <Button type="submit">▶ trigger run</Button>
    </form>
  );
}
