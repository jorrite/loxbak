"use client";

import { useEffect, useState } from "react";

const FRAMES = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
const FRAME_INTERVAL_MS = 80;

interface SpinnerProps {
  /** Whether the underlying operation is still running. Defaults to true (always spinning). */
  isLoading?: boolean;
  /** Minimum time (ms) the spinner stays visible once shown, so fast operations don't flash it in and out. */
  minTime?: number;
  label?: string;
  className?: string;
}

/**
 * A braille-frame spinner in a TUI-inspired style. Exposes a `minTime`
 * guard: once shown, keep it visible for at least `minTime` ms after
 * `isLoading` goes back to false, so a near-instant operation doesn't
 * flash the spinner.
 */
export function Spinner({
  isLoading = true,
  minTime = 500,
  label,
  className = "",
}: SpinnerProps) {
  const [frame, setFrame] = useState(0);
  const [shown, setShown] = useState(isLoading);

  // Adjust state in response to a prop change, during render (the pattern
  // React recommends over an effect for this): as soon as loading starts,
  // show immediately rather than waiting a tick.
  // https://react.dev/learn/you-might-not-need-an-effect#adjusting-state-when-a-prop-changes
  const [prevIsLoading, setPrevIsLoading] = useState(isLoading);
  if (isLoading !== prevIsLoading) {
    setPrevIsLoading(isLoading);
    if (isLoading) setShown(true);
  }

  useEffect(() => {
    if (!shown) return;
    const id = setInterval(() => {
      setFrame((f) => (f + 1) % FRAMES.length);
    }, FRAME_INTERVAL_MS);
    return () => clearInterval(id);
  }, [shown]);

  useEffect(() => {
    if (isLoading) return;
    const id = setTimeout(() => setShown(false), minTime);
    return () => clearTimeout(id);
  }, [isLoading, minTime]);

  if (!shown) return null;

  return (
    <span
      className={`inline-flex items-center gap-2 font-mono text-content-accent ${className}`}
      role="status"
      aria-live="polite"
    >
      <span aria-hidden>{FRAMES[frame]}</span>
      {label && <span className="text-content-accent">{label}</span>}
    </span>
  );
}
