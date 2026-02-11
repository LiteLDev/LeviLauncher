import React from "react";
import {
  Modal,
  ModalProps,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalHeaderProps,
  ModalBodyProps,
  ModalFooterProps,
} from "@heroui/react";

export const BaseModal: React.FC<ModalProps> = (props) => {
  const { classNames, ...rest } = props;

  const finalClassNames = {
    ...classNames,
    backdrop: `top-14 backdrop-blur-md bg-black/50 ${classNames?.backdrop || ""}`,
    base: `bg-white! dark:bg-zinc-950! border border-default-200 dark:border-zinc-800 shadow-2xl rounded-[2.5rem] ${
      classNames?.base || ""
    }`,
    closeButton: `absolute right-5 top-5 z-50 hover:bg-black/5 dark:hover:bg-white/5 active:bg-black/10 dark:active:bg-white/10 text-default-500 ${
      classNames?.closeButton || ""
    }`,
  };

  return <Modal {...rest} classNames={finalClassNames} />;
};

export const BaseModalHeader: React.FC<ModalHeaderProps> = ({
  className,
  ...props
}) => {
  return (
    <ModalHeader
      className={`flex flex-col gap-1 px-8 pt-6 pb-2 ${className || ""}`}
      {...props}
    />
  );
};

export const BaseModalBody: React.FC<ModalBodyProps> = ({
  className,
  ...props
}) => {
  return <ModalBody className={`px-8 py-4 ${className || ""}`} {...props} />;
};

export const BaseModalFooter: React.FC<ModalFooterProps> = ({
  className,
  ...props
}) => {
  return (
    <ModalFooter className={`px-8 pb-8 pt-4 ${className || ""}`} {...props} />
  );
};
