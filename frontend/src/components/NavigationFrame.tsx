import { useLayoutEffect, useRef, useState } from "react";
import { LAYOUT } from "@/constants/layout";

/** Top bar and side rail share one backdrop and one continuous inner edge.
 * Sizes are in rem so the frame follows the fluid root font size. */
export const NavigationFrame = ({
  railRem,
  topRem,
}: {
  railRem: number;
  topRem: number;
}) => {
  const ref = useRef<HTMLDivElement>(null);
  const [geometry, setGeometry] = useState({ width: 0, height: 0, rail: 0, top: 0 });

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const update = () => {
      const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
      const rail = rem * railRem;
      const top = rem * topRem;
      const width = element.clientWidth;
      const height = element.clientHeight;
      setGeometry((previous) =>
        previous.width === width &&
        previous.height === height &&
        previous.rail === rail &&
        previous.top === top
          ? previous
          : { width, height, rail, top },
      );
    };
    update();
    // The root font size only changes with the viewport, which this sees.
    const observer = new ResizeObserver(update);
    observer.observe(element);
    return () => observer.disconnect();
  }, [railRem, topRem]);

  const { width, height, rail, top } = geometry;
  const radius = 20;
  const edge = `H ${rail + radius} A ${radius} ${radius} 0 0 0 ${rail} ${top + radius} V ${height}`;
  const contour = `M 0 0 H ${width} V ${top} ${edge} H 0 Z`;

  return (
    <div
      ref={ref}
      aria-hidden="true"
      className={`launcher-navigation-frame launcher-material-panel ${LAYOUT.NAVBAR_BG}`}
      style={{
        clipPath: `path("${contour}")`,
        visibility: width && height ? undefined : "hidden",
      }}
    >
      {width > 0 && height > 0 && (
        <svg
          className="launcher-navigation-outline"
          viewBox={`0 0 ${width} ${height}`}
          focusable="false"
        >
          <path d={`M ${width} ${top} ${edge}`} />
        </svg>
      )}
    </div>
  );
};
