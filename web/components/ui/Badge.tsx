import { type HTMLAttributes } from "react";

type Tone = "accent" | "neutral" | "error" | "running" | "retention";

// A soft tint of the status color as background, a 1px *inset ring* in
// that same color at low opacity (not a hard border), tiny (h-4) and
// uppercase mono text — a wash of color for definition rather than a
// filled chip or a bare outline.
// "running" is the same green wash as "accent" plus a pulse, for a backup
// that's actively in progress. "retention" is a dedicated orange, not part
// of the status palette, used only for a schedule's "keep N" value.
const toneClasses: Record<Tone, string> = {
  accent: "bg-status-success/10 text-content-accent ring-status-success/30",
  neutral: "bg-content-muted/10 text-content-secondary ring-content-muted/30",
  error: "bg-status-error/10 text-status-error ring-status-error/30",
  running: "bg-status-success/10 text-content-accent ring-status-success/30 animate-pulse",
  retention: "bg-status-retention/10 text-status-retention ring-status-retention/30",
};

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: Tone;
}

export function Badge({ tone = "neutral", className = "", children, ...props }: BadgeProps) {
  return (
    <span
      className={`inline-flex h-4 items-center whitespace-nowrap rounded-sm px-1 text-mono-xs ring-1 ring-inset ${toneClasses[tone]} ${className}`}
      {...props}
    >
      {children}
    </span>
  );
}
