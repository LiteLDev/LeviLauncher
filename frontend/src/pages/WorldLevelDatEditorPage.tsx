import {
  Button,
  Card,
  Chip,
  CloseButton,
  Input,
  InputGroup,
  Label,
  ListBox,
  Select,
  Spinner,
  Switch,
  TextField,
  Tooltip,
  toast,
} from "@heroui/react";

import React from "react";
import { useTranslation } from "react-i18next";

import { motion, AnimatePresence } from "framer-motion";
import { useBlocker, useLocation, useNavigate } from "react-router-dom";
import {
  FaArrowLeft,
  FaSave,
  FaSync,
  FaPlus,
  FaTimes,
  FaSearch,
} from "react-icons/fa";
import { PageHeader } from "@/components/PageHeader";
import { PageContainer } from "@/components/PageContainer";
import { LAYOUT } from "@/constants/layout";
import { COMPONENT_STYLES } from "@/constants/componentStyles";
import { cn } from "@/utils/cn";
import { UnifiedModal } from "@/components/UnifiedModal";
import { useRouteTitle } from "@/hooks/useRouteTitle";
import {
  useLevelDatEditor,
  normTag,
  getEnumOpts,
  parseListJSON,
  stringifyList,
  TAG_OPTIONS,
} from "@/hooks/useLevelDatEditor";

