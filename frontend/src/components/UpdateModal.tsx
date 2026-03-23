import React from "react";
import { Button, ModalContent } from "@heroui/react";
import {
  BaseModal,
  BaseModalBody,
  BaseModalFooter,
  BaseModalHeader,
} from "@/components/BaseModal";
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
    <BaseModal
      size="lg"
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          onDismiss();
        }
      }}
      scrollBehavior="inside"
      classNames={{
        base: "overflow-hidden bg-white/80! dark:bg-zinc-900/80! backdrop-blur-2xl border-white/40! dark:border-zinc-700/50! shadow-2xl rounded-4xl",
        wrapper: "overflow-hidden",
      }}
      hideCloseButton
    >
      <ModalContent className="shadow-none">
        {() => (
          <>
            <BaseModalHeader className="flex flex-row items-center gap-3">
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full border border-primary-100 bg-primary-50 dark:border-primary-500/20 dark:bg-primary-500/10">
                <FaRocket className="h-5 w-5 text-primary-500" />
              </div>
              <div className="flex flex-col">
                <h2 className="text-xl font-bold text-primary-500">
                  <span className="truncate">
                    {t("settings.body.version.hasnew")}
                    {version}
                  </span>
                </h2>
              </div>
            </BaseModalHeader>

            <BaseModalBody className="flex min-h-0 flex-1 flex-col !overflow-hidden">
              {body ? (
                <div className="flex min-h-0 flex-1 flex-col rounded-2xl border border-default-200 bg-default-100/60 overflow-hidden dark:border-zinc-700 dark:bg-zinc-800/60">
                  <div className="border-b border-default-200/80 px-4 py-3 dark:border-zinc-700/80">
                    <div className="text-small font-semibold text-default-700 dark:text-zinc-200">
                      {t("downloadpage.changelog.title")}
                    </div>
                    <div className="mt-0.5 text-tiny text-default-500 dark:text-zinc-400">
                      {version}
                    </div>
                  </div>
                  <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3 pr-3 text-small text-default-600 leading-6 wrap-break-word custom-scrollbar dark:text-zinc-300">
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
            </BaseModalBody>

            <BaseModalFooter>
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
            </BaseModalFooter>
          </>
        )}
      </ModalContent>
    </BaseModal>
  );
};
