import React from "react";
import { useTranslation } from "react-i18next";
import { Button, Checkbox, Card, CardBody } from "@heroui/react";
import { FaTrash } from "react-icons/fa";
import { motion, AnimatePresence } from "framer-motion";

import { LAYOUT } from "@/constants/layout";

interface SelectionBarProps {
  selectedCount: number;
  totalCount: number;
  onSelectAll: (isSelected: boolean) => void;
  onDelete: () => void;
  isSelectMode: boolean;
}

export const SelectionBar: React.FC<SelectionBarProps> = ({
  selectedCount,
  totalCount,
  onSelectAll,
  onDelete,
  isSelectMode,
}) => {
  const { t } = useTranslation();

  return (
    <AnimatePresence>
      {isSelectMode && (
        <motion.div
          initial={{ opacity: 0, y: 50 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: 50 }}
          className="fixed bottom-6 left-0 right-0 z-50 flex justify-center pointer-events-none"
        >
          <Card
            className={`pointer-events-auto shadow-lg ${LAYOUT.NAVBAR_BG} border border-default-200/50 dark:border-zinc-800/50 min-w-[300px]`}
          >
            <CardBody className="py-2 px-4 flex-row items-center gap-4">
              <Checkbox
                isSelected={totalCount > 0 && selectedCount === totalCount}
                onValueChange={onSelectAll}
                radius="full"
              >
                {t("common.select_all")}
              </Checkbox>

              <div className="h-4 w-px bg-default-300" />

              <span className="text-small text-default-500">
                {t("common.selected_count", { count: selectedCount })}
              </span>

              <div className="flex-1" />

              <Button
                size="sm"
                color="danger"
                variant="flat"
                startContent={<FaTrash />}
                onPress={onDelete}
                isDisabled={selectedCount === 0}
              >
                {t("common.delete")}
              </Button>
            </CardBody>
          </Card>
        </motion.div>
      )}
    </AnimatePresence>
  );
};
