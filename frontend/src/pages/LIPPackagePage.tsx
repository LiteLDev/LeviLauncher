import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  addToast,
  Button,
  Card,
  CardBody,
  Chip,
  Image,
  Select,
  SelectItem,
  Spinner,
  Skeleton,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
  Tabs,
  Tooltip,
} from "@heroui/react";
import { Call, Browser } from "@wailsio/runtime";
import { motion } from "framer-motion";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  LuClock,
  LuDownload,
  LuFileDigit,
  LuFlame,
  LuGamepad2,
  LuGlobe,
  LuShare2,
  LuTag,
  LuUser,
} from "react-icons/lu";
import {
  GetVersionLogoDataUrl,
  ListVersionMetas,
} from "bindings/github.com/liteldev/LeviLauncher/versionservice";
import { PageContainer } from "@/components/PageContainer";
import { UnifiedModal } from "@/components/UnifiedModal";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { useCurrentVersion } from "@/utils/CurrentVersionContext";
import { useModIntelligence } from "@/utils/ModIntelligenceContext";
import { formatDateStr } from "@/utils/formatting";
import { useLipTaskConsole } from "@/utils/LipTaskConsoleContext";
import { compareVersions } from "@/utils/version";
import type { LIPPackageInstallState } from "@/utils/modIntelligenceResolver";
import {
  fetchLIPLeviLaminaClientMapping,
  fetchLIPPackageDetail,
  isLeviLaminaVersionCompatible,
  resolveSupportedGameVersionsByLLRanges,
  type LIPPackageDetail,
  type LIPPackageFileInfo,
  type LIPPackageVariantDetail,
} from "@/utils/content";

type GithubRepoRef = {
  owner: string;
  repo: string;
};

type HeadingTag = "h1" | "h2" | "h3" | "h4" | "h5" | "h6";

type FileGameVersionState = {
  file: LIPPackageFileInfo;
  supportedGameVersions: string[];
  hasLLRequirement: boolean;
};

type InstanceLLState = {
  installed: boolean;
  explicitInstalled: boolean;
  installedVersion: string;
  error: string;
  loading: boolean;
};

type InstancePackageState = InstanceLLState;
type InstallDialogActionKind =
  | "install"
  | "upgrade"
  | "downgrade"
  | "reinstall";

const EMPTY_INSTANCE_LL_STATE: InstanceLLState = {
  installed: false,
  explicitInstalled: false,
  installedVersion: "",
  error: "",
  loading: false,
};

const INSTALL_DIALOG_ACTION_LABEL_KEYS: Record<
  InstallDialogActionKind,
  string
> = {
  install: "lip.files.install",
  upgrade: "lip.files.upgrade",
  downgrade: "lip.files.downgrade",
  reinstall: "lip.files.reinstall",
};

const INSTALL_DIALOG_ACTION_SUCCESS_KEYS: Record<
  InstallDialogActionKind,
  string
> = {
  install: "lip.files.install_success",
  upgrade: "lip.files.upgrade_success",
  downgrade: "lip.files.downgrade_success",
  reinstall: "lip.files.reinstall_success",
};

const getVariantDisplayLabel = (
  variant: Pick<LIPPackageVariantDetail, "key" | "label"> | null | undefined,
  defaultLabel: string,
): string => {
  const explicitLabel = String(variant?.label || "").trim();
  if (explicitLabel) return explicitLabel;

  const variantKey = String(variant?.key || "").trim();
  return variantKey || defaultLabel;
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

const collectNodeText = (node: React.ReactNode): string => {
  if (node === null || node === undefined || typeof node === "boolean") {
    return "";
  }
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) {
    return node.map((item) => collectNodeText(item)).join("");
  }
  if (React.isValidElement(node)) {
    return collectNodeText(
      (node.props as { children?: React.ReactNode }).children,
    );
  }
  return "";
};

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

