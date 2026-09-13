import { useLayoutEffect, useRef, useState } from "react";
import { LAYOUT } from "@/constants/layout";

/** Top bar and side rail share one backdrop and one continuous inner edge. */
export const NavigationFrame = () => {
  const ref = useRef<HTMLDivElement>(null);
  const [geometry, setGeometry] = useState({ width: 0, height: 0, rail: 0 });

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const update = () => {
      // Match the h-14 / w-14 controls in CSS pixels, including root font scaling.
      const rail = parseFloat(getComputedStyle(document.documentElement).fontSize) * 3.5;
      const width = element.clientWidth;
      const height = element.clientHeight;
      setGeometry((previous) =>
        previous.width === width && previous.height === height && previous.rail === rail
          ? previous
          : { width, height, rail },
      );
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const { width, height, rail } = geometry;
  const radius = 20;
  const edge = `H ${rail + radius} A ${radius} ${radius} 0 0 0 ${rail} ${rail + radius} V ${height}`;
  const contour = `M 0 0 H ${width} V ${rail} ${edge} H 0 Z`;

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
          <path d={`M ${width} ${rail} ${edge}`} />
        </svg>
      )}
    </div>
  );
};
