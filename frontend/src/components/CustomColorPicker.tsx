import { FieldError, Input, Label, TextField } from "@heroui/react";
import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

interface CustomColorPickerProps {
  color: string;
  onChange: (color: string) => void;
  className?: string;
}

const normalizeHex = (value: string): string | null => {
  const match = value.trim().match(/^#?([0-9a-f]{6})$/i);
  return match ? `#${match[1].toUpperCase()}` : null;
};

const toChannels = (hex: string): string[] =>
  [1, 3, 5].map((offset) => String(parseInt(hex.slice(offset, offset + 2), 16)));

const isValidChannel = (value: string): boolean =>
  /^\d{1,3}$/.test(value) && Number(value) <= 255;

export const CustomColorPicker: React.FC<CustomColorPickerProps> = ({
  color,
  onChange,
  className,
}) => {
  const { t } = useTranslation();
  const safeColor = normalizeHex(color) ?? "#000000";
  const [hexValue, setHexValue] = useState(safeColor);
  const [channels, setChannels] = useState(() => toChannels(safeColor));

  useEffect(() => {
    setHexValue(safeColor);
    setChannels(toChannels(safeColor));
  }, [safeColor]);

  const handleHexChange = (value: string) => {
    setHexValue(value);
    const normalized = normalizeHex(value);
    if (!normalized) return;
    setChannels(toChannels(normalized));
    onChange(normalized);
  };

  const handleChannelChange = (index: number, value: string) => {
    const next = channels.map((channel, currentIndex) =>
      currentIndex === index ? value : channel,
    );
    setChannels(next);
    if (!next.every(isValidChannel)) return;
    const hex = `#${next
      .map((channel) => Number(channel).toString(16).padStart(2, "0"))
      .join("")}`.toUpperCase();
    setHexValue(hex);
    onChange(hex);
  };

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      <div className="flex items-end gap-3">
        <div
          role="img"
          aria-label={t("audit.mods.color_preview", { hex: safeColor })}
          className="h-12 w-16 shrink-0 rounded-lg border border-border"
          style={{ backgroundColor: safeColor }}
        />
        <TextField
          value={hexValue}
          onChange={handleHexChange}
          isInvalid={!normalizeHex(hexValue)}
          className={cn("group flex-1", COMPONENT_STYLES.input.mainWrapper)}
        >
          <Label className={COMPONENT_STYLES.input.label}>HEX</Label>
          <Input
            spellCheck={false}
            className={cn(
              COMPONENT_STYLES.input.inputWrapper,
              COMPONENT_STYLES.input.input,
              "min-h-8 text-sm",
            )}
          />
          <FieldError>{t("audit.mods.invalid_hex")}</FieldError>
        </TextField>
      </div>
      <div className="grid grid-cols-3 gap-3">
        {(["red", "green", "blue"] as const).map((channel, index) => (
          <TextField
            key={channel}
            value={channels[index]}
            onChange={(value) => handleChannelChange(index, value)}
            isInvalid={!isValidChannel(channels[index])}
            className={cn("group min-w-0", COMPONENT_STYLES.input.mainWrapper)}
          >
            <Label className={COMPONENT_STYLES.input.label}>
              {t(`audit.mods.rgb_${channel}`)}
            </Label>
            <Input
              inputMode="numeric"
              spellCheck={false}
              className={cn(
                COMPONENT_STYLES.input.inputWrapper,
                COMPONENT_STYLES.input.input,
                "min-h-8 text-sm",
              )}
            />
            <FieldError>{t("audit.mods.invalid_rgb")}</FieldError>
          </TextField>
        ))}
      </div>
    </div>
  );
};
