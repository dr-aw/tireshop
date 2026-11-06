import { useEffect, useRef, type ReactNode } from "react";

function rows(grid: HTMLDivElement) {
  const result: { top: number; details: HTMLDetailsElement[] }[] = [];
  for (const card of grid.querySelectorAll<HTMLElement>(":scope > .product")) {
    const top = card.getBoundingClientRect().top;
    let row = result.find((row) => Math.abs(row.top - top) < 2);
    if (!row) {
      row = { top, details: [] };
      result.push(row);
    }
    const details = card.querySelector<HTMLDetailsElement>(".catalog-specs");
    if (details) row.details.push(details);
  }
  return result;
}

export function ProductGrid({ children }: { children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const grid = ref.current;
    if (!grid) return;
    let width = grid.getBoundingClientRect().width;
    const observer = new ResizeObserver(() => {
      const nextWidth = grid.getBoundingClientRect().width;
      if (Math.abs(nextWidth - width) < 1) return;
      width = nextWidth;
      // Read all row positions before changing any details heights.
      const groups = rows(grid).map((row) => ({
        ...row,
        open: row.details.some((details) => details.open),
      }));
      for (const row of groups)
        for (const details of row.details) details.open = row.open;
    });
    observer.observe(grid);
    return () => observer.disconnect();
  }, []);
  return (
    <div
      ref={ref}
      className="products"
      onClick={(event) => {
        const summary = (event.target as HTMLElement).closest(
          ".catalog-specs > summary",
        );
        if (!summary || !ref.current?.contains(summary)) return;
        event.preventDefault();
        const clicked = summary.parentElement as HTMLDetailsElement;
        const row = rows(ref.current).find((row) =>
          row.details.includes(clicked),
        );
        const open = !clicked.open;
        for (const details of row?.details ?? []) details.open = open;
      }}
    >
      {children}
    </div>
  );
}
