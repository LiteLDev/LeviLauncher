import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import { TakeFiles } from "bindings/github.com/liteldev/LeviLauncher/internal/app/fileopenservice";
import { UnifiedModal } from "@/components/UnifiedModal";
import { setNavLockReason } from "@/hooks/useAppNavigation";
import { ROUTES } from "@/constants/routes";
import { readCurrentVersionName } from "@/utils/currentVersion";

export function OpenedContentFiles({ blocked }: { blocked: boolean }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [files, setFiles] = useState<string[]>([]);
  const [revision, setRevision] = useState(0);
  const taking = useRef(false);
  const events = useRef(0);
  const importing = Boolean(location.state?.openedContentFiles);

  useEffect(() => Events.On("content-files.open", () => {
    events.current += 1;
    setRevision(events.current);
  }), []);

  useEffect(() => {
    if (blocked || importing || files.length || taking.current) return;
    taking.current = true;
    const requestedAt = events.current;
    void TakeFiles()
      .then((pending) => setFiles(pending ?? []))
      .catch((err) => console.warn("Unable to retrieve opened content files", err))
      .finally(() => {
        taking.current = false;
        if (events.current !== requestedAt) {
          events.current += 1;
          setRevision(events.current);
        }
      });
  }, [blocked, importing, files.length, revision]);

  const instance = readCurrentVersionName();
  const close = () => setFiles([]);
  return (
    <UnifiedModal
      isOpen={!blocked && files.length > 0}
      onOpenChange={(open) => { if (!open) close(); }}
      title={t("mods.import_button")}
      showCancelButton
      cancelText={t("common.cancel")}
      confirmText={t("common.confirm")}
      confirmButtonProps={{ isDisabled: !instance }}
      onCancel={close}
      onConfirm={() => {
        if (!instance || !files.length || blocked) return;
        const route = files[0].toLowerCase().endsWith(".mcmod")
          ? ROUTES.mods
          : ROUTES.content;
        navigate(route, { state: { openedContentFiles: files } });
        close();
      }}
    >
      <p>{instance || t("launcherpage.currentVersion_none")}</p>
      <ul className="list-disc pl-5">
        {files.map((file, index) => (
          <li className="break-all" key={index}>{file.split(/[/\\]/).pop()}</li>
        ))}
      </ul>
    </UnifiedModal>
  );
}

export function useOpenedContentImport(
  importFiles: (files: string[]) => Promise<void>,
) {
  const location = useLocation();
  const navigate = useNavigate();
  const started = useRef("");
  const importRef = useRef(importFiles);
  importRef.current = importFiles;

  useEffect(() => {
    const files = location.state?.openedContentFiles;
    if (!Array.isArray(files) || !files.length || started.current === location.key) return;
    started.current = location.key;
    // Consume the route state immediately so browser history cannot replay it.
    navigate(location.pathname + location.search, { replace: true, state: null });
    setNavLockReason("opened-content-import", true);
    void importRef.current(files).finally(() => {
      setNavLockReason("opened-content-import", false);
    });
  }, [location, navigate]);
}
