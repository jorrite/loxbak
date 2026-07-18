"use client";

import * as RadixTooltip from "@radix-ui/react-tooltip";
import { type ReactNode } from "react";

/** App-wide delay/skip tuning for every Tooltip below — one Provider near
 * the root rather than each Tooltip instantiating its own with defaults. */
export function TooltipProvider({ children }: { children: ReactNode }) {
  return (
    <RadixTooltip.Provider delayDuration={300} skipDelayDuration={100}>
      {children}
    </RadixTooltip.Provider>
  );
}

interface TooltipProps {
  content: ReactNode;
  children: ReactNode;
}

/** Same dark, bordered, ring-lit chrome as RowActionsMenu's dropdown —
 * replaces the browser's own unstyled title="" tooltip. */
export function Tooltip({ content, children }: TooltipProps) {
  return (
    <RadixTooltip.Root>
      <RadixTooltip.Trigger asChild>{children}</RadixTooltip.Trigger>
      <RadixTooltip.Portal>
        <RadixTooltip.Content
          sideOffset={6}
          className="z-50 max-w-64 text-balance rounded-sm border border-border-default bg-surface-overlay px-2 py-1 font-mono text-[11px] leading-4 tracking-wide text-content-secondary shadow-lg"
        >
          {content}
          <RadixTooltip.Arrow className="fill-surface-overlay" width={8} height={4} />
        </RadixTooltip.Content>
      </RadixTooltip.Portal>
    </RadixTooltip.Root>
  );
}
