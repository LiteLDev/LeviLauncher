import {
  Button,
  Card,
  Chip,
  Label,
  ListBox,
  Select,
  Skeleton,
  Spinner,
  Tabs,
  Tooltip,
} from "@heroui/react";

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useRouteTitle } from "@/hooks/useRouteTitle";
import { ProjectShareButton } from "@/components/ProjectShareButton";

import { Browser } from "@wailsio/runtime";
import { motion } from "framer-motion";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import rehypeSlug from "rehype-slug";
import {
  getLIPVariantDisplayLabel,
  LIPInstallDialog,
  resolveDefaultLIPVariant,
} from "@/components/LIPInstallDialog";
import {
  LIPVersionList,
  type FileGameVersionState,
} from "@/components/LIPVersionList";
import { shouldDisableAnimations } from "@/hooks/useAnimations";
import {
  LuClock,
  LuDownload,
  LuFlame,
  LuGamepad2,
  LuGlobe,
  LuTag,
  LuUser,
} from "react-icons/lu";
import { GetLIPPackageReadme } from "bindings/github.com/liteldev/LeviLauncher/internal/app/minecraft";
import { PageContainer } from "@/components/PageContainer";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { useLeviLamina } from "@/utils/LeviLaminaContext";
import { formatDateStr } from "@/utils/formatting";
import { normalizeIdentifierLookupKey } from "@/utils/modIntelligenceResolver";
import {
  fetchLIPPackageDetail,
  resolveSupportedGameVersionsByLLRanges,
  type LIPPackageDetail,
  type LIPPackageFileInfo,
  type LIPPackageVariantDetail,
} from "@/utils/content";

type GithubRepoRef = {
  owner: string;
  repo: string;
};

const hasUrlScheme = (value: string): boolean =>
  /^[a-zA-Z][a-zA-Z\d+.-]*:/.test(value);

const resolveMarkdownUrl = (
  rawUrl: string | undefined,
  repoRef: GithubRepoRef | null,
  target: "link" | "image",
): string => {
  const raw = String(rawUrl || "").trim();
  if (!raw) return "";
  if (raw.startsWith("#")) return raw;
  if (raw.startsWith("//")) return `https:${raw}`;
  if (hasUrlScheme(raw)) return raw;
  if (!repoRef) return raw;

  const root = target === "image" ? "raw.githubusercontent.com" : "github.com";
  const type = target === "image" ? "HEAD" : "blob/HEAD";
  const base = `https://${root}/${repoRef.owner}/${repoRef.repo}/${type}/`;
  const normalized = raw.startsWith("/") ? raw.slice(1) : raw;

  try {
    return new URL(normalized, base).toString();
  } catch {
    return raw;
  }
};

const isBadgeImage = (src: string): boolean =>
  /(?:^|\/\/)img\.shields\.io\//i.test(src);

const slugifyHeading = (value: string): string => {
  const slug = value
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s-]/gu, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
  return slug || "section";
};

const decodeHash = (href: string): string => {
  const raw = href.startsWith("#") ? href.slice(1) : href;
  try {
    return decodeURIComponent(raw).trim();
  } catch {
    return raw.trim();
  }
};

const isOpenableExternalUrl = (href: string): boolean => {
  try {
    const parsed = new URL(href);
    return (
      parsed.protocol === "http:" ||
      parsed.protocol === "https:" ||
      parsed.protocol === "mailto:"
    );
  } catch {
    return false;
  }
};

