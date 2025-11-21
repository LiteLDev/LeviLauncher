import React, { useEffect, useState } from "react";
import { Button, Card, CardBody } from "@heroui/react";
import { Events } from "@wailsio/runtime";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";
import { useTranslation } from "react-i18next";

export default function WineGDKSetupPage() {
  const [status, setStatus] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [running, setRunning] = useState<boolean>(false);
  const [logs, setLogs] = useState<Array<{phase?: string; stream?: string; line: string}>>([]);
  const logRef = React.useRef<HTMLPreElement | null>(null);
  const { t } = useTranslation();

  useEffect(() => {
    const off1 = Events.On("winegdk.setup.status", (event: any) => {
      try { setStatus(String(event?.data ?? event ?? "")); } catch {}
    });
    const off2 = Events.On("winegdk.setup.error", (event: any) => {
      try { setError(String(event?.data ?? event ?? "")); setRunning(false); } catch {}
    });
    const off3 = Events.On("winegdk.setup.done", () => {
      setRunning(false);
      setStatus("done");
    });
    const off4 = Events.On("winegdk.setup.progress", (event: any) => {
      try {
        const data = event?.data ?? event;
        const line = typeof data === "string" ? data : String(data?.line ?? "");
        const phase = typeof data === "object" ? String(data?.phase ?? "") : undefined;
        const stream = typeof data === "object" ? String(data?.stream ?? "") : undefined;
        if (line) setLogs(prev => {
          const next = prev.concat([{ phase, stream, line }]);
          return next.length > 1000 ? next.slice(next.length - 1000) : next;
        });
      } catch {}
    });
    return () => {
      try { off1 && off1(); } catch {}
      try { off2 && off2(); } catch {}
      try { off3 && off3(); } catch {}
      try { off4 && off4(); } catch {}
    };
  }, []);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [logs]);

  const start = async () => {
    setError("");
    setStatus("start");
    setRunning(true);
    setLogs([]);
    try {
      const rc = await minecraft.SetupWineGDK?.();
      if (rc && String(rc).trim() !== "") {
        setError(String(rc));
        setRunning(false);
      }
    } catch (e: any) {
      setError(String(e?.message || e || "ERR_START"));
      setRunning(false);
    }
  };

  return (
    <div className="w-full h-full p-4">
      <div className="max-w-3xl mx-auto flex flex-col gap-4">
        <Card>
          <CardBody>
            <div className="text-lg font-semibold mb-2">{t("winegdk.install.title", { defaultValue: "WineGDK 安装" })}</div>
            <div className="text-small text-default-700 mb-4">
              {t("winegdk.install.desc", { defaultValue: "在 BaseRoot 目录下克隆并编译 WineGDK，随后安装前缀依赖（vkd3d、dxvk、dxvk_nvapi）。" })}
            </div>
            <div className="flex items-center gap-3 mb-3">
              <Button color="primary" isLoading={running} onPress={start}>
                {t("winegdk.install.start", { defaultValue: "开始安装" })}
              </Button>
              {status && (
                <span className="text-small">{t("winegdk.install.status", { defaultValue: "当前状态：" })}{status}</span>
              )}
              {error && (
                <span className="text-small text-danger">{t("winegdk.install.error", { defaultValue: "错误：" })}{error}</span>
              )}
            </div>
            <pre ref={logRef} className="w-full h-80 overflow-auto bg-default-100 rounded-md p-3 text-small">
              {logs.map((l, i) => (
                <div key={i}>
                  {l.phase ? `[${l.phase}] ` : ""}{l.stream ? `<${l.stream}> ` : ""}{l.line}
                </div>
              ))}
            </pre>
          </CardBody>
        </Card>
      </div>
    </div>
  );
}