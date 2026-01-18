import React, { useEffect, useState } from "react";
import { Card, CardBody, ModalContent, Button, Progress } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { FaRocket } from "react-icons/fa";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";
import { Events, Window } from "@wailsio/runtime";
import {
  BaseModal,
  BaseModalHeader,
  BaseModalBody,
  BaseModalFooter,
} from "@/components/BaseModal";

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
    <div className="relative w-full h-full px-6 py-6">
      <Card className="border-none shadow-md bg-white/50 dark:bg-zinc-900/40 backdrop-blur-md rounded-4xl">
        <CardBody className="p-6">
          <div className="flex items-center gap-2 text-primary-600 mb-3">
            <FaRocket className="w-5 h-5" />
            <span>
              {
                t("updating.title", {
                  defaultValue: "正在更新 LeviLauncher",
                }) as string
              }
            </span>
          </div>
          <div className="text-default-700 text-sm">
            {
              t("updating.body", {
                defaultValue:
                  "正在请求管理员权限并执行更新，请在系统提示中选择“是”。更新期间请勿关闭窗口。",
              }) as string
            }
          </div>
          <div className="mt-3 space-y-4">
            <div>
              <div className="text-small text-default-600 mb-1">
                {
                  t("updating.phase.download", {
                    defaultValue: "下载更新包",
                  }) as string
                }
              </div>
              {total > 0 ? (
                <Progress
                  aria-label="download"
                  className="w-full"
                  value={Math.max(
                    0,
                    Math.min(100, Math.round((downloaded / total) * 100)),
                  )}
                />
              ) : (
                <Progress
                  isIndeterminate
                  aria-label="download"
                  className="w-full"
                />
              )}
              <div className="text-tiny text-default-500 mt-1">
                {total > 0
                  ? `${(downloaded / 1024 / 1024).toFixed(1)} MB / ${(
                      total /
                      1024 /
                      1024
                    ).toFixed(1)} MB`
                  : `${(downloaded / 1024 / 1024).toFixed(1)} MB`}
              </div>
            </div>
            <div>
              <div className="text-small text-default-600 mb-1">
                {
                  t("updating.phase.install", {
                    defaultValue: "安装更新",
                  }) as string
                }
              </div>
              {status === "installing" ? (
                <Progress
                  isIndeterminate
                  aria-label="install"
                  className="w-full"
                />
              ) : (
                <Progress
                  aria-label="install"
                  className="w-full"
                  value={status === "installed" ? 100 : 0}
                />
              )}
              <div className="text-tiny text-default-500 mt-1">
                {status === "installing"
                  ? t("common.processing", { defaultValue: "正在安装..." })
                  : status === "installed"
                    ? t("common.done", { defaultValue: "安装完成" })
                    : t("common.wait", { defaultValue: "请稍候..." })}
              </div>
            </div>
          </div>
        </CardBody>
      </Card>

      <BaseModal
        size="md"
        isOpen={!!error}
        hideCloseButton
        isDismissable={false}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <BaseModalHeader className="text-danger-600">
                {
                  t("updating.failed_title", {
                    defaultValue: "更新失败",
                  }) as string
                }
              </BaseModalHeader>
              <BaseModalBody>
                <div className="text-default-700 text-sm wrap-break-word whitespace-pre-wrap">
                  {error}
                </div>
              </BaseModalBody>
              <BaseModalFooter>
                <Button
                  color="primary"
                  onPress={() => {
                    Window.Close();
                  }}
                >
                  {t("common.confirm", { defaultValue: "确定" }) as string}
                </Button>
              </BaseModalFooter>
            </>
          )}
        </ModalContent>
      </BaseModal>
    </div>
  );
}