const LIPPackagePage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const {
    gameToLLVersions,
    mappingAvailable,
    mappingInitialized,
    loading: llMappingLoading,
  } = useLeviLamina();

  const [pkg, setPkg] = useState<LIPPackageDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selectedTab, setSelectedTab] = useState<string>("description");
  const [readmeContent, setReadmeContent] = useState<string>("");
  const [readmeLoading, setReadmeLoading] = useState<boolean>(false);
  const [installTarget, setInstallTarget] = useState<{
    version: string;
  } | null>(null);
  const [selectedVariantIdentifier, setSelectedVariantIdentifier] =
    useState<string>("");

  const tabsRef = useRef<HTMLDivElement>(null);
  const mappingReady = mappingInitialized && !llMappingLoading;

  const identifier = useMemo(() => {
    const raw = String(id || "");
    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }, [id]);
  useRouteTitle(
    !loading &&
      !error &&
      Boolean(pkg?.identifier) &&
      normalizeIdentifierLookupKey(pkg?.identifier || "") ===
        normalizeIdentifierLookupKey(identifier)
      ? pkg?.name
      : undefined,
  );

  const githubRepoRef = useMemo<GithubRepoRef | null>(() => {
    if (!pkg?.projectUrl) return null;
    try {
      const url = new URL(pkg.projectUrl);
      if (url.hostname !== "github.com") return null;
      const [owner, repo] = url.pathname.split("/").filter(Boolean);
      if (!owner || !repo) return null;
      return { owner, repo };
    } catch {
      return null;
    }
  }, [pkg?.projectUrl]);

  const markdownComponents = useMemo<Components>(() => {
    return {
      img: ({ src, alt, className, node: _node, ...props }) => {
        const resolvedSrc = resolveMarkdownUrl(src, githubRepoRef, "image");
        const badge = isBadgeImage(resolvedSrc);
        return (
          <img
            {...props}
            src={resolvedSrc}
            alt={alt || ""}
            loading="lazy"
            className={cn(
              className,
              "max-w-full h-auto",
              badge
                ? "!inline-block !align-middle !my-0 !mx-0 !rounded-none"
                : "inline-block align-middle rounded-lg",
            )}
          />
        );
      },
      a: ({ href, children, className, node: _node, ...props }) => {
        const resolvedHref = resolveMarkdownUrl(href, githubRepoRef, "link");
        const isHashAnchor = resolvedHref.startsWith("#");
        return (
          <a
            {...props}
            href={resolvedHref || href}
            target={isHashAnchor ? undefined : "_blank"}
            rel={isHashAnchor ? undefined : "noreferrer"}
            className={cn(className, "text-brand-600 dark:text-brand-500")}
            onClick={(event) => {
              if (!resolvedHref) return;
              if (isHashAnchor) {
                event.preventDefault();
                const anchor = decodeHash(resolvedHref);
                if (!anchor) return;

                const candidates = [
                  anchor,
                  anchor.toLowerCase(),
                  slugifyHeading(anchor),
                ];
                for (const candidate of candidates) {
                  const target = tabsRef.current?.querySelector(
                    `[id="${CSS.escape(candidate)}"], [id="${CSS.escape(`user-content-${candidate}`)}"]`,
                  );
                  if (!(target instanceof HTMLElement)) continue;
                  target.scrollIntoView({
                    behavior: shouldDisableAnimations() ? "auto" : "smooth",
                    block: "start",
                  });
                  break;
                }
                return;
              }

              if (!isOpenableExternalUrl(resolvedHref)) return;
              event.preventDefault();
              Browser.OpenURL(resolvedHref);
            }}
          >
            {children}
          </a>
        );
      },
    };
  }, [githubRepoRef]);

  const loadPackage = useCallback(
    async (targetIdentifier: string, forceRefresh = false) => {
      setLoading(true);
      setError("");

      try {
        const detail = await fetchLIPPackageDetail(targetIdentifier, {
          forceRefresh,
        });
        setPkg(detail);
      } catch (err) {
        console.error(err);
        setPkg(null);
        setError(t("common.load_failed"));
      } finally {
        setLoading(false);
      }
    },
    [t],
  );

  useEffect(() => {
    if (!identifier) {
      setPkg(null);
      setLoading(false);
      return;
    }
    void loadPackage(identifier);
  }, [identifier, loadPackage]);

  useEffect(() => {
    if (!pkg?.projectUrl) {
      setReadmeContent("");
      setReadmeLoading(false);
      return;
    }

    let cancelled = false;
    setReadmeContent("");
    setReadmeLoading(true);

    const fetchReadme = async () => {
      try {
        const text = await GetLIPPackageReadme(pkg.projectUrl);
        if (cancelled) return;
        setReadmeContent(String(text || ""));
        return;
      } catch (readmeError) {
        console.warn("Failed to fetch README", readmeError);
      }

      if (!cancelled) {
        setReadmeContent("");
      }
    };

    void fetchReadme().finally(() => {
      if (!cancelled) {
        setReadmeLoading(false);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [pkg?.projectUrl]);

  const routeVariantIdentifier = useMemo<string>(() => {
    const raw = String(identifier || "").trim();
    return raw.includes("#") ? raw : "";
  }, [identifier]);

  const packageVariants = useMemo<LIPPackageVariantDetail[]>(
    () => pkg?.variantDetails || [],
    [pkg?.variantDetails],
  );

  const visiblePackageVariants = useMemo<LIPPackageVariantDetail[]>(
    () =>
      packageVariants.filter((variant) =>
        String(variant.key || "")
          .trim()
          .toLowerCase()
          .includes("client"),
      ),
    [packageVariants],
  );

  const activeVariant = useMemo<LIPPackageVariantDetail | null>(() => {
    if (visiblePackageVariants.length === 0) return null;

    const candidates = [selectedVariantIdentifier, routeVariantIdentifier]
      .map((value) =>
        String(value || "")
          .trim()
          .toLowerCase(),
      )
      .filter(Boolean);
    for (const candidate of candidates) {
      const matched = visiblePackageVariants.find(
        (variant) =>
          String(variant.packageIdentifier || "")
            .trim()
            .toLowerCase() === candidate,
      );
      if (matched) return matched;
    }

    return resolveDefaultLIPVariant(pkg);
  }, [
    visiblePackageVariants,
    pkg,
    routeVariantIdentifier,
    selectedVariantIdentifier,
  ]);

  const activePackageIdentifier = useMemo<string>(
    () =>
      String(activeVariant?.packageIdentifier || pkg?.identifier || "").trim(),
    [activeVariant?.packageIdentifier, pkg?.identifier],
  );

  const activeVariantFiles = useMemo<LIPPackageFileInfo[]>(
    () => activeVariant?.files || pkg?.files || [],
    [activeVariant?.files, pkg?.files],
  );

  const filesWithGameVersionState = useMemo<FileGameVersionState[]>(
    () =>
      activeVariantFiles.map((file) => {
        const hasLLRequirement = file.llDependencyRanges.length > 0;
        const supportedGameVersions = hasLLRequirement
          ? resolveSupportedGameVersionsByLLRanges(
              file.llDependencyRanges,
              gameToLLVersions,
            )
          : [];

        return {
          file,
          supportedGameVersions,
          hasLLRequirement,
        };
      }),
    [activeVariantFiles, gameToLLVersions],
  );

  useEffect(() => {
    setSelectedVariantIdentifier("");
  }, [pkg?.identifier]);

  if (loading) {
    return (
      <PageContainer animate={false} className="min-h-0 no-scrollbar">
        <Card className={cn(LAYOUT.GLASS_CARD.BASE, "@container")}>
          <Card.Content className="p-4 @[40rem]:p-6">
            <div className="flex flex-col @[56rem]:flex-row gap-6">
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
              <div className="flex flex-col gap-3 w-full @[56rem]:w-60 justify-center">
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

        <Card className={cn(LAYOUT.GLASS_CARD.BASE, "min-h-48")}>
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

  if (error) {
    return (
      <div className="w-full h-full min-h-0 flex flex-col p-4 sm:p-6 gap-4 items-center justify-center">
        <Card className="bg-surface/50 dark:bg-surface/40 launcher-material-blur rounded-4xl p-8">
          <Card.Content className="flex flex-col items-center gap-4">
            <p className="text-xl font-bold text-rose-500">{error}</p>
            <Button
              onPress={() => void loadPackage(identifier, true)}
              variant={"primary"}
              className={
                "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20"
              }
            >
              {t("common.retry")}
            </Button>
            <Button onPress={() => navigate(-1)} variant={"ghost"}>
              {t("common.back")}
            </Button>
          </Card.Content>
        </Card>
      </div>
    );
  }

  if (!pkg) {
    return (
      <div className="w-full h-full min-h-0 flex flex-col p-4 sm:p-6 gap-4 items-center justify-center">
        <Card className="bg-surface/50 dark:bg-surface/40 launcher-material-blur rounded-4xl p-8">
          <Card.Content className="flex flex-col items-center gap-4">
            <p className="text-xl font-bold">{t("common.empty")}</p>
            <Button
              onPress={() => navigate(-1)}
              variant={"primary"}
              className={
                "bg-brand-500 hover:bg-brand-500 brand-primary-foreground font-bold shadow-lg shadow-brand-900/20"
              }
            >
              {t("common.back")}
            </Button>
          </Card.Content>
        </Card>
      </div>
    );
  }

  return (
    <PageContainer animate={false} className="min-h-0 no-scrollbar">
      <div className="max-w-7xl mx-auto w-full">
        <motion.div
          data-material-motion
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={cn(LAYOUT.GLASS_CARD.BASE, "mb-4 @container")}>
            <Card.Content className="p-4 @[40rem]:p-6">
              {/* One column below ~900px of content width. */}
              <div className="flex flex-col @[56rem]:flex-row gap-4 @[56rem]:gap-6">
                <div className="flex min-w-0 grow gap-4 @[48rem]:gap-6">
                  <div className="shrink-0">
                    {pkg.avatarUrl ? (
                      <img
                        src={pkg.avatarUrl}
                        alt={pkg.name}
                        className={
                          "size-16 @[48rem]:size-32 object-cover rounded-2xl shadow-lg bg-surface-secondary"
                        }
                      />
                    ) : (
                      <div className="size-16 @[48rem]:size-32 rounded-2xl bg-surface-secondary shadow-lg flex items-center justify-center text-muted">
                        <LuDownload className="size-1/3" />
                      </div>
                    )}
                  </div>

                  <div className="flex min-w-0 flex-col grow gap-3">
                    <h1 className="text-2xl @[48rem]:text-4xl font-black tracking-tight text-foreground dark:text-zinc-100 pb-1 break-words">
                      {pkg.name}
                    </h1>

                    <div className="flex items-center gap-3 text-muted dark:text-zinc-400 text-sm flex-wrap">
                      <div className="flex items-center gap-1">
                        <LuUser size={14} />
                        <span className="font-medium text-foreground dark:text-zinc-200">
                          {pkg.author || t("common.unknown")}
                        </span>
                      </div>
                      <span className="w-1 h-1 rounded-full bg-surface-quaternary"></span>
                      <div className="flex items-center gap-1">
                        <LuClock size={14} />
                        <span>{formatDateStr(pkg.updated)}</span>
                      </div>
                      <span className="w-1 h-1 rounded-full bg-surface-quaternary"></span>
                      <div className="flex items-center gap-1">
                        <LuFlame size={14} className="text-orange-500" />
                        <span>{pkg.hotness}</span>
                      </div>
                    </div>

                    <div className="flex flex-wrap gap-2 mt-1">
                      {pkg.tags.map((tag) => (
                        <Chip key={tag} size="sm" variant="soft">
                          {<LuTag size={12} />}
                          <Chip.Label>{tag}</Chip.Label>
                        </Chip>
                      ))}
                      <Chip size="sm" variant="secondary">
                        {<LuGamepad2 size={12} />}
                        <Chip.Label className="select-text">
                          ID: {pkg.identifier}
                        </Chip.Label>
                      </Chip>
                    </div>

                    <p className="text-muted dark:text-zinc-400 mt-2 @[40rem]:mt-4 text-sm leading-relaxed max-w-4xl">
                      {pkg.description || t("lip.no_description")}
                    </p>
                  </div>
                </div>

                <div className="flex flex-wrap items-center gap-3 @[56rem]:flex-col @[56rem]:flex-nowrap @[56rem]:items-stretch @[56rem]:w-60 @[56rem]:shrink-0 @[56rem]:border-l @[56rem]:border-border @[56rem]:pl-8 justify-center">
                  {/* Opens the install dialog with the newest version that
                      fits the target instance; the version can be changed
                      there. */}
                  <Button
                    size="lg"
                    onPress={() => setInstallTarget({ version: "" })}
                    isDisabled={activeVariantFiles.length === 0}
                    variant={"secondary"}
                    className={
                      "grow @[56rem]:grow-0 @[56rem]:w-full font-semibold shadow-md shadow-brand-900/20 brand-primary-foreground bg-brand-500 hover:bg-brand-500"
                    }
                  >
                    {<LuDownload size={20} />}
                    {t("lip.files.install")}
                  </Button>
                  <div className="flex gap-2 justify-center">
                    {pkg.projectUrl && (
                      <Tooltip>
                        <Button
                          onPress={() => Browser.OpenURL(pkg.projectUrl)}
                          isIconOnly
                          aria-label={t("audit.mods.project_page")}
                          variant={"secondary"}
                        >
                          <LuGlobe size={20} />
                        </Button>
                        <Tooltip.Content>
                          {t("audit.mods.project_page")}
                        </Tooltip.Content>
                      </Tooltip>
                    )}
                    <ProjectShareButton url={pkg.projectUrl} />
                  </div>
                </div>
              </div>
            </Card.Content>
          </Card>
        </motion.div>

        <motion.div
          data-material-motion
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, delay: 0.1 }}
        >
          <Card className={cn(LAYOUT.GLASS_CARD.BASE, "@container")}>
            <Card.Content className="p-4 @[40rem]:p-6 overflow-hidden">
              <div ref={tabsRef} className="flex w-full flex-col scroll-mt-24">
                <Tabs
                  selectedKey={selectedTab}
                  onSelectionChange={(key) => setSelectedTab(key as string)}
                  variant="secondary"
                >
                  <Tabs.ListContainer>
                    <Tabs.List
                      aria-label={t("lip.package_details_aria_label")}
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
                        {t("common.details")}
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
                        {t("lip.files.tab_label")}
                        <Tabs.Indicator
                          className={
                            "w-full bg-accent h-[3px]"
                          }
                        />
                      </Tabs.Tab>
                    </Tabs.List>
                  </Tabs.ListContainer>

                  <Tabs.Panel id="description">
                    <div className="prose dark:prose-invert max-w-none min-w-0 break-words prose-pre:max-w-full prose-pre:overflow-x-auto [&_table]:block [&_table]:max-w-full [&_table]:overflow-x-auto">
                      {readmeLoading ? (
                        <div className="flex flex-col items-center justify-center py-12 gap-3 text-muted dark:text-zinc-500">
                          <Spinner size="lg" color={"accent"} />
                          <p>{t("common.loading")}</p>
                        </div>
                      ) : readmeContent || pkg.description ? (
                        <ReactMarkdown
                          remarkPlugins={[remarkGfm]}
                          rehypePlugins={[rehypeRaw, rehypeSlug, rehypeSanitize]}
                          components={markdownComponents}
                        >
                          {readmeContent || pkg.description}
                        </ReactMarkdown>
                      ) : (
                        <div className="flex flex-col items-center justify-center py-12 text-muted gap-3">
                          <p>{t("lip.no_description")}</p>
                        </div>
                      )}
                    </div>
                  </Tabs.Panel>

                  <Tabs.Panel id="files">
                    <div className="flex flex-col gap-4">
                      {visiblePackageVariants.length > 1 ? (
                        <div className="max-w-sm">
                          <Select
                            placeholder={t("lip.files.variant_placeholder")}
                            value={
                              Array.from(
                                activePackageIdentifier
                                  ? [activePackageIdentifier]
                                  : [],
                              )[0] ?? null
                            }
                            onChange={(e) =>
                              setSelectedVariantIdentifier(
                                String(e || "").trim(),
                              )
                            }
                          >
                            <Label>
                              {
                                <div className="flex items-center gap-2">
                                  <span>{t("lip.files.variant_label")}</span>
                                  <span className="text-xs font-normal text-muted dark:text-zinc-500">
                                    {t("lip.files.variant_hint_keep_default")}
                                  </span>
                                </div>
                              }
                            </Label>
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
                                className={COMPONENT_STYLES.select.listbox}
                              >
                                {visiblePackageVariants.map((variant) => {
                                  const label = getLIPVariantDisplayLabel(
                                    variant,
                                    t("lip.files.variant_default"),
                                  );
                                  return (
                                    <ListBox.Item
                                      key={variant.packageIdentifier}
                                      id={variant.packageIdentifier}
                                      textValue={label}
                                    >
                                      <Label>
                                        <div className="flex flex-col">
                                          <span className="text-sm">
                                            {label}
                                          </span>
                                          <span className="text-xs text-muted font-mono">
                                            {variant.key
                                              ? variant.packageIdentifier
                                              : String(
                                                  pkg?.identifier || "",
                                                ).trim()}
                                          </span>
                                        </div>
                                      </Label>
                                      <ListBox.ItemIndicator />
                                    </ListBox.Item>
                                  );
                                })}
                              </ListBox>
                            </Select.Popover>
                          </Select>
                        </div>
                      ) : null}
                      <LIPVersionList
                        key={activePackageIdentifier}
                        files={filesWithGameVersionState}
                        packageName={pkg.name}
                        mappingReady={mappingReady}
                        mappingAvailable={mappingAvailable}
                        installDisabled={false}
                        onInstall={(version) => setInstallTarget({ version })}
                      />
                    </div>
                  </Tabs.Panel>
                </Tabs>
              </div>
            </Card.Content>
          </Card>
        </motion.div>
      </div>

      <LIPInstallDialog
        isOpen={installTarget !== null}
        onClose={() => setInstallTarget(null)}
        identifier={activePackageIdentifier}
        pkg={pkg}
        variant={activeVariant}
        initialVersion={installTarget?.version}
      />

    </PageContainer>
  );
};

export default LIPPackagePage;
