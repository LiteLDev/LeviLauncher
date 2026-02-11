import React, { useEffect, useState } from "react";
import { Card, CardBody, Progress } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { FaRocket } from "react-icons/fa";
import { motion } from "framer-motion";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { Events, Window } from "@wailsio/runtime";
import { UnifiedModal } from "@/components/UnifiedModal";
import { PageContainer } from "@/components/PageContainer";
import { PageHeader } from "@/components/PageHeader";
import { LAYOUT } from "@/constants/layout";

export default function UpdatingPage() {
  const { t } = useTranslation();
  const [running, setRunning] = useState<boolean>(true);
  const [error, setError] = useState<string>("");
  const [status, setStatus] = useState<string>("");
  const [downloaded, setDownloaded] = useState<number>(0);
  const [total, setTotal] = useState<number>(0);

  useEffect(() => {
    try {
      (window as any).llNavLock = true;
      window.dispatchEvent(
        new CustomEvent("ll-nav-lock-changed", { detail: { lock: true } }),
      );
    } catch {}
    const off1 = Events.On("app_update_status", (event) => {
      setStatus(String(event.data || ""));
    });
    const off2 = Events.On("app_update_progress", (event) => {
      try {
        event;
        if (event && String(event.data.phase || "") === "download") {
          setDownloaded(Number(event.data.downloaded || 0));
          setTotal(Number(event.data.total || 0));
        }
      } catch {}
    });
    const off3 = Events.On("app_update_error", (event) => {
      setError(String(event.data || "UPDATE_FAILED"));
    });
    const run = async () => {
      try {
        setRunning(true);
        setError("");
        const ok = await minecraft?.Update?.();
        if (!ok) {
          setError("UPDATE_FAILED");
        }
      } catch (e: any) {
        setError(String(e?.message || e || "UPDATE_FAILED"));
      } finally {
        setRunning(false);
      }
    };
    run();
    return () => {
      off1();
      off2();
      off3();
      try {
        (window as any).llNavLock = false;
        window.dispatchEvent(
          new CustomEvent("ll-nav-lock-changed", { detail: { lock: false } }),
        );
      } catch {}
    };
  }, []);

  return (
    <PageContainer>
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
      >
        <Card className={LAYOUT.GLASS_CARD.BASE}>
          <CardBody className="p-6">
            <PageHeader
              title={t("updating.title")}
              description={t("updating.body")}
              startContent={
                <div className="p-2 rounded-xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-500">
                  <FaRocket className="w-6 h-6" />
                </div>
              }
            />

            <div className="mt-8 space-y-8 w-full pb-4">
              {/* Download Progress */}
              <div className="space-y-2">
                <div className="flex justify-between items-end">
                  <div className="text-small font-medium text-default-600">
                    {t("updating.phase.download")}
                  </div>
                  <div className="text-tiny text-default-500 font-mono">
                    {total > 0
                      ? `${(downloaded / 1024 / 1024).toFixed(1)} / ${(
                          total /
                          1024 /
                          1024
                        ).toFixed(1)} MB`
                      : `${(downloaded / 1024 / 1024).toFixed(1)} MB`}
                  </div>
                </div>
                {total > 0 ? (
                  <Progress
                    aria-label="download"
                    className="w-full"
                    color="success"
                    size="lg"
                    radius="sm"
                    value={Math.max(
                      0,
                      Math.min(100, Math.round((downloaded / total) * 100)),
                    )}
                    classNames={{
                      indicator:
                        "bg-gradient-to-r from-emerald-500 to-teal-500",
                    }}
                  />
                ) : (
                  <Progress
                    isIndeterminate
                    aria-label="download"
                    className="w-full"
                    color="success"
                    size="lg"
                    radius="sm"
                    classNames={{
                      indicator:
                        "bg-gradient-to-r from-emerald-500 to-teal-500",
                    }}
                  />
                )}
              </div>

              {/* Install Progress */}
              <div className="space-y-2">
                <div className="flex justify-between items-end">
                  <div className="text-small font-medium text-default-600">
                    {t("updating.phase.install")}
                  </div>
                  <div className="text-tiny text-default-500">
                    {status === "installing"
                      ? t("common.processing")
                      : status === "installed"
                        ? t("common.done")
                        : t("common.wait")}
                  </div>
                </div>
                {status === "installing" ? (
                  <Progress
                    isIndeterminate
                    aria-label="install"
                    className="w-full"
                    color="success"
                    size="lg"
                    radius="sm"
                    classNames={{
                      indicator:
                        "bg-gradient-to-r from-emerald-500 to-teal-500",
                    }}
                  />
                ) : (
                  <Progress
                    aria-label="install"
                    className="w-full"
                    color="success"
                    size="lg"
                    radius="sm"
                    value={status === "installed" ? 100 : 0}
                    classNames={{
                      indicator:
                        "bg-gradient-to-r from-emerald-500 to-teal-500",
                    }}
                  />
                )}
              </div>
            </div>
          </CardBody>
        </Card>
      </motion.div>

      <UnifiedModal
        size="md"
        isOpen={!!error}
        onOpenChange={() => {}}
        type="error"
        title={t("updating.failed_title")}
        confirmText={t("common.confirm")}
        onConfirm={() => Window.Close()}
        showCancelButton={false}
        hideCloseButton
        isDismissable={false}
      >
        <div className="text-default-700 text-sm wrap-break-word whitespace-pre-wrap">
          {error}
        </div>
      </UnifiedModal>
    </PageContainer>
  );
}
