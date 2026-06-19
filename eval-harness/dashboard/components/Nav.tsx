"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

const LINKS = [
  { href: "/", label: "runs", match: (p: string) => p === "/" || p.startsWith("/run/") },
  { href: "/scenarios", label: "scenarios", match: (p: string) => p.startsWith("/scenarios") },
];

export function Nav() {
  const pathname = usePathname();
  return (
    <header className="border-b border-border">
      <div className="mx-auto flex h-12 w-full max-w-6xl items-center gap-6 px-8">
        <span className="text-sm font-semibold tracking-tight">fiskaly eval</span>
        <nav className="flex items-center gap-4 text-sm">
          {LINKS.map((l) => (
            <Link
              key={l.href}
              href={l.href}
              className={cn(
                "transition-colors",
                l.match(pathname) ? "text-foreground" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {l.label}
            </Link>
          ))}
        </nav>
      </div>
    </header>
  );
}
