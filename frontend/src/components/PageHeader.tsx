import React from "react";

interface PageHeaderProps {
  title: React.ReactNode;
  description?: React.ReactNode;
  className?: string;
  titleClassName?: string;
  startContent?: React.ReactNode;
  endContent?: React.ReactNode;
}

export const PageHeader: React.FC<PageHeaderProps> = ({
  title,
  description,
  className = "",
  titleClassName = "",
  startContent,
  endContent,
}) => {
  return (
    <div className={className}>
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="flex items-center gap-4">
          {startContent}
          <div>
            <h1
              className={`text-left text-3xl font-black tracking-tight text-emerald-600 dark:text-emerald-500 pb-1 ${titleClassName}`}
            >
              {title}
            </h1>
            {description && (
              <div className="mt-1 text-base sm:text-lg font-medium text-default-500 dark:text-zinc-400">
                {description}
              </div>
            )}
          </div>
        </div>
        {endContent && (
          <div className="flex items-center gap-2">{endContent}</div>
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
  iconWrapperClassName = "bg-emerald-500/10 text-emerald-600 dark:text-emerald-500",
}) => {
  return (
    <div className={`flex flex-col gap-1 ${className}`}>
      <div className="flex items-center justify-between w-full">
        <div className="flex items-center gap-2">
          {icon && (
            <div className={`p-2 rounded-xl ${iconWrapperClassName}`}>
              {icon}
            </div>
          )}
          <h2 className="text-xl font-bold text-default-800 dark:text-zinc-100">
            {title}
          </h2>
        </div>
        {action}
      </div>
      {description && (
        <p className={`text-sm text-default-500 ${icon ? "ml-12" : ""}`}>
          {description}
        </p>
      )}
    </div>
  );
};
