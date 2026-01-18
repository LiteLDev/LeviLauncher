import React, { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { GetLIPPackage } from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import * as liptypes from "bindings/github.com/liteldev/LeviLauncher/internal/lip/client/types";
import {
  Button,
  Card,
  CardBody,
  Chip,
  Skeleton,
  Image,
  Link,
  Table,
  TableHeader,
  TableColumn,
  TableBody,
  TableRow,
  TableCell,
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
import { motion } from "framer-motion";

const LIPPackagePage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [pkg, setPkg] = useState<liptypes.GetPackageResponse | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (id) {
      loadPackage(id);
    }
  }, [id]);

  const loadPackage = async (identifier: string) => {
    setLoading(true);
    try {
      const res = await GetLIPPackage(identifier);
      setPkg(res);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="p-8 max-w-7xl mx-auto space-y-6">
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

  if (!pkg) {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <p className="text-xl text-default-500">Package not found</p>
        <Button onPress={() => navigate(-1)} className="mt-4">
          Go Back
        </Button>
      </div>
    );
  }

  return (
    <div className="h-full w-full overflow-y-auto p-4 sm:p-6 lg:p-8">
      <div className="max-w-7xl mx-auto">
        <Button
          variant="light"
          startContent={<LuArrowLeft />}
          onPress={() => navigate(-1)}
          className="mb-4"
        >
          {t("common.back", { defaultValue: "Back" })}
        </Button>

        <Card className="bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md border-none shadow-md mb-6">
          <CardBody className="p-6 sm:p-8">
            <div className="flex flex-col md:flex-row gap-8">
              <div className="shrink-0">
                <Image
                  src={pkg.avatarUrl}
                  alt={pkg.name}
                  className="w-32 h-32 object-cover rounded-2xl shadow-lg bg-content2"
                  fallbackSrc="https://via.placeholder.com/128"
                />
              </div>

              <div className="flex flex-col grow gap-3">
                <PageHeader title={pkg.name} />

                <div className="flex items-center gap-4 text-default-500 text-sm flex-wrap">
                  <div className="flex items-center gap-1">
                    <LuUser />
                    <span className="font-medium text-default-700">
                      {pkg.author}
                    </span>
                  </div>
                  <div className="flex items-center gap-1">
                    <LuFlame className="text-orange-500" />
                    <span>{pkg.hotness}</span>
                  </div>
                  <div className="flex items-center gap-1">
                    <LuClock />
                    <span>
                      Updated {new Date(pkg.updated).toLocaleDateString()}
                    </span>
                  </div>
                </div>

                <p className="text-default-600 text-lg leading-relaxed max-w-4xl">
                  {pkg.description}
                </p>

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
                <Table
                  aria-label="Versions table"
                  removeWrapper
                  color="default"
                >
                  <TableHeader>
                    <TableColumn>VERSION</TableColumn>
                    <TableColumn>RELEASED</TableColumn>
                    <TableColumn>REQUIREMENT</TableColumn>
                    <TableColumn>ACTION</TableColumn>
                  </TableHeader>
                  <TableBody>
                    {pkg.versions.map((ver) => (
                      <TableRow key={ver.version}>
                        <TableCell className="font-medium">
                          {ver.version}
                        </TableCell>
                        <TableCell>
                          {new Date(ver.releasedAt).toLocaleDateString()}
                        </TableCell>
                        <TableCell>{ver.platformVersionRequirement}</TableCell>
                        <TableCell>
                          <Button
                            size="sm"
                            color="primary"
                            variant="flat"
                            startContent={<LuDownload />}
                            isDisabled={!ver.source}
                            onPress={() => {
                              // TODO: Implement install logic
                              window.open(ver.source, "_blank");
                            }}
                          >
                            Download
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
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
                  {pkg.contributors.map((contrib) => (
                    <div
                      key={contrib.username}
                      className="flex justify-between items-center"
                    >
                      <div className="flex items-center gap-2">
                        <LuUser className="text-default-400" />
                        <span>{contrib.username}</span>
                      </div>
                      <Chip size="sm" variant="flat">
                        {contrib.contributions} commits
                      </Chip>
                    </div>
                  ))}
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
