import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export const HEAD = "h-9 whitespace-nowrap px-3 text-left text-[0.7rem] font-medium uppercase tracking-[0.08em] text-muted-foreground"
export const CELL = "whitespace-nowrap px-3 py-2.5"

export function errMsg(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
