import { type ButtonHTMLAttributes } from "react";
import { Tooltip } from "@/components/ui/Tooltip";

type Tone = "default" | "accent" | "danger";
type Size = "sm" | "md";

const toneClasses: Record<Tone, string> = {
  default: "text-content-secondary hover:bg-surface-overlay hover:text-content-default",
  accent: "text-content-secondary hover:bg-surface-overlay hover:text-content-accent",
  danger: "text-content-secondary hover:bg-status-error/10 hover:text-status-error",
};

const sizeClasses: Record<Size, string> = {
  sm: "size-6",
  md: "size-8",
};

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  tone?: Tone;
  /** `md` (32px) for toolbar-level actions sitting next to a filled Button;
   * `sm` (24px) for compact inline row actions in a table. */
  size?: Size;
  /** Bordered variant for toolbar-level actions (e.g. refresh) that sit
   * next to a filled primary Button and need the same visual weight. */
  bordered?: boolean;
}

export function IconButton({
  tone = "default",
  size = "md",
  bordered = false,
  className = "",
  title,
  ...props
}: IconButtonProps) {
  const button = (
    <button
      className={`inline-flex shrink-0 cursor-pointer items-center justify-center rounded-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
        sizeClasses[size]
      } ${bordered ? "border border-border-default" : ""} ${toneClasses[tone]} ${className}`}
      {...props}
    />
  );
  // A styled Tooltip instead of the browser's own title="" one — the
  // <button> above is a raw host element, so Radix's asChild clone can
  // attach a ref to it directly without needing IconButton itself to
  // forward one.
  return title ? <Tooltip content={title}>{button}</Tooltip> : button;
}
