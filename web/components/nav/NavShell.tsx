"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { BackupIcon, DashboardIcon, DestinationIcon, ScheduleIcon } from "@/components/icons";
import { LogoMark } from "@/components/LogoMark";
import { Tooltip } from "@/components/ui/Tooltip";
import { api, type Me } from "@/lib/api";

// Destinations before Schedules: a destination has to exist before a
// schedule can reference one, so the nav order follows the setup order.
// Backups last: it's the output of everything before it.
const NAV_ITEMS = [
  { href: "/", label: "Dashboard", icon: DashboardIcon },
  { href: "/destinations/", label: "Destinations", icon: DestinationIcon },
  { href: "/schedules/", label: "Schedules", icon: ScheduleIcon },
  { href: "/backups/", label: "Backups", icon: BackupIcon },
];

type AuthState = "checking" | "authed" | "anonymous";

export function NavShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const isLogin = pathname?.startsWith("/login") ?? false;

  const [authState, setAuthState] = useState<AuthState>("checking");
  const [me, setMe] = useState<Me | null>(null);

  useEffect(() => {
    if (isLogin) return;
    let cancelled = false;
    api.me().then(
      (m) => {
        if (cancelled) return;
        setMe(m);
        setAuthState("authed");
      },
      () => {
        if (cancelled) return;
        setAuthState("anonymous");
        router.replace("/login/");
      },
    );
    return () => {
      cancelled = true;
    };
  }, [isLogin, router]);

  if (isLogin) return <>{children}</>;

  if (authState !== "authed" || !me) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <span className="font-mono text-xs text-content-muted">
          {authState === "checking" ? "checking session…" : "redirecting…"}
        </span>
      </div>
    );
  }

  async function handleLogout() {
    try {
      await api.logout();
    } finally {
      router.replace("/login/");
    }
  }

  return (
    <div className="flex min-h-screen">
      <aside className="flex w-56 shrink-0 flex-col border-r border-border-default bg-surface-raised">
        <div className="flex items-center gap-2 px-5 py-4">
          <LogoMark className="size-4 shrink-0" />
          <span className="text-sm font-medium tracking-tight">loxbak</span>
        </div>
        <div className="border-t border-border-subtle px-5 pt-3 pb-1 text-left">
          <span className="text-mono-xs text-content-muted">Navigation</span>
        </div>
        <nav className="flex flex-col gap-0.5 px-2 py-2">
          {NAV_ITEMS.map((item) => {
            const isActive =
              item.href === "/" ? pathname === "/" : pathname?.startsWith(item.href);
            const Icon = item.icon;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`relative flex items-center gap-1.5 rounded-sm px-2 py-1 text-[11px] transition-colors ${
                  isActive
                    ? "text-content-accent"
                    : "text-content-secondary hover:text-content-default"
                }`}
              >
                {isActive && (
                  <span
                    className="absolute -inset-x-1 -inset-y-px rounded-sm bg-surface-accent/10"
                    aria-hidden
                  />
                )}
                <Icon className="relative size-3 shrink-0" />
                <span className="relative">{item.label}</span>
              </Link>
            );
          })}
        </nav>
        <div className="mt-auto flex flex-col gap-2 px-5 py-4">
          <Tooltip content={`${me.username}@${me.host}`}>
            <span className="truncate text-mono-xs text-content-secondary">
              {me.username}@{me.host}
            </span>
          </Tooltip>
          <div className="flex items-center justify-between">
            <span className="text-mono-xs text-content-muted">v0.1.0</span>
            <button
              onClick={handleLogout}
              className="cursor-pointer text-mono-xs text-content-muted transition-colors hover:text-content-default"
            >
              log out
            </button>
          </div>
        </div>
      </aside>
      <main className="flex-1 px-8 py-8">{children}</main>
    </div>
  );
}
