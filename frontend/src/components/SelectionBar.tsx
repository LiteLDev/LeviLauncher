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

  return (
    <AnimatePresence>
      {isSelectMode && (
        <motion.div
          data-material-motion
          initial={{ opacity: 0, y: 50 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 50 }}
          className="fixed bottom-6 left-0 right-0 z-50 flex justify-center pointer-events-none"
        >
          <Card
            className={`pointer-events-auto shadow-lg ${LAYOUT.NAVBAR_BG} border border-border/50 dark:border-zinc-800/50 min-w-[300px]`}
          >
            <Card.Content className="py-2 px-4 flex-row items-center gap-4">
              <Checkbox
                isSelected={totalCount > 0 && selectedCount - hiddenSelectedCount === totalCount}
                onChange={onSelectAll}
                className={"group"}
              >
                <Checkbox.Content>
                  <Checkbox.Control className={"rounded-full"}>
                    <Checkbox.Indicator />
                  </Checkbox.Control>
                  <span>{t("common.select_all")}</span>
                </Checkbox.Content>
              </Checkbox>
              <div className="h-4 w-px bg-surface-quaternary" />
              <span className="text-sm text-muted">
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
                  isDisabled={isTransferDisabled ?? selectedCount === 0}
                  variant={"secondary"}
                >
                  {<FaExchangeAlt />}
                  {transferLabel || t("contentpage.transfer_resources_button")}
                </Button>
              )}
              <Button
                size="sm"
                onPress={onDelete}
                isDisabled={selectedCount === 0}
                variant={"danger-soft"}
              >
                {<FaTrash />}
                {t("common.delete")}
              </Button>
            </Card.Content>
          </Card>
        </motion.div>
      )}
    </AnimatePresence>
  );
};
