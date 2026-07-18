import { Badge } from "@/components/ui/Badge";

/** A schedule-destination's "keep N" value — the orange retention tone
 * when set, or a ghost "N/A" (no badge chrome at all) when unset, so an
 * unlimited-retention destination doesn't read as a zero. */
export function RetentionBadge({ keep }: { keep: number }) {
  if (keep <= 0) {
    return <span className="text-mono-xs text-content-muted">N/A</span>;
  }
  return <Badge tone="retention">{keep}</Badge>;
}
