"use client";

import { Badge } from "@/components/ui/badge";
import type { Check } from "@/lib/types";

export function JudgeBadge({ value }: { value: Check }) {
  if (value === "PASS") return <Badge className="bg-green-700 text-white">PASS</Badge>;
  if (value === "FAIL") return <Badge className="bg-red-700 text-white">FAIL</Badge>;
  return <span className="text-muted-foreground">—</span>;
}
