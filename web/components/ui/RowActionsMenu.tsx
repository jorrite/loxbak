"use client";

import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { MoreIcon } from "@/components/icons";

interface RowAction {
  label: string;
  onSelect: () => void;
  tone?: "default" | "danger";
  disabled?: boolean;
}

interface RowActionsMenuProps {
  actions: RowAction[];
}

// A single vertical-ellipsis trigger per row, opening a dropdown with the
// row's actions — not persistent inline icon buttons per action.
export function RowActionsMenu({ actions }: RowActionsMenuProps) {
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button
          onClick={(e) => e.stopPropagation()}
          aria-label="Row actions"
          className="flex size-6 cursor-pointer items-center justify-center rounded-sm text-content-secondary outline-none transition-colors hover:bg-surface-overlay hover:text-content-default"
        >
          <MoreIcon className="size-3" />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={4}
          className="z-50 min-w-36 rounded-sm border border-border-default bg-surface-overlay p-1 shadow-lg"
        >
          {actions.map((action) => (
            <DropdownMenu.Item
              key={action.label}
              disabled={action.disabled}
              onSelect={() => action.onSelect()}
              className={`flex cursor-pointer items-center rounded-sm px-2 py-1.5 text-mono-xs outline-none transition-colors data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50 ${
                action.tone === "danger"
                  ? "text-status-error data-[highlighted]:bg-status-error/10"
                  : "text-content-default data-[highlighted]:bg-surface-raised"
              }`}
            >
              {action.label}
            </DropdownMenu.Item>
          ))}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
