import { cn } from "@/utils/cn";
import { Chip } from "@heroui/react";
import React from "react";

import { useTranslation } from "react-i18next";

export function ModdedChip() {
  return (
    <Chip
      variant="primary"
      className={cn(
        "shadow-md",
        "bg-indigo-300 border border-white/50 shadow-indigo-500/20",
      )}
    >
      <Chip.Label className={"font-medium text-black"}>Modded</Chip.Label>
    </Chip>
  );
}

export function ShaderChip() {
  const { t } = useTranslation();
  return (
    <Chip
      variant="primary"
      className={cn(
        "shadow-md",
        "bg-emerald-300 border border-white/50 shadow-emerald-500/20",
      )}
    >
      <Chip.Label className={"font-medium text-black"}>
        {t("contentpage.shader_chip")}
      </Chip.Label>
    </Chip>
  );
}

export function VanillaChip() {
  return (
    <Chip
      variant="primary"
      className={cn(
        "shadow-md",
        "bg-green-300 border border-white/50 shadow-green-500/20",
      )}
    >
      <Chip.Label className={"font-medium text-black"}>Vanilla</Chip.Label>
    </Chip>
  );
}

export function ReleaseChip() {
  return (
    <Chip
      variant="primary"
      className={cn(
        "shadow-md",
        "bg-blue-300 border border-white/50 shadow-blue-500/20",
      )}
    >
      <Chip.Label className={"font-medium text-black"}>Release</Chip.Label>
    </Chip>
  );
}

export function PreviewChip() {
  return (
    <Chip
      variant="primary"
      className={cn(
        "shadow-md",
        "bg-purple-300 border border-white/50 shadow-purple-500/20",
      )}
    >
      <Chip.Label className={"font-medium text-black"}>Preview</Chip.Label>
    </Chip>
  );
}
