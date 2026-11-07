import React from "react";
import { cn } from "@/utils/cn";

interface PageHeaderProps {
  title: React.ReactNode;
  description?: React.ReactNode;
  className?: string;
  titleClassName?: string;
  descriptionClassName?: string;
  startContent?: React.ReactNode;
  endContent?: React.ReactNode;
}

export const PageHeader: React.FC<PageHeaderProps> = ({
  title,
  description,
  className = "",
  titleClassName = "",
  descriptionClassName = "",
  startContent,
  endContent,
}) => {
  return (
    <div className={className}>
      {/* Actions sit beside the title and wrap below it when space runs out. */}
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
        <div className="flex min-w-0 flex-[1_1_16rem] items-center gap-[clamp(0.75rem,1.5vw,1rem)]">
          {startContent}
          <div className="min-w-0">
            <h1
              className={cn(
                "text-left text-[clamp(1.5rem,1.1rem+1.2vw,1.875rem)] leading-tight font-black tracking-tight text-brand-600 dark:text-brand-500 pb-1 [overflow-wrap:anywhere]",
                titleClassName,
              )}
            >
              {title}
            </h1>
            {description && (
              <div
                className={cn(
                  "mt-1 text-[clamp(0.9375rem,0.85rem+0.3vw,1.125rem)] font-medium text-muted dark:text-zinc-400",
                  descriptionClassName,
                )}
              >
                {description}
              </div>
            )}
          </div>
        </div>
        {endContent && (
          <div className="ml-auto flex max-w-full flex-wrap items-center justify-end gap-2">
            {endContent}
          </div>
        )}
      </div>
    </div>
  );
};

interface SectionHeaderProps {
  title: React.ReactNode;
  description?: React.ReactNode;
  icon?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
  iconWrapperClassName?: string;
}

export const SectionHeader: React.FC<SectionHeaderProps> = ({
  title,
  description,
  icon,
  action,
  className = "",
  iconWrapperClassName = "bg-brand-500/10 text-brand-600 dark:text-brand-500",
}) => {
  return (
    <div className={`flex flex-col gap-1 ${className}`}>
      <div className="flex w-full flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <div className="flex min-w-0 items-center gap-2">
          {icon && (
            <div className={`p-2 rounded-xl shrink-0 ${iconWrapperClassName}`}>
              {icon}
            </div>
          )}
          <h2 className="min-w-0 text-[clamp(1.0625rem,0.95rem+0.4vw,1.25rem)] font-bold text-foreground dark:text-zinc-100 [overflow-wrap:anywhere]">
            {title}
          </h2>
        </div>
        {action}
      </div>
      {description && (
        <p
          className={`text-sm text-muted dark:text-zinc-400 ${icon ? "ml-12" : ""}`}
        >
          {description}
        </p>
      )}
    </div>
  );
};
