import { Chip } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { LipTagIcon } from "@/components/LipTagIcon";
import { cn } from "@/utils/cn";
import { featuredTagsOf, plainTagsOf } from "@/utils/lipCatalog";

type Props = {
  /** A mod's raw manifest tags. */
  tags: readonly string[];
  /** Show at most this many chips, then "+N". */
  max?: number;
  className?: string;
  chipClassName?: string;
};

/**
 * A mod's tags as the catalogue shows them (contract A4): the featured tags
 * first, each with its sprite, its localised name and the raw tags behind it
 * in small grey monospace text (`Teleport tp`), then the other visible tags
 * as `#tag`. Loader,
 * kind and side tags are hidden.
 */
export function LipTagChips({ tags, max, className, chipClassName }: Props) {
  const { t } = useTranslation();
  const featured = featuredTagsOf(tags);
  const plain = plainTagsOf(tags);
  const chips = [
    ...featured.map(({ tag, raw }) => ({
      key: `featured-${tag.id}`,
      label: t(`lip.tags.${tag.id}`),
      raw: raw.join(" "),
      icon: tag.icon,
    })),
    ...plain.map((tag) => ({ key: `tag-${tag}`, label: `#${tag}`, raw: "", icon: undefined })),
  ];
  if (!chips.length) return null;
  const shown = max !== undefined ? chips.slice(0, max) : chips;
  const hidden = chips.length - shown.length;
  return (
    <div className={cn("flex flex-wrap gap-1", className)}>
      {shown.map((chip) => (
        <Chip
          key={chip.key}
          size="sm"
          variant="soft"
          className={cn("rounded-sm", chipClassName)}
          aria-label={chip.raw ? `${chip.label} (${chip.raw})` : chip.label}
        >
          {chip.icon ? <LipTagIcon sprite={chip.icon} size={12} /> : null}
          <Chip.Label>
            {chip.label}
            {chip.raw ? (
              <span className="ml-1 whitespace-nowrap font-mono text-[0.6875rem] font-normal text-muted dark:text-zinc-500">
                {chip.raw}
              </span>
            ) : null}
          </Chip.Label>
        </Chip>
      ))}
      {hidden > 0 ? (
        <Chip size="sm" variant="soft" className={cn("rounded-sm", chipClassName)}>
          <Chip.Label>{t("lip.filters.more_tags", { count: hidden })}</Chip.Label>
        </Chip>
      ) : null}
    </div>
  );
}
