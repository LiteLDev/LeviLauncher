import React, { forwardRef } from "react";
import { motion } from "framer-motion";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";

interface PageContainerProps extends React.HTMLAttributes<HTMLDivElement> {
  children: React.ReactNode;
  animate?: boolean;
  /** Drop the `--page-max-width` cap, e.g. for edge-to-edge media pages. */
  fullBleed?: boolean;
}

export const PageContainer = forwardRef<HTMLDivElement, PageContainerProps>(
  ({ children, className, animate = true, fullBleed = false, ...props }, ref) => {
    const styles = cn(LAYOUT.PAGE.CONTAINER, fullBleed && "max-w-none", className);

    if (!animate) {
      return (
        <div ref={ref} className={styles} {...props}>
          {children}
        </div>
      );
    }

    return (
      <motion.div
        data-material-motion
        ref={ref}
        className={styles}
        initial={LAYOUT.PAGE.ANIMATION.initial}
        animate={LAYOUT.PAGE.ANIMATION.animate}
        transition={LAYOUT.PAGE.ANIMATION.transition}
        {...(props as any)}
      >
        {children}
      </motion.div>
    );
  },
);

PageContainer.displayName = "PageContainer";
