import { type SVGProps } from "react";

// The "Fold" mark — a square split along its own diagonal into the two
// brand greens. Same shape as app/icon.svg (the favicon); this is the
// inline version used wherever the mark needs to sit next to text (nav,
// login) rather than as a standalone file.
export function LogoMark(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden {...props}>
      <path d="M4 4H20V20Z" fill="var(--color-loxone-green)" />
      <path d="M4 4V20H20Z" fill="var(--color-loxone-green-deep)" />
    </svg>
  );
}
