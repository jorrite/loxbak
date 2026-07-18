// Faint placeholder shapes for empty states: muted rectangles standing in
// for content, rather than a bare "nothing here" sentence, so an empty
// table/grid still shows the shape it'll eventually hold.

const BAR_WIDTHS = ["60%", "40%", "75%", "50%"];

/** A single placeholder table row, one bar per column, bordered the same as
 * a real row (grid lines should look identical whether the table has data
 * or not) — `last:` drops the bottom border on whichever one ends up being
 * the last row, so it doesn't double up with the panel's own frame.
 * `pulse` animates the bars for an actively-loading table; leave it off for
 * a genuinely-empty result, where nothing is happening. */
export function SkeletonRow({ columns, pulse = false }: { columns: number; pulse?: boolean }) {
  return (
    <tr className="h-10 [&:last-child>td]:border-b-0">
      {Array.from({ length: columns }).map((_, i) => (
        <td
          key={i}
          className={`border-b border-border-default px-4 ${
            i < columns - 1 ? "border-r" : ""
          }`}
        >
          <div
            className={`h-2 rounded-sm bg-content-muted/15 ${pulse ? "animate-pulse" : ""}`}
            style={{ width: BAR_WIDTHS[i % BAR_WIDTHS.length] }}
          />
        </td>
      ))}
    </tr>
  );
}

/** A placeholder card matching the shape of a real destination/schedule
 * card (title bar + detail bar + badge block), for empty card grids. */
export function SkeletonCard() {
  return (
    <div className="rounded-sm border border-border-subtle bg-surface-raised p-5">
      <div className="flex items-start justify-between">
        <div className="flex flex-col gap-2">
          <div className="h-2.5 w-28 rounded-sm bg-content-muted/15" />
          <div className="h-2 w-40 rounded-sm bg-content-muted/10" />
        </div>
        <div className="h-4 w-12 rounded-sm bg-content-muted/15" />
      </div>
    </div>
  );
}
