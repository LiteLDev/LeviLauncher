import React from "react";
import { useTranslation } from "react-i18next";
import {
  Button,
  Card,
  CardBody,
  Input,
  Spinner,
  Switch,
  Select,
  SelectItem,
  Chip,
  Tooltip,
} from "@heroui/react";
import { motion, AnimatePresence } from "framer-motion";
import { useLocation, useNavigate } from "react-router-dom";
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

  const handleSave = async () => {
    const ok = await saveAll();
    if (ok) navigate(-1);
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
          className="group relative overflow-hidden rounded-2xl border border-default-200 dark:border-default-100/10 bg-white/50 dark:bg-zinc-900/50 p-4 transition-all hover:bg-default-100 dark:hover:bg-zinc-800/50 hover:shadow-lg"
        >
          <div className="flex items-center justify-between mb-3">
            <div
              className="text-sm font-semibold text-default-700 dark:text-zinc-200 truncate"
              title={title}
            >
              {title}
            </div>
            <Chip
              size="sm"
              variant="flat"
              color={chipColor(type)}
              className="h-6 min-w-12 justify-center font-mono text-xs uppercase"
            >
              {type}
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
        <CardBody className="p-0 flex flex-col h-full overflow-hidden">
          <div className="shrink-0 p-6 flex flex-col gap-6 border-b border-default-200 dark:border-white/10">
            <PageHeader
              title={t("contentpage.world_leveldat_editor")}
              startContent={
                <Button
                  isIconOnly
                  radius="full"
                  variant="light"
                  onPress={() => navigate(-1)}
                >
                  <FaArrowLeft size={20} />
                </Button>
              }
              endContent={
                <>
                  <Input
                    size="sm"
                    radius="full"
                    variant="flat"
                    placeholder={t("common.search") as string}
                    value={filterText}
                    onValueChange={(v) => {
                      beforeUpdate();
                      setFilterText(v);
                    }}
                    isClearable
                    startContent={<FaSearch className="text-default-400" />}
                    className="w-48 sm:w-64"
                    classNames={COMPONENT_STYLES.input}
                  />
                  <Tooltip content={t("common.refresh")}>
                    <Button
                      isIconOnly
                      radius="full"
                      variant="flat"
                      onPress={load}
                      isLoading={loading}
                      className="bg-default-100 dark:bg-default-50/20 text-default-600 dark:text-zinc-300"
                    >
                      <FaSync className={loading ? "animate-spin" : ""} />
                    </Button>
                  </Tooltip>
                  <Tooltip content={t("common.save")}>
                    <Button
                      isIconOnly
                      radius="full"
                      color="primary"
                      onPress={handleSave}
                      isLoading={saving}
                      isDisabled={!hasBackend || loading}
                      className="bg-linear-to-r from-primary-500 to-primary-400 text-white shadow-lg shadow-primary-900/20"
                    >
                      <FaSave className="w-4 h-4" />
                    </Button>
                  </Tooltip>
                </>
              }
            />

            {error && (
              <div className="w-full p-4 rounded-2xl bg-danger-50 dark:bg-danger-900/20 border border-danger-200 dark:border-danger-800/50 text-danger flex items-center gap-2">
                <FaTimes className="w-4 h-4" />
                <span>{t(error)}</span>
              </div>
            )}
          </div>
          <div
            ref={scrollRef}
            className="flex-1 overflow-y-auto pretty-scrollbar p-4 sm:p-6 pt-0"
          >
            {loading ? (
              <div className="flex flex-col items-center justify-center h-64 gap-4">
                <Spinner size="lg" color="primary" />
                <div className="text-default-400 animate-pulse">
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
                  className="p-6 rounded-2xl bg-white/50 dark:bg-zinc-900/50 border border-default-200 dark:border-default-100/10 backdrop-blur-md shadow-sm"
                >
                  <div className="flex items-center gap-4 mb-6">
                    <div className="w-1 h-6 rounded-full bg-linear-to-b from-primary-500 to-primary-400 shadow-[0_0_10px_rgba(16,185,129,0.4)]" />
                    <h3 className="text-lg font-bold text-default-700 dark:text-zinc-200">
                      {t("contentpage.basic_info")}
                    </h3>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <Input
                      label={t("contentpage.level_name")}
                      labelPlacement="outside"
                      placeholder="My World"
                      value={levelName}
                      onValueChange={(v) => {
                        beforeUpdate();
                        setLevelName(v);
                      }}
                      variant="flat"
                      radius="lg"
                      classNames={COMPONENT_STYLES.input}
                    />
                    <div className="flex flex-col gap-2">
                      <label className="text-sm text-default-600 dark:text-zinc-400">
                        {t("contentpage.version")}
                      </label>
                      <div className="h-10 px-3 flex items-center rounded-lg bg-default-100 dark:bg-zinc-800/50 text-default-500 dark:text-zinc-400 text-sm font-mono border border-transparent dark:border-zinc-700/50">
                        {typedVersion}
                      </div>
                    </div>
                  </div>
                </motion.div>
                <div role="separator" className="h-px bg-default-200/50 my-2" />
                {/* Add Field Section */}
                <div className="flex flex-col gap-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <div className="w-1 h-6 rounded-full bg-linear-to-b from-primary-500 to-primary-400 shadow-[0_0_10px_rgba(16,185,129,0.4)]" />
                      <h3 className="text-lg font-bold text-default-700 dark:text-zinc-200">
                        {t("contentpage.add_field")}
                      </h3>
                    </div>
                    <Button
                      size="sm"
                      radius="full"
                      variant="flat"
                      className="bg-default-100 dark:bg-default-50/20 text-default-600 dark:text-zinc-300"
                      onPress={() => setAddOpen((o) => !o)}
                      startContent={addOpen ? <FaTimes /> : <FaPlus />}
                    >
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
                        <div className="p-6 rounded-2xl bg-white/50 dark:bg-zinc-900/50 border border-default-200 dark:border-default-100/10 backdrop-blur-md shadow-sm transition-all">
                          <div className="grid grid-cols-1 sm:grid-cols-[140px_1fr_100px_1fr_80px] gap-4 items-end">
                            <Select
                              label="Target"
                              labelPlacement="outside"
                              size="sm"
                              radius="lg"
                              variant="flat"
                              selectedKeys={new Set([addTargetKey])}
                              onSelectionChange={(keys: any) => {
                                const v = String(Array.from(keys)[0] || "root");
                                setAddTargetKey(v);
                              }}
                              classNames={COMPONENT_STYLES.select}
                            >
                              {compoundTargetKeys.map((o) => (
                                <SelectItem key={o}>{o}</SelectItem>
                              ))}
                            </Select>
                            <Input
                              label="Name"
                              labelPlacement="outside"
                              size="sm"
                              radius="lg"
                              variant="flat"
                              placeholder={
                                t("contentpage.field_name") as string
                              }
                              value={newUnifiedField.name}
                              onValueChange={(v) =>
                                setNewUnifiedField((prev) => ({
                                  ...prev,
                                  name: v,
                                }))
                              }
                              classNames={COMPONENT_STYLES.input}
                            />
                            <Select
                              label="Type"
                              labelPlacement="outside"
                              size="sm"
                              radius="lg"
                              variant="flat"
                              selectedKeys={new Set([newUnifiedField.tag])}
                              onSelectionChange={(keys: any) => {
                                const v = String(
                                  Array.from(keys)[0] || "string",
                                );
                                setNewUnifiedField((prev) => ({
                                  ...prev,
                                  tag: v,
                                }));
                              }}
                              classNames={COMPONENT_STYLES.select}
                            >
                              {TAG_OPTIONS.map((o) => (
                                <SelectItem key={o}>{o}</SelectItem>
                              ))}
                            </Select>
                            <Input
                              label="Value"
                              labelPlacement="outside"
                              size="sm"
                              radius="lg"
                              variant="flat"
                              placeholder={
                                t("contentpage.initial_value") as string
                              }
                              value={newUnifiedField.value}
                              onValueChange={(v) =>
                                setNewUnifiedField((prev) => ({
                                  ...prev,
                                  value: v,
                                }))
                              }
                              classNames={COMPONENT_STYLES.input}
                            />
                            <Button
                              size="sm"
                              radius="lg"
                              className="bg-linear-to-r from-primary-500 to-primary-400 text-white shadow-lg shadow-primary-900/20"
                              onPress={addField}
                            >
                              {t("common.add")}
                            </Button>
                          </div>
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
                <div role="separator" className="h-px bg-default-200/50 my-2" />
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
                                  <Input
                                    key={`tf-${k}-li-${idx}`}
                                    aria-label={`${k} item ${idx}`}
                                    size="sm"
                                    variant="flat"
                                    radius="lg"
                                    classNames={COMPONENT_STYLES.input}
                                    className="w-12 shrink-0"
                                    value={String(it ?? "")}
                                    onValueChange={(v) => {
                                      const next = items.slice();
                                      next[idx] = v;
                                      beforeUpdate();
                                      setTypedDrafts((prev) => ({
                                        ...prev,
                                        [dk]: stringifyList(next),
                                      }));
                                    }}
                                    onBlur={() => {
                                      const val = String(
                                        typedDrafts[dk] ?? stringifyList(items),
                                      );
                                      setTypedFieldValueByName(String(k), {
                                        valueJSON: val,
                                      });
                                      setTypedDrafts((prev) => {
                                        const nn = { ...prev };
                                        delete nn[dk];
                                        return nn;
                                      });
                                    }}
                                  />
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
                                size="sm"
                                color="success"
                                isSelected={isOn}
                                onValueChange={(c: boolean) => {
                                  setTypedFieldValueByName(String(k), {
                                    valueString: c ? "1" : "0",
                                  });
                                }}
                                thumbIcon={
                                  <span className="block w-2 h-2 bg-black rounded-full" />
                                }
                              />
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
                                size="sm"
                                radius="lg"
                                variant="flat"
                                classNames={{
                                  trigger:
                                    "bg-default-100 dark:bg-default-50/20",
                                }}
                                selectedKeys={
                                  new Set([
                                    String((f as any).valueString || "0"),
                                  ])
                                }
                                onSelectionChange={(keys: any) => {
                                  const v = Array.from(keys)[0] || "0";
                                  setTypedFieldValueByName(String(k), {
                                    valueString: String(v),
                                  });
                                }}
                              >
                                {opts.map((o) => (
                                  <SelectItem key={o.value}>
                                    {o.label}
                                  </SelectItem>
                                ))}
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
                              <Input
                                aria-label={String(k)}
                                size="sm"
                                variant="flat"
                                radius="lg"
                                classNames={COMPONENT_STYLES.input}
                                value={display}
                                onValueChange={(v) => {
                                  beforeUpdate();
                                  setTypedDrafts((prev) => ({
                                    ...prev,
                                    [dk]: v,
                                  }));
                                }}
                                onBlur={() => {
                                  const val = String(typedDrafts[dk] ?? "");
                                  setTypedFieldValueByName(String(k), {
                                    valueString: val,
                                  });
                                  setTypedDrafts((prev) => {
                                    const next = { ...prev };
                                    delete next[dk];
                                    return next;
                                  });
                                }}
                              />
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
                          <div className="text-xs text-default-500 dark:text-zinc-400">
                            {String(k)}
                          </div>
                          <div className="flex items-center gap-2">
                            <Button
                              size="sm"
                              radius="lg"
                              variant="flat"
                              className="bg-default-100 dark:bg-default-50/20 text-default-600 dark:text-zinc-300"
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
                            >
                              {compoundOpen[pathKey]
                                ? t("common.collapse")
                                : t("common.expand")}
                            </Button>
                          </div>
                        </div>
                        <div
                          role="separator"
                          className="h-px bg-default-200 my-2"
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
                                        size="sm"
                                        radius="lg"
                                        variant="flat"
                                        classNames={{
                                          trigger:
                                            "bg-default-100/50 dark:bg-zinc-800/50 data-[hover=true]:bg-default-200/50 dark:data-[hover=true]:bg-zinc-700/50 data-[focus=true]:border-primary-600 rounded-xl",
                                          popoverContent:
                                            "bg-default-100/80 dark:bg-zinc-800/80 border border-default-200/50 dark:border-white/10",
                                        }}
                                        selectedKeys={
                                          new Set([
                                            String(sf.valueString || "0"),
                                          ])
                                        }
                                        onSelectionChange={(keys: any) => {
                                          const v = Array.from(keys)[0] || "0";
                                          setCompoundFieldValue(pathKey, si, {
                                            valueString: String(v),
                                          });
                                        }}
                                      >
                                        {opts.map((o) => (
                                          <SelectItem key={o.value}>
                                            {o.label}
                                          </SelectItem>
                                        ))}
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
                                    <Input
                                      aria-label={String(sf.name)}
                                      size="sm"
                                      variant="flat"
                                      radius="lg"
                                      classNames={COMPONENT_STYLES.input}
                                      value={display}
                                      onValueChange={(v) => {
                                        beforeUpdate();
                                        setTypedDrafts((prev) => ({
                                          ...prev,
                                          [dk]: v,
                                        }));
                                      }}
                                      onBlur={() => {
                                        const val = String(
                                          typedDrafts[dk] ?? "",
                                        );
                                        setCompoundFieldValue(pathKey, si, {
                                          valueString: val,
                                        });
                                        setTypedDrafts((prev) => {
                                          const next = { ...prev };
                                          delete next[dk];
                                          return next;
                                        });
                                      }}
                                    />
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
                                          <Input
                                            key={`c-${k}-li-${idx}`}
                                            aria-label={`${sf.name} item ${idx}`}
                                            size="sm"
                                            variant="flat"
                                            radius="lg"
                                            classNames={COMPONENT_STYLES.input}
                                            className="w-12 shrink-0"
                                            value={String(it ?? "")}
                                            onValueChange={(v) => {
                                              const next = items.slice();
                                              next[idx] = v;
                                              beforeUpdate();
                                              setTypedDrafts((prev) => ({
                                                ...prev,
                                                [dk]: stringifyList(next),
                                              }));
                                            }}
                                            onBlur={() => {
                                              const val = String(
                                                typedDrafts[dk] ??
                                                  stringifyList(items),
                                              );
                                              setCompoundFieldValue(
                                                pathKey,
                                                si,
                                                { valueJSON: val },
                                              );
                                              setTypedDrafts((prev) => {
                                                const nn = { ...prev };
                                                delete nn[dk];
                                                return nn;
                                              });
                                            }}
                                          />
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
                                          size="sm"
                                          color="success"
                                          isSelected={isOn}
                                          onValueChange={(c: boolean) => {
                                            setCompoundFieldValue(pathKey, si, {
                                              valueString: c ? "1" : "0",
                                            });
                                          }}
                                          thumbIcon={
                                            <span className="block w-2 h-2 bg-black rounded-full" />
                                          }
                                        />
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
                                    <Input
                                      aria-label={String(sf.name)}
                                      size="sm"
                                      variant="flat"
                                      radius="lg"
                                      classNames={COMPONENT_STYLES.input}
                                      value={display}
                                      onValueChange={(v) => {
                                        beforeUpdate();
                                        setTypedDrafts((prev) => ({
                                          ...prev,
                                          [dk]: v,
                                        }));
                                      }}
                                      onBlur={() => {
                                        const val = String(
                                          typedDrafts[dk] ?? "",
                                        );
                                        setCompoundFieldValue(pathKey, si, {
                                          valueString: val,
                                        });
                                        setTypedDrafts((prev) => {
                                          const next = { ...prev };
                                          delete next[dk];
                                          return next;
                                        });
                                      }}
                                    />
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
                                        <Input
                                          aria-label={String(sf.name)}
                                          size="sm"
                                          variant="flat"
                                          radius="lg"
                                          classNames={COMPONENT_STYLES.input}
                                          value={display}
                                          onValueChange={(v) => {
                                            beforeUpdate();
                                            setTypedDrafts((prev) => ({
                                              ...prev,
                                              [dk]: v,
                                            }));
                                          }}
                                          onBlur={() => {
                                            const val = String(
                                              typedDrafts[dk] ?? "",
                                            );
                                            setCompoundFieldValue(pathKey, si, {
                                              valueJSON: val,
                                            });
                                            setTypedDrafts((prev) => {
                                              const next = { ...prev };
                                              delete next[dk];
                                              return next;
                                            });
                                          }}
                                        />
                                        <div className="mt-2">
                                          <Button
                                            size="sm"
                                            radius="lg"
                                            variant="flat"
                                            className="bg-default-100 dark:bg-default-50/20 text-default-600 dark:text-zinc-300"
                                            onPress={() => {
                                              const segs = pathKey.split("/");
                                              const nextPath = [
                                                ...segs,
                                                String(sf.name || ""),
                                              ];
                                              loadCompound(nextPath);
                                            }}
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
        </CardBody>
      </Card>
    </PageContainer>
  );
}
