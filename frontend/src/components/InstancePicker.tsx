import React from "react";
import { Label, ListBox, Select } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { ListVersionMetas } from "bindings/github.com/liteldev/LeviLauncher/internal/app/versionservice";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { cn } from "@/utils/cn";

export type InstanceSummary = {
  name: string;
  gameVersion: string;
  type: string;
};

let cached: InstanceSummary[] | null = null;
let inflight: Promise<InstanceSummary[]> | null = null;

const fetchInstances = (): Promise<InstanceSummary[]> => {
  if (!inflight) {
    inflight = Promise.resolve((ListVersionMetas as any)?.())
      .then((metas: any[]) =>
        (metas || [])
          .map((meta: any) => ({
            name: String(meta?.name || ""),
            gameVersion: String(meta?.gameVersion || ""),
            type: String(meta?.type || "release").toLowerCase(),
          }))
          .filter((meta) => meta.name),
      )
      .then((list) => {
        cached = list;
        return list;
      })
      .finally(() => {
        inflight = null;
      });
  }
  return inflight;
};

/** Installed instances, shared across pickers; `refresh` re-reads the list. */
export function useInstanceList() {
  const [instances, setInstances] = React.useState<InstanceSummary[]>(
    () => cached ?? [],
  );
  const [loading, setLoading] = React.useState(cached === null);

  const refresh = React.useCallback(() => {
    setLoading(true);
    return fetchInstances()
      .then((list) => {
        setInstances(list);
        return list;
      })
      .catch(() => [] as InstanceSummary[])
      .finally(() => setLoading(false));
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  return { instances, loading, refresh };
}

type InstancePickerProps = {
  /** Visible label; omitted labels still get an accessible name. */
  label?: string;
  className?: string;
  size?: "sm" | "md";
  isDisabled?: boolean;
  /** Called after the current instance changes. */
  onChange?: (name: string) => void;
};

/**
 * Switches the app-wide current instance in place, so pages that act on the
 * current instance never have to send the user to the instance list.
 */
export const InstancePicker: React.FC<InstancePickerProps> = ({
  label,
  className,
  size = "md",
  isDisabled,
  onChange,
}) => {
  const { t } = useTranslation();
  const { currentVersionName, setCurrentVersionName } = useCurrentVersion();
  const { instances, loading } = useInstanceList();
  const selected = instances.some((item) => item.name === currentVersionName)
    ? currentVersionName
    : null;

  return (
    <Select
      aria-label={label ? undefined : t("instance_picker.label")}
      className={cn("min-w-0", size === "sm" ? "w-48" : "w-64", "max-w-full", className)}
      placeholder={
        loading
          ? t("common.loading")
          : instances.length === 0
            ? t("instance_picker.none_installed")
            : t("instance_picker.placeholder")
      }
      isDisabled={isDisabled || instances.length === 0}
      value={selected}
      onChange={(key) => {
        const name = String(key ?? "");
        if (!name || name === currentVersionName) return;
        setCurrentVersionName(name, "instance-picker");
        onChange?.(name);
      }}
    >
      {label && <Label>{label}</Label>}
      <Select.Trigger
        className={cn(
          COMPONENT_STYLES.select.trigger,
          // The game version is only shown in the list, not in the trigger.
          "[&_[data-instance-version]]:hidden",
          size === "sm" && "min-h-8 h-8 text-sm",
        )}
      >
        <Select.Value />
        <Select.Indicator />
      </Select.Trigger>
      <Select.Popover className={COMPONENT_STYLES.select.popoverContent}>
        <ListBox items={instances} className={COMPONENT_STYLES.select.listbox}>
          {(item) => (
            <ListBox.Item key={item.name} id={item.name} textValue={item.name}>
              <div className="flex min-w-0 flex-col">
                <span className="truncate text-sm">{item.name}</span>
                {item.gameVersion && (
                  <span
                    data-instance-version
                    className="truncate text-xs text-muted"
                  >
                    {item.gameVersion}
                  </span>
                )}
              </div>
              <ListBox.ItemIndicator />
            </ListBox.Item>
          )}
        </ListBox>
      </Select.Popover>
    </Select>
  );
};
