"use client";

import { useEffect, useRef, useState, useSyncExternalStore } from "react";

const CHAR_INTERVAL_MS = 18;
const LINE_PAUSE_MS = 200;

function subscribeReducedMotion(callback: () => void) {
  const mql = window.matchMedia("(prefers-reduced-motion: reduce)");
  mql.addEventListener("change", callback);
  return () => mql.removeEventListener("change", callback);
}

function getReducedMotionSnapshot() {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function getReducedMotionServerSnapshot() {
  // Static export has no real server; assume motion is fine until the
  // client subscription reconciles with the actual OS preference.
  return false;
}

interface BootSequenceProps {
  lines: string[];
  onComplete?: () => void;
  className?: string;
}

/**
 * Reveals `lines` one at a time with a typewriter effect and a blinking
 * block cursor, in a TUI/retro-computing personality. Respects
 * `prefers-reduced-motion`: when set, all lines render fully revealed
 * immediately, with no animation.
 */
export function BootSequence({ lines, onComplete, className = "" }: BootSequenceProps) {
  const reduceMotion = useSyncExternalStore(
    subscribeReducedMotion,
    getReducedMotionSnapshot,
    getReducedMotionServerSnapshot,
  );
  const [lineIndex, setLineIndex] = useState(0);
  const [charIndex, setCharIndex] = useState(0);
  const done = reduceMotion || lineIndex >= lines.length;

  // Keep the latest onComplete without re-triggering the effect below on
  // every parent render (the "ref updated in an effect" pattern).
  const onCompleteRef = useRef(onComplete);
  useEffect(() => {
    onCompleteRef.current = onComplete;
  });

  useEffect(() => {
    if (!done) return;
    onCompleteRef.current?.();
  }, [done]);

  useEffect(() => {
    if (reduceMotion) return;
    if (lineIndex >= lines.length) return;

    const currentLine = lines[lineIndex] ?? "";

    if (charIndex < currentLine.length) {
      const t = setTimeout(() => setCharIndex((c) => c + 1), CHAR_INTERVAL_MS);
      return () => clearTimeout(t);
    }

    const t = setTimeout(() => {
      setLineIndex((l) => l + 1);
      setCharIndex(0);
    }, LINE_PAUSE_MS);
    return () => clearTimeout(t);
  }, [reduceMotion, lineIndex, charIndex, lines]);

  if (reduceMotion) {
    return (
      <div className={`font-mono text-sm text-content-secondary ${className}`}>
        {lines.map((line, i) => (
          <div key={i}>{line}</div>
        ))}
      </div>
    );
  }

  return (
    <div className={`font-mono text-sm text-content-secondary ${className}`} aria-live="polite">
      {lines.slice(0, lineIndex).map((line, i) => (
        <div key={i}>{line}</div>
      ))}
      {lineIndex < lines.length && (
        <div>
          {lines[lineIndex].slice(0, charIndex)}
          <span className="animate-pulse text-content-accent">█</span>
        </div>
      )}
      {done && lineIndex >= lines.length && <div className="text-content-accent">█</div>}
    </div>
  );
}
