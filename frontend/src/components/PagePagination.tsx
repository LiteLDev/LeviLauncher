import { Pagination } from "@heroui/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/utils/cn";

interface PagePaginationProps {
  pageCount: number;
  currentPage: number;
  onPageChange: (page: number) => void;
  size?: "sm" | "md" | "lg";
  className?: string;
  pageClassName?: string;
  /**
   * `auto` (default) shows numbered pages on wide windows and only
   * prev / "page x / y" / next below 768px; `compact` and `full` force one.
   */
  variant?: "auto" | "full" | "compact";
}
export function PagePagination({
  pageCount,
  currentPage,
  onPageChange,
  size = "sm",
  className,
  pageClassName,
  variant = "auto",
}: PagePaginationProps) {
  const { t } = useTranslation();
  const lastPage = Math.max(1, pageCount);
  const page = Math.min(lastPage, Math.max(1, currentPage));
  const pages = [...new Set([1, lastPage, page - 1, page, page + 1])]
    .filter((p) => p >= 1 && p <= lastPage)
    .sort((a, b) => a - b);
  const items: (number | string)[] = [];
  for (const [index, p] of pages.entries()) {
    const previous = pages[index - 1];
    if (previous && p - previous === 2) items.push(previous + 1);
    else if (previous && p - previous > 2) items.push(`gap-${p}`);
    items.push(p);
  }
  const numberedClass =
    variant === "compact" ? "hidden" : variant === "auto" ? "max-md:hidden" : "";
  const summaryClass =
    variant === "full" ? "hidden" : variant === "auto" ? "md:hidden" : "";
  return (
    <Pagination size={size} className={className}>
      <Pagination.Content>
        <Pagination.Item>
          <Pagination.Previous
            aria-label={t("common.previous", { defaultValue: "Previous page" })}
            isDisabled={page === 1}
            onPress={() => onPageChange(page - 1)}
          >
            <Pagination.PreviousIcon />
          </Pagination.Previous>
        </Pagination.Item>
        <Pagination.Item className={summaryClass}>
          <span
            className="px-2 text-sm font-medium tabular-nums whitespace-nowrap text-foreground"
          >
            {t("primitives.page_of", { page, total: lastPage })}
          </span>
        </Pagination.Item>
        {items.map((item) => (
          <Pagination.Item key={item} className={numberedClass}>
            {typeof item === "number" ? (
              <Pagination.Link
                isActive={page === item}
                aria-label={t("common.page_number", {
                  defaultValue: "Page {{page}}",
                  page: item,
                })}
                className={cn(
                  pageClassName,
                  page === item &&
                    "bg-accent brand-primary-foreground font-bold",
                )}
                onPress={() => onPageChange(item)}
              >
                {item}
              </Pagination.Link>
            ) : (
              <Pagination.Ellipsis />
            )}
          </Pagination.Item>
        ))}
        <Pagination.Item>
          <Pagination.Next
            aria-label={t("common.next", { defaultValue: "Next page" })}
            isDisabled={page === lastPage}
            onPress={() => onPageChange(page + 1)}
          >
            <Pagination.NextIcon />
          </Pagination.Next>
        </Pagination.Item>
      </Pagination.Content>
    </Pagination>
  );
}
