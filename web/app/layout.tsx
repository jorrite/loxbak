import type { Metadata } from "next";
import { GeistSans } from "geist/font/sans";
import { GeistMono } from "geist/font/mono";
import { NavShell } from "@/components/nav/NavShell";
import { TooltipProvider } from "@/components/ui/Tooltip";
import "./globals.css";

export const metadata: Metadata = {
  title: "loxbak",
  description: "Scheduled backups for your Loxone Miniserver.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      data-theme="dark"
      className={`${GeistSans.variable} ${GeistMono.variable} h-full antialiased`}
    >
      <body className="min-h-full bg-surface-default text-content-default font-sans">
        <TooltipProvider>
          <NavShell>{children}</NavShell>
        </TooltipProvider>
      </body>
    </html>
  );
}
