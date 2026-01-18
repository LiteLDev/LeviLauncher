import React, { useEffect, useState, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  GetCurseForgeModsByIDs,
  GetCurseForgeModDescription,
  GetCurseForgeModFiles,
  ListVersionMetasWithRegistered,
  GetContentRoots,
  ImportMcpackPath,
  ImportMcpackPathWithPlayer,
  ImportMcaddonPath,
  ImportMcaddonPathWithPlayer,
  ImportMcworldPath,
  IsMcpackSkinPackPath,
  StartFileDownload,
  CancelFileDownload,
  GetVersionLogoDataUrl,
} from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { Events } from "@wailsio/runtime";
import * as types from "bindings/github.com/liteldev/LeviLauncher/internal/types/models";
import { VersionMeta } from "bindings/github.com/liteldev/LeviLauncher/internal/versions/models";
import {
  ModData,
  File as ModFile,
} from "bindings/github.com/liteldev/LeviLauncher/internal/curseforge/client/types";
import { listPlayers } from "@/utils/content";
import { readCurrentVersionName } from "@/utils/currentVersion";
import { compareVersions } from "@/utils/version";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";
import {
  Button,
  Spinner,
  Chip,
  Image,
  ScrollShadow,
  Link,
  Divider,
  Card,
  CardBody,
  Tabs,
  Tab,
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
  Tooltip,
  Select,
  SelectItem,
  ModalContent,
  Progress,
  Skeleton,
} from "@heroui/react";
import {
  LuArrowLeft,
  LuDownload,
  LuClock,
  LuCalendar,
  LuFileDigit,
  LuExternalLink,
  LuGlobe,
  LuGithub,
  LuBug,
  LuShare2,
  LuGamepad2,
} from "react-icons/lu";
import { motion } from "framer-motion";

const formatNumber = (num: number | undefined) => {
  if (num === undefined) return "0";
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + "M";
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + "K";
  }
  return num.toString();
};

const formatDate = (dateStr: string | undefined) => {
  if (!dateStr) return "-";
  return new Date(dateStr).toLocaleDateString();
};

const formatFileSize = (bytes: number | undefined) => {
  if (bytes === undefined) return "-";
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};

const sortGameVersions = (versions: string[] | undefined) => {
  if (!versions) return [];
  const sorted = [...versions].sort((a, b) => {
    const aIsVer = /^\d/.test(a);
    const bIsVer = /^\d/.test(b);

    if (aIsVer && !bIsVer) return -1;
    if (!aIsVer && bIsVer) return 1;
    if (!aIsVer && !bIsVer) return a.localeCompare(b);

    const partsA = a.split(".").map((p) => parseInt(p) || 0);
    const partsB = b.split(".").map((p) => parseInt(p) || 0);

    for (let i = 0; i < Math.max(partsA.length, partsB.length); i++) {
      const valA = partsA[i] || 0;
      const valB = partsB[i] || 0;
      if (valA !== valB) return valB - valA;
    }
    return 0;
  });
  return sorted;
};

const CurseForgeModPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();

  const [mod, setMod] = useState<any | null>(null);
  const [description, setDescription] = useState<string>("");
  const [files, setFiles] = useState<ModFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedGameVersion, setSelectedGameVersion] = useState<string>("all");
  const [selectedTab, setSelectedTab] = useState<string>("description");
  const tabsRef = useRef<HTMLDivElement>(null);

  const [installModalOpen, setInstallModalOpen] = useState(false);
  const [installStep, setInstallStep] = useState<
    | "downloading"
    | "version_select"
    | "player_select"
    | "importing"
    | "success"
    | "error"
  >("downloading");
  const [installFile, setInstallFile] = useState<{
    name: string;
    path: string;
    type: string;
  } | null>(null);
  const [downloadProgress, setDownloadProgress] = useState<{
    downloaded: number;
    total: number;
  } | null>(null);
  const [availableVersions, setAvailableVersions] = useState<VersionMeta[]>([]);
  const [versionLogos, setVersionLogos] = useState<Record<string, string>>({});
  const [availablePlayers, setAvailablePlayers] = useState<string[]>([]);
  const [selectedVersion, setSelectedVersion] = useState<string>("");
  const [selectedPlayer, setSelectedPlayer] = useState<string>("");
  const [installError, setInstallError] = useState<string>("");
  const [dupOpen, setDupOpen] = useState(false);
  const [dupName, setDupName] = useState<string>("");
  const dupResolveRef = useRef<((overwrite: boolean) => void) | null>(null);
  const isCancelling = useRef(false);
  const cleanupRef = useRef<() => void>(() => {});

  React.useLayoutEffect(() => {
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
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(t0);
      clearTimeout(t1);
    };
  }, [id]);

  const handleCancelDownload = async () => {
    isCancelling.current = true;
    cleanupRef.current();
    try {
      await CancelFileDownload();
    } catch (e) {
      console.warn("Cancel failed", e);
    }
    setInstallModalOpen(false);
  };

  const handleInstall = async (file: ModFile) => {
    if (!file.downloadUrl) {
      alert(
        t("curseforge.no_download_url", {
          defaultValue: "No download URL available",
        }),
      );
      return;
    }

    setInstallModalOpen(true);
    setInstallStep("downloading");
    setInstallError("");
    setInstallFile(null);
    setDownloadProgress(null);
    isCancelling.current = false;

    try {
      const dest = await StartFileDownload(file.downloadUrl, file.fileName);

      const cleanup = () => {
        Events.Off("file.download.progress");
        Events.Off("file.download.done");
        Events.Off("file.download.error");
      };
      cleanupRef.current = cleanup;

      Events.On("file.download.progress", (event: any) => {
        const data = event.data || {};
        setDownloadProgress({
          downloaded: Number(data.Downloaded || 0),
          total: Number(data.Total || 0),
        });
      });

      Events.On("file.download.done", async () => {
        cleanup();
        try {
          let type = "unknown";
          const lowerName = file.fileName.toLowerCase();
          if (lowerName.endsWith(".mcworld")) type = "mcworld";
          else if (lowerName.endsWith(".mcaddon")) type = "mcaddon";
          else if (lowerName.endsWith(".mcpack")) {
            type = (await IsMcpackSkinPackPath(dest)) ? "skin_pack" : "mcpack";
          }

          setInstallFile({ name: file.fileName, path: dest, type });

          const metas = await ListVersionMetasWithRegistered();
          if (metas) {
            metas.sort((a, b) => {
              const cmp = compareVersions(
                a.gameVersion || "0",
                b.gameVersion || "0",
              );
              return -cmp;
            });
            setAvailableVersions(metas);

            const currentName = readCurrentVersionName();
            let defaultSelect = "";
            if (currentName && metas.some((m) => m.name === currentName)) {
              defaultSelect = currentName;
            } else if (metas.length > 0) {
              defaultSelect = metas[0].name;
            }
            setSelectedVersion(defaultSelect);

            const logoMap: Record<string, string> = {};
            await Promise.all(
              metas.map(async (m) => {
                try {
                  const url = await GetVersionLogoDataUrl(m.name);
                  if (url) logoMap[m.name] = url;
                } catch (e) {
                  console.warn("Failed to fetch logo for", m.name, e);
                }
              }),
            );
            setVersionLogos(logoMap);
          }

          setInstallStep("version_select");
        } catch (e: any) {
          setInstallError(e.message || "Detection failed");
          setInstallStep("error");
        }
      });

      Events.On("file.download.error", (event: any) => {
        cleanup();
        if (isCancelling.current) return;
        const err = event.data;
        setInstallError(err || "Download failed");
        setInstallStep("error");
      });
    } catch (e: any) {
      if (isCancelling.current) return;
      setInstallError(e.message || "Download start failed");
      setInstallStep("error");
    }
  };

  const handleVersionSelectNext = async () => {
    if (!installFile) return;

    if (installStep === "player_select") {
      await executeImport();
      return;
    }

    if (installFile.type === "mcworld" || installFile.type === "skin_pack") {
      setInstallStep("player_select");
      try {
        const roots = await GetContentRoots(selectedVersion);
        if (roots && roots.usersRoot) {
          const players = await listPlayers(roots.usersRoot);
          setAvailablePlayers(players);
          if (players.length > 0) setSelectedPlayer(players[0]);
        }
      } catch (e) {
        console.error(e);
        setAvailablePlayers([]);
      }
    } else {
      await executeImport();
    }
  };

  const executeImport = async () => {
    if (!installFile || !selectedVersion) return;

    setInstallStep("importing");
    setInstallError("");

    try {
      const { name, path, type } = installFile;
      const runImport = async (overwrite: boolean): Promise<string> => {
        if (type === "mcworld") {
          if (!selectedPlayer) throw new Error("No player selected");
          return String(
            await ImportMcworldPath(
              selectedVersion,
              selectedPlayer,
              path,
              overwrite,
            ),
          );
        }
        if (type === "mcaddon") {
          if (selectedPlayer) {
            return String(
              await ImportMcaddonPathWithPlayer(
                selectedVersion,
                selectedPlayer,
                path,
                overwrite,
              ),
            );
          }
          return String(
            await ImportMcaddonPath(selectedVersion, path, overwrite),
          );
        }
        if (selectedPlayer) {
          if (type === "skin_pack" && !selectedPlayer) {
            throw new Error("No player selected for skin pack");
          }
          if (type === "skin_pack") {
            return String(
              await ImportMcpackPathWithPlayer(
                selectedVersion,
                selectedPlayer,
                path,
                overwrite,
              ),
            );
          }
          return String(
            await ImportMcpackPath(selectedVersion, path, overwrite),
          );
        }
        return String(await ImportMcpackPath(selectedVersion, path, overwrite));
      };

      let err = await runImport(false);
      if (err) {
        if (
          String(err) === "ERR_DUPLICATE_FOLDER" ||
          String(err) === "ERR_DUPLICATE_UUID"
        ) {
          setDupName(name);
          await new Promise<void>((resolve) => {
            dupResolveRef.current = (overwrite) => {
              resolve();
              if (!overwrite) {
                err = "";
              }
            };
            setDupOpen(true);
          });
          if (!err) {
            setInstallModalOpen(false);
            return;
          }
          err = await runImport(true);
        }
      }

      if (err) {
        throw new Error(err);
      }

      setInstallStep("success");
    } catch (e: any) {
      setInstallError(e.message || "Import failed");
      setInstallStep("error");
    }
  };

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
    if (!id) return;
    const modId = parseInt(id);
    if (isNaN(modId)) return;

    setLoading(true);
    Promise.all([
      GetCurseForgeModsByIDs([modId]),
      GetCurseForgeModDescription(modId),
      GetCurseForgeModFiles(modId),
    ])
      .then(([modRes, descRes, filesRes]) => {
        if (modRes?.data && modRes.data.length > 0) {
          setMod(modRes.data[0]);
        }
        if (descRes?.data) {
          setDescription(descRes.data);
        }
        if (filesRes?.data) {
          setFiles(filesRes.data);
        }
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <div className="w-full h-full flex flex-col overflow-hidden relative bg-background">
        <ScrollShadow className="w-full h-full">
          <div className="w-full px-4 sm:px-6 py-8 flex flex-col gap-6">
            <div className="flex items-center">
              <Button
                variant="light"
                isDisabled
                className="gap-2 px-3 text-default-500"
                startContent={<LuArrowLeft size={20} />}
              >
                {t("curseforge.go_back", { defaultValue: "Go Back" })}
              </Button>
            </div>
            <Card className="shrink-0 bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl shadow-md border-none">
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

            <Card className="min-h-[300px] bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl shadow-md border-none">
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
          </div>
        </ScrollShadow>
      </div>
    );
  }

  if (!mod) {
    return (
      <div className="w-full h-full min-h-0 flex flex-col p-4 sm:p-6 gap-4 items-center justify-center">
        <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl p-8">
          <CardBody className="flex flex-col items-center gap-4">
            <p className="text-xl font-bold">
              {t("curseforge.mod_not_found", { defaultValue: "Mod not found" })}
            </p>
            <Button
              onPress={() => navigate(-1)}
              color="primary"
              className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
            >
              {t("curseforge.go_back", { defaultValue: "Go Back" })}
            </Button>
          </CardBody>
        </Card>
      </div>
    );
  }

  return (
    <div className="w-full h-full flex flex-col overflow-hidden relative bg-background">
      <ScrollShadow className="w-full h-full">
        <div className="w-full max-w-full mx-auto p-4 flex flex-col gap-6">
          <div className="flex items-center">
            <Button
              variant="light"
              onPress={() => navigate(-1)}
              className="gap-2 px-3 text-default-500 hover:text-default-900"
              startContent={<LuArrowLeft size={20} />}
            >
              {t("curseforge.go_back", { defaultValue: "Go Back" })}
            </Button>
          </div>
          {/* Header Card */}
          <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl shadow-md border-none">
            <CardBody className="p-6 sm:p-8">
              <div className="flex flex-col md:flex-row gap-8">
                <div className="shrink-0">
                  <Image
                    src={mod.logo?.url}
                    alt={mod.name}
                    className="w-32 h-32 object-cover rounded-2xl shadow-lg bg-content2"
                  />
                </div>

                <div className="flex flex-col grow gap-3">
                  <h1 className="text-3xl sm:text-4xl font-black tracking-tight text-default-900 pb-1">
                    {mod.name}
                  </h1>

                  <div className="flex items-center gap-3 text-default-500 text-sm flex-wrap">
                    <span className="flex items-center gap-1">
                      {t("curseforge.by", { defaultValue: "By" })}
                      {mod.authors?.map((author: any, idx: number) => (
                        <React.Fragment key={author.id}>
                          <Link
                            href={author.url}
                            isExternal
                            size="sm"
                            className="text-primary hover:underline"
                          >
                            {author.name}
                          </Link>
                          {idx < (mod.authors?.length || 0) - 1 && ", "}
                        </React.Fragment>
                      ))}
                    </span>
                    <span className="w-1 h-1 rounded-full bg-default-300"></span>
                    <span className="flex items-center gap-1">
                      <LuCalendar size={14} />
                      {t("curseforge.updated_date", {
                        date: formatDate(mod.dateModified),
                        defaultValue: `Updated ${formatDate(mod.dateModified)}`,
                      })}
                    </span>
                    <span className="w-1 h-1 rounded-full bg-default-300"></span>
                    <span className="flex items-center gap-1">
                      <LuDownload size={14} />
                      {t("curseforge.download_count", {
                        count: formatNumber(mod.downloadCount),
                        defaultValue: `${formatNumber(mod.downloadCount)} Downloads`,
                      })}
                    </span>
                  </div>

                  <div className="flex flex-wrap gap-2 mt-1">
                    {mod.categories?.map((cat: any) => (
                      <Chip
                        key={cat.id}
                        size="sm"
                        variant="flat"
                        className="pl-1"
                        avatar={
                          cat.iconUrl ? (
                            <Image src={cat.iconUrl} className="w-4 h-4" />
                          ) : undefined
                        }
                      >
                        {cat.name}
                      </Chip>
                    ))}
                    <Chip
                      size="sm"
                      variant="bordered"
                      startContent={<LuGamepad2 size={12} />}
                    >
                      ID: {mod.id}
                    </Chip>
                  </div>

                  <p className="text-default-600 mt-2 text-sm leading-relaxed max-w-4xl">
                    {mod.summary}
                  </p>
                </div>

                {/* Actions */}
                <div className="flex flex-col gap-3 min-w-[240px] md:border-l md:border-default-100 md:pl-8 justify-center">
                  <Button
                    className="w-full font-semibold shadow-md shadow-emerald-900/20 text-white bg-linear-to-r from-emerald-500 to-teal-500"
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
                    {t("curseforge.install_action", {
                      defaultValue: "Install",
                    })}
                  </Button>
                  <div className="flex gap-2 justify-center">
                    {mod.links?.websiteUrl && (
                      <Button
                        as={Link}
                        href={mod.links.websiteUrl}
                        isExternal
                        isIconOnly
                        variant="flat"
                        aria-label={t("curseforge.website", {
                          defaultValue: "Website",
                        })}
                      >
                        <LuGlobe size={20} />
                      </Button>
                    )}
                    {mod.links?.sourceUrl && (
                      <Button
                        as={Link}
                        href={mod.links.sourceUrl}
                        isExternal
                        isIconOnly
                        variant="flat"
                        aria-label={t("curseforge.source", {
                          defaultValue: "Source",
                        })}
                      >
                        <LuGithub size={20} />
                      </Button>
                    )}
                    {mod.links?.issuesUrl && (
                      <Button
                        as={Link}
                        href={mod.links.issuesUrl}
                        isExternal
                        isIconOnly
                        variant="flat"
                        aria-label={t("curseforge.issues", {
                          defaultValue: "Issues",
                        })}
                      >
                        <LuBug size={20} />
                      </Button>
                    )}
                    <Button
                      isIconOnly
                      variant="flat"
                      aria-label={t("curseforge.share", {
                        defaultValue: "Share",
                      })}
                    >
                      <LuShare2 size={20} />
                    </Button>
                  </div>
                </div>
              </div>
            </CardBody>
          </Card>

          {/* Content Card */}
          <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl shadow-md border-none min-h-[500px]">
            <CardBody className="p-6 sm:p-8">
              <div ref={tabsRef} className="flex w-full flex-col scroll-mt-24">
                <Tabs
                  aria-label="Mod Details"
                  variant="underlined"
                  color="primary"
                  selectedKey={selectedTab}
                  onSelectionChange={(key) => setSelectedTab(key as string)}
                  classNames={{
                    tabList:
                      "gap-8 w-full relative rounded-none p-0 border-b border-default-200 mb-6",
                    cursor:
                      "w-full bg-linear-to-r from-emerald-500 to-teal-500 h-[3px]",
                    tab: "max-w-fit px-0 h-12 text-base font-medium text-default-500",
                    tabContent:
                      "group-data-[selected=true]:text-emerald-600 dark:group-data-[selected=true]:text-emerald-500 font-bold",
                  }}
                >
                  <Tab key="description" title="Description">
                    <ScrollShadow className="max-h-[800px] w-full pr-4 pretty-scrollbar">
                      <div className="prose dark:prose-invert max-w-none prose-img:rounded-xl prose-img:mx-auto prose-a:text-emerald-600 dark:prose-a:text-emerald-500">
                        {description ? (
                          <div
                            dangerouslySetInnerHTML={{ __html: description }}
                          />
                        ) : (
                          <div className="flex flex-col items-center justify-center py-12 text-default-400 gap-3">
                            <Spinner color="success" />
                            <p>Loading description...</p>
                          </div>
                        )}
                      </div>
                    </ScrollShadow>
                  </Tab>
                  <Tab key="files" title="Files">
                    <div className="flex flex-col gap-4">
                      <div className="flex justify-between items-center mb-2">
                        <h3 className="text-lg font-semibold">All Files</h3>
                        <div className="w-48">
                          <Select
                            label="Game Version"
                            size="sm"
                            selectedKeys={[selectedGameVersion]}
                            onChange={(e) =>
                              setSelectedGameVersion(e.target.value || "all")
                            }
                          >
                            <SelectItem key="all" value="all">
                              All Versions
                            </SelectItem>
                            {gameVersions.map((version) => (
                              <SelectItem key={version} value={version}>
                                {version}
                              </SelectItem>
                            ))}
                          </Select>
                        </div>
                      </div>

                      {filteredFiles.length > 0 ? (
                        <Table aria-label="Mod files table" removeWrapper>
                          <TableHeader>
                            <TableColumn>Type</TableColumn>
                            <TableColumn>Name</TableColumn>
                            <TableColumn>Uploaded</TableColumn>
                            <TableColumn>Size</TableColumn>
                            <TableColumn>Game Version</TableColumn>
                            <TableColumn>Downloads</TableColumn>
                            <TableColumn>Actions</TableColumn>
                          </TableHeader>
                          <TableBody>
                            {filteredFiles.map((file) => {
                              const sortedVersions = sortGameVersions(
                                file.gameVersions,
                              );
                              return (
                                <TableRow key={file.id}>
                                  <TableCell>
                                    <Chip
                                      size="sm"
                                      color={
                                        file.releaseType === 1
                                          ? "success"
                                          : file.releaseType === 2
                                            ? "primary"
                                            : "warning"
                                      }
                                      variant="flat"
                                      className="capitalize"
                                    >
                                      {file.releaseType === 1
                                        ? "R"
                                        : file.releaseType === 2
                                          ? "B"
                                          : "A"}
                                    </Chip>
                                  </TableCell>
                                  <TableCell>
                                    <span className="font-medium">
                                      {file.displayName}
                                    </span>
                                  </TableCell>
                                  <TableCell>
                                    <span className="text-default-500">
                                      {formatDate(file.fileDate)}
                                    </span>
                                  </TableCell>
                                  <TableCell>
                                    <span className="text-default-500">
                                      {formatFileSize(file.fileLength)}
                                    </span>
                                  </TableCell>
                                  <TableCell>
                                    <div className="flex items-center gap-1">
                                      {sortedVersions.length > 0 ? (
                                        <>
                                          <span className="text-default-600 bg-default-100 px-2 py-1 rounded text-xs">
                                            {sortedVersions[0]}
                                          </span>
                                          {sortedVersions.length > 1 && (
                                            <Tooltip
                                              content={
                                                <div className="flex flex-wrap gap-1 max-w-xs p-2">
                                                  {sortedVersions
                                                    .slice(1)
                                                    .map((v) => (
                                                      <span
                                                        key={v}
                                                        className="text-xs bg-default-50 text-default-500 px-1.5 py-0.5 rounded border border-default-100"
                                                      >
                                                        {v}
                                                      </span>
                                                    ))}
                                                </div>
                                              }
                                            >
                                              <span className="text-xs text-primary cursor-pointer">
                                                +{sortedVersions.length - 1}
                                              </span>
                                            </Tooltip>
                                          )}
                                        </>
                                      ) : (
                                        <span className="text-default-400">
                                          -
                                        </span>
                                      )}
                                    </div>
                                  </TableCell>
                                  <TableCell>
                                    <span className="text-default-500">
                                      {formatNumber(file.downloadCount)}
                                    </span>
                                  </TableCell>
                                  <TableCell>
                                    <Button
                                      isIconOnly
                                      variant="light"
                                      size="sm"
                                      className="text-default-500 hover:text-primary"
                                      onPress={() => handleInstall(file)}
                                    >
                                      <LuDownload size={20} />
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
                            {t("curseforge.no_files_found", {
                              defaultValue: "No files found",
                            })}
                          </p>
                        </div>
                      )}
                    </div>
                  </Tab>
                </Tabs>
              </div>
            </CardBody>
          </Card>
        </div>
      </ScrollShadow>

      <BaseModal
        isOpen={installModalOpen}
        onOpenChange={(open) => {
          if (!open) {
            if (installStep === "downloading") {
              handleCancelDownload();
            } else {
              setInstallModalOpen(false);
            }
          }
        }}
        isDismissable={false}
        hideCloseButton={
          installStep === "downloading" || installStep === "importing"
        }
        backdrop="blur"
        classNames={{
          base: "bg-white/80! dark:bg-zinc-900/80! backdrop-blur-2xl border-white/40! dark:border-zinc-700/50! shadow-2xl rounded-4xl",
        }}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="flex flex-col gap-1">
                <span className="text-xl font-bold bg-linear-to-r from-emerald-600 to-teal-600 dark:from-emerald-500 dark:to-teal-500 bg-clip-text text-transparent">
                  {installStep === "downloading" &&
                    t("curseforge.install.downloading_title", {
                      defaultValue: "Downloading Content",
                    })}
                  {installStep === "version_select" &&
                    t("curseforge.install.select_version_title", {
                      defaultValue: "Select Game Version",
                    })}
                  {installStep === "player_select" &&
                    t("curseforge.install.select_player_title", {
                      defaultValue: "Select Player",
                    })}
                  {installStep === "importing" &&
                    t("curseforge.install.importing_title", {
                      defaultValue: "Importing Content",
                    })}
                  {installStep === "success" &&
                    t("curseforge.install.success_title", {
                      defaultValue: "Success",
                    })}
                  {installStep === "error" &&
                    t("curseforge.install.error_title", {
                      defaultValue: "Error",
                    })}
                </span>
              </BaseModalHeader>
              <BaseModalBody>
                {installStep === "downloading" && (
                  <div className="flex flex-col items-center gap-4 py-4 w-full">
                    <p className="text-default-500">
                      {t("curseforge.install.downloading_body", {
                        defaultValue: "Downloading file...",
                      })}
                    </p>
                    {downloadProgress ? (
                      <Progress
                        aria-label="Downloading..."
                        value={
                          (downloadProgress.downloaded /
                            downloadProgress.total) *
                          100
                        }
                        className="max-w-md w-full"
                        showValueLabel={true}
                      />
                    ) : (
                      <Spinner size="lg" />
                    )}
                    {downloadProgress && (
                      <p className="text-tiny text-default-400">
                        {formatFileSize(downloadProgress.downloaded)} /{" "}
                        {formatFileSize(downloadProgress.total)}
                      </p>
                    )}
                  </div>
                )}

                {installStep === "version_select" && (
                  <div className="flex flex-col gap-4">
                    <p className="text-small text-default-500">
                      {t("curseforge.install.select_version_body", {
                        defaultValue:
                          "Please select the game version to import to:",
                      })}
                    </p>
                    <Select
                      label={t("curseforge.install.local_installation", {
                        defaultValue: "Local Installation",
                      })}
                      placeholder={t(
                        "curseforge.install.select_version_placeholder",
                        { defaultValue: "Select a version" },
                      )}
                      selectedKeys={selectedVersion ? [selectedVersion] : []}
                      onChange={(e) => setSelectedVersion(e.target.value)}
                    >
                      {availableVersions.map((ver) => (
                        <SelectItem
                          key={ver.name}
                          value={ver.name}
                          textValue={ver.name}
                        >
                          <div className="flex gap-2 items-center">
                            <div className="w-8 h-8 rounded bg-default-200 flex items-center justify-center overflow-hidden">
                              <img
                                src={
                                  versionLogos[ver.name] ||
                                  "https://raw.githubusercontent.com/LiteLDev/LeviLauncher/main/build/appicon.png"
                                }
                                alt="icon"
                                className="w-full h-full object-cover"
                                onError={(e) =>
                                  (e.currentTarget.style.display = "none")
                                }
                              />
                            </div>
                            <div className="flex flex-col">
                              <span className="text-small">{ver.name}</span>
                              <span className="text-tiny text-default-400">
                                {ver.gameVersion}
                              </span>
                            </div>
                            {ver.registered && (
                              <Chip
                                size="sm"
                                color="success"
                                variant="flat"
                                className="ml-auto"
                              >
                                {t("curseforge.install.registered", {
                                  defaultValue: "Registered",
                                })}
                              </Chip>
                            )}
                          </div>
                        </SelectItem>
                      ))}
                    </Select>
                  </div>
                )}

                {installStep === "player_select" && (
                  <div className="flex flex-col gap-4">
                    <p className="text-small text-default-500">
                      {t("curseforge.install.select_player_body", {
                        defaultValue:
                          "This content requires selecting a player:",
                      })}
                    </p>
                    <Select
                      label={t("curseforge.install.player_label", {
                        defaultValue: "Player",
                      })}
                      placeholder={t(
                        "curseforge.install.select_player_placeholder",
                        { defaultValue: "Select a player" },
                      )}
                      selectedKeys={selectedPlayer ? [selectedPlayer] : []}
                      onChange={(e) => setSelectedPlayer(e.target.value)}
                    >
                      {availablePlayers.map((player) => (
                        <SelectItem key={player} value={player}>
                          {player}
                        </SelectItem>
                      ))}
                    </Select>
                  </div>
                )}

                {installStep === "importing" && (
                  <div className="flex flex-col items-center gap-4 py-4">
                    <Spinner size="lg" />
                    <p className="text-default-500">
                      {t("curseforge.install.importing_body", {
                        defaultValue: "Importing content...",
                      })}
                    </p>
                  </div>
                )}

                {installStep === "success" && (
                  <div className="flex flex-col items-center gap-4 py-4">
                    <div className="w-12 h-12 rounded-full bg-success/20 flex items-center justify-center text-success">
                      <LuDownload size={24} />
                    </div>
                    <p className="text-lg font-semibold">
                      {t("curseforge.install.success_msg", {
                        defaultValue: "Import Successful",
                      })}
                    </p>
                    <p className="text-default-500 text-center">
                      {t("curseforge.install.success_desc", {
                        defaultValue:
                          "The content has been successfully imported to the selected version.",
                      })}
                    </p>
                  </div>
                )}

                {installStep === "error" && (
                  <div className="flex flex-col items-center gap-4 py-4">
                    <div className="w-12 h-12 rounded-full bg-danger/20 flex items-center justify-center text-danger">
                      <LuBug size={24} />
                    </div>
                    <p className="text-lg font-semibold text-danger">
                      {t("curseforge.install.failed_msg", {
                        defaultValue: "Import Failed",
                      })}
                    </p>
                    <p className="text-default-500 text-center">
                      {installError}
                    </p>
                  </div>
                )}
              </BaseModalBody>
              <BaseModalFooter>
                {installStep === "downloading" && (
                  <Button
                    color="danger"
                    variant="flat"
                    onPress={handleCancelDownload}
                  >
                    {t("common.cancel", { defaultValue: "Cancel" })}
                  </Button>
                )}
                {(installStep === "version_select" ||
                  installStep === "player_select") && (
                  <>
                    <Button
                      variant="flat"
                      onPress={() => setInstallModalOpen(false)}
                    >
                      {t("common.cancel", { defaultValue: "Cancel" })}
                    </Button>
                    <Button
                      color="primary"
                      onPress={handleVersionSelectNext}
                      className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                    >
                      {t("curseforge.install.next", { defaultValue: "Next" })}
                    </Button>
                  </>
                )}
                {(installStep === "success" || installStep === "error") && (
                  <Button
                    color="primary"
                    onPress={onClose}
                    className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                  >
                    {t("curseforge.install.close", { defaultValue: "Close" })}
                  </Button>
                )}
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
      <BaseModal
        size="md"
        isOpen={dupOpen}
        onOpenChange={(open) => {
          if (!open) {
            setDupOpen(false);
          }
        }}
        hideCloseButton
        backdrop="blur"
        classNames={{
          base: "bg-white/80! dark:bg-zinc-900/80! backdrop-blur-2xl border-white/40! dark:border-zinc-700/50! shadow-2xl rounded-4xl",
        }}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="">
                <span className="text-xl font-bold bg-linear-to-r from-emerald-600 to-teal-600 dark:from-emerald-500 dark:to-teal-500 bg-clip-text text-transparent">
                  {t("mods.overwrite_modal_title", {
                    defaultValue: "检测到重复",
                  })}
                </span>
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-sm text-default-700">
                  {t("mods.overwrite_modal_body", {
                    defaultValue: "同名模块文件夹已存在，是否覆盖（更新）？",
                  })}
                </div>
                {dupName ? (
                  <div className="mt-1 rounded-md bg-default-100/60 border border-default-200 px-3 py-2 text-default-800 text-sm wrap-break-word whitespace-pre-wrap">
                    {dupName}
                  </div>
                ) : null}
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  variant="light"
                  onPress={() => {
                    try {
                      if (dupResolveRef.current) dupResolveRef.current(false);
                    } finally {
                      onClose();
                    }
                  }}
                >
                  {t("common.cancel", { defaultValue: "取消" })}
                </Button>
                <Button
                  color="primary"
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold shadow-lg shadow-emerald-900/20"
                  onPress={() => {
                    try {
                      if (dupResolveRef.current) dupResolveRef.current(true);
                    } finally {
                      onClose();
                    }
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" })}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </div>
  );
};

export default CurseForgeModPage;
