"use client";

import * as AlertDialog from "@radix-ui/react-alert-dialog";
import { type ReactNode } from "react";
import { Button } from "@/components/ui/Button";

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  confirmLabel?: string;
  onConfirm: () => void;
  children?: ReactNode;
}

// Controlled rather than trigger-based: a dropdown menu item's onSelect
// just sets `open`, since nesting AlertDialog.Trigger inside a
// DropdownMenu.Item is a known-fragile Radix combination (the menu
// unmounting the trigger can race the dialog opening).
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = "Delete",
  onConfirm,
  children,
}: ConfirmDialogProps) {
  return (
    <AlertDialog.Root open={open} onOpenChange={onOpenChange}>
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="fixed inset-0 z-50 bg-black/60" />
        <AlertDialog.Content className="fixed top-1/2 left-1/2 z-50 w-80 -translate-x-1/2 -translate-y-1/2 rounded-sm border border-border-default bg-surface-overlay p-4 shadow-lg">
          <AlertDialog.Title className="text-sm font-medium text-content-default">
            {title}
          </AlertDialog.Title>
          {description && (
            <AlertDialog.Description className="mt-1.5 font-mono text-xs text-content-secondary">
              {description}
            </AlertDialog.Description>
          )}
          {children && <div className="mt-3">{children}</div>}
          <div className="mt-4 flex justify-end gap-2">
            <AlertDialog.Cancel asChild>
              <Button variant="secondary">Cancel</Button>
            </AlertDialog.Cancel>
            <AlertDialog.Action asChild>
              <Button variant="danger" onClick={onConfirm}>
                {confirmLabel}
              </Button>
            </AlertDialog.Action>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}
