import { Button, Card, Checkbox } from "@heroui/react";

import React from "react";
import { useTranslation } from "react-i18next";

import { FaExchangeAlt, FaTrash } from "react-icons/fa";
import { motion, AnimatePresence } from "framer-motion";

import { LAYOUT } from "@/constants/layout";

interface SelectionBarProps {
  selectedCount: number;
  totalCount: number;
  hiddenSelectedCount?: number;
  onSelectAll: (isSelected: boolean) => void;
  onDelete: () => void;
  isSelectMode: boolean;
  onTransfer?: () => void;
  transferLabel?: string;
  isTransferDisabled?: boolean;
}

export const SelectionBar: React.FC<SelectionBarProps> = ({
  selectedCount,
  totalCount,
  hiddenSelectedCount = 0,
  onSelectAll,
  onDelete,
  isSelectMode,
  onTransfer,
  transferLabel,
  isTransferDisabled,
}) => {
  const { t } = useTranslation();
  const resolvedTransferLabel =
    transferLabel || t("contentpage.transfer_resources_button");
  const transferDisabled = isTransferDisabled ?? selectedCount === 0;

  return (
    <>
      {/* Reserve room at the end of the page (flex `order-last`) so the
          floating bar never covers the last row of content. */}
      {isSelectMode && (
        <div
          aria-hidden="true"
          className={`order-last shrink-0 ${hiddenSelectedCount > 0 ? "h-24" : "h-20"}`}
        />
      )}
      <AnimatePresence>
        {isSelectMode && (
          <motion.div
            data-material-motion
            initial={{ opacity: 0, y: 50 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 50 }}
            className="fixed bottom-[clamp(0.75rem,2vh,1.5rem)] left-0 right-0 z-50 flex justify-center px-4 pointer-events-none"
          >
            <Card
              className={`pointer-events-auto shadow-lg ${LAYOUT.NAVBAR_BG} border border-border/50 dark:border-zinc-800/50 min-w-[min(20rem,100%)] max-w-full`}
            >
              <Card.Content className="py-2 px-4 flex-row items-center gap-[clamp(0.5rem,1.5vw,1rem)]">
                <Checkbox
                  isSelected={totalCount > 0 && selectedCount - hiddenSelectedCount === totalCount}
                  onChange={onSelectAll}
                  className={"group shrink-0"}
                >
                  <Checkbox.Content>
                    <Checkbox.Control className={"rounded-full"}>
                      <Checkbox.Indicator />
                    </Checkbox.Control>
                    <span className="whitespace-nowrap">{t("common.select_all")}</span>
                  </Checkbox.Content>
                </Checkbox>
                <div className="h-4 w-px shrink-0 bg-surface-quaternary" />
                <span className="min-w-0 text-sm text-muted whitespace-nowrap">
                  {t("common.selected_count", { count: selectedCount })}
                  {hiddenSelectedCount > 0 && (
                    <span className="block text-xs" role="status">
                      {t("contentpage.selected_outside_filter", { count: hiddenSelectedCount })}
                    </span>
                  )}
                </span>
                <div className="flex-1" />
                {onTransfer && (
                  <Button
                    size="sm"
                    onPress={onTransfer}
                    isDisabled={transferDisabled}
                    variant={"secondary"}
                    aria-label={resolvedTransferLabel}
                    className="shrink-0 max-md:min-w-0 max-md:px-2.5"
                  >
                    {<FaExchangeAlt aria-hidden="true" />}
                    {/* Narrow windows: label collapses, icon and accessible name stay. */}
                    <span className="max-md:sr-only">{resolvedTransferLabel}</span>
                  </Button>
                )}
                <Button
                  size="sm"
                  onPress={onDelete}
                  isDisabled={selectedCount === 0}
                  variant={"danger-soft"}
                  className="shrink-0"
                >
                  {<FaTrash />}
                  {t("common.delete")}
                </Button>
              </Card.Content>
            </Card>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
};
