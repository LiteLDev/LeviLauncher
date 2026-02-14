import React, { useEffect, useState } from "react";
import {
  ColorPicker,
  ColorArea,
  ColorThumb,
  ColorSlider,
  SliderTrack,
  parseColor,
} from "react-aria-components";
import { Input } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/utils/cn";
import { COMPONENT_STYLES } from "@/constants/componentStyles";

interface CustomColorPickerProps {
  color: string;
  onChange: (color: string) => void;
  className?: string;
}

export const CustomColorPicker: React.FC<CustomColorPickerProps> = ({
  color,
  onChange,
  className,
}) => {
  const { t } = useTranslation();

  const [hexValue, setHexValue] = useState(color);

  useEffect(() => {
    if (color.toUpperCase() !== hexValue.toUpperCase()) {
      setHexValue(color.toUpperCase());
    }
  }, [color]);

  const safeColor = React.useMemo(() => {
    try {
      return parseColor(color.startsWith("#") ? color : `#${color}`).toFormat(
        "hsb",
      );
    } catch {
      return parseColor("#000000").toFormat("hsb");
    }
  }, [color]);

  const handleHexChange = (val: string) => {
    setHexValue(val);
    if (/^#[0-9A-F]{6}$/i.test(val)) {
      onChange(val);
    } else if (/^[0-9A-F]{6}$/i.test(val)) {
      onChange("#" + val);
    }
  };

  return (
    <div className={cn(className)}>
      <ColorPicker
        value={safeColor}
        onChange={(c) => {
          const newHex = c.toString("hex");
          onChange(newHex);
          setHexValue(newHex.toUpperCase());
        }}
      >
        <div className="flex flex-row gap-4 h-40">
          <div className="aspect-square h-full rounded-xl overflow-hidden shadow-sm border border-default-200/50 relative group">
            <ColorArea
              xChannel="saturation"
              yChannel="brightness"
              className="w-full h-full"
            >
              <ColorThumb className="w-4 h-4 rounded-full border-2 border-white shadow-[0_0_10px_rgba(0,0,0,0.3)] ring-1 ring-black/20 z-10 focus:outline-none focus:ring-2 focus:ring-primary-500 transition-transform dragging:scale-125 cursor-grab active:cursor-grabbing" />
            </ColorArea>
          </div>

          <div className="flex flex-col gap-3 flex-1 justify-center">
            <div className="flex flex-col gap-1">
              <div className="flex justify-between items-center">
                <span className="text-tiny font-bold text-default-400 uppercase tracking-wider">
                  {t("settings.appearance.hue") || "HUE"}
                </span>
              </div>
              <ColorSlider channel="hue" className="w-full touch-none">
                <SliderTrack className="h-4 w-full rounded-lg border border-default-200/50 relative overflow-hidden ring-offset-2 ring-offset-background focus-within:ring-2 focus-within:ring-primary-500 transition-all">
                  <ColorThumb className="top-1/2 w-4 h-4 rounded-full border-2 border-white shadow-md ring-1 ring-black/10 focus:outline-none focus:ring-2 focus:ring-primary-500 transition-transform dragging:scale-110 cursor-col-resize" />
                </SliderTrack>
              </ColorSlider>
            </div>

            <div className="flex gap-2 items-end pt-1">
              <div className="flex-1">
                <Input
                  size="sm"
                  variant="bordered"
                  label="HEX"
                  labelPlacement="outside"
                  classNames={COMPONENT_STYLES.input}
                  value={hexValue}
                  onChange={(e) => handleHexChange(e.target.value)}
                  startContent={
                    !hexValue.startsWith("#") && (
                      <span className="text-default-400">#</span>
                    )
                  }
                />
              </div>
            </div>
          </div>
        </div>
      </ColorPicker>
    </div>
  );
};
