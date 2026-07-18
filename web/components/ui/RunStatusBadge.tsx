import { Badge } from "@/components/ui/Badge";
import type { Run } from "@/lib/api";

const LABEL: Record<Run["status"], string> = {
  running: "RUNNING",
  success: "SUCCEEDED",
  partial: "PARTIAL",
  failed: "FAILED",
};

const TONE: Record<Run["status"], "accent" | "neutral" | "error" | "running"> = {
  running: "running",
  success: "accent",
  partial: "neutral",
  failed: "error",
};

/** Same ring-pill treatment as the schedule ENABLED/DISABLED badge, applied
 * to a run's status — green for a succeeded run, red for a failed one, and
 * a pulsing green while it's still running. */
export function RunStatusBadge({ status }: { status: Run["status"] }) {
  return <Badge tone={TONE[status]}>{LABEL[status]}</Badge>;
}
