"use client";

import { useEffect, useState } from "react";
import { Spinner } from "@/components/ascii/Spinner";
import { AddRoundelIcon, DestinationIcon, RefreshIcon } from "@/components/icons";
import { Button } from "@/components/ui/Button";
import { Checkbox } from "@/components/ui/Checkbox";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { IconButton } from "@/components/ui/IconButton";
import { Input } from "@/components/ui/Input";
import { Panel } from "@/components/ui/Panel";
import { RowActionsMenu } from "@/components/ui/RowActionsMenu";
import { SkeletonCard } from "@/components/ui/EmptySkeleton";
import { ApiError, api, type Destination } from "@/lib/api";

// "Path" for local, "URL" for webdav, "Host" for ftp — matches
// CapacityBar's ValueCell pattern (a label over a mono value) rather than a
// plain caption line.
function destinationDetail(d: Destination): { label: string; value: string } {
  try {
    const cfg = JSON.parse(d.config_json) as Record<string, unknown>;
    if (typeof cfg.path === "string") return { label: "Path", value: cfg.path };
    if (typeof cfg.url === "string") {
      const dir = typeof cfg.dir === "string" && cfg.dir !== "" && cfg.dir !== "/" ? cfg.dir : "";
      const value = dir ? `${cfg.url.replace(/\/$/, "")}/${dir.replace(/^\//, "")}` : cfg.url;
      return { label: "URL", value };
    }
    if (typeof cfg.host === "string") {
      const port = typeof cfg.port === "number" ? cfg.port : 21;
      const dir = typeof cfg.dir === "string" && cfg.dir !== "" ? cfg.dir : "/";
      return { label: "Host", value: `${cfg.host}:${port}${dir}` };
    }
    return { label: "Config", value: d.config_json };
  } catch {
    return { label: "Config", value: d.config_json };
  }
}

type NewDestinationType = "local" | "webdav" | "ftp";

