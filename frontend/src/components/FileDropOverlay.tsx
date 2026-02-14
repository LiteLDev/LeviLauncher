import React from "react";
import { createPortal } from "react-dom";
import { FiUploadCloud } from "react-icons/fi";
import { motion, AnimatePresence } from "framer-motion";
import { cn } from "@/utils/cn";
import { LAYOUT } from "@/constants/layout";

interface FileDropOverlayProps {
  isDragActive: boolean;
  className?: string;
  text?: React.ReactNode;
}

export const FileDropOverlay: React.FC<FileDropOverlayProps> = ({
  isDragActive,
  className,
  text,
}) => {
  return createPortal(
    <AnimatePresence>
      {isDragActive && (
        <div
          className={cn("fixed inset-0 z-[100] pointer-events-none", className)}
        >
          <motion.div
            initial={{ opacity: 0 }}
            animate={{
              opacity: 1,
              transition: { duration: 0.3, ease: "easeOut" },
            }}
            exit={{ opacity: 0, transition: { duration: 0.2, ease: "easeIn" } }}
            className="absolute inset-0 backdrop-blur-md backdrop-saturate-150 bg-black/30"
          />

          <div className="absolute inset-0 flex items-center justify-center p-4">
            <motion.div
              initial={{ scale: 0.9, y: 20, opacity: 0 }}
              animate={{ scale: 1, y: 0, opacity: 1 }}
              exit={{ scale: 0.9, y: 20, opacity: 0 }}
              transition={{ type: "spring", damping: 25, stiffness: 300 }}
              className={cn(
                LAYOUT.GLASS_CARD.BASE,
                "p-8 flex flex-col items-center gap-4 text-center max-w-sm mx-4 border border-white/20 dark:border-white/10",
              )}
            >
              <div className="p-4 rounded-full bg-primary-500/10 text-primary-500 ring-1 ring-primary-500/20">
                <FiUploadCloud className="w-10 h-10" />
              </div>
              <div className="space-y-2">
                <div className="text-xl font-bold text-default-900 dark:text-white">
                  {text}
                </div>
              </div>
            </motion.div>
          </div>
        </div>
      )}
    </AnimatePresence>,
    document.body,
  );
};
