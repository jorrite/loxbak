import { type ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";

// Colored variants (primary/danger) follow one button formula (see the
// --button-* tokens in globals.css): a dark, desaturated tint of the
// accent as background, the bright accent as text, and a near-invisible
// full-perimeter hairline (5-10% opacity of the text color) rather than a
// visible colored border — plus an `active:translate-y-px` press feedback.
const variantClasses: Record<Variant, string> = {
  primary:
    "border border-content-accent/10 bg-[var(--button-accent-bg)] text-content-accent hover:bg-[var(--button-accent-bg-hover)]",
  secondary:
    "bg-surface-raised text-content-default border border-border-default hover:border-border-accent",
  danger:
    "border border-status-error/10 bg-[var(--button-danger-bg)] text-status-error hover:bg-[var(--button-danger-bg-hover)]",
  ghost:
    "bg-transparent text-content-secondary border border-transparent hover:text-content-default hover:bg-surface-raised",
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
}

export function Button({
  variant = "primary",
  className = "",
  ...props
}: ButtonProps) {
  return (
    <button
      className={`inline-flex h-8 cursor-pointer items-center justify-center gap-1.5 rounded-sm px-3 text-mono-sm font-medium transition-colors active:motion-safe:translate-y-px disabled:cursor-not-allowed disabled:opacity-50 ${variantClasses[variant]} ${className}`}
      {...props}
    />
  );
}
