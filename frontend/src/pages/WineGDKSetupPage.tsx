import React, { useEffect, useState } from "react";
import { Button, Card, CardBody } from "@heroui/react";
import { Events } from "@wailsio/runtime";
import * as minecraft from "../../bindings/github.com/liteldev/LeviLauncher/minecraft";

export default function WineGDKSetupPage() {
  const [status, setStatus] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [running, setRunning] = useState<boolean>(false);

  useEffect(() => {
    const off1 = Events.On("winegdk.setup.status", (e: any) => {
      try { setStatus(String(e)); } catch {}
    });
    const off2 = Events.On("winegdk.setup.error", (e: any) => {
      try { setError(String(e)); setRunning(false); } catch {}
    });
    const off3 = Events.On("winegdk.setup.done", () => {
      setRunning(false);
      setStatus("done");
    });
    return () => {
      try { off1 && off1(); } catch {}
      try { off2 && off2(); } catch {}
      try { off3 && off3(); } catch {}
    };
  }, []);

  const start = async () => {
    setError("");
    setStatus("start");
    setRunning(true);
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
            <div className="text-lg font-semibold mb-2">WineGDK 安装</div>
            <div className="text-small text-default-700 mb-4">
              在 BaseRoot 目录下克隆并编译 WineGDK，随后安装前缀依赖（vkd3d、dxvk、dxvk_nvapi）。
            </div>
            <div className="flex items-center gap-3">
              <Button color="primary" isLoading={running} onPress={start}>
                开始安装
              </Button>
              {status && (
                <span className="text-small">当前状态：{status}</span>
              )}
              {error && (
                <span className="text-small text-danger">错误：{error}</span>
              )}
            </div>
          </CardBody>
        </Card>
      </div>
    </div>
  );
}