"use client";

import { useEffect, useState } from "react";
import { Spinner } from "@/components/ascii/Spinner";
import { BackupIcon, RefreshIcon } from "@/components/icons";
import { Badge } from "@/components/ui/Badge";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { IconButton } from "@/components/ui/IconButton";
import { Panel } from "@/components/ui/Panel";
import { RelativeTime } from "@/components/ui/RelativeTime";
import { RowActionsMenu } from "@/components/ui/RowActionsMenu";
import { SkeletonRow } from "@/components/ui/EmptySkeleton";
import { formatBytes, formatDuration } from "@/lib/format";
import { ApiError, api, type BackupEntry, type Destination, type Schedule } from "@/lib/api";

const COLUMN_COUNT = 7;

function backupKey(b: Pick<BackupEntry, "destination_id" | "filename">): string {
  return `${b.destination_id}/${b.filename}`;
}

export default function BackupsPage() {
  const [backups, setBackups] = useState<BackupEntry[] | null>(null);
  const [schedules, setSchedules] = useState<Schedule[] | null>(null);
  const [destinations, setDestinations] = useState<Destination[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [removeTarget, setRemoveTarget] = useState<BackupEntry | null>(null);
  const [removingKey, setRemovingKey] = useState<string | null>(null);

  function reload() {
    Promise.all([api.listBackups(), api.listSchedules(), api.listDestinations()])
      .then(([b, s, d]) => {
        setBackups(b);
        setSchedules(s);
        setDestinations(d);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "failed to load backups");
      });
  }

  useEffect(reload, []);

  function scheduleName(id?: number): string {
    if (!id) return "unknown";
    return schedules?.find((s) => s.id === id)?.name ?? `#${id}`;
  }

  function destinationName(id: number): string {
    return destinations?.find((d) => d.id === id)?.name ?? `#${id}`;
  }

  function handleDownload(destinationId: number, filename: string) {
    const link = document.createElement("a");
    link.href = api.downloadBackupUrl(destinationId, filename);
    link.rel = "noopener";
    document.body.appendChild(link);
    link.click();
    link.remove();
  }

  async function handleConfirmRemove() {
    if (!removeTarget) return;
    const target = removeTarget;
    setRemoveTarget(null);
    setRemovingKey(backupKey(target));
    try {
      await api.deleteBackup(target.destination_id, target.filename);
      reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "failed to remove backup");
      setRemovingKey(null);
    }
  }

  return (
    <div className="flex flex-col gap-8">
      <div className="flex items-center justify-between">
        <h1 className="flex items-center gap-2 text-2xl font-medium tracking-tight text-content-accent">
          <BackupIcon className="size-4 text-content-accent" />
          Backups
        </h1>
        <IconButton bordered onClick={reload} title="Refresh">
          <RefreshIcon className="size-3.5" />
        </IconButton>
      </div>

      {error && (
        <p className="rounded-sm border border-status-error/50 px-3 py-2 font-mono text-xs text-status-error">
          {error}
        </p>
      )}

      <Panel flush className="overflow-hidden">
        <table className="w-full table-fixed text-sm">
          <colgroup>
            <col />
            <col className="w-36" />
            <col className="w-36" />
            <col className="w-40" />
            <col className="w-28" />
            <col className="w-24" />
            <col className="w-10" />
          </colgroup>
          <thead>
            <tr className="bg-surface-overlay text-left text-mono-xs text-content-secondary">
              <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                Filename
              </th>
              <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                Schedule
              </th>
              <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                Destination
              </th>
              <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                Started
              </th>
              <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                Duration
              </th>
              <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                Size
              </th>
              <th className="border-b border-border-default px-4 py-2 font-medium" />
            </tr>
          </thead>
          <tbody>
            {backups === null ? (
              <>
                <SkeletonRow columns={COLUMN_COUNT} pulse />
                <SkeletonRow columns={COLUMN_COUNT} pulse />
                <SkeletonRow columns={COLUMN_COUNT} pulse />
                <SkeletonRow columns={COLUMN_COUNT} pulse />
              </>
            ) : backups.length === 0 ? (
              <>
                <SkeletonRow columns={COLUMN_COUNT} />
                <SkeletonRow columns={COLUMN_COUNT} />
              </>
            ) : (
              backups.map((b) => {
                const key = backupKey(b);
                const removing = removingKey === key;
                // Opacity wraps each cell's *content*, not the <td> (and
                // never the <tr>) — an opacity'd ancestor composites its
                // whole subtree at that alpha, which would both break the
                // border it shares with the undimmed actions cell and dull
                // the Spinner's green along with everything else, instead
                // of borders staying put and just the data fading.
                const dim = `block truncate transition-opacity ${removing ? "opacity-40" : ""}`;
                return (
                  <tr key={key} className="h-10 [&:last-child>td]:border-b-0">
                    <td className="truncate border-r border-b border-border-default px-4 font-mono text-xs text-content-default">
                      <span className={dim}>{b.filename}</span>
                    </td>
                    <td className="truncate border-r border-b border-border-default px-4 text-content-default">
                      <span className={dim}>{scheduleName(b.schedule_id)}</span>
                    </td>
                    <td className="truncate border-r border-b border-border-default px-4">
                      <span className={`inline-block transition-opacity ${removing ? "opacity-40" : ""}`}>
                        <Badge tone="neutral">{destinationName(b.destination_id)}</Badge>
                      </span>
                    </td>
                    <td className="border-r border-b border-border-default px-4 font-mono text-xs text-content-secondary tabular-nums">
                      <span className={dim}>
                        <RelativeTime date={b.started_at} />
                      </span>
                    </td>
                    <td className="border-r border-b border-border-default px-4 font-mono text-xs text-content-secondary tabular-nums">
                      <span className={dim}>
                        {b.finished_at
                          ? formatDuration(
                              new Date(b.finished_at).getTime() - new Date(b.started_at).getTime(),
                            )
                          : "—"}
                      </span>
                    </td>
                    <td className="border-r border-b border-border-default px-4 font-mono text-xs text-content-secondary tabular-nums">
                      <span className={dim}>{formatBytes(b.size_bytes)}</span>
                    </td>
                    <td className="border-b border-border-default px-4">
                      <div className="flex justify-end">
                        {removing ? (
                          <Spinner minTime={0} />
                        ) : (
                          <RowActionsMenu
                            actions={[
                              ...(b.downloadable
                                ? [
                                    {
                                      label: "Download",
                                      onSelect: () => handleDownload(b.destination_id, b.filename),
                                    },
                                  ]
                                : []),
                              {
                                label: "Remove backup",
                                tone: "danger" as const,
                                onSelect: () => setRemoveTarget(b),
                              },
                            ]}
                          />
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </Panel>

      <ConfirmDialog
        open={removeTarget !== null}
        onOpenChange={(open) => {
          if (!open) setRemoveTarget(null);
        }}
        title={`Remove ${removeTarget?.filename ?? "this backup"}?`}
        description="This deletes the archive from its destination. It can't be undone."
        confirmLabel="Remove"
        onConfirm={handleConfirmRemove}
      />
    </div>
  );
}
