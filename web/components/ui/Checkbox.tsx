"use client";

import * as RadixCheckbox from "@radix-ui/react-checkbox";
import { type ComponentProps } from "react";

// Same formula as the colored Button variants (see --button-* tokens in
// globals.css): a near-invisible accent-tinted hairline border (not a
// solid/bright one — see Button.tsx's primary variant), a dark tint of the
// accent for the fill once checked, full-strength accent for the mark.
// Marked with an X (not a checkmark) per request.
export function Checkbox({ className = "", ...props }: ComponentProps<typeof RadixCheckbox.Root>) {
  return (
    <RadixCheckbox.Root
      className={`flex size-4 shrink-0 cursor-pointer items-center justify-center rounded-sm border border-border-default bg-surface-default transition-colors hover:border-content-accent/10 hover:bg-[var(--button-accent-bg)] data-[state=checked]:border-content-accent/10 data-[state=checked]:bg-[var(--button-accent-bg)] data-[state=checked]:hover:bg-[var(--button-accent-bg-hover)] ${className}`}
      {...props}
    >
      <RadixCheckbox.Indicator>
        <svg
          viewBox="0 0 10 10"
          className="size-2.5 text-content-accent"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinecap="round"
          aria-hidden="true"
        >
          <path d="M2 2L8 8M8 2L2 8" />
        </svg>
      </RadixCheckbox.Indicator>
    </RadixCheckbox.Root>
  );
}
