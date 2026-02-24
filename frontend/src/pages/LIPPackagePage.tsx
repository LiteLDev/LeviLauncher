import React, { useEffect, useMemo, useState, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  Button,
  Card,
  CardBody,
  Chip,
  Skeleton,
  Image,
  Tabs,
  Tab,
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
} from "@heroui/react";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { cn } from "@/utils/cn";
import {
  LuDownload,
  LuGlobe,
  LuUser,
  LuClock,
  LuFlame,
  LuTag,
  LuGamepad2,
  LuShare2,
  LuFileDigit,
} from "react-icons/lu";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { motion } from "framer-motion";
import {
  fetchLIPPackagesIndex,
  type LIPPackageBasicInfo,
} from "@/utils/content";
import { formatDateStr } from "@/utils/formatting";
import { Browser } from "@wailsio/runtime";

type GithubRepoRef = {
  owner: string;
  repo: string;
};
type HeadingTag = "h1" | "h2" | "h3" | "h4" | "h5" | "h6";

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
  if (node === null || node === undefined || typeof node === "boolean")
    return "";
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node))
    return node.map((item) => collectNodeText(item)).join("");
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

const LIPPackagePage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [pkg, setPkg] = useState<LIPPackageBasicInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selectedTab, setSelectedTab] = useState<string>("description");
  const [readmeContent, setReadmeContent] = useState<string>("");
  const tabsRef = useRef<HTMLDivElement>(null);
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
    const nextHeadingId = createHeadingSlugger();
    const createHeading =
      (tag: HeadingTag) =>
      ({
        children,
        className,
        id,
        ...props
      }: React.HTMLAttributes<HTMLHeadingElement> & {
        children?: React.ReactNode;
      }) =>
        React.createElement(
          tag,
          {
            ...props,
            id:
              typeof id === "string" && id.trim()
                ? id
                : nextHeadingId(collectNodeText(children)),
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

  useEffect(() => {
    if (!pkg?.projectUrl) {
      setReadmeContent("");
      return;
    }

    const fetchReadme = async () => {
      try {
        const url = new URL(pkg.projectUrl);
        if (url.hostname === "github.com") {
          const parts = url.pathname.split("/").filter(Boolean);
          if (parts.length >= 2) {
            const owner = parts[0];
            const repo = parts[1];
            const apiUrl = `https://api.github.com/repos/${owner}/${repo}/readme`;
            const response = await fetch(apiUrl, {
              headers: { Accept: "application/vnd.github.raw+json" },
            });
            if (response.ok) {
              const text = await response.text();
              setReadmeContent(text);
              return;
            }
          }
        }
      } catch (e) {
        console.warn("Failed to fetch README", e);
      }
      setReadmeContent("");
    };

    fetchReadme();
  }, [pkg]);

  const identifier = useMemo(() => {
    const raw = String(id || "");
    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }, [id]);

  const loadPackage = async (
    targetIdentifier: string,
    forceRefresh = false,
  ) => {
    setLoading(true);
    setError("");
    try {
      const list = await fetchLIPPackagesIndex({ forceRefresh });
      const found =
        list.find((item) => item.identifier === targetIdentifier) || null;
      setPkg(found);
    } catch (err) {
      console.error(err);
      setError(t("common.load_failed"));
      setPkg(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!identifier) {
      setPkg(null);
      setLoading(false);
      return;
    }
    void loadPackage(identifier);
  }, [identifier]);

  const handleInstall = (version: string) => {
    // TODO: Implement LIP package installation
    console.log("Install version:", version);
  };

  if (loading) {
    return (
      <PageContainer animate={false} className="min-h-0">
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
            <p className="text-xl font-bold">Package not found</p>
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
    <PageContainer animate={false} className="min-h-0">
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

                {/* Actions */}
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
                    Install
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
                    {/* Placeholder buttons for consistency if needed, but only show if applicable */}
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
            <CardBody className="p-6">
              <div ref={tabsRef} className="flex w-full flex-col scroll-mt-24">
                <Tabs
                  aria-label="Package Details"
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
                  <Tab key="description" title="Description">
                    <div className="prose dark:prose-invert max-w-none">
                      {readmeContent ? (
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
                  <Tab key="files" title="Files">
                    <div className="flex flex-col gap-4">
                      <div className="flex justify-between items-center mb-2">
                        <h3 className="text-lg font-semibold">All Files</h3>
                      </div>

                      {pkg.versions.length > 0 ? (
                        <Table aria-label="Package files table" removeWrapper>
                          <TableHeader>
                            <TableColumn>Version</TableColumn>
                            <TableColumn>Actions</TableColumn>
                          </TableHeader>
                          <TableBody>
                            {pkg.versions.map((version) => (
                              <TableRow key={version}>
                                <TableCell>
                                  <Chip
                                    size="sm"
                                    variant="flat"
                                    className="font-mono"
                                  >
                                    {version}
                                  </Chip>
                                </TableCell>
                                <TableCell>
                                  <Button
                                    isIconOnly
                                    variant="light"
                                    size="sm"
                                    className="text-default-500 dark:text-zinc-400 hover:text-primary"
                                    onPress={() => handleInstall(version)}
                                  >
                                    <LuDownload size={20} />
                                  </Button>
                                </TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      ) : (
                        <div className="flex flex-col items-center justify-center py-12 text-default-400 border border-dashed border-default-200 rounded-xl">
                          <LuFileDigit size={48} className="mb-4 opacity-50" />
                          <p className="text-lg font-medium">
                            {t("common.empty")}
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
    </PageContainer>
  );
};

export default LIPPackagePage;
