import React from "react";
import { Button, Kbd, Modal } from "@heroui/react";
import { useTranslation } from "react-i18next";
import {
  BaseModal,
  BaseModalBody,
  BaseModalFooter,
  BaseModalHeader,
} from "@/components/BaseModal";
import { formatCombo, useKeybinding } from "@/utils/KeybindingContext";
import { OPEN_SHORTCUTS_HELP_EVENT } from "@/utils/commands";

/** Lists every registered keybinding that carries a description. */
export const ShortcutsHelp: React.FC = () => {
  const { t } = useTranslation();
  const { bindings } = useKeybinding();
  const [isOpen, setIsOpen] = React.useState(false);

  React.useEffect(() => {
    const open = () => setIsOpen(true);
    window.addEventListener(OPEN_SHORTCUTS_HELP_EVENT, open);
    return () => window.removeEventListener(OPEN_SHORTCUTS_HELP_EVENT, open);
  }, []);

  // Bindings that share a description (Ctrl+K and Ctrl+P) become one row.
  const rows = React.useMemo(() => {
    const byDescription = new Map<string, string[]>();
    for (const binding of bindings) {
      if (!binding.description) continue;
      const combos = byDescription.get(binding.description) ?? [];
      combos.push(binding.combo);
      byDescription.set(binding.description, combos);
    }
    return Array.from(byDescription, ([description, combos]) => ({
      description,
      combos,
    }));
  }, [bindings]);

  return (
    <BaseModal
      isOpen={isOpen}
      onOpenChange={setIsOpen}
      isDismissable
      size="compact"
      className="max-w-[min(32rem,100%)]"
    >
      <BaseModalHeader>
        <Modal.Heading className="text-lg font-semibold">
          {t("palette.shortcuts_title")}
        </Modal.Heading>
      </BaseModalHeader>
      <BaseModalBody>
        <dl className="flex flex-col divide-y divide-border/60">
          {rows.map((row) => (
            <div
              key={row.description}
              className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 py-2"
            >
              <dt className="min-w-0 text-foreground">{row.description}</dt>
              <dd className="flex flex-wrap items-center gap-2">
                {row.combos.map((combo, index) => (
                  <React.Fragment key={combo}>
                    {index > 0 && (
                      <span className="text-xs text-muted">/</span>
                    )}
                    <span className="inline-flex items-center gap-1">
                      {formatCombo(combo).map((label) => (
                        <Kbd key={label}>
                          <Kbd.Content>{label}</Kbd.Content>
                        </Kbd>
                      ))}
                    </span>
                  </React.Fragment>
                ))}
              </dd>
            </div>
          ))}
        </dl>
      </BaseModalBody>
      <BaseModalFooter className="justify-end">
        <Button variant="secondary" onPress={() => setIsOpen(false)}>
          {t("common.close")}
        </Button>
      </BaseModalFooter>
    </BaseModal>
  );
};
