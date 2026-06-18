"use client";

import { Menu } from "@base-ui/react/menu";
import { ChevronDown, Play } from "lucide-react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { runScenario } from "@/app/actions";
import type { ScenarioConfig } from "@/lib/types";

export function RunMenu({ scenarios }: { scenarios: ScenarioConfig[] }) {
  const router = useRouter();
  return (
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
                  await runScenario(s.id);
                  router.refresh();
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
  );
}
