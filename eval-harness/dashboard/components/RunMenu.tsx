"use client";

import { useState } from "react";
import { Menu } from "@base-ui/react/menu";
import { ChevronDown, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { postRun } from "@/lib/api";
import type { ScenarioConfig } from "@/lib/types";

export function RunMenu({ scenarios }: { scenarios: ScenarioConfig[] }) {
  const [error, setError] = useState<string | null>(null);
  return (
    <div className="flex flex-col items-end gap-1">
      <Menu.Root>
        <Menu.Trigger render={<Button variant="outline" size="sm" />}>
          <Play className="size-3.5" />
          run
          <ChevronDown className="size-3.5" />
        </Menu.Trigger>
        <Menu.Portal>
          <Menu.Positioner sideOffset={6} align="end">
            <Menu.Popup className="z-50 max-h-80 min-w-64 overflow-auto rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-md">
              {scenarios.map((s) => (
                <Menu.Item
                  key={s.id}
                  className="flex cursor-pointer items-center gap-3 rounded-md px-2.5 py-2 text-sm outline-none data-[highlighted]:bg-muted"
                  onClick={async () => {
                    setError(null);
                    try {
                      await postRun(s.id);
                    } catch (e) {
                      setError(e instanceof Error ? e.message : String(e));
                    }
                  }}
                >
                  <span className="font-mono text-xs text-muted-foreground">{s.id}</span>
                  <span className="truncate">{s.title}</span>
                </Menu.Item>
              ))}
            </Menu.Popup>
          </Menu.Positioner>
        </Menu.Portal>
      </Menu.Root>
      {error && <span className="text-xs text-danger">{error}</span>}
    </div>
  );
}
