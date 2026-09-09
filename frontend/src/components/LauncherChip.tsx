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
        "bg-linear-to-br from-indigo-500 to-pink-500 border border-white/50 shadow-pink-500/30",
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
        "bg-linear-to-br from-teal-400 to-emerald-500 border border-white/50 shadow-emerald-500/30",
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
        "bg-linear-to-br from-green-500 to-yellow-500 border border-white/50 shadow-yellow-500/30",
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
        "bg-linear-to-br from-blue-500 to-cyan-500 border border-white/50 shadow-cyan-500/30",
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
        "bg-linear-to-br from-purple-500 to-fuchsia-500 border border-white/50 shadow-fuchsia-500/30",
      )}
    >
      <Chip.Label className={"font-medium text-black"}>Preview</Chip.Label>
    </Chip>
  );
}
