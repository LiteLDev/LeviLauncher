import { Button, Chip, InputGroup, TextField } from "@heroui/react";
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { LuCheck, LuChevronDown, LuSearch, LuX } from "react-icons/lu";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { LipTagIcon } from "@/components/LipTagIcon";
import { cn } from "@/utils/cn";
import { getIntlLocale } from "@/utils/intlLocale";
import {
  excludeTag,
  FEATURED_BY_ID,
  includeTag,
  toggleValue,
  type LicenseFamilyId,
  type LipFacet,
  type LipFacetCounts,
  type LipFacetOptions,
  type LipTagSprite,
} from "@/utils/lipCatalog";

/** The filters the launcher offers besides the search and the version selects (contract A3). */
export type LipCatalogFilterState = {
  tags: LipFacet;
  license: string[];
  dep: string[];
  author: string[];
};

type Props = {
  state: LipCatalogFilterState;
  options: LipFacetOptions;
  counts: LipFacetCounts;
  /** Bare id → mod name, for the Dependencies group. */
  names: ReadonlyMap<string, string>;
  onChange: (next: LipCatalogFilterState) => void;
};

/** Options shown before "Show more" in long groups. */
const SHORT_LIST = 8;

type OptionProps = {
  label: ReactNode;
  /** The label as text (accessible names). */
  text: string;
  icon?: LipTagSprite;
  /** Small grey text after the label (the tag a featured tag stands for). */
  raw?: string;
  mono?: boolean;
  nested?: boolean;
  count: number;
  included: boolean;
  excluded?: boolean;
  onInclude: () => void;
  onExclude?: () => void;
};

function Option({ label, text, icon, raw, mono, nested, count, included, excluded, onInclude, onExclude }: OptionProps) {
  const { t } = useTranslation();
  const dimmed = count === 0 && !included && !excluded;
  return (
    <li className={cn("flex min-w-0 items-center gap-1", nested && "pl-5")}>
      <button
        type="button"
        aria-pressed={included}
        aria-disabled={dimmed || undefined}
        onClick={dimmed ? undefined : onInclude}
        className={cn(
          "flex min-w-0 flex-1 items-center gap-2 rounded-lg px-2 py-1 text-left text-sm transition-colors",
          "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-focus",
          included
            ? "bg-brand-500/15 text-brand-700 dark:text-brand-300"
            : "text-foreground hover:bg-surface-secondary dark:text-zinc-200",
          excluded && "text-rose-600 line-through dark:text-rose-400",
          dimmed && "cursor-default opacity-45 hover:bg-transparent",
        )}
      >
        <span className="flex w-4 shrink-0 justify-center" aria-hidden="true">
          {included ? <LuCheck size={14} /> : icon ? <LipTagIcon sprite={icon} size={14} /> : null}
        </span>
        <span className={cn("min-w-0 flex-1 truncate", mono && "font-mono text-xs")}>
          {label}
          {raw ? (
            <span className="ml-1 whitespace-nowrap font-mono text-[0.6875rem] font-normal text-muted dark:text-zinc-500">{raw}</span>
          ) : null}
        </span>
        <span className="shrink-0 text-xs tabular-nums text-muted dark:text-zinc-500">
          {count.toLocaleString(getIntlLocale())}
        </span>
      </button>
      {onExclude ? (
        <button
          type="button"
          aria-label={t("lip.filters.exclude", { label: text })}
          aria-pressed={!!excluded}
          aria-disabled={dimmed || undefined}
          onClick={dimmed ? undefined : onExclude}
          className={cn(
            "flex h-6 w-6 shrink-0 items-center justify-center rounded-md transition-colors",
            "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-focus",
            excluded
              ? "bg-rose-500/15 text-rose-600 dark:text-rose-400"
              : "text-muted hover:bg-surface-secondary hover:text-foreground",
            dimmed && "cursor-default opacity-45 hover:bg-transparent",
          )}
        >
          <LuX size={12} aria-hidden="true" />
        </button>
      ) : (
        <span className="w-6 shrink-0" aria-hidden="true" />
      )}
    </li>
  );
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex min-w-0 flex-col gap-1">
      <h3 className="px-2 text-xs font-semibold uppercase tracking-wide text-muted dark:text-zinc-400">
        {title}
      </h3>
      {children}
    </section>
  );
}

/**
 * A long list of options: the first few, the chosen ones, then "Show more";
 * past {@link SHORT_LIST} options a search field narrows it by label or id
 * (the site's LongOptions).
 */
