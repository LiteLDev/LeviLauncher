import React from "react";
import { Button } from "@heroui/react";
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
      size="lg"
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          onDismiss();
        }
      }}
      type="primary"
      classNames={{
        body: "!overflow-hidden",
      }}
      title={
        <span className="truncate">
          {t("settings.body.version.hasnew")}
          {version}
        </span>
      }
      icon={<FaRocket className="w-5 h-5" />}
      hideCloseButton
      showConfirmButton={false}
      showCancelButton={false}
      footer={
        <div className="flex w-full flex-wrap justify-end gap-2">
          <Button variant="light" onPress={onDismiss}>
            {t("common.cancel")}
          </Button>
          <Button variant="flat" onPress={onIgnore}>
            {t("settings.body.version.ignore")}
          </Button>
          <Button color="primary" isLoading={loading} onPress={onUpdate}>
            {t("settings.modal.2.footer.download_button")}
          </Button>
        </div>
      }
    >
      {body ? (
        <div className="rounded-2xl bg-default-100/60 dark:bg-zinc-800/60 border border-default-200 dark:border-zinc-700 overflow-hidden">
          <div className="px-4 py-3 border-b border-default-200/80 dark:border-zinc-700/80">
            <div className="text-small font-semibold text-default-700 dark:text-zinc-200">
              {t("downloadpage.changelog.title")}
            </div>
            <div className="text-tiny text-default-500 dark:text-zinc-400 mt-0.5">
              {version}
            </div>
          </div>
          <div className="text-small text-default-600 dark:text-zinc-300 wrap-break-word leading-6 max-h-[50vh] overflow-y-auto px-4 py-3 pr-3 custom-scrollbar">
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
                p: ({ children }) => <p className="my-1">{children}</p>,
                ul: ({ children }) => (
                  <ul className="list-disc pl-6 my-2">{children}</ul>
                ),
                ol: ({ children }) => (
                  <ol className="list-decimal pl-6 my-2">{children}</ol>
                ),
                li: ({ children }) => <li className="my-1">{children}</li>,
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
                      className="text-primary underline cursor-pointer"
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
                hr: () => <hr className="my-3 border-default-200" />,
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
