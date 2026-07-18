"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { BootSequence } from "@/components/ascii/BootSequence";
import { Spinner } from "@/components/ascii/Spinner";
import { LogoMark } from "@/components/LogoMark";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Panel } from "@/components/ui/Panel";
import { ApiError, api } from "@/lib/api";

type Status = "idle" | "pending" | "error" | "success";

export default function LoginPage() {
  const router = useRouter();
  const [status, setStatus] = useState<Status>("idle");
  const [error, setError] = useState<string | null>(null);
  const [host, setHost] = useState("");
  const [username, setUsername] = useState("admin");

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const password = String(form.get("password") ?? "");

    setStatus("pending");
    setError(null);
    try {
      await api.login({ host, username, password });
      setStatus("success");
      setTimeout(() => router.replace("/"), 900);
    } catch (err) {
      setStatus("error");
      setError(err instanceof ApiError ? err.message : "Could not reach the loxbak backend.");
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex items-center justify-center gap-2">
          <LogoMark className="size-5 shrink-0" />
          <span className="text-lg font-medium tracking-tight">loxbak</span>
        </div>

        <Panel>
          {status === "success" ? (
            <BootSequence lines={[`authenticated as ${username}@${host}`, "redirecting..."]} />
          ) : (
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <Input
                label="Miniserver host"
                name="host"
                mono
                placeholder="192.168.1.77"
                required
                value={host}
                onChange={(e) => setHost(e.target.value)}
                disabled={status === "pending"}
              />
              <Input
                label="Username"
                name="username"
                placeholder="admin"
                required
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={status === "pending"}
              />
              <Input
                label="Password"
                name="password"
                type="password"
                required
                disabled={status === "pending"}
              />

              {status === "error" && error && (
                <p className="rounded-sm border border-status-error/50 px-3 py-2 font-mono text-xs text-status-error">
                  {error}
                </p>
              )}

              <Button type="submit" className="mt-1 w-full" disabled={status === "pending"}>
                {status === "pending" ? (
                  <Spinner label={`connecting to ${host || "miniserver"}...`} />
                ) : (
                  "Connect"
                )}
              </Button>
            </form>
          )}
        </Panel>
      </div>
    </div>
  );
}
