import {
  Button,
  Card,
  Chip,
  Label,
  Link,
  ListBox,
  Select,
  Skeleton,
  Spinner,
  Table,
  Tabs,
  Tooltip,
} from "@heroui/react";

import { cn } from "@/utils/cn";

import React, { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useRouteTitle } from "@/hooks/useRouteTitle";
import { ProjectShareButton } from "@/components/ProjectShareButton";
import {
  GetCurseForgeModsByIDs,
  GetCurseForgeModDescription,
  GetCurseForgeModFiles,
} from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { Browser } from "@wailsio/runtime";
import { File as ModFile } from "bindings/github.com/liteldev/LeviLauncher/internal/curseforge/client/types";
import {
  CurseForgeInstallDialog,
  type CurseForgeInstallTarget,
} from "@/components/CurseForgeInstallDialog";
import { motion } from "framer-motion";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";

import {
  LuDownload,
  LuCalendar,
  LuFileDigit,
  LuGlobe,
  LuGithub,
  LuBug,
  LuGamepad2,
} from "react-icons/lu";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import {
  formatNumber,
  formatDateStr,
  formatFileSize,
  sortGameVersions,
} from "@/utils/formatting";

const releaseColor = (type: number) =>
  type === 1 ? "success" : type === 2 ? "accent" : "warning";
const releaseLetter = (type: number) =>
  type === 1 ? "R" : type === 2 ? "B" : "A";

const CurseForgeModPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();

  const [mod, setMod] = useState<any | null>(null);
  useRouteTitle(mod?.id === Number(id) ? mod?.name : undefined);
  const [description, setDescription] = useState<string>("");
  const [loadError, setLoadError] = useState(false);
  const [descriptionLoading, setDescriptionLoading] = useState(true);
  const [descriptionError, setDescriptionError] = useState(false);
  const [filesLoading, setFilesLoading] = useState(true);
  const [filesError, setFilesError] = useState(false);
  const [detailRetry, setDetailRetry] = useState(0);
  const [descriptionRetry, setDescriptionRetry] = useState(0);
  const [filesRetry, setFilesRetry] = useState(0);
  const [files, setFiles] = useState<ModFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedGameVersion, setSelectedGameVersion] = useState<string>("all");
  const [selectedTab, setSelectedTab] = useState<string>("description");
  const [isEntering, setIsEntering] = useState(true);

  const [installTarget, setInstallTarget] =
    useState<CurseForgeInstallTarget | null>(null);
  const [installOpen, setInstallOpen] = useState(false);
  const openInstall = (file?: ModFile) => {
    if (!mod) return;
    setInstallTarget({ modId: mod.id, modName: mod.name, file: file ?? null });
    setInstallOpen(true);
  };

  React.useLayoutEffect(() => {
    setIsEntering(true);
    const reset = () => {
      try {
        window.scrollTo({ top: 0, left: 0, behavior: "auto" });
      } catch {}
      try {
        const el = document.scrollingElement as HTMLElement | null;
        if (el) el.scrollTop = 0;
      } catch {}
      try {
        document.documentElement.scrollTop = 0;
      } catch {}
      try {
        document.body.scrollTop = 0;
      } catch {}
      try {
        const root = document.getElementById("root");
        if (root) (root as HTMLElement).scrollTop = 0;
      } catch {}
    };
    reset();
    const raf = requestAnimationFrame(reset);
    const t0 = window.setTimeout(reset, 0);
    const t1 = window.setTimeout(reset, 120);
    const t2 = window.setTimeout(() => setIsEntering(false), 550);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(t0);
      clearTimeout(t1);
      clearTimeout(t2);
    };
  }, [id]);


  const gameVersions = React.useMemo(() => {
    if (!files || files.length === 0) return [];
    const versions = new Set<string>();
    files.forEach((file) => {
      file.gameVersions?.forEach((v) => {
        versions.add(v);
      });
    });
    return sortGameVersions(Array.from(versions));
  }, [files]);

  const filteredFiles = React.useMemo(() => {
    if (selectedGameVersion === "all") return files;
    return files.filter((file) =>
      file.gameVersions?.includes(selectedGameVersion),
    );
  }, [files, selectedGameVersion]);

  useEffect(() => {
    let cancelled = false;
    const modId = Number(id);
    setMod(null);
    setLoadError(false);
    setLoading(true);
    if (!Number.isSafeInteger(modId) || modId <= 0) {
      setLoading(false);
      return;
    }
    GetCurseForgeModsByIDs([modId])
      .then((result) => {
        if (!cancelled) setMod(result?.data?.[0] ?? null);
      })
      .catch(() => {
        if (!cancelled) setLoadError(true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, detailRetry]);

  useEffect(() => {
    let cancelled = false;
    const modId = Number(id);
    setDescription("");
    setDescriptionError(false);
    setDescriptionLoading(true);
    if (!Number.isSafeInteger(modId) || modId <= 0) {
      setDescriptionLoading(false);
      return;
    }
    GetCurseForgeModDescription(modId)
      .then((result) => {
        if (!cancelled) setDescription(result?.data ?? "");
      })
      .catch(() => {
        if (!cancelled) setDescriptionError(true);
      })
      .finally(() => {
        if (!cancelled) setDescriptionLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, descriptionRetry]);

  useEffect(() => {
    let cancelled = false;
    const modId = Number(id);
    setFiles([]);
    setFilesError(false);
    setFilesLoading(true);
    if (!Number.isSafeInteger(modId) || modId <= 0) {
      setFilesLoading(false);
      return;
    }
    GetCurseForgeModFiles(modId)
      .then((result) => {
        if (!cancelled) setFiles(result?.data ?? []);
      })
      .catch(() => {
        if (!cancelled) setFilesError(true);
      })
      .finally(() => {
        if (!cancelled) setFilesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, filesRetry]);

  const handleDescriptionClick = (e: React.MouseEvent<HTMLDivElement>) => {
    // A middle click fires auxclick instead of click; left unhandled, WebView2
    // opens the link in a new launcher window.
    if (e.type === "auxclick" && e.button !== 1) return;
    const target = (e.target as HTMLElement).closest("a");
    const href = target?.getAttribute("href");

    if (!target || !href) return;

    e.preventDefault();

    let url = href;

    if (url.includes("linkout?remoteUrl=")) {
      const match = url.match(/remoteUrl=(.+)$/);
      if (match?.[1]) {
        try {
          url = decodeURIComponent(match[1]);
        } catch (err) {
          console.warn("Failed to decode remoteUrl", err);
        }
      }
    } else if (url.startsWith("/")) {
      url = "https://www.curseforge.com" + url;
    }

    if (/^https?%3a/i.test(url)) {
      try {
        url = decodeURIComponent(url);
      } catch (e) {
        console.warn("Failed to decode encoded URL", e);
      }
    }

    if (!url || url.startsWith("#")) return;

    if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(url)) {
      url = "https://" + url;
    }

    try {
      const parsedUrl = new URL(url);
      if (["http:", "https:"].includes(parsedUrl.protocol)) {
        Browser.OpenURL(parsedUrl.toString());
      } else {
        console.warn("Blocked opening non-http/https URL:", url);
      }
    } catch (e) {
      console.warn("Invalid URL:", url, e);
    }
  };

  if (loading) {
    return (
      <PageContainer animate={false}>
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="p-6">
            <div className="flex flex-col md:flex-row gap-6">
              <div className="flex items-start gap-4 flex-1">
                <Skeleton className="w-24 h-24 rounded-2xl shrink-0" />
                <div className="flex flex-col gap-3 w-full max-w-lg">
                  <Skeleton className="h-8 w-3/4 rounded-lg" />
                  <div className="flex gap-2">
                    <Skeleton className="h-4 w-20 rounded-md" />
                    <Skeleton className="h-4 w-20 rounded-md" />
                  </div>
                  <div className="flex gap-2">
                    <Skeleton className="h-6 w-16 rounded-full" />
                    <Skeleton className="h-6 w-16 rounded-full" />
                  </div>
                </div>
              </div>
              <div className="flex flex-col gap-3 md:w-52 justify-center">
                <Skeleton className="h-12 w-full rounded-xl" />
                <div className="flex gap-2 justify-center">
                  <Skeleton className="h-10 w-10 rounded-lg" />
                  <Skeleton className="h-10 w-10 rounded-lg" />
                  <Skeleton className="h-10 w-10 rounded-lg" />
                </div>
              </div>
            </div>
          </Card.Content>
        </Card>

        <Card className={`${LAYOUT.GLASS_CARD.BASE} min-h-[300px]`}>
          <Card.Content className="p-6">
            <div className="flex gap-6 mb-6">
              <Skeleton className="h-8 w-24 rounded-lg" />
              <Skeleton className="h-8 w-24 rounded-lg" />
            </div>
            <div className="space-y-4">
              <Skeleton className="h-4 w-full rounded-md" />
              <Skeleton className="h-4 w-full rounded-md" />
              <Skeleton className="h-4 w-3/4 rounded-md" />
            </div>
          </Card.Content>
        </Card>
      </PageContainer>
    );
  }

  if (!mod) {
    return (
      <div className="w-full h-full min-h-0 flex flex-col p-4 sm:p-6 gap-4 items-center justify-center">
        <Card className="bg-surface/50 dark:bg-surface/40 launcher-material-blur rounded-4xl p-8">
          <Card.Content className="flex flex-col items-center gap-4">
            <p role={loadError ? "alert" : "status"} className="text-xl font-bold">
              {t(
                loadError
                  ? "audit.mods.detail_load_failed"
                  : "curseforge.mod_not_found",
              )}
            </p>
            {loadError && (
              <Button
                variant="primary"
                onPress={() => setDetailRetry((value) => value + 1)}
              >
                {t("common.retry")}
              </Button>
            )}
            <Button
              onPress={() => navigate(-1)}
              variant={"primary"}
              className={
                "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20"
              }
            >
              {t("curseforge.go_back")}
            </Button>
          </Card.Content>
        </Card>
      </div>
    );
  }

  return (
    <PageContainer
      animate={false}
      className={isEntering ? "overflow-y-hidden" : undefined}
    >
      {/* Capped so very wide windows don't stretch the page into empty slabs. */}
      <div className="mx-auto flex w-full max-w-[90rem] flex-col gap-4">
      {/* Header Card */}
      <motion.div
        data-material-motion
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="@container p-4 @[40rem]:p-6">
            <div className="flex flex-col gap-4 @[56rem]:flex-row @[56rem]:gap-6">
              <div className="flex min-w-0 grow gap-4 @[40rem]:gap-6">
              <div className="shrink-0">
                <img
                  src={mod.logo?.url}
                  alt={mod.name}
                  className={
                    "size-16 @[40rem]:size-32 object-cover rounded-2xl shadow-lg bg-surface-secondary"
                  }
                />
              </div>

              <div className="flex min-w-0 flex-col grow gap-3">
                <h1 className="text-2xl @[40rem]:text-4xl font-black tracking-tight text-foreground dark:text-zinc-100 pb-1 [overflow-wrap:anywhere]">
                  {mod.name}
                </h1>

                <div className="flex items-center gap-3 text-muted dark:text-zinc-400 text-sm flex-wrap">
                  <span className="flex items-center gap-1">
                    {t("curseforge.by")}
                    {mod.authors?.map((author: any, idx: number) => (
                      <React.Fragment key={author.id}>
                        <Link
                          onPress={() => Browser.OpenURL(author.url)}
                          className={cn(
                            "text-sm",
                            "text-accent hover:underline cursor-pointer",
                          )}
                        >
                          {author.name}
                        </Link>
                        {idx < (mod.authors?.length || 0) - 1 && ", "}
                      </React.Fragment>
                    ))}
                  </span>
                  <span className="w-1 h-1 rounded-full bg-surface-quaternary"></span>
                  <span className="flex items-center gap-1">
                    <LuCalendar size={14} />
                    {t("curseforge.updated_date", {
                      date: formatDateStr(mod.dateModified),
                    })}
                  </span>
                  <span className="w-1 h-1 rounded-full bg-surface-quaternary"></span>
                  <span className="flex items-center gap-1">
                    <LuDownload size={14} />
                    {t("curseforge.download_count", {
                      count: mod.downloadCount,
                    })}
                  </span>
                </div>

                <div className="flex flex-wrap gap-2 mt-1">
                  {mod.categories?.map((cat: any) => (
                    <Chip
                      key={cat.id}
                      size="sm"
                      variant="soft"
                      className={"pl-1"}
                    >
                      {cat.iconUrl ? (
                        <img src={cat.iconUrl} alt="" className={"w-4 h-4"} />
                      ) : undefined}
                      <Chip.Label>{cat.name}</Chip.Label>
                    </Chip>
                  ))}
                  <Chip size="sm" variant="secondary">
                    {<LuGamepad2 size={12} />}
                    <Chip.Label>ID: {mod.id}</Chip.Label>
                  </Chip>
                </div>

                <p className="text-foreground dark:text-zinc-300 mt-2 text-sm leading-relaxed max-w-4xl">
                  {mod.summary}
                </p>
              </div>
              </div>

              {/* Actions: a row under the details when narrow, a side column when wide. */}
              <div className="flex flex-wrap items-center gap-3 @[56rem]:w-60 @[56rem]:shrink-0 @[56rem]:flex-col @[56rem]:flex-nowrap @[56rem]:items-stretch @[56rem]:justify-center @[56rem]:border-l @[56rem]:border-border @[56rem]:pl-8">
                <Button
                  size="lg"
                  onPress={() => openInstall()}
                  variant={"secondary"}
                  data-testid="curseforge-mod-install"
                  className={
                    "flex-[1_1_12rem] @[56rem]:flex-none @[56rem]:w-full font-semibold shadow-md shadow-brand-900/20 brand-primary-foreground bg-brand-500 hover:bg-brand-500"
                  }
                >
                  {<LuDownload size={20} />}
                  {t("curseforge.install_action")}
                </Button>
                <div className="flex gap-2 justify-center">
                  {mod.links?.websiteUrl && (
                    <Button
                      onPress={() => Browser.OpenURL(mod.links.websiteUrl)}
                      isIconOnly
                      aria-label={t("curseforge.website")}
                      variant={"secondary"}
                    >
                      <LuGlobe size={20} />
                    </Button>
                  )}
                  {mod.links?.sourceUrl && (
                    <Button
                      onPress={() => Browser.OpenURL(mod.links.sourceUrl)}
                      isIconOnly
                      aria-label={t("curseforge.source")}
                      variant={"secondary"}
                    >
                      <LuGithub size={20} />
                    </Button>
                  )}
                  {mod.links?.issuesUrl && (
                    <Button
                      onPress={() => Browser.OpenURL(mod.links.issuesUrl)}
                      isIconOnly
                      aria-label={t("curseforge.issues")}
                      variant={"secondary"}
                    >
                      <LuBug size={20} />
                    </Button>
                  )}
                  <ProjectShareButton url={mod.links?.websiteUrl} />
                </div>
              </div>
            </div>
          </Card.Content>
        </Card>
      </motion.div>

      {/* Content Card */}
      <motion.div
        data-material-motion
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, delay: 0.1 }}
      >
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <Card.Content className="@container p-4 @[40rem]:p-6">
            <div className="flex w-full min-w-0 flex-col">
              <Tabs
                selectedKey={selectedTab}
                onSelectionChange={(key) => setSelectedTab(key as string)}
                variant="secondary"
              >
                <Tabs.ListContainer>
                  <Tabs.List
                    aria-label={t("curseforge.mod_details_aria_label")}
                    className={
                      "gap-6 w-full relative rounded-none p-0 border-b border-border mb-6"
                    }
                  >
                    <Tabs.Tab
                      key="description"
                      id={"description"}
                      className={
                        "group-data-[selected]:text-brand-600 dark:group-data-[selected]:text-brand-500 font-bold"
                      }
                    >
                      {t("curseforge.mod_tabs.description")}
                      <Tabs.Indicator
                        className={
                          "w-full bg-accent h-[3px]"
                        }
                      />
                    </Tabs.Tab>
                    <Tabs.Tab
                      key="files"
                      id={"files"}
                      className={
                        "group-data-[selected]:text-brand-600 dark:group-data-[selected]:text-brand-500 font-bold"
                      }
                    >
                      {t("curseforge.mod_tabs.files")}
                      <Tabs.Indicator
                        className={
                          "w-full bg-accent h-[3px]"
                        }
                      />
                    </Tabs.Tab>
                  </Tabs.List>
                </Tabs.ListContainer>

                <Tabs.Panel id="description">
                  <div className="prose dark:prose-invert max-w-4xl mx-auto min-w-0 [overflow-wrap:anywhere] prose-img:rounded-xl prose-img:mx-auto prose-a:text-brand-600 dark:prose-a:text-brand-500">
                    {descriptionLoading ? (
                      <div
                        role="status"
                        className="flex flex-col items-center justify-center py-12 text-muted gap-3"
                      >
                        <Spinner color="accent" />
                        <p>{t("curseforge.loading_description")}</p>
                      </div>
                    ) : descriptionError ? (
                      <div
                        role="alert"
                        className="flex flex-col items-center justify-center py-12 gap-3"
                      >
                        <p>{t("audit.mods.description_load_failed")}</p>
                        <Button
                          variant="secondary"
                          onPress={() => setDescriptionRetry((value) => value + 1)}
                        >
                          {t("common.retry")}
                        </Button>
                      </div>
                    ) : description.trim() ? (
                      <div
                        dangerouslySetInnerHTML={{ __html: description }}
                        onClick={handleDescriptionClick}
                        onAuxClick={handleDescriptionClick}
                      />
                    ) : (
                      <div
                        role="status"
                        className="flex flex-col items-center justify-center py-12 text-muted gap-3"
                      >
                        <p>{t("curseforge.no_description")}</p>
                      </div>
                    )}
                  </div>
                </Tabs.Panel>

                <Tabs.Panel id="files">
                  <div className="flex flex-col gap-4">
                    <div className="flex flex-wrap justify-between items-end gap-3 mb-2">
                      <h3 className="text-lg font-semibold">
                        {t("curseforge.mod_files.title")}
                      </h3>
                      <div className="w-48 max-w-full">
                        <Select
                          value={Array.from([selectedGameVersion])[0] ?? null}
                          onChange={(e) =>
                            setSelectedGameVersion(String(e ?? "all"))
                          }
                        >
                          <Label>{t("curseforge.minecraft_version")}</Label>
                          <Select.Trigger
                            className={cn(
                              COMPONENT_STYLES.select.trigger,
                              "min-h-8 text-sm",
                            )}
                          >
                            <Select.Value />
                            <Select.Indicator />
                          </Select.Trigger>
                          <Select.Popover
                            className={COMPONENT_STYLES.select.popoverContent}
                          >
                            <ListBox
                              items={[
                                {
                                  key: "all",
                                  label: t("curseforge.all_versions"),
                                },
                                ...gameVersions.map((v) => ({
                                  key: v,
                                  label: v,
                                })),
                              ]}
                              className={COMPONENT_STYLES.select.listbox}
                            >
                              {(item) => (
                                <ListBox.Item
                                  key={item.key}
                                  id={item.key}
                                  textValue={item.label}
                                >
                                  <Label>{item.label}</Label>
                                  <ListBox.ItemIndicator />
                                </ListBox.Item>
                              )}
                            </ListBox>
                          </Select.Popover>
                        </Select>
                      </div>
                    </div>

                    {filesLoading ? (
                      <div
                        role="status"
                        className="flex flex-col items-center py-12 gap-3"
                      >
                        <Spinner color="accent" />
                        <p>{t("common.loading")}</p>
                      </div>
                    ) : filesError ? (
                      <div
                        role="alert"
                        className="flex flex-col items-center py-12 gap-3"
                      >
                        <p>{t("audit.mods.files_load_failed")}</p>
                        <Button
                          variant="secondary"
                          onPress={() => setFilesRetry((value) => value + 1)}
                        >
                          {t("common.retry")}
                        </Button>
                      </div>
                    ) : filteredFiles.length > 0 ? (
                      <>
                      <div className="hidden @[48rem]:block">
                      <Table variant="secondary">
                        <Table.ScrollContainer className="h-full">
                          <Table.Content
                            aria-label={t(
                              "curseforge.mod_files.table_aria_label",
                            )}
                          >
                            <Table.Header
                              className={cn(COMPONENT_STYLES.table.thead)}
                            >
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                                isRowHeader
                              >
                                {t("curseforge.mod_files.columns.type")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.name")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.uploaded")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.size")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.minecraft_version")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.downloads")}
                              </Table.Column>
                              <Table.Column
                                className={cn(COMPONENT_STYLES.table.th)}
                              >
                                {t("curseforge.mod_files.columns.actions")}
                              </Table.Column>
                            </Table.Header>
                            <Table.Body>
                              {filteredFiles.map((file) => {
                                const sortedVersions = sortGameVersions(
                                  file.gameVersions,
                                );
                                return (
                                  <Table.Row
                                    className={cn(
                                      "group transition-colors hover:bg-surface/50 dark:hover:bg-surface-secondary/30",
                                    )}
                                    key={file.id}
                                    id={file.id}
                                  >
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <Chip
                                        size="sm"
                                        variant="soft"
                                        color={releaseColor(file.releaseType)}
                                        className={"capitalize"}
                                      >
                                        <Chip.Label>
                                          {releaseLetter(file.releaseType)}
                                        </Chip.Label>
                                      </Chip>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <span className="font-medium">
                                        {file.displayName}
                                      </span>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <span className="text-muted dark:text-zinc-400">
                                        {formatDateStr(file.fileDate)}
                                      </span>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <span className="text-muted dark:text-zinc-400">
                                        {formatFileSize(file.fileLength)}
                                      </span>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <div className="flex items-center gap-1">
                                        {sortedVersions.length > 0 ? (
                                          <>
                                            <span className="text-foreground dark:text-zinc-300 bg-surface-secondary px-2 py-1 rounded text-xs">
                                              {sortedVersions[0]}
                                            </span>
                                            {sortedVersions.length > 1 && (
                                              <Tooltip>
                                                <span className="text-xs text-accent cursor-pointer">
                                                  +{sortedVersions.length - 1}
                                                </span>
                                                <Tooltip.Content>
                                                  {
                                                    <div className="flex flex-wrap gap-1 max-w-xs p-2">
                                                      {sortedVersions
                                                        .slice(1)
                                                        .map((v) => (
                                                          <span
                                                            key={v}
                                                            className="text-xs bg-surface dark:bg-surface-secondary text-muted dark:text-zinc-400 px-1.5 py-0.5 rounded border border-border dark:border-zinc-700"
                                                          >
                                                            {v}
                                                          </span>
                                                        ))}
                                                    </div>
                                                  }
                                                </Tooltip.Content>
                                              </Tooltip>
                                            )}
                                          </>
                                        ) : (
                                          <span className="text-muted">-</span>
                                        )}
                                      </div>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <span className="text-muted dark:text-zinc-400">
                                        {formatNumber(file.downloadCount)}
                                      </span>
                                    </Table.Cell>
                                    <Table.Cell
                                      className={cn(
                                        "py-3 border-b border-border/50 dark:border-white/5 group-last:border-0",
                                      )}
                                    >
                                      <Tooltip>
                                        <Button
                                          isIconOnly
                                          aria-label={t("audit.mods.install_file", {
                                            name: file.displayName || file.fileName,
                                          })}
                                          size="sm"
                                          onPress={() => openInstall(file)}
                                          variant={"ghost"}
                                          className={
                                            "text-muted dark:text-zinc-400 hover:text-accent"
                                          }
                                        >
                                          <LuDownload size={20} />
                                        </Button>
                                        <Tooltip.Content>
                                          {t("audit.mods.install_file", {
                                            name: file.displayName || file.fileName,
                                          })}
                                        </Tooltip.Content>
                                      </Tooltip>
                                    </Table.Cell>
                                  </Table.Row>
                                );
                              })}
                            </Table.Body>
                          </Table.Content>
                        </Table.ScrollContainer>
                      </Table>
                      </div>
                      <ul
                        className="flex flex-col gap-2 @[48rem]:hidden"
                        aria-label={t("curseforge.mod_files.table_aria_label")}
                      >
                        {filteredFiles.map((file) => {
                          const sortedVersions = sortGameVersions(
                            file.gameVersions,
                          );
                          const name = file.displayName || file.fileName;
                          return (
                            <li
                              key={file.id}
                              className="flex min-w-0 items-center gap-3 rounded-2xl border border-border/60 dark:border-white/5 bg-surface/40 p-3"
                            >
                              <Chip
                                size="sm"
                                variant="soft"
                                color={releaseColor(file.releaseType)}
                                className="shrink-0"
                              >
                                <Chip.Label>
                                  {releaseLetter(file.releaseType)}
                                </Chip.Label>
                              </Chip>
                              <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                                <span className="font-medium [overflow-wrap:anywhere]">
                                  {name}
                                </span>
                                <span className="text-xs text-muted dark:text-zinc-400 [overflow-wrap:anywhere]">
                                  {[
                                    sortedVersions.length > 0
                                      ? sortedVersions.length > 1
                                        ? `${sortedVersions[0]} +${sortedVersions.length - 1}`
                                        : sortedVersions[0]
                                      : "",
                                    formatDateStr(file.fileDate),
                                    formatFileSize(file.fileLength),
                                    t("curseforge.download_count", {
                                      count: formatNumber(file.downloadCount),
                                    }),
                                  ]
                                    .filter(Boolean)
                                    .join(" · ")}
                                </span>
                              </div>
                              <Button
                                isIconOnly
                                size="sm"
                                variant="secondary"
                                aria-label={t("audit.mods.install_file", {
                                  name,
                                })}
                                onPress={() => openInstall(file)}
                                className="shrink-0"
                              >
                                <LuDownload size={18} />
                              </Button>
                            </li>
                          );
                        })}
                      </ul>
                      </>
                    ) : (
                      <div className="flex flex-col items-center justify-center py-12 text-muted border border-dashed border-border rounded-xl">
                        <LuFileDigit size={48} className="mb-4 opacity-50" />
                        <p className="text-lg font-medium">
                          {t("curseforge.no_files_found")}
                        </p>
                      </div>
                    )}
                  </div>
                </Tabs.Panel>
              </Tabs>
            </div>
          </Card.Content>
        </Card>
      </motion.div>

      </div>

      {installTarget && (
        <CurseForgeInstallDialog
          target={installTarget}
          isOpen={installOpen}
          onClose={() => setInstallOpen(false)}
          files={files.length > 0 ? files : undefined}
        />
      )}
    </PageContainer>
  );
};

export default CurseForgeModPage;
