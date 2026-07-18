"use client";

import { useEffect, useState } from "react";
import { Spinner } from "@/components/ascii/Spinner";
import { DashboardIcon, PlayIcon, RefreshIcon } from "@/components/icons";
import { Badge } from "@/components/ui/Badge";
import { IconButton } from "@/components/ui/IconButton";
import { Panel } from "@/components/ui/Panel";
import { RelativeTime } from "@/components/ui/RelativeTime";
import { RetentionBadge } from "@/components/ui/RetentionBadge";
import { RunStatusBadge } from "@/components/ui/RunStatusBadge";
import { SkeletonRow } from "@/components/ui/EmptySkeleton";
import { formatBytes } from "@/lib/format";
import { ApiError, api, type Destination, type Run, type Schedule } from "@/lib/api";

export default function DashboardPage() {
  const [schedules, setSchedules] = useState<Schedule[] | null>(null);
  const [destinations, setDestinations] = useState<Destination[] | null>(null);
  const [runs, setRuns] = useState<Run[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [runningId, setRunningId] = useState<number | null>(null);
  const [justStarted, setJustStarted] = useState<number | null>(null);

  function reload() {
    Promise.all([api.listSchedules(), api.listDestinations(), api.listRuns({ limit: 10 })])
      .then(([s, d, r]) => {
        setSchedules(s);
        setDestinations(d);
        setRuns(r);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "failed to load dashboard data");
      });
  }

  useEffect(reload, []);

  // While any run is still "running", keep polling so its status/duration
  // updates without a manual refresh. This re-schedules itself off the
  // latest `runs` each time, and naturally stops once none are running
  // anymore.
  useEffect(() => {
    if (!runs?.some((r) => r.status === "running")) return;
    const timer = setTimeout(reload, 2000);
    return () => clearTimeout(timer);
  }, [runs]);

  function scheduleName(id?: number): string {
    if (!id) return "manual run";
    return schedules?.find((s) => s.id === id)?.name ?? `#${id}`;
  }

  function destinationName(id: number): string {
    return destinations?.find((d) => d.id === id)?.name ?? `#${id}`;
  }

  async function handleRunNow(id: number) {
    setRunningId(id);
    setError(null);
    setJustStarted(null);
    try {
      await api.runSchedule(id);
      setJustStarted(id);
      setTimeout(() => setJustStarted(null), 5000);
      reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "failed to start backup run");
    } finally {
      setRunningId(null);
    }
  }

  const loading = schedules === null || runs === null;

  return (
    <div className="flex flex-col gap-8">
      <div className="flex items-center justify-between">
        <h1 className="flex items-center gap-2 text-2xl font-medium tracking-tight text-content-accent">
          <DashboardIcon className="size-4 text-content-accent" />
          Dashboard
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

      {loading && !error ? (
        <Spinner label="loading..." />
      ) : (
        <>
          {schedules && schedules.length > 0 && (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {schedules.map((s) => (
                <Panel key={s.id}>
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <h3 className="text-sm font-medium">{s.name}</h3>
                      <p className="mt-1 font-mono text-xs text-content-secondary">
                        {s.cron_expr}
                      </p>
                    </div>
                    <div className="flex shrink-0 items-center gap-2">
                      {justStarted === s.id && (
                        <span className="text-mono-xs text-content-accent">started</span>
                      )}
                      <Badge tone={s.enabled ? "accent" : "neutral"}>
                        {s.enabled ? "enabled" : "disabled"}
                      </Badge>
                      {runningId === s.id ? (
                        <Spinner minTime={0} />
                      ) : (
                        <IconButton
                          size="sm"
                          tone="accent"
                          onClick={() => handleRunNow(s.id)}
                          title="Run now"
                        >
                          <PlayIcon className="size-3" />
                        </IconButton>
                      )}
                    </div>
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-1">
                    {s.destinations.length === 0 ? (
                      <span className="font-mono text-xs text-content-muted">
                        no destinations configured
                      </span>
                    ) : (
                      s.destinations.map((link) => (
                        <span key={link.destination_id} className="inline-flex items-center gap-1">
                          <Badge tone="neutral">{destinationName(link.destination_id)}</Badge>
                          {link.retention_count > 0 && <RetentionBadge keep={link.retention_count} />}
                        </span>
                      ))
                    )}
                  </div>
                </Panel>
              ))}
            </div>
          )}

          <div className="flex flex-col gap-3">
            <h2 className="text-mono-xs text-content-secondary">Recent runs</h2>
            <Panel flush className="overflow-hidden">
              <table className="w-full table-fixed text-sm">
                <colgroup>
                  <col className="w-28" />
                  <col />
                  <col className="w-40" />
                  <col className="w-24" />
                </colgroup>
                <thead>
                  <tr className="bg-surface-overlay text-left text-mono-xs text-content-secondary">
                    <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                      Status
                    </th>
                    <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                      Schedule
                    </th>
                    <th className="border-r border-b border-border-default px-4 py-2 font-medium">
                      Started
                    </th>
                    <th className="border-b border-border-default px-4 py-2 font-medium">
                      Size
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {runs && runs.length === 0 ? (
                    <>
                      <SkeletonRow columns={4} />
                      <SkeletonRow columns={4} />
                    </>
                  ) : (
                    runs?.map((run) => (
                      <tr key={run.id} className="h-10 [&:last-child>td]:border-b-0">
                        <td className="border-r border-b border-border-default px-4">
                          <RunStatusBadge status={run.status} />
                        </td>
                        <td className="truncate border-r border-b border-border-default px-4 text-content-default">
                          {scheduleName(run.schedule_id)}
                        </td>
                        <td className="border-r border-b border-border-default px-4 font-mono text-xs text-content-secondary tabular-nums">
                          <RelativeTime date={run.started_at} />
                        </td>
                        <td className="border-b border-border-default px-4 font-mono text-xs text-content-secondary tabular-nums">
                          {run.size_bytes ? formatBytes(run.size_bytes) : "—"}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </Panel>
          </div>
        </>
      )}
    </div>
  );
}
