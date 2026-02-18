import React from "react";
import { Chip } from "@heroui/react";
import { useTranslation } from "react-i18next";

export function ModdedChip() {
  return (
    <Chip
      variant="shadow"
      classNames={{
        base: "bg-linear-to-br from-indigo-500 to-pink-500 border-small border-white/50 shadow-pink-500/30",
        content: "drop-shadow shadow-black text-white",
      }}
    >
      Modded
    </Chip>
  );
}

export function ShaderChip() {
  const { t } = useTranslation();
  return (
    <Chip
      variant="shadow"
      classNames={{
        base: "bg-linear-to-br from-teal-400 to-emerald-500 border-small border-white/50 shadow-emerald-500/30",
        content: "drop-shadow shadow-black text-white",
      }}
    >
      {t("contentpage.shader_chip")}
    </Chip>
  );
}

export function VanillaChip() {
  return (
    <Chip
      variant="shadow"
      classNames={{
        base: "bg-linear-to-br from-green-500 to-yellow-500 border-small border-white/50 shadow-yellow-500/30",
        content: "drop-shadow shadow-black text-white",
      }}
    >
      Vanilla
    </Chip>
  );
}

export function ReleaseChip() {
  return (
    <Chip
      variant="shadow"
      classNames={{
        base: "bg-linear-to-br from-blue-500 to-cyan-500 border-small border-white/50 shadow-cyan-500/30",
        content: "drop-shadow shadow-black text-white",
      }}
    >
      Release
    </Chip>
  );
}

export function PreviewChip() {
  return (
    <Chip
      variant="shadow"
      classNames={{
        base: "bg-linear-to-br from-purple-500 to-fuchsia-500 border-small border-white/50 shadow-fuchsia-500/30",
        content: "drop-shadow shadow-black text-white",
      }}
    >
      Preview
    </Chip>
  );
}
