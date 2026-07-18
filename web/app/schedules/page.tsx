"use client";

import { useEffect, useState } from "react";
import { Spinner } from "@/components/ascii/Spinner";
import { AddRoundelIcon, RefreshIcon, ScheduleIcon } from "@/components/icons";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Checkbox } from "@/components/ui/Checkbox";
import { SkeletonRow } from "@/components/ui/EmptySkeleton";
import { IconButton } from "@/components/ui/IconButton";
import { Input } from "@/components/ui/Input";
import { Panel } from "@/components/ui/Panel";
import { RetentionBadge } from "@/components/ui/RetentionBadge";
import { RowActionsMenu } from "@/components/ui/RowActionsMenu";
import { Tooltip } from "@/components/ui/Tooltip";
import { ApiError, api, type Destination, type Schedule } from "@/lib/api";

// A selected destination's retention: `keep` is the raw text of the "keep"
// input (blank = unlimited), kept as a string while editing so the field
// can be empty mid-typing rather than snapping to 0.
interface DestinationSelection {
  checked: boolean;
  keep: string;
}

export default function SchedulesPage() {
  const [schedules, setSchedules] = useState<Schedule[] | null>(null);
  const [destinations, setDestinations] = useState<Destination[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Schedule | null>(null);
  const [saving, setSaving] = useState(false);
  const [selectedDestinations, setSelectedDestinations] = useState<
    Record<number, DestinationSelection>
  >({});
  const [runningId, setRunningId] = useState<number | null>(null);
  const [justStarted, setJustStarted] = useState<{ scheduleId: number; runId: number } | null>(
    null,
  );

  function reload() {
    Promise.all([api.listSchedules(), api.listDestinations()])
      .then(([s, d]) => {
        setSchedules(s);
        setDestinations(d);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "failed to load schedules");
      });
  }

  useEffect(reload, []);

  function destinationName(id: number): string {
    return destinations?.find((d) => d.id === id)?.name ?? `#${id}`;
  }

  function openCreateForm() {
    setEditing(null);
    setSelectedDestinations({});
    setFormOpen(true);
  }

  function openEditForm(s: Schedule) {
    const sel: Record<number, DestinationSelection> = {};
    for (const link of s.destinations) {
      sel[link.destination_id] = {
        checked: true,
        keep: link.retention_count > 0 ? String(link.retention_count) : "",
      };
    }
    setEditing(s);
    setSelectedDestinations(sel);
    setFormOpen(true);
  }

  function closeForm() {
    setFormOpen(false);
    setEditing(null);
    setSelectedDestinations({});
  }

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const name = String(form.get("name") ?? "");
    const cronExpr = String(form.get("cron_expr") ?? "");

    const chosen = Object.entries(selectedDestinations)
      .filter(([, sel]) => sel.checked)
      .map(([id, sel]) => ({
        destination_id: Number(id),
        retention_count: sel.keep.trim() === "" ? 0 : Math.max(0, Math.trunc(Number(sel.keep))),
      }));

    setSaving(true);
    setError(null);
    try {
      if (editing) {
        await api.updateSchedule(editing.id, {
          name,
          cron_expr: cronExpr,
          enabled: editing.enabled,
          destinations: chosen,
        });
      } else {
        await api.createSchedule({
          name,
          cron_expr: cronExpr,
          enabled: true,
          destinations: chosen,
        });
      }
      closeForm();
      reload();
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : `failed to ${editing ? "update" : "create"} schedule`,
      );
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: number) {
    try {
      await api.deleteSchedule(id);
      reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "failed to delete schedule");
    }
  }

  async function handleRunNow(id: number) {
    setRunningId(id);
    setError(null);
    setJustStarted(null);
    try {
      const run = await api.runSchedule(id);
      setJustStarted({ scheduleId: id, runId: run.id });
      setTimeout(() => setJustStarted(null), 5000);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "failed to start backup run");
    } finally {
      setRunningId(null);
    }
  }

  async function handleToggleEnabled(s: Schedule) {
    try {
      await api.updateSchedule(s.id, {
        name: s.name,
        cron_expr: s.cron_expr,
        enabled: !s.enabled,
        destinations: s.destinations,
      });
      reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "failed to update schedule");
    }
  }

  return (
    <div className="flex flex-col gap-8">
      <div className="flex items-center justify-between">
        <h1 className="flex items-center gap-2 text-2xl font-medium tracking-tight text-content-accent">
          <ScheduleIcon className="size-4 text-content-accent" />
          Schedules
        </h1>
        <div className="flex items-center gap-2">
          {!formOpen && (
            <IconButton bordered onClick={reload} title="Refresh">
              <RefreshIcon className="size-3.5" />
            </IconButton>
          )}
          <Button
            variant={formOpen ? "danger" : "primary"}
            onClick={() => (formOpen ? closeForm() : openCreateForm())}
          >
            {!formOpen && <AddRoundelIcon className="size-3.5" />}
            {formOpen ? "Cancel" : "New schedule"}
          </Button>
        </div>
      </div>

      {error && (
        <p className="rounded-sm border border-status-error/50 px-3 py-2 font-mono text-xs text-status-error">
          {error}
        </p>
      )}

      {formOpen && (
        <Panel title={editing ? "Edit schedule" : "New schedule"}>
          <form
            key={editing ? `edit-${editing.id}` : "new"}
            onSubmit={handleSubmit}
            className="flex flex-col gap-4"
          >
            <Input
              label="Name"
              name="name"
              placeholder="daily-local"
              defaultValue={editing?.name}
              required
            />
            <Input
              label="Cron expression"
              name="cron_expr"
              mono
              placeholder="0 3 * * *"
              defaultValue={editing?.cron_expr}
              required
            />
            <div className="flex flex-col gap-1.5">
              <span className="text-xs font-medium text-content-secondary">Destinations</span>
              {!destinations || destinations.length === 0 ? (
                <p className="font-mono text-xs text-content-muted">
                  No destinations configured yet — add one on the Destinations page first.
                </p>
              ) : (
                <div className="flex flex-col gap-2">
                  {destinations.map((d) => {
                    const sel = selectedDestinations[d.id];
                    return (
                      // Fixed height so the row doesn't grow when the
                      // "keep" input (taller than the label's own text-sm
                      // line) appears on check.
                      <div key={d.id} className="flex h-7 items-center gap-2">
                        <label className="flex cursor-pointer items-center gap-2 text-sm">
                          <Checkbox
                            checked={!!sel?.checked}
                            onCheckedChange={(checked) =>
                              setSelectedDestinations((prev) => ({
                                ...prev,
                                [d.id]: {
                                  checked: checked === true,
                                  keep: prev[d.id]?.keep ?? "",
                                },
                              }))
                            }
                          />
                          {d.name}
                          <span className="font-mono text-xs text-content-muted">({d.type})</span>
                        </label>
                        {/* Unlabeled spacer, not the label itself, absorbs
                            the row's remaining width — a flex-1 label would
                            extend its (invisible) clickable area into the
                            blank space up to the input, so clicking there
                            toggled the checkbox by mistake. */}
                        <div className="flex-1" />
                        {sel?.checked && (
                          <Tooltip content="Number of backups to keep at this destination — blank keeps them all">
                            <input
                              type="number"
                              min={0}
                              step={1}
                              placeholder="keep"
                              value={sel.keep}
                              onChange={(e) =>
                                setSelectedDestinations((prev) => ({
                                  ...prev,
                                  [d.id]: { checked: true, keep: e.target.value },
                                }))
                              }
                              className="w-24 rounded-sm border border-border-default bg-surface-default px-2 py-1 text-left font-mono text-xs text-content-default outline-none focus:border-border-accent"
                            />
                          </Tooltip>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
            <Button type="submit" disabled={saving} className="w-full">
              {saving ? <Spinner label="saving..." /> : editing ? "Save changes" : "Save schedule"}
            </Button>
          </form>
        </Panel>
      )}

      {schedules === null ? (
        <Spinner label="loading..." />
      ) : (
        <Panel flush className="overflow-hidden">
          <table className="w-full table-fixed text-sm">
            <colgroup>
              <col className="w-40" />
              <col className="w-32" />
              <col />
              <col className="w-24" />
              <col className="w-24" />
              <col className="w-10" />
            </colgroup>
            <thead>
              <tr className="bg-surface-overlay text-left text-mono-xs text-content-secondary">
                <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                  Name
                </th>
                <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                  Cron
                </th>
                <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                  Destinations
                </th>
                <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                  Retention
                </th>
                <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                  Status
                </th>
                <th className="border-b border-border-default px-4 py-2 font-medium" />
              </tr>
            </thead>
            <tbody>
              {schedules.length === 0 ? (
                <>
                  <SkeletonRow columns={6} />
                  <SkeletonRow columns={6} />
                </>
              ) : (
                schedules.map((s) => (
                  <tr key={s.id} className="h-10 [&:last-child>td]:border-b-0">
                    <td className="truncate border-r border-b border-border-default px-4 text-content-default">
                      {s.name}
                    </td>
                    <td className="truncate border-r border-b border-border-default px-4 font-mono text-xs text-content-secondary">
                      {s.cron_expr}
                    </td>
                    <td className="border-r border-b border-border-default px-4">
                      <div className="flex flex-wrap gap-1">
                        {s.destinations.length === 0 ? (
                          <span className="font-mono text-xs text-content-muted">none</span>
                        ) : (
                          s.destinations.map((link) => (
                            <Badge key={link.destination_id} tone="neutral">
                              {destinationName(link.destination_id)}
                            </Badge>
                          ))
                        )}
                      </div>
                    </td>
                    <td className="border-r border-b border-border-default px-4">
                      <div className="flex flex-wrap gap-1">
                        {s.destinations.length === 0 ? (
                          <span className="text-mono-xs text-content-muted">N/A</span>
                        ) : (
                          s.destinations.map((link) => (
                            <RetentionBadge key={link.destination_id} keep={link.retention_count} />
                          ))
                        )}
                      </div>
                    </td>
                    <td className="border-r border-b border-border-default px-4">
                      <Badge tone={s.enabled ? "accent" : "neutral"}>
                        {s.enabled ? "enabled" : "disabled"}
                      </Badge>
                    </td>
                    <td className="border-b border-border-default px-4">
                      <div className="flex items-center justify-end gap-1">
                        {justStarted?.scheduleId === s.id && (
                          <span className="mr-1 text-mono-xs text-content-accent">
                            run #{justStarted.runId} started
                          </span>
                        )}
                        {runningId === s.id ? (
                          <Spinner minTime={0} />
                        ) : (
                          <RowActionsMenu
                            actions={[
                              { label: "Edit", onSelect: () => openEditForm(s) },
                              { label: "Run now", onSelect: () => handleRunNow(s.id) },
                              {
                                label: s.enabled ? "Disable" : "Enable",
                                onSelect: () => handleToggleEnabled(s),
                              },
                              {
                                label: "Delete",
                                tone: "danger",
                                onSelect: () => handleDelete(s.id),
                              },
                            ]}
                          />
                        )}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </Panel>
      )}
    </div>
  );
}
