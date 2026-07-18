// Thin fetch wrapper for the loxbak backend API.
//
// In production this app is a static export served by the same Go binary
// that exposes /api/*, so relative paths just work. In local dev the
// frontend runs on `next dev` (:3000) while the backend runs separately
// (:8080) — cross-origin, so we point at an absolute URL instead. This is
// purely a NODE_ENV heuristic (set automatically by `next dev` vs
// `next build`), not something you need to configure by hand; override
// with NEXT_PUBLIC_API_BASE if you're running the backend somewhere else.
const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE ??
  (process.env.NODE_ENV === "development" ? "http://localhost:8080" : "");

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      ...init,
      credentials: "include",
      headers: {
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
    });
  } catch {
    throw new ApiError(0, "Could not reach the loxbak backend.");
  }

  if (!res.ok) {
    let message = res.statusText || `request failed (${res.status})`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body?.error) message = body.error;
    } catch {
      // body wasn't JSON — keep the statusText fallback
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) return undefined as T;
  const text = await res.text();
  return text ? (JSON.parse(text) as T) : (undefined as T);
}

export interface Me {
  host: string;
  username: string;
}

export interface ScheduleDestinationLink {
  destination_id: number;
  retention_count: number;
}

export interface Schedule {
  id: number;
  name: string;
  cron_expr: string;
  enabled: boolean;
  destinations: ScheduleDestinationLink[];
}

export interface Destination {
  id: number;
  name: string;
  type: "local" | "webdav" | "ftp";
  config_json: string;
  has_secret: boolean;
}

export interface RunDestinationResult {
  id: number;
  destination_id?: number;
  status: "success" | "failed";
  size_bytes?: number;
  error?: string;
}

export interface Run {
  id: number;
  schedule_id?: number;
  started_at: string;
  finished_at?: string;
  status: "running" | "success" | "partial" | "failed";
  size_bytes?: number;
  error?: string;
  destinations?: RunDestinationResult[];
}

// One backup archive actually present at a destination — the Backups
// page's unit of display. Sourced live from GET /api/backups, which lists
// each configured destination's own contents rather than run history, so
// it includes backups that predate loxbak tracking runs at all.
export interface BackupEntry {
  destination_id: number;
  filename: string;
  size_bytes: number;
  schedule_id?: number;
  started_at: string;
  finished_at?: string;
  downloadable: boolean;
}

export interface LoginRequest {
  host: string;
  port?: number;
  username: string;
  password: string;
}

export const api = {
  login: (body: LoginRequest) =>
    request<Me>("/api/login", { method: "POST", body: JSON.stringify(body) }),
  logout: () => request<void>("/api/logout", { method: "POST" }),
  me: () => request<Me>("/api/me"),

  listSchedules: () => request<Schedule[]>("/api/schedules"),
  createSchedule: (body: {
    name: string;
    cron_expr: string;
    enabled: boolean;
    destinations: ScheduleDestinationLink[];
  }) => request<Schedule>("/api/schedules", { method: "POST", body: JSON.stringify(body) }),
  deleteSchedule: (id: number) => request<void>(`/api/schedules/${id}`, { method: "DELETE" }),
  runSchedule: (id: number) => request<Run>(`/api/schedules/${id}/run`, { method: "POST" }),
  updateSchedule: (
    id: number,
    body: {
      name: string;
      cron_expr: string;
      enabled: boolean;
      destinations: ScheduleDestinationLink[];
    },
  ) => request<Schedule>(`/api/schedules/${id}`, { method: "PUT", body: JSON.stringify(body) }),

  listDestinations: () => request<Destination[]>("/api/destinations"),
  createDestination: (body: { name: string; type: string; config_json: string; secret?: string }) =>
    request<Destination>("/api/destinations", { method: "POST", body: JSON.stringify(body) }),
  updateDestination: (
    id: number,
    body: { name: string; type: string; config_json: string; secret?: string },
  ) => request<Destination>(`/api/destinations/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteDestination: (id: number, purge?: boolean) =>
    request<void>(`/api/destinations/${id}${purge ? "?purge=true" : ""}`, { method: "DELETE" }),

  listRuns: (opts?: { limit?: number }) =>
    request<Run[]>(`/api/runs${opts?.limit ? `?limit=${opts.limit}` : ""}`),

  listBackups: () => request<BackupEntry[]>("/api/backups"),
  deleteBackup: (destinationId: number, filename: string) =>
    request<void>(`/api/backups/${destinationId}/${encodeURIComponent(filename)}`, {
      method: "DELETE",
    }),
  downloadBackupUrl: (destinationId: number, filename: string) =>
    `${API_BASE}/api/backups/${destinationId}/${encodeURIComponent(filename)}/download`,
};