const createHeadingSlugger = () => {
  const used = new Map<string, number>();
  return (text: string) => {
    const base = slugifyHeading(text);
    const current = used.get(base) || 0;
    used.set(base, current + 1);
    return current === 0 ? base : `${base}-${current}`;
  };
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

const getErrorCode = (value: unknown): string => {
  const source =
    typeof value === "string"
      ? value
      : value instanceof Error
        ? value.message
        : String(value || "");
  const raw = source.trim();
  if (!raw) return "";
  const match = raw.match(/ERR_[A-Z0-9_]+/);
  return match ? match[0] : raw;
};

const normalizeGameVersion = (value: string): string => {
  const normalized = String(value || "")
    .trim()
    .replace(/^v/i, "");
  if (!normalized) return "";
  const match = normalized.match(/(\d+\.\d+\.\d+)/);
  return match ? match[1] : normalized;
};

const LIPPackagePage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { runWithLipTask } = useLipTaskConsole();
  const { currentVersionName } = useCurrentVersion();
  const {
    ensureInstanceHydrated,
    getInstanceSnapshot,
    ensurePackageInstallState,
    refreshInstance,
    snapshotRevision,
  } = useModIntelligence();

  const [pkg, setPkg] = useState<LIPPackageDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selectedTab, setSelectedTab] = useState<string>("description");
  const [readmeContent, setReadmeContent] = useState<string>("");
  const [readmeLoading, setReadmeLoading] = useState<boolean>(false);
  const [instanceOptions, setInstanceOptions] = useState<string[]>([]);
  const [selectedInstance, setSelectedInstance] = useState<string>("");
  const [instanceGameVersions, setInstanceGameVersions] = useState<
    Record<string, string>
  >({});
  const [instanceLogos, setInstanceLogos] = useState<Record<string, string>>(
    {},
  );
  const [instanceLLStates, setInstanceLLStates] = useState<
    Record<string, InstanceLLState>
  >({});
  const [instancePackageStates, setInstancePackageStates] = useState<
    Record<string, InstancePackageState>
  >({});
  const [installDialogSelectedInstance, setInstallDialogSelectedInstance] =
    useState<string>("");
  const [gameToLLVersions, setGameToLLVersions] = useState<
    Record<string, string[]>
  >({});
  const [mappingUnavailable, setMappingUnavailable] = useState<boolean>(false);
  const [installDialogOpen, setInstallDialogOpen] = useState<boolean>(false);
  const [installDialogTriggerVersion, setInstallDialogTriggerVersion] =
    useState<string>("");
  const [selectedVariantIdentifier, setSelectedVariantIdentifier] =
    useState<string>("");
  const [actionRunning, setActionRunning] = useState<boolean>(false);

  const tabsRef = useRef<HTMLDivElement>(null);

  const identifier = useMemo(() => {
    const raw = String(id || "");
    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }, [id]);

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
    const nextHeadingID = createHeadingSlugger();
    const createHeading =
      (tag: HeadingTag) =>
      ({
        children,
        className,
        id: headingID,
        ...props
      }: React.HTMLAttributes<HTMLHeadingElement> & {
        children?: React.ReactNode;
      }) =>
        React.createElement(
          tag,
          {
            ...props,
            id:
              typeof headingID === "string" && headingID.trim()
                ? headingID
                : nextHeadingID(collectNodeText(children)),
            className,
          },
          children,
        );

    return {
      img: ({ src, alt, className, ...props }) => {
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
              "max-w-full",
              badge
                ? "!inline-block !align-middle !my-0 !mx-0 !rounded-none"
                : "block my-4 rounded-xl",
            )}
          />
        );
      },
      a: ({ href, children, className, ...props }) => {
        const resolvedHref = resolveMarkdownUrl(href, githubRepoRef, "link");
        const isHashAnchor = resolvedHref.startsWith("#");
        return (
          <a
            {...props}
            href={resolvedHref || href}
            target={isHashAnchor ? undefined : "_blank"}
            rel={isHashAnchor ? undefined : "noreferrer"}
            className={cn(className, "text-primary-600 dark:text-primary-500")}
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
                  const target = document.getElementById(candidate);
                  if (!(target instanceof HTMLElement)) continue;
                  target.scrollIntoView({ behavior: "smooth", block: "start" });
                  if (window.location.hash !== `#${candidate}`) {
                    window.history.replaceState(null, "", `#${candidate}`);
                  }
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
      h1: createHeading("h1"),
      h2: createHeading("h2"),
      h3: createHeading("h3"),
      h4: createHeading("h4"),
      h5: createHeading("h5"),
      h6: createHeading("h6"),
    };
  }, [githubRepoRef]);

  const resolveErrorText = useCallback(
    (value: unknown) => {
      const code = getErrorCode(value);
      if (!code) return t("common.error");

      const translatedByErrorCode = t(`errors.${code}`);
      if (translatedByErrorCode !== `errors.${code}`) {
        return translatedByErrorCode;
      }

      const translatedByKey = t(code);
      if (translatedByKey !== code) {
        return translatedByKey;
      }

      return code;
    },
    [t],
  );

  const toInstanceState = useCallback(
    (
      value: LIPPackageInstallState | null | undefined,
      loading = false,
    ): InstanceLLState => ({
      installed: Boolean(value?.installed),
      explicitInstalled: Boolean(value?.explicitInstalled),
      installedVersion: String(value?.installedVersion || "").trim(),
      error: String(value?.error || "").trim(),
      loading,
    }),
    [],
  );

  const callMinecraftByName = useCallback(
    async <T,>(method: string, ...args: unknown[]): Promise<T> =>
      (await Call.ByName(`main.Minecraft.${method}`, ...args)) as T,
    [],
  );

  const queryInstanceLLState = useCallback(
    async (instanceName: string): Promise<InstanceLLState> => {
      const normalizedName = String(instanceName || "").trim();
      if (!normalizedName) {
        return {
          ...EMPTY_INSTANCE_LL_STATE,
          error: "ERR_TARGET_NOT_FOUND",
        };
      }

      try {
        await ensureInstanceHydrated(normalizedName, {
          background: true,
          reason: "lip-package-query-ll",
        });
        const snapshot = getInstanceSnapshot(normalizedName);
        if (snapshot?.llState) {
          return toInstanceState(snapshot.llState);
        }
        return {
          ...EMPTY_INSTANCE_LL_STATE,
          error: "ERR_LIP_PACKAGE_QUERY_FAILED",
        };
      } catch (queryError) {
        return {
          ...EMPTY_INSTANCE_LL_STATE,
          error: getErrorCode(queryError) || "ERR_LIP_PACKAGE_QUERY_FAILED",
        };
      }
    },
    [ensureInstanceHydrated, getInstanceSnapshot, toInstanceState],
  );

  const queryInstancePackageState = useCallback(
    async (
      instanceName: string,
      packageIdentifier: string,
    ): Promise<InstancePackageState> => {
      const normalizedName = String(instanceName || "").trim();
      const normalizedIdentifier = String(packageIdentifier || "").trim();
      if (!normalizedName) {
        return {
          ...EMPTY_INSTANCE_LL_STATE,
          error: "ERR_TARGET_NOT_FOUND",
        };
      }
      if (!normalizedIdentifier) {
        return {
          ...EMPTY_INSTANCE_LL_STATE,
          error: "ERR_LIP_PACKAGE_INVALID_IDENTIFIER",
        };
      }

      try {
        const state = await ensurePackageInstallState(
          normalizedName,
          normalizedIdentifier,
        );
        return toInstanceState(state);
      } catch (queryError) {
        return {
          ...EMPTY_INSTANCE_LL_STATE,
          error: getErrorCode(queryError) || "ERR_LIP_PACKAGE_QUERY_FAILED",
        };
      }
    },
    [ensurePackageInstallState, toInstanceState],
  );

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
        const text = await callMinecraftByName<string>(
          "GetLIPPackageReadme",
          pkg.projectUrl,
        );
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
  }, [callMinecraftByName, pkg?.projectUrl]);

  useEffect(() => {
    let cancelled = false;

    const loadInstances = async () => {
      try {
        const metas = await ListVersionMetas();
        const names: string[] = [];
        const gameVersionMap: Record<string, string> = {};
        if (Array.isArray(metas)) {
          for (const meta of metas as any[]) {
            const name = String(meta?.name || "").trim();
            if (!name) continue;
            names.push(name);
            gameVersionMap[name] = String(meta?.gameVersion || "").trim();
          }
        }

        if (cancelled) return;
        setInstanceOptions(names);
        setInstanceGameVersions(gameVersionMap);
        const logoMap: Record<string, string> = {};
        await Promise.all(
          names.map(async (name) => {
            try {
              const logoUrl = await GetVersionLogoDataUrl(name);
              if (logoUrl) {
                logoMap[name] = logoUrl;
              }
            } catch (logoError) {
              console.warn("Failed to fetch logo for", name, logoError);
            }
          }),
        );
        if (!cancelled) {
          setInstanceLogos(logoMap);
        }
        setSelectedInstance((prev) => {
          if (prev && names.includes(prev)) {
            return prev;
          }
          const current = (
            String(currentVersionName || "").trim() ||
            readCurrentVersionName().trim()
          ).trim();
          if (current && names.includes(current)) {
            return current;
          }
          return names[0] || "";
        });

        if (names.length === 0) {
          setInstanceLLStates({});
          return;
        }

        setInstanceLLStates((prev) => {
          const next = { ...prev };
          for (const name of names) {
            next[name] = {
              ...(next[name] || EMPTY_INSTANCE_LL_STATE),
              loading: true,
              error: "",
            };
          }
          return next;
        });

        const llEntries = await Promise.all(
          names.map(
            async (name) => [name, await queryInstanceLLState(name)] as const,
          ),
        );
        if (cancelled) return;
        setInstanceLLStates((prev) => {
          const next = { ...prev };
          for (const [name, llState] of llEntries) {
            next[name] = llState;
          }
          return next;
        });
      } catch {
        if (cancelled) return;
        setInstanceOptions([]);
        setInstanceGameVersions({});
        setInstanceLogos({});
        setInstanceLLStates({});
        setSelectedInstance("");
      }
    };

    void loadInstances();
    return () => {
      cancelled = true;
    };
  }, [currentVersionName, queryInstanceLLState]);

  useEffect(() => {
    if (instanceOptions.length === 0) return;

    setInstanceLLStates((prev) => {
      let changed = false;
      const next = { ...prev };
      for (const name of instanceOptions) {
        const snapshot = getInstanceSnapshot(name);
        if (!snapshot?.llState) continue;
        const incoming = toInstanceState(snapshot.llState);
        const existing = next[name];
        if (
          existing &&
          existing.installed === incoming.installed &&
          existing.explicitInstalled === incoming.explicitInstalled &&
          existing.installedVersion === incoming.installedVersion &&
          existing.error === incoming.error &&
          existing.loading === incoming.loading
        ) {
          continue;
        }
        next[name] = incoming;
        changed = true;
      }
      return changed ? next : prev;
    });
  }, [getInstanceSnapshot, instanceOptions, snapshotRevision, toInstanceState]);

  useEffect(() => {
    let cancelled = false;

    const loadLeviLaminaMapping = async () => {
      try {
        const mapping = await fetchLIPLeviLaminaClientMapping();
        if (cancelled) return;
        setGameToLLVersions(mapping.gameToLLVersions || {});
        setMappingUnavailable(false);
      } catch (mappingError) {
        console.warn("Failed to load LeviLamina game mapping", mappingError);
        if (cancelled) return;
        setGameToLLVersions({});
        setMappingUnavailable(true);
      }
    };

    void loadLeviLaminaMapping();
    return () => {
      cancelled = true;
    };
  }, []);

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
      .map((value) => String(value || "").trim().toLowerCase())
      .filter(Boolean);
    for (const candidate of candidates) {
      const matched = visiblePackageVariants.find(
        (variant) =>
          String(variant.packageIdentifier || "").trim().toLowerCase() ===
          candidate,
      );
      if (matched) return matched;
    }

    const preferredVariantKey = String(pkg?.preferredVariantKey || "")
      .trim()
      .toLowerCase();
    if (preferredVariantKey) {
      const preferredVariant = visiblePackageVariants.find(
        (variant) => String(variant.key || "").trim().toLowerCase() === preferredVariantKey,
      );
      if (preferredVariant) return preferredVariant;
    }

    return visiblePackageVariants[0] || null;
  }, [
    visiblePackageVariants,
    pkg?.preferredVariantKey,
    routeVariantIdentifier,
    selectedVariantIdentifier,
  ]);

  const activePackageIdentifier = useMemo<string>(
    () => String(activeVariant?.packageIdentifier || pkg?.identifier || "").trim(),
    [activeVariant?.packageIdentifier, pkg?.identifier],
  );

  const activeVariantDisplayLabel = useMemo<string>(
    () => getVariantDisplayLabel(activeVariant, t("lip.files.variant_default")),
    [activeVariant, t],
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

  const installDialogFileState = useMemo<FileGameVersionState | null>(() => {
    if (!installDialogTriggerVersion) return null;
    return (
      filesWithGameVersionState.find(
        (state) => state.file.version === installDialogTriggerVersion,
      ) || null
    );
  }, [filesWithGameVersionState, installDialogTriggerVersion]);

  const installDialogInstanceGameVersion = useMemo<string>(() => {
    const raw = instanceGameVersions[installDialogSelectedInstance] || "";
    return normalizeGameVersion(raw);
  }, [instanceGameVersions, installDialogSelectedInstance]);

  const dialogRequiresLL = Boolean(installDialogFileState?.hasLLRequirement);

  const dialogInstancePackageState =
    useMemo<InstancePackageState | null>(() => {
      const instanceName = String(installDialogSelectedInstance || "").trim();
      if (!instanceName) return null;
      return instancePackageStates[instanceName] || null;
    }, [installDialogSelectedInstance, instancePackageStates]);

  const dialogInstalledPackageVersion = useMemo<string>(() => {
    const installedVersion = String(
      dialogInstancePackageState?.installedVersion || "",
    ).trim();
    if (!dialogInstancePackageState?.installed || !installedVersion) return "";
    return installedVersion;
  }, [dialogInstancePackageState]);

  const dialogPackageStateLoading = Boolean(dialogInstancePackageState?.loading);

  const installDialogActionKind = useMemo<InstallDialogActionKind>(() => {
    if (!installDialogTriggerVersion || dialogPackageStateLoading) {
      return "install";
    }
    if (!dialogInstancePackageState?.installed) {
      return "install";
    }
    if (!dialogInstalledPackageVersion) {
      return "reinstall";
    }

    const versionCompare = compareVersions(
      installDialogTriggerVersion,
      dialogInstalledPackageVersion,
    );
    if (versionCompare > 0) return "upgrade";
    if (versionCompare < 0) return "downgrade";
    return "reinstall";
  }, [
    dialogInstalledPackageVersion,
    dialogInstancePackageState?.installed,
    dialogPackageStateLoading,
    installDialogTriggerVersion,
  ]);

  const installDialogActionLabelKey =
    INSTALL_DIALOG_ACTION_LABEL_KEYS[installDialogActionKind];
  const installDialogSuccessKey =
    INSTALL_DIALOG_ACTION_SUCCESS_KEYS[installDialogActionKind];

  const dialogInstanceLLState = useMemo<InstanceLLState | null>(() => {
    const instanceName = String(installDialogSelectedInstance || "").trim();
    if (!instanceName) return null;
    return instanceLLStates[instanceName] || null;
  }, [instanceLLStates, installDialogSelectedInstance]);

  const dialogLLStateLoading =
    dialogRequiresLL &&
    (!dialogInstanceLLState || dialogInstanceLLState.loading);
  const dialogLLInstalled = Boolean(dialogInstanceLLState?.installed);
  const dialogLLVersion = String(
    dialogInstanceLLState?.installedVersion || "",
  ).trim();

  const installDialogPrimaryActionLabelKey = useMemo<string>(() => {
    if (dialogRequiresLL && !dialogLLStateLoading && !dialogLLInstalled) {
      return "lip.files.redirect";
    }
    return installDialogActionLabelKey;
  }, [
    dialogLLInstalled,
    dialogLLStateLoading,
    dialogRequiresLL,
    installDialogActionLabelKey,
  ]);

  const dialogInstalledLLCompatible = useMemo<boolean>(() => {
    if (!dialogRequiresLL) return true;
    if (!dialogLLInstalled || !dialogLLVersion || !installDialogFileState) {
      return false;
    }
    return isLeviLaminaVersionCompatible(
      dialogLLVersion,
      installDialogFileState.file.llDependencyRanges,
    );
  }, [
    dialogRequiresLL,
    dialogLLInstalled,
    dialogLLVersion,
    installDialogFileState,
  ]);

  useEffect(() => {
    const instanceName = String(installDialogSelectedInstance || "").trim();
    if (!instanceName) return;
    const existingState = instanceLLStates[instanceName];
    if (existingState && !existingState.loading) return;

    let cancelled = false;
    setInstanceLLStates((prev) => ({
      ...prev,
      [instanceName]: {
        ...EMPTY_INSTANCE_LL_STATE,
        loading: true,
      },
    }));

    const run = async () => {
      const llState = await queryInstanceLLState(instanceName);
      if (cancelled) return;
      setInstanceLLStates((prev) => ({
        ...prev,
        [instanceName]: llState,
      }));
    };
    void run();

    return () => {
      cancelled = true;
    };
  }, [installDialogSelectedInstance, queryInstanceLLState]);

  useEffect(() => {
    setSelectedVariantIdentifier("");
  }, [pkg?.identifier]);

  useEffect(() => {
    setInstancePackageStates({});
  }, [activePackageIdentifier]);

  useEffect(() => {
    const instanceName = String(installDialogSelectedInstance || "").trim();
    const packageIdentifier = activePackageIdentifier;
    if (!installDialogOpen || !instanceName || !packageIdentifier) return;
    const existingState = instancePackageStates[instanceName];
    if (existingState && !existingState.loading) return;

    let cancelled = false;
    setInstancePackageStates((prev) => ({
      ...prev,
      [instanceName]: {
        ...EMPTY_INSTANCE_LL_STATE,
        loading: true,
      },
    }));

    const run = async () => {
      const state = await queryInstancePackageState(
        instanceName,
        packageIdentifier,
      );
      if (cancelled) return;
      setInstancePackageStates((prev) => ({
        ...prev,
        [instanceName]: state,
      }));
    };
    void run();

    return () => {
      cancelled = true;
    };
  }, [
    activePackageIdentifier,
    installDialogOpen,
    installDialogSelectedInstance,
    queryInstancePackageState,
  ]);

  const installDialogGameVersionCompatible = useMemo<boolean>(() => {
    if (!installDialogFileState) return false;
    if (!installDialogFileState.hasLLRequirement) return true;
    if (mappingUnavailable) return true;
    if (!installDialogInstanceGameVersion) return false;

    const supported = new Set(
      installDialogFileState.supportedGameVersions.map((version) =>
        normalizeGameVersion(version),
      ),
    );
    return supported.has(installDialogInstanceGameVersion);
  }, [
    installDialogFileState,
    installDialogInstanceGameVersion,
    mappingUnavailable,
  ]);

  const installDialogVersionInstallable = useMemo<boolean>(() => {
    if (!installDialogFileState) return false;
    if (!installDialogGameVersionCompatible) return false;
    if (!dialogRequiresLL) return true;
    if (dialogLLStateLoading) return false;

    if (dialogLLInstalled) {
      return dialogInstalledLLCompatible;
    }

    return true;
  }, [
    installDialogFileState,
    installDialogGameVersionCompatible,
    dialogRequiresLL,
    dialogLLStateLoading,
    dialogLLInstalled,
    dialogInstalledLLCompatible,
  ]);

  const redirectToInstanceLoaderSettings = useCallback(
    (instanceName: string) => {
      const normalizedName = String(instanceName || "").trim();
      if (!normalizedName) return;

      setInstallDialogOpen(false);
      setInstallDialogTriggerVersion("");
      addToast({
        color: "warning",
        title: t("common.tip"),
        description: t("lip.files.ll_missing_redirect_to_loader"),
      });
      navigate("/versionSettings", {
        state: {
          name: normalizedName,
          returnTo: `${location.pathname}${location.search}`,
          tab: "loader",
        },
      });
    },
    [location.pathname, location.search, navigate, t],
  );

  const openInstallDialog = useCallback(
    (version: string) => {
      const normalized = String(version || "").trim();
      if (!normalized) return;

      setInstallDialogTriggerVersion(normalized);
      setInstallDialogSelectedInstance((prev) => {
        if (prev && instanceOptions.includes(prev)) {
          return prev;
        }
        if (selectedInstance && instanceOptions.includes(selectedInstance)) {
          return selectedInstance;
        }
        return instanceOptions[0] || "";
      });
      setInstallDialogOpen(true);
    },
    [instanceOptions, selectedInstance],
  );

  const closeInstallDialog = useCallback(() => {
    if (actionRunning) return;
    setInstallDialogOpen(false);
    setInstallDialogTriggerVersion("");
  }, [actionRunning]);

  const handleConfirmInstall = useCallback(async () => {
    if (
      !pkg ||
      !installDialogSelectedInstance ||
      !installDialogTriggerVersion ||
      dialogPackageStateLoading
    ) {
      return;
    }

    if (dialogRequiresLL && !dialogLLStateLoading && !dialogLLInstalled) {
      redirectToInstanceLoaderSettings(installDialogSelectedInstance);
      return;
    }

    setActionRunning(true);
    try {
      await runWithLipTask(
        {
          action: installDialogActionKind,
          target: installDialogSelectedInstance,
          methods: ["Install", "Update"],
          feedbackMode: "on_error",
        },
        async ({ addLog }) => {
          addLog(
            "info",
            `${t(installDialogActionLabelKey)} ${activePackageIdentifier}@${installDialogTriggerVersion}`,
          );

          const err = await callMinecraftByName<string>(
            "InstallLIPPackage",
            installDialogSelectedInstance,
            activePackageIdentifier,
            installDialogTriggerVersion,
          );
          if (String(err || "").trim()) {
            throw new Error(String(err));
          }
        },
      );

      addToast({
        color: "success",
        title: t("common.success"),
        description: t(installDialogSuccessKey),
      });
      await refreshInstance(installDialogSelectedInstance, "lip-package-install");
      const latestSnapshot = getInstanceSnapshot(installDialogSelectedInstance);
      if (latestSnapshot?.llState) {
        setInstanceLLStates((prev) => ({
          ...prev,
          [installDialogSelectedInstance]: toInstanceState(latestSnapshot.llState),
        }));
      }
      const packageState = await ensurePackageInstallState(
        installDialogSelectedInstance,
        activePackageIdentifier,
      );
      setInstancePackageStates((prev) => ({
        ...prev,
        [installDialogSelectedInstance]: toInstanceState(packageState),
      }));
      setInstallDialogOpen(false);
      setInstallDialogTriggerVersion("");
    } catch (actionError) {
      addToast({
        color: "danger",
        title: t("common.error"),
        description: t("lip.files.action_failed", {
          error: resolveErrorText(actionError),
        }),
      });
    } finally {
      setActionRunning(false);
    }
  }, [
    activePackageIdentifier,
    callMinecraftByName,
    dialogLLInstalled,
    dialogLLStateLoading,
    dialogPackageStateLoading,
    dialogRequiresLL,
    installDialogActionKind,
    installDialogActionLabelKey,
    installDialogSelectedInstance,
    pkg,
    redirectToInstanceLoaderSettings,
    resolveErrorText,
    runWithLipTask,
    ensurePackageInstallState,
    getInstanceSnapshot,
    installDialogSuccessKey,
    installDialogTriggerVersion,
    refreshInstance,
    t,
    toInstanceState,
  ]);

  if (loading) {
    return (
      <PageContainer animate={false} className="min-h-0 no-scrollbar">
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <CardBody className="p-6">
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
              <div className="flex flex-col gap-3 min-w-[200px] justify-center">
                <Skeleton className="h-12 w-full rounded-xl" />
                <div className="flex gap-2 justify-center">
                  <Skeleton className="h-10 w-10 rounded-lg" />
                  <Skeleton className="h-10 w-10 rounded-lg" />
                  <Skeleton className="h-10 w-10 rounded-lg" />
                </div>
              </div>
            </div>
          </CardBody>
        </Card>

        <Card className={cn(LAYOUT.GLASS_CARD.BASE, "mt-6 min-h-[300px]")}>
          <CardBody className="p-6">
            <div className="flex gap-6 mb-6">
              <Skeleton className="h-8 w-24 rounded-lg" />
              <Skeleton className="h-8 w-24 rounded-lg" />
            </div>
            <div className="space-y-4">
              <Skeleton className="h-4 w-full rounded-md" />
              <Skeleton className="h-4 w-full rounded-md" />
              <Skeleton className="h-4 w-3/4 rounded-md" />
            </div>
          </CardBody>
        </Card>
      </PageContainer>
    );
  }

  if (error) {
    return (
      <div className="w-full h-full min-h-0 flex flex-col p-4 sm:p-6 gap-4 items-center justify-center">
        <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl p-8">
          <CardBody className="flex flex-col items-center gap-4">
            <p className="text-xl font-bold text-danger-500">{error}</p>
            <Button
              onPress={() => void loadPackage(identifier, true)}
              color="primary"
              className="bg-primary-500 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
            >
              {t("common.retry")}
            </Button>
            <Button onPress={() => navigate(-1)} variant="light">
              {t("common.back")}
            </Button>
          </CardBody>
        </Card>
      </div>
    );
  }

  if (!pkg) {
    return (
      <div className="w-full h-full min-h-0 flex flex-col p-4 sm:p-6 gap-4 items-center justify-center">
        <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl p-8">
          <CardBody className="flex flex-col items-center gap-4">
            <p className="text-xl font-bold">{t("common.empty")}</p>
            <Button
              onPress={() => navigate(-1)}
              color="primary"
              className="bg-primary-500 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
            >
              {t("common.back")}
            </Button>
          </CardBody>
        </Card>
      </div>
    );
  }

  return (
    <PageContainer animate={false} className="min-h-0 no-scrollbar">
      <div className="max-w-7xl mx-auto w-full">
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4 }}
        >
          <Card className={cn(LAYOUT.GLASS_CARD.BASE, "mb-6")}>
            <CardBody className="p-6">
              <div className="flex flex-col md:flex-row gap-6">
                <div className="shrink-0">
                  {pkg.avatarUrl ? (
                    <Image
                      src={pkg.avatarUrl}
                      alt={pkg.name}
                      className="w-32 h-32 object-cover rounded-2xl shadow-lg bg-content2"
                    />
                  ) : (
                    <div className="w-32 h-32 rounded-2xl bg-content2 shadow-lg flex items-center justify-center text-default-300">
                      <LuDownload size={36} />
                    </div>
                  )}
                </div>

                <div className="flex flex-col grow gap-3">
                  <h1 className="text-3xl sm:text-4xl font-black tracking-tight text-default-900 dark:text-zinc-100 pb-1">
                    {pkg.name}
                  </h1>

                  <div className="flex items-center gap-3 text-default-500 dark:text-zinc-400 text-sm flex-wrap">
                    <div className="flex items-center gap-1">
                      <LuUser size={14} />
                      <span className="font-medium text-default-700 dark:text-zinc-200">
                        {pkg.author || t("common.unknown")}
                      </span>
                    </div>
                    <span className="w-1 h-1 rounded-full bg-default-300"></span>
                    <div className="flex items-center gap-1">
                      <LuClock size={14} />
                      <span>{formatDateStr(pkg.updated)}</span>
                    </div>
                    <span className="w-1 h-1 rounded-full bg-default-300"></span>
                    <div className="flex items-center gap-1">
                      <LuFlame size={14} className="text-orange-500" />
                      <span>{pkg.hotness}</span>
                    </div>
                  </div>

                  <div className="flex flex-wrap gap-2 mt-1">
                    {pkg.tags.map((tag) => (
                      <Chip
                        key={tag}
                        variant="flat"
                        size="sm"
                        startContent={<LuTag size={12} />}
                      >
                        {tag}
                      </Chip>
                    ))}
                    <Chip
                      size="sm"
                      variant="bordered"
                      startContent={<LuGamepad2 size={12} />}
                    >
                      ID: {pkg.identifier}
                    </Chip>
                  </div>

                  <p className="text-default-500 dark:text-zinc-400 mt-4 text-sm leading-relaxed max-w-4xl">
                    {pkg.description || t("lip.no_description")}
                  </p>
                </div>

                <div className="flex flex-col gap-3 min-w-[240px] md:border-l md:border-default-100 md:pl-8 justify-center">
                  <Button
                    className="w-full font-semibold shadow-md shadow-primary-900/20 text-white bg-primary-500 hover:bg-primary-500"
                    startContent={<LuDownload size={20} />}
                    size="lg"
                    onPress={() => {
                      setSelectedTab("files");
                      setTimeout(() => {
                        tabsRef.current?.scrollIntoView({
                          behavior: "smooth",
                          block: "start",
                        });
                      }, 100);
                    }}
                  >
                    {t("lip.files.install")}
                  </Button>
                  <div className="flex gap-2 justify-center">
                    {pkg.projectUrl && (
                      <Button
                        onPress={() => Browser.OpenURL(pkg.projectUrl)}
                        isIconOnly
                        variant="flat"
                        aria-label="Project Page"
                      >
                        <LuGlobe size={20} />
                      </Button>
                    )}
                    <Button isIconOnly variant="flat" aria-label="Share">
                      <LuShare2 size={20} />
                    </Button>
                  </div>
                </div>
              </div>
            </CardBody>
          </Card>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, delay: 0.1 }}
        >
          <Card className={cn(LAYOUT.GLASS_CARD.BASE, "min-h-[500px]")}>
            <CardBody className="p-6 overflow-hidden">
              <div ref={tabsRef} className="flex w-full flex-col scroll-mt-24">
                <Tabs
                  aria-label={t("lip.package_details_aria_label")}
                  variant="underlined"
                  color="primary"
                  selectedKey={selectedTab}
                  onSelectionChange={(key) => setSelectedTab(key as string)}
                  classNames={{
                    tabList:
                      "gap-6 w-full relative rounded-none p-0 border-b border-default-200 mb-6",
                    cursor:
                      "w-full bg-linear-to-r from-primary-500 to-primary-400 h-[3px]",
                    tab: "max-w-fit px-0 h-12 text-base font-medium text-default-500 dark:text-zinc-400",
                    tabContent:
                      "group-data-[selected=true]:text-primary-600 dark:group-data-[selected=true]:text-primary-500 font-bold",
                  }}
                >
                  <Tab key="description" title={t("common.details")}>
                    <div className="prose dark:prose-invert max-w-none">
                      {readmeLoading ? (
                        <div className="flex flex-col items-center justify-center py-12 gap-3 text-default-400 dark:text-zinc-500">
                          <Spinner color="primary" size="lg" />
                          <p>{t("common.loading")}</p>
                        </div>
                      ) : readmeContent ? (
                        <ReactMarkdown
                          remarkPlugins={[remarkGfm]}
                          components={markdownComponents}
                        >
                          {readmeContent}
                        </ReactMarkdown>
                      ) : (
                        <div className="flex flex-col items-center justify-center py-12 text-default-400 gap-3">
                          <p>{t("lip.no_description")}</p>
                        </div>
                      )}
                    </div>
                  </Tab>

                  <Tab key="files" title={t("lip.files.tab_label")}>
                    <div className="flex flex-col gap-4">
                      {visiblePackageVariants.length > 1 ? (
                        <div className="max-w-sm">
                          <Select
                            label={
                              <div className="flex items-center gap-2">
                                <span>{t("lip.files.variant_label")}</span>
                                <span className="text-tiny font-normal text-default-400 dark:text-zinc-500">
                                  {t("lip.files.variant_hint_keep_default")}
                                </span>
                              </div>
                            }
                            placeholder={t("lip.files.variant_placeholder")}
                            selectedKeys={
                              activePackageIdentifier ? [activePackageIdentifier] : []
                            }
                            onChange={(e) =>
                              setSelectedVariantIdentifier(String(e.target.value || "").trim())
                            }
                            size="sm"
                            classNames={COMPONENT_STYLES.select}
                            isDisabled={actionRunning}
                          >
                            {visiblePackageVariants.map((variant) => {
                              const label = getVariantDisplayLabel(
                                variant,
                                t("lip.files.variant_default"),
                              );
                              return (
                                <SelectItem
                                  key={variant.packageIdentifier}
                                  textValue={label}
                                >
                                  <div className="flex flex-col">
                                    <span className="text-small">{label}</span>
                                    <span className="text-tiny text-default-400 font-mono">
                                      {variant.key
                                        ? variant.packageIdentifier
                                        : String(pkg?.identifier || "").trim()}
                                    </span>
                                  </div>
                                </SelectItem>
                              );
                            })}
                          </Select>
                        </div>
                      ) : null}
                      {filesWithGameVersionState.length > 0 ? (
                        <Table
                          aria-label={t("lip.files.table_aria_label")}
                          removeWrapper
                          classNames={COMPONENT_STYLES.table}
                        >
                          <TableHeader>
                            <TableColumn>{t("common.version")}</TableColumn>
                            <TableColumn>
                              {t("lip.files.ll_requirement")}
                            </TableColumn>
                            <TableColumn>
                              {t("lip.files.dependencies")}
                            </TableColumn>
                            <TableColumn>
                              {t("lip.files.game_versions_label")}
                            </TableColumn>
                            <TableColumn>{t("lip.files.action")}</TableColumn>
                          </TableHeader>
                          <TableBody>
                            {filesWithGameVersionState.map((fileState) => {
                              const { file } = fileState;
                              const dependencyEntries = Object.entries(
                                file.otherDependencies,
                              );

                              return (
                                <TableRow key={file.version}>
                                  <TableCell>
                                    <Chip
                                      size="sm"
                                      variant="flat"
                                      className="font-mono"
                                    >
                                      {file.version}
                                    </Chip>
                                  </TableCell>
                                  <TableCell>
                                    {file.llDependencyRanges.length > 0 ? (
                                      <div className="text-xs text-default-700 dark:text-zinc-300 break-all">
                                        {file.llDependencyRanges.join(", ")}
                                      </div>
                                    ) : (
                                      <span className="text-default-400">
                                        -
                                      </span>
                                    )}
                                  </TableCell>
                                  <TableCell>
                                    {dependencyEntries.length > 0 ? (
                                      <Tooltip
                                        content={
                                          <div className="max-w-sm p-2 space-y-1">
                                            {dependencyEntries.map(
                                              ([key, value]) => (
                                                <div
                                                  key={key}
                                                  className="text-xs break-all"
                                                >
                                                  <span className="font-semibold">
                                                    {key}
                                                  </span>
                                                  : {value}
                                                </div>
                                              ),
                                            )}
                                          </div>
                                        }
                                      >
                                        <span className="cursor-help text-xs text-default-700 dark:text-zinc-300">
                                          {t("lip.files.dependencies_count", {
                                            count: dependencyEntries.length,
                                          })}
                                        </span>
                                      </Tooltip>
                                    ) : (
                                      <span className="text-default-400">
                                        -
                                      </span>
                                    )}
                                  </TableCell>
                                  <TableCell>
                                    {!fileState.hasLLRequirement ? (
                                      <Chip
                                        size="sm"
                                        variant="flat"
                                        color="default"
                                      >
                                        {t(
                                          "lip.files.game_versions_unrestricted",
                                        )}
                                      </Chip>
                                    ) : mappingUnavailable ? (
                                      <Chip
                                        size="sm"
                                        variant="flat"
                                        color="warning"
                                      >
                                        {t(
                                          "lip.files.game_versions_unavailable",
                                        )}
                                      </Chip>
                                    ) : fileState.supportedGameVersions.length >
                                      0 ? (
                                      <div className="flex flex-wrap gap-1">
                                        {fileState.supportedGameVersions.map(
                                          (gameVersion) => (
                                            <Chip
                                              key={`${file.version}-${gameVersion}`}
                                              size="sm"
                                              variant="flat"
                                              color="default"
                                            >
                                              {gameVersion}
                                            </Chip>
                                          ),
                                        )}
                                      </div>
                                    ) : (
                                      <Chip
                                        size="sm"
                                        variant="flat"
                                        color="danger"
                                      >
                                        {t("contentpage.none")}
                                      </Chip>
                                    )}
                                  </TableCell>
                                  <TableCell>
                                    <Button
                                      size="sm"
                                      color="primary"
                                      variant="flat"
                                      onPress={() =>
                                        openInstallDialog(file.version)
                                      }
                                      isDisabled={
                                        instanceOptions.length === 0 ||
                                        actionRunning
                                      }
                                    >
                                      {t("lip.files.install")}
                                    </Button>
                                  </TableCell>
                                </TableRow>
                              );
                            })}
                          </TableBody>
                        </Table>
                      ) : (
                        <div className="flex flex-col items-center justify-center py-12 text-default-400 border border-dashed border-default-200 rounded-xl">
                          <LuFileDigit size={48} className="mb-4 opacity-50" />
                          <p className="text-lg font-medium">
                            {t("common.no_results")}
                          </p>
                        </div>
                      )}
                    </div>
                  </Tab>
                </Tabs>
              </div>
            </CardBody>
          </Card>
        </motion.div>
      </div>

      <UnifiedModal
        size="md"
        isOpen={installDialogOpen}
        onOpenChange={(open) => {
          if (open) {
            setInstallDialogOpen(true);
            return;
          }
          closeInstallDialog();
        }}
        type="primary"
        icon={<LuGamepad2 size={24} />}
        title={t("lip.files.select_instance_title")}
        isDismissable={!actionRunning}
        footer={
          <>
            <Button
              variant="light"
              onPress={() => closeInstallDialog()}
              isDisabled={actionRunning}
            >
              {t("common.cancel")}
            </Button>
            <Button
              color="primary"
              onPress={() => void handleConfirmInstall()}
              className="bg-primary-500 hover:bg-primary-500 text-white font-bold shadow-lg shadow-primary-900/20"
              isLoading={actionRunning}
              isDisabled={
                !installDialogSelectedInstance ||
                !installDialogTriggerVersion ||
                dialogPackageStateLoading ||
                !installDialogVersionInstallable ||
                actionRunning
              }
            >
              {t(installDialogPrimaryActionLabelKey)}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <p className="text-small text-default-500 dark:text-zinc-400">
            {t("curseforge.install.select_version_body")}
          </p>
          <Select
            label={t("curseforge.install.local_installation")}
            placeholder={t("curseforge.install.select_version_placeholder")}
            selectedKeys={
              installDialogSelectedInstance
                ? [installDialogSelectedInstance]
                : []
            }
            onChange={(e) => setInstallDialogSelectedInstance(e.target.value)}
            size="sm"
            classNames={COMPONENT_STYLES.select}
            isDisabled={actionRunning || instanceOptions.length === 0}
          >
            {instanceOptions.map((instanceName) => (
              <SelectItem key={instanceName} textValue={instanceName}>
                <div className="flex gap-2 items-center">
                  <div className="w-8 h-8 rounded bg-default-200 flex items-center justify-center overflow-hidden">
                    <img
                      src={
                        instanceLogos[instanceName] ||
                        "https://raw.githubusercontent.com/LiteLDev/LeviLauncher/main/build/appicon.png"
                      }
                      alt="icon"
                      className="w-full h-full object-cover"
                      onError={(e) => (e.currentTarget.style.display = "none")}
                    />
                  </div>
                  <div className="flex flex-col">
                    <span className="text-small">{instanceName}</span>
                    <span className="text-tiny text-default-400">
                      {normalizeGameVersion(
                        instanceGameVersions[instanceName],
                      ) || t("contentpage.none")}
                    </span>
                  </div>
                </div>
              </SelectItem>
            ))}
          </Select>
          <div className="grid grid-cols-2 gap-x-8 gap-y-3">
            <div className="min-w-0 text-small leading-7 text-default-600 dark:text-zinc-300">
              <span>{t("lip.files.variant_label")}:</span>{" "}
              <span>{activeVariantDisplayLabel || t("contentpage.none")}</span>
            </div>
            <div className="min-w-0 text-small leading-7 text-default-600 dark:text-zinc-300">
              <span>{t("lip.files.installing_version_label")}:</span>{" "}
              <span>{installDialogTriggerVersion || t("contentpage.none")}</span>
            </div>
            <div className="min-w-0 text-small leading-7 text-default-600 dark:text-zinc-300">
              {dialogInstancePackageState?.loading ? (
                t("lip.files.checking_current_installed_version")
              ) : (
                <>
                  <span>{t("lip.files.current_installed_version_label")}:</span>{" "}
                  <span>{dialogInstalledPackageVersion || t("contentpage.none")}</span>
                </>
              )}
            </div>
            <div className="min-w-0 text-small leading-7 text-default-600 dark:text-zinc-300">
              <span>{t("lip.files.instance_game_version_label")}:</span>{" "}
              <span>{installDialogInstanceGameVersion || t("contentpage.none")}</span>
            </div>
          </div>
          {dialogRequiresLL && dialogLLStateLoading ? (
            <div className="text-xs text-default-500 dark:text-zinc-400">
              {t("lip.files.checking_ll_state")}
            </div>
          ) : null}
          {dialogRequiresLL &&
          !dialogLLStateLoading &&
          !dialogLLInstalled ? (
            <div className="text-xs text-warning-600 dark:text-warning-400">
              {t("lip.files.ll_missing_redirect_hint")}
            </div>
          ) : null}
          {dialogRequiresLL &&
          !dialogLLStateLoading &&
          dialogLLInstalled &&
          !dialogInstalledLLCompatible ? (
            <div className="text-xs text-danger-500">
              {t("lip.files.ll_installed_incompatible_hint", {
                installedVersion: dialogLLVersion || t("contentpage.none"),
              })}
            </div>
          ) : null}
          {installDialogFileState?.hasLLRequirement &&
          !mappingUnavailable &&
          !installDialogGameVersionCompatible ? (
            <div className="text-xs text-danger-500">
              {t("lip.files.select_version_incompatible_hint", {
                version: installDialogTriggerVersion || t("contentpage.none"),
                gameVersion:
                  installDialogInstanceGameVersion || t("contentpage.none"),
              })}
            </div>
          ) : null}
        </div>
      </UnifiedModal>
    </PageContainer>
  );
};

export default LIPPackagePage;
