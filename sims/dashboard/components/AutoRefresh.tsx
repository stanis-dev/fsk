"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

// Refresh the server-rendered data on an interval, scoped to the page that mounts
// this. Navigating away unmounts it and clears the timer - unlike a
// <meta http-equiv="refresh">, whose scheduled reload survives client-side
// navigation and would yank the user back to this page's URL.
export function AutoRefresh({ seconds = 10 }: { seconds?: number }) {
  const router = useRouter();
  useEffect(() => {
    const id = setInterval(() => router.refresh(), seconds * 1000);
    return () => clearInterval(id);
  }, [router, seconds]);
  return null;
}