function LongList<T extends { id: string; label: string; chosen: boolean }>({
  items,
  placeholder,
  render,
}: {
  items: T[];
  placeholder: string;
  render: (item: T) => ReactNode;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const [more, setMore] = useState(false);
  const q = query.trim().toLowerCase();
  const found = q
    ? items.filter((item) => item.label.toLowerCase().includes(q) || item.id.toLowerCase().includes(q))
    : items;
  const shown = q || more ? found : found.filter((item, i) => i < SHORT_LIST || item.chosen);
  const hidden = found.length - shown.length;
  return (
    <>
      {items.length > SHORT_LIST ? (
        <TextField
          aria-label={placeholder}
          className={cn("group", COMPONENT_STYLES.input.mainWrapper, "mb-1")}
          value={query}
          onChange={setQuery}
        >
          <InputGroup
            className={cn(
              COMPONENT_STYLES.input.inputWrapper,
              COMPONENT_STYLES.input.innerWrapper,
              "min-h-8 text-sm",
            )}
          >
            <InputGroup.Prefix>
              <LuSearch aria-hidden="true" />
            </InputGroup.Prefix>
            <InputGroup.Input
              type="search"
              placeholder={placeholder}
              autoComplete="off"
              spellCheck={false}
              className={COMPONENT_STYLES.input.input}
            />
          </InputGroup>
        </TextField>
      ) : null}
      <ul className="flex flex-col gap-0.5">{shown.map(render)}</ul>
      {!q && (hidden > 0 || more) ? (
        <Button size="sm" variant="ghost" className="self-start" aria-expanded={more} onPress={() => setMore(!more)}>
          <LuChevronDown aria-hidden="true" className={cn("transition-transform", more && "rotate-180")} />
          {more ? t("lip.filters.show_fewer") : t("lip.filters.show_more", { count: hidden })}
        </Button>
      ) : null}
    </>
  );
}

/**
 * The Tags, License and Dependencies groups of the catalogue (contract A3,
 * the site's filter groups): tags AND with exclusions, the featured tags
 * first with their sprites; licenses OR over families and SPDX ids;
 * dependencies AND. Every option shows how many results it would give.
 */
export function LIPCatalogFilters({ state, options, counts, names, onChange }: Props) {
  const { t } = useTranslation();
  const [openFamilies, setOpenFamilies] = useState<Set<LicenseFamilyId>>(new Set());
  const set = (patch: Partial<LipCatalogFilterState>) => onChange({ ...state, ...patch });
  const chosen = (tag: string) => state.tags.inc.includes(tag) || state.tags.exc.includes(tag);
  const tagOptions = [
    ...options.tags,
    ...[...state.tags.inc, ...state.tags.exc].filter((tag) => !options.tags.includes(tag)),
  ];
  // A featured tag is offered while it matches something (or is chosen).
  const featured = tagOptions.filter(
    (tag) => FEATURED_BY_ID.has(tag) && ((counts.tags[tag] ?? 0) > 0 || chosen(tag)),
  );
  const plain = tagOptions.filter((tag) => !FEATURED_BY_ID.has(tag));
  const deps = state.dep.map((d) => d.toLowerCase());
  const licenses = state.license.map((l) => l.toLowerCase());
  const depLabel = (d: string) => names.get(d.toLowerCase()) ?? d.split("/").pop() ?? d;
  const depOptions = [
    ...options.dep,
    ...state.dep.filter((d) => !options.dep.some((o) => o.toLowerCase() === d.toLowerCase())),
  ];

  return (
    <div className="grid grid-cols-1 gap-4 min-[700px]:grid-cols-2 min-[1100px]:grid-cols-3">
      {tagOptions.length ? (
        <Group title={t("lip.filters.tags")}>
          {featured.length ? (
            <ul className="flex flex-col gap-0.5">
              {featured.map((tag) => {
                const f = FEATURED_BY_ID.get(tag)!;
                const label = t(`lip.tags.${f.id}`);
                return (
                  <Option
                    key={tag}
                    label={label}
                    text={label}
                    icon={f.icon}
                    raw={tag}
                    count={counts.tags[tag] ?? 0}
                    included={state.tags.inc.includes(tag)}
                    excluded={state.tags.exc.includes(tag)}
                    onInclude={() => set({ tags: includeTag(state.tags, tag) })}
                    onExclude={() => set({ tags: excludeTag(state.tags, tag) })}
                  />
                );
              })}
            </ul>
          ) : null}
          {plain.length ? (
            <>
              {featured.length ? (
                <h4 className="mt-2 px-2 text-xs text-muted dark:text-zinc-500">
                  {t("lip.filters.other_tags")}
                </h4>
              ) : null}
              <LongList
                items={plain.map((tag) => ({ id: tag, label: `#${tag}`, chosen: chosen(tag) }))}
                placeholder={t("lip.filters.search_tags")}
                render={({ id: tag }) => (
                  <Option
                    key={tag}
                    label={`#${tag}`}
                    text={tag}
                    count={counts.tags[tag] ?? 0}
                    included={state.tags.inc.includes(tag)}
                    excluded={state.tags.exc.includes(tag)}
                    onInclude={() => set({ tags: includeTag(state.tags, tag) })}
                    onExclude={() => set({ tags: excludeTag(state.tags, tag) })}
                  />
                )}
              />
            </>
          ) : null}
        </Group>
      ) : null}

      {options.license.length ? (
        <Group title={t("lip.filters.license")}>
          <ul className="flex flex-col gap-0.5">
            {options.license.map(({ family, ids }) => {
              const familyLabel = t(`lip.license.${family}`);
              const open = openFamilies.has(family) || ids.some((id) => licenses.includes(id.toLowerCase()));
              return (
                <li key={family} className="flex flex-col gap-0.5">
                  <ul className="flex flex-col">
                    <Option
                      label={familyLabel}
                      text={familyLabel}
                      count={counts.license[family] ?? 0}
                      included={licenses.includes(family)}
                      onInclude={() => set({ license: toggleValue(state.license, family) })}
                    />
                  </ul>
                  {ids.length ? (
                    <>
                      <button
                        type="button"
                        aria-expanded={open}
                        onClick={() =>
                          setOpenFamilies((prev) => {
                            const next = new Set(prev);
                            if (next.has(family)) next.delete(family);
                            else next.add(family);
                            return next;
                          })
                        }
                        className="ml-8 flex w-fit items-center gap-1 rounded text-xs text-muted hover:text-foreground focus-visible:outline-2 focus-visible:outline-focus"
                      >
                        <LuChevronDown
                          size={12}
                          aria-hidden="true"
                          className={cn("transition-transform", open && "rotate-180")}
                        />
                        {t("lip.filters.license_ids", { family: familyLabel })}
                      </button>
                      {open ? (
                        <ul className="flex flex-col gap-0.5">
                          {ids.map((id) => (
                            <Option
                              key={id}
                              label={id}
                              text={id}
                              mono
                              nested
                              count={counts.license[id] ?? 0}
                              included={licenses.includes(id.toLowerCase())}
                              onInclude={() => set({ license: toggleValue(state.license, id) })}
                            />
                          ))}
                        </ul>
                      ) : null}
                    </>
                  ) : null}
                </li>
              );
            })}
          </ul>
        </Group>
      ) : null}

      {depOptions.length ? (
        <Group title={t("lip.filters.dependencies")}>
          <LongList
            items={depOptions.map((d) => ({ id: d, label: depLabel(d), chosen: deps.includes(d.toLowerCase()) }))}
            placeholder={t("lip.search_placeholder")}
            render={({ id: d, label }) => {
              return (
                <Option
                  key={d}
                  label={label}
                  text={label}
                  count={counts.dep[d] ?? 0}
                  included={deps.includes(d.toLowerCase())}
                  onInclude={() => set({ dep: toggleValue(state.dep, d) })}
                />
              );
            }}
          />
        </Group>
      ) : null}
    </div>
  );
}

/**
 * The filters in force, each a chip that takes it off: tags (excluded ones
 * as "not …"), licenses, dependencies and authors.
 */
export function LIPActiveFilters({
  state,
  names,
  onChange,
}: {
  state: LipCatalogFilterState;
  names: ReadonlyMap<string, string>;
  onChange: (next: LipCatalogFilterState) => void;
}) {
  const { t } = useTranslation();
  const tagLabel = (tag: string) => (FEATURED_BY_ID.has(tag) ? t(`lip.tags.${tag}`) : `#${tag}`);
  const licenseLabel = (value: string) =>
    ["permissive", "weak-copyleft", "copyleft", "other", "none"].includes(value)
      ? t(`lip.license.${value}`)
      : value;
  const chips: { key: string; label: string; remove: () => void; excluded?: boolean }[] = [
    ...state.author.map((a) => ({
      key: `author-${a}`,
      label: t("lip.by_author_inline", { author: a }),
      remove: () => onChange({ ...state, author: toggleValue(state.author, a) }),
    })),
    ...state.tags.inc.map((tag) => ({
      key: `tag-${tag}`,
      label: tagLabel(tag),
      remove: () => onChange({ ...state, tags: includeTag(state.tags, tag) }),
    })),
    ...state.tags.exc.map((tag) => ({
      key: `not-${tag}`,
      label: t("lip.filters.not_label", { label: tagLabel(tag) }),
      excluded: true,
      remove: () => onChange({ ...state, tags: excludeTag(state.tags, tag) }),
    })),
    ...state.license.map((l) => ({
      key: `license-${l}`,
      label: licenseLabel(l),
      remove: () => onChange({ ...state, license: toggleValue(state.license, l) }),
    })),
    ...state.dep.map((d) => ({
      key: `dep-${d}`,
      label: t("lip.filters.depends_on", { name: names.get(d.toLowerCase()) ?? d.split("/").pop() ?? d }),
      remove: () => onChange({ ...state, dep: toggleValue(state.dep, d) }),
    })),
  ];
  if (!chips.length) return null;
  return (
    <ul className="flex flex-wrap gap-1.5" aria-label={t("lip.filters.active")}>
      {chips.map((chip) => (
        <li key={chip.key}>
          <Chip size="sm" variant="soft" className={cn("rounded-md pr-0.5", chip.excluded && "text-rose-600 dark:text-rose-400")}>
            <Chip.Label>{chip.label}</Chip.Label>
            <button
              type="button"
              aria-label={t("lip.filters.remove", { label: chip.label })}
              onClick={chip.remove}
              className="ml-0.5 flex h-4 w-4 items-center justify-center rounded-full hover:bg-surface-tertiary focus-visible:outline-2 focus-visible:outline-focus"
            >
              <LuX size={10} aria-hidden="true" />
            </button>
          </Chip>
        </li>
      ))}
    </ul>
  );
}
