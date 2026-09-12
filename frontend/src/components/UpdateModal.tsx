import { ModalDescription, ModalAction } from "@/components/ModalPrimitives";
import { Spinner } from "@heroui/react";
import React from "react";

import { UnifiedModal } from "@/components/UnifiedModal";
import { useTranslation } from "react-i18next";
import { Browser } from "@wailsio/runtime";
import { FaRocket } from "react-icons/fa";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

interface UpdateModalProps {
  isOpen: boolean;
  version: string;
  body: string;
  loading: boolean;
  onDismiss: () => void;
  onIgnore: () => void;
  onUpdate: () => void;
}

export const UpdateModal: React.FC<UpdateModalProps> = ({
  isOpen,
  version,
  body,
  loading,
  onDismiss,
  onIgnore,
  onUpdate,
}) => {
  const { t } = useTranslation();

  return (
    <UnifiedModal
      size="wide"
      isOpen={isOpen}
      onOpenChange={(open) => { if (!open) onDismiss(); }}
      isPending={loading}
      icon={<FaRocket />}
      title={<>{t("settings.body.version.hasnew")}{version}</>}
      footer={<>
        <div className="flex w-full flex-wrap justify-end gap-2">
          <ModalAction onPress={onDismiss} isDisabled={loading} variant="secondary">
            {t("common.cancel")}
          </ModalAction>
          <ModalAction onPress={onIgnore} isDisabled={loading} variant={"secondary"}>
            {t("settings.body.version.ignore")}
          </ModalAction>
          <ModalAction
            onPress={onUpdate}
            variant={"primary"}
            isPending={loading}
          >
            {({ isPending }) => (
              <>
                <Spinner
                  size="sm"
                  color="current"
                  className={isPending ? "" : "hidden"}
                />
                {t("settings.modal.2.footer.download_button")}
              </>
            )}
          </ModalAction>
        </div>
      </>}
    >
      {body ? (
        <div className="flex min-h-0 flex-1 flex-col rounded-2xl border border-border bg-surface-secondary/60 overflow-hidden dark:border-zinc-700 ">
          <div className="border-b border-border/80 px-4 py-3 dark:border-zinc-700/80">
            <ModalDescription>
              {t("downloadpage.changelog.title")}
            </ModalDescription>
            <div className="mt-0.5 text-xs text-muted dark:text-zinc-400">
              {version}
            </div>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3 pr-3 text-sm text-foreground leading-6 wrap-break-word custom-scrollbar dark:text-zinc-300">
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={{
                h1: ({ children }) => (
                  <h1 className="text-xl font-semibold mt-2 mb-2">
                    {children}
                  </h1>
                ),
                h2: ({ children }) => (
                  <h2 className="text-lg font-semibold mt-2 mb-2">
                    {children}
                  </h2>
                ),
                h3: ({ children }) => (
                  <h3 className="text-base font-semibold mt-2 mb-2">
                    {children}
                  </h3>
                ),
                p: ({ children }) => <ModalDescription className="my-1">{children}</ModalDescription>,
                ul: ({ children }) => (
                  <ul className="list-disc pl-6 my-2">{children}</ul>
                ),
                ol: ({ children }) => (
                  <ol className="list-decimal pl-6 my-2">{children}</ol>
                ),
                li: ({ children }) => (
                  <li className="my-1">{children}</li>
                ),
                a: ({ href, children }) => {
                  const cleanUrl = (url: string) => {
                    const target = "https://github.com";
                    const idx = url.lastIndexOf(target);
                    return idx > 0 ? url.substring(idx) : url;
                  };

                  return (
                    <a
                      href={href}
                      target="_blank"
                      rel="noreferrer"
                      className="text-accent underline cursor-pointer"
                      onClick={(e) => {
                        e.preventDefault();
                        if (href) {
                          Browser.OpenURL(cleanUrl(href));
                        }
                      }}
                    >
                      {Array.isArray(children)
                        ? children.map((child) =>
                          typeof child === "string"
                            ? cleanUrl(child)
                            : child,
                        )
                        : typeof children === "string"
                          ? cleanUrl(children)
                          : children}
                    </a>
                  );
                },
                hr: () => <hr className="my-3 border-border" />,
              }}
            >
              {body}
            </ReactMarkdown>
          </div>
        </div>
      ) : null}

    </UnifiedModal>
  );
};
