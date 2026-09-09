import { Button } from "@heroui/react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { LuChevronDown, LuDownload, LuFileDigit } from "react-icons/lu";
import type { LIPPackageFileInfo } from "@/utils/content";
import { compareVersions } from "@/utils/version";

export type FileGameVersionState = {
  file: LIPPackageFileInfo;
  supportedGameVersions: string[];
  hasLLRequirement: boolean;
};

type Props = {
  files: FileGameVersionState[];
  packageName: string;
  mappingReady: boolean;
  mappingAvailable: boolean;
  installDisabled: boolean;
  onInstall: (version: string) => void;
};

const ALL = "all";

export function BedrinthVersionList({
  files,
  packageName,
  mappingReady,
  mappingAvailable,
  installDisabled,
  onInstall,
}: Props) {
  const { t } = useTranslation();
  const [selectedGame, setSelectedGame] = useState(ALL);
  const games = useMemo(
    () =>
      Array.from(
        new Set(files.flatMap((state) => state.supportedGameVersions)),
      ).sort((a, b) => compareVersions(b, a)),
    [files],
  );
  const activeGame = games.includes(selectedGame) ? selectedGame : ALL;

  const groups = useMemo(() => {
    const result = new Map<
      string,
      {
        key: string;
        game: string;
        llRanges: string[];
        hasLLRequirement: boolean;
        files: FileGameVersionState[];
      }
    >();
    for (const state of files) {
      const llRanges = [...new Set(state.file.llDependencyRanges)].sort();
      // Keep unmapped and dependency-free releases visible without inventing compatibility.
      const targets = state.supportedGameVersions.length
        ? state.supportedGameVersions
        : [""];
      for (const game of targets) {
        if (activeGame !== ALL && game !== activeGame && state.hasLLRequirement)
          continue;
        const key = JSON.stringify([game, llRanges]);
        let group = result.get(key);
        if (!group) {
          group = {
            key,
            game,
            llRanges,
            hasLLRequirement: state.hasLLRequirement,
            files: [],
          };
          result.set(key, group);
        }
        group.files.push(state);
      }
    }
    return Array.from(result.values()).sort((a, b) => {
      if (a.game !== b.game) {
        if (!a.game) return 1;
        if (!b.game) return -1;
        return compareVersions(b.game, a.game);
      }
      return compareVersions(b.files[0].file.version, a.files[0].file.version);
    });
  }, [files, activeGame]);

  const gameLabel = (group: (typeof groups)[number]) => {
    if (group.game) return `Minecraft ${group.game}`;
    if (!group.hasLLRequirement)
      return t("lip.files.game_versions_unrestricted");
    if (!mappingReady) return t("common.loading");
    return t(
      mappingAvailable
        ? "lip.files.game_versions_unknown"
        : "lip.files.game_versions_unavailable",
    );
  };

  if (!files.length) {
    return (
      <div className="flex flex-col items-center py-12 text-muted border border-dashed border-border rounded-xl">
        <LuFileDigit size={40} className="mb-4" aria-hidden="true" />
        <p>{t("common.no_results")}</p>
      </div>
    );
  }

  return (
    <div
      className="min-w-0 space-y-4"
      aria-label={t("lip.files.table_aria_label")}
    >
      <div
        role="group"
        aria-label={t("lip.files.game_versions_label")}
        className="flex flex-wrap gap-2"
      >
        {[ALL, ...games].map((game) => (
          <Button
            key={game}
            size="sm"
            variant={activeGame === game ? "primary" : "ghost"}
            aria-pressed={activeGame === game}
            onPress={() => setSelectedGame(game)}
          >
            {game === ALL ? t("lip.game_all_versions") : game}
          </Button>
        ))}
      </div>
      <div key={activeGame} className="space-y-3">
        {groups.map((group, index) => (
          <details
            key={group.key}
            open={index === 0}
            className="group/version overflow-hidden rounded-xl border border-border bg-surface/40"
          >
            <summary className="flex cursor-pointer list-none items-center gap-3 p-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand-500 [&::-webkit-details-marker]:hidden">
              <div className="min-w-0 flex-1">
                <h3 className="text-sm font-semibold text-foreground">
                  {gameLabel(group)}
                </h3>
                {group.llRanges.length > 0 && (
                  <p className="mt-1 text-xs text-muted break-words">
                    LeviLamina {group.llRanges.join(", ")}
                  </p>
                )}
              </div>
              <span className="text-xs text-muted">
                {t("lip.files.version_count", { count: group.files.length })}
              </span>
              <LuChevronDown
                aria-hidden="true"
                className="shrink-0 transition-transform group-open/version:rotate-180"
              />
            </summary>
            <ul className="divide-y divide-border border-t border-border px-3">
              {group.files.map(({ file, hasLLRequirement }) => (
                <li
                  key={file.version}
                  className="flex items-start gap-3 py-3 sm:items-center"
                >
                  <span className="mt-1 rounded-lg bg-surface-secondary p-2 text-muted sm:mt-0">
                    <LuFileDigit size={20} aria-hidden="true" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-foreground break-words">
                      {packageName} {file.version}
                    </p>
                    <p className="mt-1 text-xs text-muted break-words">
                      {file.llDependencyRanges.length
                        ? `LeviLamina ${file.llDependencyRanges.join(", ")}`
                        : t("lip.files.game_versions_unrestricted")}
                    </p>
                    {Object.keys(file.otherDependencies).length > 0 && (
                      <details className="mt-1 text-xs text-muted">
                        <summary className="w-fit cursor-pointer rounded focus-visible:outline-2 focus-visible:outline-brand-500">
                          {t("lip.files.dependencies_count", {
                            count: Object.keys(file.otherDependencies).length,
                          })}
                        </summary>
                        <ul className="mt-1 space-y-1">
                          {Object.entries(file.otherDependencies).map(
                            ([name, range]) => (
                              <li key={name} className="break-all">
                                {name}: {range}
                              </li>
                            ),
                          )}
                        </ul>
                      </details>
                    )}
                  </div>
                  <Button
                    size="sm"
                    variant="secondary"
                    className="shrink-0"
                    aria-label={`${t("lip.files.install")} ${packageName} ${file.version}`}
                    isDisabled={
                      installDisabled || (hasLLRequirement && !mappingAvailable)
                    }
                    onPress={() => onInstall(file.version)}
                  >
                    <LuDownload aria-hidden="true" />
                    {t("lip.files.install")}
                  </Button>
                </li>
              ))}
            </ul>
          </details>
        ))}
      </div>
    </div>
  );
}
