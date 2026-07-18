"use client";

import { useEffect, useState } from "react";
import { format } from "timeago.js";
import { Tooltip } from "@/components/ui/Tooltip";

interface RelativeTimeProps {
  date: string | Date;
  className?: string;
}

const MINUTE_MS = 60_000;

// timeago.js's own bucketing shows "just now" for the first 9 seconds and
// only switches to "N seconds ago" after that — under a minute we want
// literal live seconds throughout instead, so that range is handled here
// rather than deferred to the library.
function relativeLabel(d: Date): string {
  const diffMs = Date.now() - d.getTime();
  if (diffMs >= 0 && diffMs < MINUTE_MS) {
    const seconds = Math.floor(diffMs / 1000);
    return seconds <= 1 ? "1 second ago" : `${seconds} seconds ago`;
  }
  return format(d);
}

/** Renders a relative "2 minutes ago" timestamp, live-ticking every second
 * for the first minute and every 30s after that, with the exact local
 * timestamp available in a styled Tooltip on hover. */
export function RelativeTime({ date, className = "" }: RelativeTimeProps) {
  const d = typeof date === "string" ? new Date(date) : date;
  const iso = d.toISOString();

  const [text, setText] = useState(() => relativeLabel(d));

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>;

    function scheduleNext() {
      const diffMs = Date.now() - new Date(iso).getTime();
      const delay = diffMs < MINUTE_MS ? 1_000 : 30_000;
      timer = setTimeout(() => {
        setText(relativeLabel(new Date(iso)));
        scheduleNext();
      }, delay);
    }
    scheduleNext();

    return () => clearTimeout(timer);
  }, [iso]);

  return (
    <Tooltip content={d.toLocaleString()}>
      <time dateTime={iso} className={className}>
        {text}
      </time>
    </Tooltip>
  );
}
