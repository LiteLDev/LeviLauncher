import React, { useEffect, useMemo, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  Button,
  Card,
  CardBody,
  Chip,
  Skeleton,
  Image,
  Link,
} from "@heroui/react";
import { PageHeader, SectionHeader } from "@/components/PageHeader";
import {
  LuArrowLeft,
  LuDownload,
  LuGlobe,
  LuUser,
  LuClock,
  LuFlame,
  LuTag,
} from "react-icons/lu";
import { fetchLIPPackagesIndex, type LIPPackageBasicInfo } from "@/utils/content";

const formatUpdatedText = (value: string) => {
  if (!value) return "-";
  const ts = Date.parse(value);
  if (!Number.isFinite(ts)) return value;
  return new Date(ts).toLocaleDateString();
};

const LIPPackagePage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [pkg, setPkg] = useState<LIPPackageBasicInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const identifier = useMemo(() => {
    const raw = String(id || "");
    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }, [id]);

  const loadPackage = async (targetIdentifier: string, forceRefresh = false) => {
    setLoading(true);
    setError("");
    try {
      const list = await fetchLIPPackagesIndex({ forceRefresh });
      const found = list.find((item) => item.identifier === targetIdentifier) || null;
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

  if (loading) {
    return (
      <div className="p-6 max-w-7xl mx-auto space-y-6">
        <Skeleton className="h-8 w-32 rounded-lg" />
        <div className="flex gap-6">
          <Skeleton className="w-32 h-32 rounded-2xl" />
          <div className="flex-1 space-y-4">
            <Skeleton className="h-10 w-1/2 rounded-lg" />
            <Skeleton className="h-4 w-1/4 rounded-lg" />
            <Skeleton className="h-20 w-full rounded-lg" />
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 max-w-7xl mx-auto space-y-4">
        <Button
          variant="light"
          startContent={<LuArrowLeft />}
          onPress={() => navigate(-1)}
        >
          {t("common.back")}
        </Button>
        <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none shadow-md">
          <CardBody className="p-6 flex flex-col gap-4">
            <p className="text-danger-500">{error}</p>
            <div className="flex gap-2">
              <Button color="primary" onPress={() => void loadPackage(identifier, true)}>
                {t("common.retry")}
              </Button>
            </div>
          </CardBody>
        </Card>
      </div>
    );
  }

  if (!pkg) {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <p className="text-xl text-default-500 dark:text-zinc-400">Package not found</p>
        <Button onPress={() => navigate(-1)} className="mt-4">
          {t("common.back")}
        </Button>
      </div>
    );
  }

  return (
    <div className="h-full w-full overflow-y-auto p-6">
      <div className="max-w-7xl mx-auto">
        <Button
          variant="light"
          startContent={<LuArrowLeft />}
          onPress={() => navigate(-1)}
          className="mb-4"
        >
          {t("common.back")}
        </Button>

        <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none shadow-md mb-6">
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

              <div className="flex flex-col grow gap-6">
                <PageHeader title={pkg.name} />

                <div className="flex items-center gap-4 text-default-500 dark:text-zinc-400 text-sm flex-wrap">
                  <div className="flex items-center gap-1">
                    <LuUser />
                    <span className="font-medium text-default-700 dark:text-zinc-200">
                      {pkg.author || t("common.unknown")}
                    </span>
                  </div>
                  <div className="flex items-center gap-1">
                    <LuFlame className="text-orange-500" />
                    <span>{pkg.hotness}</span>
                  </div>
                  <div className="flex items-center gap-1">
                    <LuClock />
                    <span>
                      Updated {formatUpdatedText(pkg.updated)}
                    </span>
                  </div>
                </div>

                <p className="text-default-600 dark:text-zinc-300 text-lg leading-relaxed max-w-4xl">
                  {pkg.description || t("lip.no_description")}
                </p>

                <div className="text-xs text-default-400 font-mono">{pkg.identifier}</div>

                <div className="flex flex-wrap gap-2 mt-2">
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
                </div>

                <div className="flex gap-2 mt-4">
                  {pkg.projectUrl && (
                    <Button
                      as={Link}
                      href={pkg.projectUrl}
                      isExternal
                      showAnchorIcon
                      variant="flat"
                      startContent={<LuGlobe />}
                    >
                      Project Page
                    </Button>
                  )}
                </div>
              </div>
            </div>
          </CardBody>
        </Card>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2 space-y-6">
            <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none shadow-sm">
              <CardBody className="p-4">
                <SectionHeader
                  title="Versions"
                  className="mb-4"
                  icon={<LuDownload size={20} />}
                />
                {pkg.versions.length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {pkg.versions.map((version) => (
                      <Chip key={version} variant="flat" size="sm" className="font-mono">
                        {version}
                      </Chip>
                    ))}
                  </div>
                ) : (
                  <div className="text-sm text-default-500 dark:text-zinc-400">
                    {t("common.empty")}
                  </div>
                )}
              </CardBody>
            </Card>
          </div>

          <div className="space-y-6">
            <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none shadow-sm">
              <CardBody className="p-4">
                <SectionHeader
                  title="Contributors"
                  className="mb-4"
                  icon={<LuUser size={20} />}
                />
                <div className="flex flex-col gap-3">
                  <div className="flex justify-between items-center">
                    <div className="flex items-center gap-2">
                      <LuUser className="text-default-400" />
                      <span>{pkg.author || t("common.unknown")}</span>
                    </div>
                    <Chip size="sm" variant="flat">
                      author
                    </Chip>
                  </div>
                </div>
              </CardBody>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
};

export default LIPPackagePage;