export default function WorldLevelDatEditorPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const loc = useLocation();
  const sp = new URLSearchParams(String(loc?.search || ""));
  const worldPath = sp.get("path") || "";

  const {
    isDirty,
    loading,
    error,
    levelName,
    setLevelName,
    saving,
    typedVersion,
    compoundOpen,
    setCompoundOpen,
    compoundFields,
    typedDrafts,
    setTypedDrafts,
    filterText,
    setFilterText,
    addTargetKey,
    setAddTargetKey,
    newUnifiedField,
    setNewUnifiedField,
    addOpen,
    setAddOpen,
    scrollRef,
    hasBackend,
    orderedTopFields,
    compoundTargetKeys,
    beforeUpdate,
    matchesName,
    setTypedFieldValueByName,
    setCompoundFieldValue,
    loadCompound,
    addField,
    load,
    saveAll,
    getOrderedCompoundChildren,
  } = useLevelDatEditor(worldPath);
  useRouteTitle(levelName);
  const blocker = useBlocker(isDirty || saving);
  const [refreshPending, setRefreshPending] = React.useState(false);
  const confirmationOpen = refreshPending || blocker.state === "blocked";

  const continueEditing = () => {
    setRefreshPending(false);
    if (blocker.state === "blocked") blocker.reset();
  };
  const continueAction = () => {
    if (blocker.state === "blocked") blocker.proceed();
    else if (refreshPending) void load();
    setRefreshPending(false);
  };
  const requestRefresh = () => {
    if (saving || loading) return;
    if (isDirty) setRefreshPending(true);
    else void load();
  };

  React.useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (!isDirty && !saving) return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, [isDirty, saving]);

  const handleSave = async () => {
    const ok = await saveAll();
    if (ok) toast(t("common.success"), { variant: "success" });
  };

  const FieldBox = React.useMemo(() => {
    const chipColor = (tp: string): any => {
      const t = String(tp || "").toLowerCase();
      if (t === "compound") return "secondary";
      if (t === "list") return "warning";
      if (t === "string") return "primary";
      if (
        t === "byte" ||
        t === "short" ||
        t === "int" ||
        t === "long" ||
        t === "float" ||
        t === "double"
      )
        return "success";
      if (t === "add") return "primary";
      return "default";
    };
    return function Box({
      title,
      type,
      children,
      delay = 0,
    }: {
      title: string;
      type: string;
      children: React.ReactNode;
      delay?: number;
    }) {
      return (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, delay: delay * 0.03 }}
          className="group relative overflow-hidden rounded-2xl border border-border dark:border-border/10 bg-white/50 dark:bg-zinc-900/50 p-4 transition-all hover:bg-surface-secondary dark:hover:bg-zinc-800/50 hover:shadow-lg"
        >
          <div className="flex items-center justify-between mb-3">
            <div
              className="text-sm font-semibold text-foreground dark:text-zinc-200 truncate"
              title={title}
            >
              {title}
            </div>
            <Chip
              size="sm"
              variant="soft"
              color={chipColor(type)}
              className={
                "h-6 min-w-12 justify-center font-mono text-xs uppercase"
              }
            >
              <Chip.Label>{type}</Chip.Label>
            </Chip>
          </div>
          <div className="relative z-10">{children}</div>
        </motion.div>
      );
    };
  }, []);

  return (
    <PageContainer>
      <Card className={cn("flex-1 min-h-0", LAYOUT.GLASS_CARD.BASE)}>
        <Card.Content className="p-0 flex flex-col h-full overflow-hidden">
          <div className="shrink-0 p-6 flex flex-col gap-6 border-b border-border dark:border-white/10">
            <PageHeader
              title={t("contentpage.world_leveldat_editor")}
              startContent={
                <Button
                  isIconOnly
                  aria-label={t("common.back")}
                  isDisabled={saving || loading}
                  onPress={() => navigate(-1)}
                  variant={"ghost"}
                  className={"rounded-full"}
                >
                  <FaArrowLeft size={20} />
                </Button>
              }
              endContent={
                <>
                  <TextField
                    aria-label={t("common.search") as string}
                    className={cn(
                      "group",
                      COMPONENT_STYLES.input.mainWrapper,
                      "w-48 sm:w-64",
                    )}
                    value={filterText}
                    onChange={(v) => {
                      beforeUpdate();
                      setFilterText(v);
                    }}
                  >
                    <InputGroup
                      className={cn(
                        COMPONENT_STYLES.input.inputWrapper,
                        COMPONENT_STYLES.input.innerWrapper,
                        "rounded-full",
                        "min-h-8 text-sm",
                      )}
                    >
                      <InputGroup.Prefix>
                        {<FaSearch className="text-muted" />}
                      </InputGroup.Prefix>
                      <InputGroup.Input
                        placeholder={t("common.search") as string}
                        className={COMPONENT_STYLES.input.input}
                      />
                      <InputGroup.Suffix>
                        {filterText && (
                          <CloseButton
                            aria-label="Clear"
                            onPress={() =>
                              ((v) => {
                                beforeUpdate();
                                setFilterText(v);
                              })("")
                            }
                            className={COMPONENT_STYLES.input.clearButton}
                          />
                        )}
                      </InputGroup.Suffix>
                    </InputGroup>
                  </TextField>
                  <Tooltip>
                    <Button
                      isIconOnly
                      aria-label={t("common.refresh")}
                      onPress={requestRefresh}
                      isDisabled={saving}
                      variant={"secondary"}
                      isPending={loading}
                      className={cn(
                        "rounded-full",
                        "bg-surface-secondary dark:bg-surface/20 text-foreground dark:text-zinc-300",
                      )}
                    >
                      {({ isPending }) => (
                        <>
                          <Spinner
                            size="sm"
                            color="current"
                            className={isPending ? "" : "hidden"}
                          />
                          <FaSync className={loading ? "animate-spin" : ""} />
                        </>
                      )}
                    </Button>
                    <Tooltip.Content>{t("common.refresh")}</Tooltip.Content>
                  </Tooltip>
                  <Tooltip>
                    <Button
                      isIconOnly
                      onPress={handleSave}
                      aria-label={t("common.save")}
                      isDisabled={!hasBackend || loading || saving || !isDirty}
                      variant={"primary"}
                      isPending={saving}
                      className={cn(
                        "rounded-full",
                        "bg-brand-500 brand-primary-foreground shadow-lg shadow-brand-900/20",
                      )}
                    >
                      {({ isPending }) => (
                        <>
                          <Spinner
                            size="sm"
                            color="current"
                            className={isPending ? "" : "hidden"}
                          />
                          <FaSave className="w-4 h-4" />
                        </>
                      )}
                    </Button>
                    <Tooltip.Content>{t("common.save")}</Tooltip.Content>
                  </Tooltip>
                </>
              }
            />

            {isDirty && <p role="status" className="text-sm text-warning">{t("contentpage.editor_unsaved")}</p>}

            {error && (
              <div className="w-full p-4 rounded-2xl bg-rose-50 dark:bg-rose-900/20 border border-rose-200 dark:border-rose-800/50 text-danger flex items-center gap-2">
                <FaTimes className="w-4 h-4" />
                <span>{t(error)}</span>
              </div>
            )}
          </div>
          <div
            ref={scrollRef}
            className="flex-1 overflow-y-auto pretty-scrollbar p-4 sm:p-6 pt-0"
            inert={saving}
          >
            {loading ? (
              <div className="flex flex-col items-center justify-center h-64 gap-4">
                <Spinner size="lg" color={"accent"} />
                <div className="text-muted animate-pulse">
                  {t("common.loading")}
                </div>
              </div>
            ) : (
              <div className="flex flex-col gap-6">
                {/* Basic Info Section */}
                <motion.div
                  initial={{ opacity: 0, x: -20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.4, delay: 0.1 }}
                  className="p-6 rounded-2xl bg-white/50 dark:bg-zinc-900/50 border border-border dark:border-border/10 backdrop-blur-md shadow-sm"
                >
                  <div className="flex items-center gap-4 mb-6">
                    <div className="w-1 h-6 rounded-full bg-brand-500 shadow-[0_0_10px_rgba(16,185,129,0.4)]" />
                    <h3 className="text-lg font-bold text-foreground dark:text-zinc-200">
                      {t("contentpage.basic_info")}
                    </h3>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <TextField
                      className={cn(
                        "group",
                        COMPONENT_STYLES.input.mainWrapper,
                      )}
                      value={levelName}
                      onChange={(v) => {
                        beforeUpdate();
                        setLevelName(v);
                      }}
                    >
                      <Label className={COMPONENT_STYLES.input.label}>
                        {t("contentpage.level_name")}
                      </Label>
                      <Input
                        placeholder="My World"
                        className={cn(
                          COMPONENT_STYLES.input.inputWrapper,
                          COMPONENT_STYLES.input.input,
                          "rounded-lg",
                        )}
                      />
                    </TextField>
                    <div className="flex flex-col gap-2">
                      <label className="text-sm text-foreground dark:text-zinc-400">
                        {t("contentpage.version")}
                      </label>
                      <div className="h-10 px-3 flex items-center rounded-lg bg-surface-secondary dark:bg-zinc-800/50 text-muted dark:text-zinc-400 text-sm font-mono border border-transparent dark:border-zinc-700/50">
                        {typedVersion}
                      </div>
                    </div>
                  </div>
                </motion.div>
                <div
                  role="separator"
                  className="h-px bg-surface-tertiary/50 my-2"
                />
                {/* Add Field Section */}
                <div className="flex flex-col gap-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <div className="w-1 h-6 rounded-full bg-brand-500 shadow-[0_0_10px_rgba(16,185,129,0.4)]" />
                      <h3 className="text-lg font-bold text-foreground dark:text-zinc-200">
                        {t("contentpage.add_field")}
                      </h3>
                    </div>
                    <Button
                      size="sm"
                      onPress={() => setAddOpen((o) => !o)}
                      variant={"secondary"}
                      className={cn(
                        "rounded-full",
                        "bg-surface-secondary dark:bg-surface/20 text-foreground dark:text-zinc-300",
                      )}
                    >
                      {addOpen ? <FaTimes /> : <FaPlus />}
                      {addOpen ? t("common.collapse") : t("common.expand")}
                    </Button>
                  </div>

                  <AnimatePresence>
                    {addOpen && (
                      <motion.div
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: "auto", opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={{ duration: 0.3 }}
                        className="overflow-hidden"
                      >
                        <div className="p-6 rounded-2xl bg-white/50 dark:bg-zinc-900/50 border border-border dark:border-border/10 backdrop-blur-md shadow-sm transition-all">
                          <div className="grid grid-cols-1 sm:grid-cols-[140px_1fr_100px_1fr_80px] gap-4 items-end">
                            <Select
                              value={
                                Array.from(new Set([addTargetKey]))[0] ?? null
                              }
                              onChange={(keys: any) => {
                                const v = String(keys || "root");
                                setAddTargetKey(v);
                              }}
                            >
                              <Label>{"Target"}</Label>
                              <Select.Trigger
                                className={cn(
                                  COMPONENT_STYLES.select.trigger,
                                  "rounded-lg",
                                  "min-h-8 text-sm",
                                )}
                              >
                                <Select.Value />
                                <Select.Indicator />
                              </Select.Trigger>
                              <Select.Popover
                                className={
                                  COMPONENT_STYLES.select.popoverContent
                                }
                              >
                                <ListBox
                                  className={COMPONENT_STYLES.select.listbox}
                                >
                                  {compoundTargetKeys.map((o) => (
                                    <ListBox.Item key={o} id={o} textValue={o}>
                                      <Label>{o}</Label>
                                      <ListBox.ItemIndicator />
                                    </ListBox.Item>
                                  ))}
                                </ListBox>
                              </Select.Popover>
                            </Select>
                            <TextField
                              className={cn(
                                "group",
                                COMPONENT_STYLES.input.mainWrapper,
                              )}
                              value={newUnifiedField.name}
                              onChange={(v) =>
                                setNewUnifiedField((prev) => ({
                                  ...prev,
                                  name: v,
                                }))
                              }
                            >
                              <Label className={COMPONENT_STYLES.input.label}>
                                {"Name"}
                              </Label>
                              <Input
                                placeholder={
                                  t("contentpage.field_name") as string
                                }
                                className={cn(
                                  COMPONENT_STYLES.input.inputWrapper,
                                  COMPONENT_STYLES.input.input,
                                  "rounded-lg",
                                  "min-h-8 text-sm",
                                )}
                              />
                            </TextField>
                            <Select
                              value={
                                Array.from(new Set([newUnifiedField.tag]))[0] ??
                                null
                              }
                              onChange={(keys: any) => {
                                const v = String(keys || "string");
                                setNewUnifiedField((prev) => ({
                                  ...prev,
                                  tag: v,
                                }));
                              }}
                            >
                              <Label>{"Type"}</Label>
                              <Select.Trigger
                                className={cn(
                                  COMPONENT_STYLES.select.trigger,
                                  "rounded-lg",
                                  "min-h-8 text-sm",
                                )}
                              >
                                <Select.Value />
                                <Select.Indicator />
                              </Select.Trigger>
                              <Select.Popover
                                className={
                                  COMPONENT_STYLES.select.popoverContent
                                }
                              >
                                <ListBox
                                  className={COMPONENT_STYLES.select.listbox}
                                >
                                  {TAG_OPTIONS.map((o) => (
                                    <ListBox.Item key={o} id={o} textValue={o}>
                                      <Label>{o}</Label>
                                      <ListBox.ItemIndicator />
                                    </ListBox.Item>
                                  ))}
                                </ListBox>
                              </Select.Popover>
                            </Select>
                            <TextField
                              className={cn(
                                "group",
                                COMPONENT_STYLES.input.mainWrapper,
                              )}
                              value={newUnifiedField.value}
                              onChange={(v) =>
                                setNewUnifiedField((prev) => ({
                                  ...prev,
                                  value: v,
                                }))
                              }
                            >
                              <Label className={COMPONENT_STYLES.input.label}>
                                {"Value"}
                              </Label>
                              <Input
                                placeholder={
                                  t("contentpage.initial_value") as string
                                }
                                className={cn(
                                  COMPONENT_STYLES.input.inputWrapper,
                                  COMPONENT_STYLES.input.input,
                                  "rounded-lg",
                                  "min-h-8 text-sm",
                                )}
                              />
                            </TextField>
                            <Button
                              size="sm"
                              onPress={addField}
                              variant={"secondary"}
                              className={cn(
                                "rounded-lg",
                                "bg-brand-500 brand-primary-foreground shadow-lg shadow-brand-900/20",
                              )}
                            >
                              {t("common.add")}
                            </Button>
                          </div>
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
                <div
                  role="separator"
                  className="h-px bg-surface-tertiary/50 my-2"
                />
                {(() => {
                  const acc: React.ReactNode[] = [];
                  const out: React.ReactNode[] = [];
                  orderedTopFields.forEach((f, i) => {
                    const k = f.name as any;
                    const tag = normTag(f.tag);
                    if (tag !== "compound") {
                      if (!matchesName(k)) return;
                      if (tag === "list") {
                        const dk = `tflist:${k}`;
                        const display =
                          typedDrafts[dk] ??
                          String((f as any).valueJSON || "[]");
                        const items = parseListJSON(display);
                        acc.push(
                          <FieldBox
                            key={`tf-${k}`}
                            title={k}
                            type={tag}
                            delay={i * 0.015}
                          >
                            <div className="flex flex-col gap-2">
                              <div className="flex gap-1 overflow-x-auto flex-nowrap pretty-scrollbar gutter-stable">
                                {items.map((it, idx) => (
                                  <TextField
                                    key={`tf-${k}-li-${idx}`}
                                    aria-label={`${k} item ${idx}`}
                                    className={cn(
                                      "group",
                                      COMPONENT_STYLES.input.mainWrapper,
                                      "w-12 shrink-0",
                                    )}
                                    value={String(it ?? "")}
                                    onChange={(v) => {
                                      const next = items.slice();
                                      next[idx] = v;
                                      beforeUpdate();
                                      setTypedDrafts((prev) => ({
                                        ...prev,
                                        [dk]: stringifyList(next),
                                      }));
                                    }}
                                  >
                                    <Input

                                      className={cn(
                                        COMPONENT_STYLES.input.inputWrapper,
                                        COMPONENT_STYLES.input.input,
                                        "rounded-lg",
                                        "min-h-8 text-sm",
                                      )}
                                    />
                                  </TextField>
                                ))}
                              </div>
                            </div>
                          </FieldBox>,
                        );
                      } else if (tag === "byte") {
                        const isOn =
                          String((f as any).valueString || "0") !== "0";
                        acc.push(
                          <FieldBox
                            key={`tf-${k}`}
                            title={k}
                            type={tag}
                            delay={i * 0.015}
                          >
                            <div className="flex justify-end">
                              <Switch
                                aria-label={String(k)}
                                size="sm"
                                isSelected={isOn}
                                onChange={(c: boolean) => {
                                  setTypedFieldValueByName(String(k), {
                                    valueString: c ? "1" : "0",
                                  });
                                }}
                                className={"group"}
                              >
                                <Switch.Content>
                                  <Switch.Control>
                                    <Switch.Thumb>
                                      {
                                        <span className="block w-2 h-2 bg-black rounded-full" />
                                      }
                                    </Switch.Thumb>
                                  </Switch.Control>
                                  <span></span>
                                </Switch.Content>
                              </Switch>
                            </div>
                          </FieldBox>,
                        );
                      } else {
                        const opts = getEnumOpts(String(k));
                        if (opts) {
                          acc.push(
                            <FieldBox
                              key={`tf-${k}`}
                              title={k}
                              type={tag}
                              delay={i * 0.015}
                            >
                              <Select
                                aria-label={String(k)}
                                value={
                                  Array.from(
                                    new Set([
                                      String((f as any).valueString || "0"),
                                    ]),
                                  )[0] ?? null
                                }
                                onChange={(keys: any) => {
                                  const v = keys || "0";
                                  setTypedFieldValueByName(String(k), {
                                    valueString: String(v),
                                  });
                                }}
                              >
                                <Select.Trigger
                                  className={cn(
                                    "bg-surface-secondary dark:bg-surface/20",
                                    "rounded-lg",
                                    "min-h-8 text-sm",
                                  )}
                                >
                                  <Select.Value />
                                  <Select.Indicator />
                                </Select.Trigger>
                                <Select.Popover>
                                  <ListBox>
                                    {opts.map((o) => (
                                      <ListBox.Item
                                        key={o.value}
                                        id={o.value}
                                        textValue={o.label}
                                      >
                                        <Label>{o.label}</Label>
                                        <ListBox.ItemIndicator />
                                      </ListBox.Item>
                                    ))}
                                  </ListBox>
                                </Select.Popover>
                              </Select>
                            </FieldBox>,
                          );
                        } else {
                          const dk = `tf:${k}`;
                          const display =
                            typedDrafts[dk] ??
                            String((f as any).valueString || "");
                          acc.push(
                            <FieldBox
                              key={`tf-${k}`}
                              title={k}
                              type={tag}
                              delay={i * 0.015}
                            >
                              <TextField
                                aria-label={String(k)}
                                className={cn(
                                  "group",
                                  COMPONENT_STYLES.input.mainWrapper,
                                )}
                                value={display}
                                onChange={(v) => {
                                  beforeUpdate();
                                  setTypedDrafts((prev) => ({
                                    ...prev,
                                    [dk]: v,
                                  }));
                                }}
                              >
                                <Input

                                  className={cn(
                                    COMPONENT_STYLES.input.inputWrapper,
                                    COMPONENT_STYLES.input.input,
                                    "rounded-lg",
                                    "min-h-8 text-sm",
                                  )}
                                />
                              </TextField>
                            </FieldBox>,
                          );
                        }
                      }
                      return;
                    }
                    const pathKey = String(k);
                    const list = getOrderedCompoundChildren(pathKey);
                    const listShow = filterText
                      ? list.filter((sf) => matchesName(sf.name))
                      : list;
                    if (filterText && !matchesName(k) && listShow.length === 0)
                      return;
                    if (acc.length) {
                      out.push(
                        <div
                          key={`grid-before-${k}`}
                          className="grid grid-cols-[repeat(auto-fit,minmax(18rem,1fr))] gap-4"
                        >
                          {acc.splice(0, acc.length)}
                        </div>,
                      );
                    }
                    out.push(
                      <div key={`c-${k}`} className="mt-3">
                        <div className="flex items-center justify-between">
                          <div className="text-xs text-muted dark:text-zinc-400">
                            {String(k)}
                          </div>
                          <div className="flex items-center gap-2">
                            <Button
                              size="sm"
                              onPress={() => {
                                if (!compoundOpen[pathKey]) {
                                  const hasLocal =
                                    (compoundFields[pathKey] || []).length > 0;
                                  if (hasLocal) {
                                    beforeUpdate();
                                    setCompoundOpen((p) => ({
                                      ...p,
                                      [pathKey]: true,
                                    }));
                                  } else {
                                    loadCompound(pathKey);
                                  }
                                } else {
                                  beforeUpdate();
                                  setCompoundOpen((p) => ({
                                    ...p,
                                    [pathKey]: !p[pathKey],
                                  }));
                                }
                              }}
                              variant={"secondary"}
                              className={cn(
                                "rounded-lg",
                                "bg-surface-secondary dark:bg-surface/20 text-foreground dark:text-zinc-300",
                              )}
                            >
                              {compoundOpen[pathKey]
                                ? t("common.collapse")
                                : t("common.expand")}
                            </Button>
                          </div>
                        </div>
                        <div
                          role="separator"
                          className="h-px bg-surface-tertiary my-2"
                        />
                        {compoundOpen[pathKey] ? (
                          <div className="grid grid-cols-[repeat(auto-fit,minmax(16rem,1fr))] gap-4">
                            {listShow.map((sf, si) => {
                              const stag = normTag(sf.tag);
                              if (stag === "string") {
                                const opts = getEnumOpts(String(sf.name));
                                if (opts) {
                                  return (
                                    <FieldBox
                                      key={`c-${k}-${sf.name}`}
                                      title={String(sf.name)}
                                      type={stag}
                                      delay={si * 0.015}
                                    >
                                      <Select
                                        aria-label={String(sf.name)}
                                        value={
                                          Array.from(
                                            new Set([
                                              String(sf.valueString || "0"),
                                            ]),
                                          )[0] ?? null
                                        }
                                        onChange={(keys: any) => {
                                          const v = keys || "0";
                                          setCompoundFieldValue(pathKey, si, {
                                            valueString: String(v),
                                          });
                                        }}
                                      >
                                        <Select.Trigger
                                          className={cn(
                                            "bg-surface-secondary/50 dark:bg-zinc-800/50 data-[hovered]:bg-surface-tertiary/50 dark:data-[hovered]:bg-zinc-700/50 data-[focused]:border-brand-600 rounded-xl",
                                            "rounded-lg",
                                            "min-h-8 text-sm",
                                          )}
                                        >
                                          <Select.Value />
                                          <Select.Indicator />
                                        </Select.Trigger>
                                        <Select.Popover
                                          className={
                                            "bg-surface-secondary/80 dark:bg-zinc-800/80 border border-border/50 dark:border-white/10"
                                          }
                                        >
                                          <ListBox>
                                            {opts.map((o) => (
                                              <ListBox.Item
                                                key={o.value}
                                                id={o.value}
                                                textValue={o.label}
                                              >
                                                <Label>{o.label}</Label>
                                                <ListBox.ItemIndicator />
                                              </ListBox.Item>
                                            ))}
                                          </ListBox>
                                        </Select.Popover>
                                      </Select>
                                    </FieldBox>
                                  );
                                }
                                const dk = `cf:${k}:${String(sf.name)}`;
                                const display =
                                  typedDrafts[dk] ??
                                  String(sf.valueString || "");
                                return (
                                  <FieldBox
                                    key={`c-${k}-${sf.name}`}
                                    title={String(sf.name)}
                                    type={stag}
                                    delay={si * 0.015}
                                  >
                                    <TextField
                                      aria-label={String(sf.name)}
                                      className={cn(
                                        "group",
                                        COMPONENT_STYLES.input.mainWrapper,
                                      )}
                                      value={display}
                                      onChange={(v) => {
                                        beforeUpdate();
                                        setTypedDrafts((prev) => ({
                                          ...prev,
                                          [dk]: v,
                                        }));
                                      }}
                                    >
                                      <Input

                                        className={cn(
                                          COMPONENT_STYLES.input.inputWrapper,
                                          COMPONENT_STYLES.input.input,
                                          "rounded-lg",
                                          "min-h-8 text-sm",
                                        )}
                                      />
                                    </TextField>
                                  </FieldBox>
                                );
                              }
                              if (stag === "list") {
                                const dk = `cflist:${k}:${String(sf.name)}`;
                                const display =
                                  typedDrafts[dk] ??
                                  String(sf.valueJSON || "[]");
                                const items = parseListJSON(display);
                                return (
                                  <FieldBox
                                    key={`c-${k}-${sf.name}`}
                                    title={String(sf.name)}
                                    type={stag}
                                    delay={si * 0.015}
                                  >
                                    <div className="flex flex-col gap-2">
                                      <div className="flex gap-1 overflow-x-auto flex-nowrap pretty-scrollbar gutter-stable">
                                        {items.map((it, idx) => (
                                          <TextField
                                            key={`c-${k}-li-${idx}`}
                                            aria-label={`${sf.name} item ${idx}`}
                                            className={cn(
                                              "group",
                                              COMPONENT_STYLES.input
                                                .mainWrapper,
                                              "w-12 shrink-0",
                                            )}
                                            value={String(it ?? "")}
                                            onChange={(v) => {
                                              const next = items.slice();
                                              next[idx] = v;
                                              beforeUpdate();
                                              setTypedDrafts((prev) => ({
                                                ...prev,
                                                [dk]: stringifyList(next),
                                              }));
                                            }}
                                          >
                                            <Input

                                              className={cn(
                                                COMPONENT_STYLES.input
                                                  .inputWrapper,
                                                COMPONENT_STYLES.input.input,
                                                "rounded-lg",
                                                "min-h-8 text-sm",
                                              )}
                                            />
                                          </TextField>
                                        ))}
                                      </div>
                                    </div>
                                  </FieldBox>
                                );
                              }
                              if (
                                stag === "byte" ||
                                stag === "short" ||
                                stag === "int" ||
                                stag === "long" ||
                                stag === "float" ||
                                stag === "double"
                              ) {
                                const isBoolLike =
                                  sf.name?.[0] >= "a" &&
                                  sf.name?.[0] <= "z" &&
                                  stag === "byte";
                                if (isBoolLike) {
                                  const isOn =
                                    String(sf.valueString || "0") !== "0";
                                  return (
                                    <FieldBox
                                      key={`c-${k}-${sf.name}`}
                                      title={String(sf.name)}
                                      type={stag}
                                      delay={si * 0.015}
                                    >
                                      <div className="flex justify-end">
                                        <Switch
                                          aria-label={String(sf.name)}
                                          size="sm"
                                          isSelected={isOn}
                                          onChange={(c: boolean) => {
                                            setCompoundFieldValue(pathKey, si, {
                                              valueString: c ? "1" : "0",
                                            });
                                          }}
                                          className={"group"}
                                        >
                                          <Switch.Content>
                                            <Switch.Control>
                                              <Switch.Thumb>
                                                {
                                                  <span className="block w-2 h-2 bg-black rounded-full" />
                                                }
                                              </Switch.Thumb>
                                            </Switch.Control>
                                            <span></span>
                                          </Switch.Content>
                                        </Switch>
                                      </div>
                                    </FieldBox>
                                  );
                                }
                                const dk = `cf:${k}:${String(sf.name)}`;
                                const display =
                                  typedDrafts[dk] ??
                                  String(sf.valueString || "");
                                return (
                                  <FieldBox
                                    key={`c-${k}-${sf.name}`}
                                    title={String(sf.name)}
                                    type={stag}
                                    delay={si * 0.015}
                                  >
                                    <TextField
                                      aria-label={String(sf.name)}
                                      className={cn(
                                        "group",
                                        COMPONENT_STYLES.input.mainWrapper,
                                      )}
                                      value={display}
                                      onChange={(v) => {
                                        beforeUpdate();
                                        setTypedDrafts((prev) => ({
                                          ...prev,
                                          [dk]: v,
                                        }));
                                      }}
                                    >
                                      <Input

                                        className={cn(
                                          COMPONENT_STYLES.input.inputWrapper,
                                          COMPONENT_STYLES.input.input,
                                          "rounded-lg",
                                          "min-h-8 text-sm",
                                        )}
                                      />
                                    </TextField>
                                  </FieldBox>
                                );
                              }
                              return (
                                <FieldBox
                                  key={`c-${k}-${sf.name}`}
                                  title={String(sf.name)}
                                  type={stag}
                                  delay={si * 0.015}
                                >
                                  {(() => {
                                    const dk = `cfjson:${k}:${String(sf.name)}`;
                                    const display =
                                      typedDrafts[dk] ??
                                      String(sf.valueJSON || "");
                                    return (
                                      <>
                                        <TextField
                                          aria-label={String(sf.name)}
                                          className={cn(
                                            "group",
                                            COMPONENT_STYLES.input.mainWrapper,
                                          )}
                                          value={display}
                                          onChange={(v) => {
                                            beforeUpdate();
                                            setTypedDrafts((prev) => ({
                                              ...prev,
                                              [dk]: v,
                                            }));
                                          }}
                                        >
                                          <Input

                                            className={cn(
                                              COMPONENT_STYLES.input
                                                .inputWrapper,
                                              COMPONENT_STYLES.input.input,
                                              "rounded-lg",
                                              "min-h-8 text-sm",
                                            )}
                                          />
                                        </TextField>
                                        <div className="mt-2">
                                          <Button
                                            size="sm"
                                            onPress={() => {
                                              const segs = pathKey.split("/");
                                              const nextPath = [
                                                ...segs,
                                                String(sf.name || ""),
                                              ];
                                              loadCompound(nextPath);
                                            }}
                                            variant={"secondary"}
                                            className={cn(
                                              "rounded-lg",
                                              "bg-surface-secondary dark:bg-surface/20 text-foreground dark:text-zinc-300",
                                            )}
                                          >
                                            {t("common.expand")}
                                          </Button>
                                        </div>
                                      </>
                                    );
                                  })()}
                                </FieldBox>
                              );
                            })}
                          </div>
                        ) : null}
                      </div>,
                    );
                  });
                  if (acc.length) {
                    out.push(
                      <div
                        key={`grid-last`}
                        className="grid grid-cols-[repeat(auto-fit,minmax(18rem,1fr))] gap-4"
                      >
                        {acc}
                      </div>,
                    );
                  }
                  return out;
                })()}
              </div>
            )}
          </div>
        </Card.Content>
      </Card>
      <UnifiedModal
        isOpen={confirmationOpen}
        onOpenChange={(open) => { if (!open && !saving) continueEditing(); }}
        type="warning"
        title={t("contentpage.editor_unsaved")}
        isDismissable={!saving}
        footer={<div className="flex flex-wrap justify-end gap-2">
          <Button variant="ghost" isDisabled={saving} onPress={continueEditing}>{t("contentpage.editor_continue")}</Button>
          <Button variant="danger-soft" isDisabled={saving} onPress={continueAction}>{t("contentpage.editor_discard")}</Button>
          <Button variant="primary" isPending={saving} onPress={async () => {
            if (await saveAll()) continueAction();
          }}>{t(refreshPending ? "contentpage.editor_save_refresh" : "contentpage.editor_save_leave")}</Button>
        </div>}
      >
        <p>{t("contentpage.editor_unsaved_body")}</p>
        {error && <p role="alert" className="mt-3 text-danger">{t(error)}</p>}
      </UnifiedModal>
    </PageContainer>
  );
}