function parsedConfig(d: Destination | null): Record<string, unknown> {
  if (!d) return {};
  try {
    return JSON.parse(d.config_json) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function configField(cfg: Record<string, unknown>, key: string): string | undefined {
  const v = cfg[key];
  if (typeof v === "string") return v;
  if (typeof v === "number") return String(v);
  return undefined;
}

export default function DestinationsPage() {
  const [destinations, setDestinations] = useState<Destination[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Destination | null>(null);
  const [saving, setSaving] = useState(false);
  const [type, setType] = useState<NewDestinationType>("local");
  const [deleteTarget, setDeleteTarget] = useState<Destination | null>(null);
  const [purgeOnDelete, setPurgeOnDelete] = useState(false);

  function reload() {
    api.listDestinations().then(setDestinations).catch((err) => {
      setError(err instanceof Error ? err.message : "failed to load destinations");
    });
  }

  useEffect(reload, []);

  function openCreateForm() {
    setEditing(null);
    setType("local");
    setFormOpen(true);
  }

  function openEditForm(d: Destination) {
    setEditing(d);
    setType(d.type as NewDestinationType);
    setFormOpen(true);
  }

  function closeForm() {
    setFormOpen(false);
    setEditing(null);
  }

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const name = String(form.get("name") ?? "");
    const password = String(form.get("password") ?? "");

    let config: Record<string, unknown>;
    let secret: string | undefined;
    if (type === "local") {
      config = { path: String(form.get("path") ?? "") };
    } else if (type === "webdav") {
      config = {
        url: String(form.get("url") ?? ""),
        dir: String(form.get("dir") ?? ""),
        username: String(form.get("username") ?? ""),
      };
      if (password) secret = password;
    } else {
      config = {
        host: String(form.get("host") ?? ""),
        port: Number(form.get("port") ?? 21) || 21,
        dir: String(form.get("dir") ?? ""),
        username: String(form.get("username") ?? ""),
      };
      if (password) secret = password;
    }

    setSaving(true);
    setError(null);
    try {
      if (editing) {
        await api.updateDestination(editing.id, { name, type, config_json: JSON.stringify(config), secret });
      } else {
        await api.createDestination({ name, type, config_json: JSON.stringify(config), secret });
      }
      closeForm();
      reload();
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : `failed to ${editing ? "update" : "create"} destination`,
      );
    } finally {
      setSaving(false);
    }
  }

  async function handleConfirmDelete() {
    if (!deleteTarget) return;
    try {
      await api.deleteDestination(deleteTarget.id, purgeOnDelete);
      reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "failed to delete destination");
    } finally {
      setDeleteTarget(null);
      setPurgeOnDelete(false);
    }
  }

  return (
    <div className="flex flex-col gap-8">
      <div className="flex items-center justify-between">
        <h1 className="flex items-center gap-2 text-2xl font-medium tracking-tight text-content-accent">
          <DestinationIcon className="size-4 text-content-accent" />
          Destinations
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
            {formOpen ? "Cancel" : "New destination"}
          </Button>
        </div>
      </div>

      {error && (
        <p className="rounded-sm border border-status-error/50 px-3 py-2 font-mono text-xs text-status-error">
          {error}
        </p>
      )}

      {formOpen && (
        <Panel title={editing ? "Edit destination" : "New destination"}>
          <form
            key={editing ? `edit-${editing.id}` : "new"}
            onSubmit={handleSubmit}
            className="flex flex-col gap-4"
          >
            <Input label="Name" name="name" defaultValue={editing?.name} required />
            <label className="flex flex-col gap-1.5">
              <span className="text-xs font-medium text-content-secondary">Type</span>
              <select
                value={type}
                onChange={(e) => setType(e.target.value as NewDestinationType)}
                disabled={!!editing}
                className="cursor-pointer rounded-sm border border-border-default bg-surface-default px-3 py-2 text-sm text-content-default outline-none focus:border-border-accent disabled:cursor-not-allowed disabled:opacity-50"
              >
                <option value="local">local</option>
                <option value="webdav">webdav</option>
                <option value="ftp">ftp</option>
              </select>
            </label>
            {type === "local" ? (
              <>
                <Input
                  label="Path"
                  name="path"
                  mono
                  placeholder="backups/local"
                  defaultValue={configField(parsedConfig(editing), "path")}
                  required
                />
              </>
            ) : type === "webdav" ? (
              <>
                <Input
                  label="WebDAV URL"
                  name="url"
                  mono
                  placeholder="https://webdav.example.com"
                  defaultValue={configField(parsedConfig(editing), "url")}
                  required
                />
                <Input
                  label="Directory"
                  name="dir"
                  mono
                  placeholder="/loxbak"
                  defaultValue={configField(parsedConfig(editing), "dir")}
                />
                <Input
                  label="Username"
                  name="username"
                  defaultValue={configField(parsedConfig(editing), "username")}
                  required
                />
                <Input
                  label="Password"
                  name="password"
                  type="password"
                  placeholder={editing?.has_secret ? "leave blank to keep the current password" : undefined}
                  required={!editing}
                />
              </>
            ) : (
              <>
                <Input
                  label="Host"
                  name="host"
                  mono
                  placeholder="ftp.example.com"
                  defaultValue={configField(parsedConfig(editing), "host")}
                  required
                />
                <Input
                  label="Port"
                  name="port"
                  mono
                  type="number"
                  placeholder="21"
                  defaultValue={configField(parsedConfig(editing), "port")}
                />
                <Input
                  label="Directory"
                  name="dir"
                  mono
                  placeholder="/backups/loxbak"
                  defaultValue={configField(parsedConfig(editing), "dir")}
                />
                <Input
                  label="Username"
                  name="username"
                  defaultValue={configField(parsedConfig(editing), "username")}
                  required
                />
                <Input
                  label="Password"
                  name="password"
                  type="password"
                  placeholder={editing?.has_secret ? "leave blank to keep the current password" : undefined}
                  required={!editing}
                />
              </>
            )}
            <Button type="submit" disabled={saving} className="w-full">
              {saving ? (
                <Spinner label="saving..." />
              ) : editing ? (
                "Save changes"
              ) : (
                "Save destination"
              )}
            </Button>
          </form>
        </Panel>
      )}

      {destinations === null ? (
        <Spinner label="loading..." />
      ) : destinations.length === 0 ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SkeletonCard />
          <SkeletonCard />
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {destinations.map((d) => {
            const detail = destinationDetail(d);
            return (
              <Panel key={d.id} flush>
                <div className="flex items-center justify-between p-3">
                  <div className="flex items-center gap-2">
                    <DestinationIcon className="size-4 text-content-accent" />
                    <span className="text-mono-sm text-content-default normal-case">
                      {d.name}
                    </span>
                    <span className="text-mono-sm text-content-muted normal-case">
                      ({d.type})
                    </span>
                  </div>
                  <RowActionsMenu
                    actions={[
                      { label: "Edit", onSelect: () => openEditForm(d) },
                      {
                        label: "Delete",
                        tone: "danger",
                        onSelect: () => {
                          setPurgeOnDelete(false);
                          setDeleteTarget(d);
                        },
                      },
                    ]}
                  />
                </div>
                <div className="border-t border-border-subtle p-3">
                  <div className="mb-px text-mono-xs text-content-secondary">{detail.label}</div>
                  <div className="truncate font-mono text-sm text-content-default normal-case">
                    {detail.value}
                  </div>
                </div>
              </Panel>
            );
          })}
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        title={`Delete ${deleteTarget?.name ?? "destination"}?`}
        description="This removes it from loxbak. It can't be undone."
        confirmLabel="Delete"
        onConfirm={handleConfirmDelete}
      >
        <label className="flex cursor-pointer items-center gap-2 text-sm">
          <Checkbox
            checked={purgeOnDelete}
            onCheckedChange={(checked) => setPurgeOnDelete(checked === true)}
          />
          Also delete all backups stored there
        </label>
      </ConfirmDialog>
    </div>
  );
}
