import { type HTMLAttributes } from "react";

interface PanelProps extends HTMLAttributes<HTMLDivElement> {
  title?: string;
  /** Skip the default inner padding, for content (a table) that should sit
   * flush against the panel's own edges/dividers rather than inset. */
  flush?: boolean;
}

export function Panel({ title, flush = false, className = "", children, ...props }: PanelProps) {
  return (
    <div
      className={`rounded-sm border border-border-default bg-surface-raised ${className}`}
      {...props}
    >
      {title && (
        <div className="border-b border-border-subtle bg-surface-overlay px-5 py-2.5">
          <h2 className="text-mono-xs text-content-secondary">{title}</h2>
        </div>
      )}
      <div className={flush ? "" : "p-5"}>{children}</div>
    </div>
  );
}
