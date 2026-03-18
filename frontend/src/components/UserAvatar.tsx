import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Avatar,
  Popover,
  PopoverTrigger,
  PopoverContent,
  User,
  Button,
  Chip,
  Tooltip,
} from "@heroui/react";
import {
  FaSync,
  FaXbox,
  FaClock,
  FaCube,
  FaSkull,
  FaRoad,
} from "react-icons/fa";
import * as userService from "bindings/github.com/liteldev/LeviLauncher/userservice";
import { useStartupInteractive } from "@/utils/startupState";

export const UserAvatar = () => {
  const { t } = useTranslation();
  const startupInteractive = useStartupInteractive();
  const [gamertag, setGamertag] = useState("");
  const [xuid, setXuid] = useState("");
  const [avatar, setAvatar] = useState("");
  const [stats, setStats] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [reloadNonce, setReloadNonce] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  const [open, setOpen] = useState(false);

  const clearUserState = React.useCallback(() => {
    setGamertag("");
    setXuid("");
    setAvatar("");
    setStats(null);
  }, []);

  const refreshSessionIfNeeded = React.useCallback(async (force = false) => {
    try {
      const getState = (userService as any)?.XUserGetState;
      const reset = (userService as any)?.ResetSession;

      if (typeof reset !== "function") {
        return false;
      }

      let shouldReset = force;
      if (!shouldReset && typeof getState === "function") {
        const state = await getState();
        shouldReset = typeof state === "number" && state !== 0;
      }

      if (!shouldReset) {
        return false;
      }

      const result = await reset();
      if (result) {
        console.error("[UserAvatar] ResetSession error", result);
      }
      return true;
    } catch (e) {
      console.error("[UserAvatar] refreshSessionIfNeeded error", e);
      return false;
    }
  }, []);

  useEffect(() => {
    if (!startupInteractive) return;
    let cancelled = false;

    const fetchUser = async () => {
      try {
        if (!userService.GetLocalUserId) {
          return;
        }

        await refreshSessionIfNeeded();

        const id = await userService.GetLocalUserId();
        if (cancelled) return;
        if (!id) {
          clearUserState();
          return;
        }

        const [tag, pic] = await Promise.all([
          userService.GetLocalUserGamertag(),
          userService.GetLocalUserGamerPicture(1).catch((err) => {
            console.error("[UserAvatar] GetLocalUserGamerPicture error", err);
            return "";
          }),
        ]);
        if (cancelled) return;

        setXuid(id);
        setGamertag(String(tag || ""));
        setAvatar(pic ? `data:image/png;base64,${pic}` : "");
        setStats(null);
      } catch (e) {
        console.error("[UserAvatar] fetchUser error", e);
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void fetchUser();
    return () => {
      cancelled = true;
    };
  }, [clearUserState, refreshSessionIfNeeded, reloadNonce, startupInteractive]);

  useEffect(() => {
    if (!startupInteractive) return;
    if (!open) return;
    if (!xuid) return;

    let cancelled = false;

    const fetchDetails = async () => {
      try {
        const getStats = (userService as any)?.GetAggregatedUserStatistics;
        if (typeof getStats === "function") {
          const nextStats = await getStats(xuid);
          if (!cancelled && nextStats) {
            setStats(nextStats);
          }
        }
      } catch {}

      try {
        const pic = await userService.GetLocalUserGamerPicture(1);
        if (!cancelled && pic) {
          setAvatar(`data:image/png;base64,${pic}`);
        }
      } catch (err) {
        console.error("[UserAvatar] GetLocalUserGamerPicture error", err);
      }
    };

    void fetchDetails();
    return () => {
      cancelled = true;
    };
  }, [open, reloadNonce, startupInteractive, xuid]);

  if (!startupInteractive || loading) {
    return (
      <div className="flex items-center gap-2">
        <Avatar
          size="sm"
          isBordered
          className="ring-2 ring-default-300/60 dark:ring-zinc-700/60 bg-default-100 dark:bg-zinc-800 text-default-400 dark:text-zinc-500"
        />
      </div>
    );
  }

  if (!gamertag) {
    return (
      <div className="flex items-center gap-2">
        <Tooltip content={t("useravatar.no_login_retry")}>
          <Button
            isIconOnly
            variant="light"
            size="sm"
            onPress={() => {
              setLoading(true);
              clearUserState();
              setReloadNonce((v) => v + 1);
            }}
          >
            <FaXbox className="text-default-400" size={24} />
          </Button>
        </Tooltip>
      </div>
    );
  }

  const formatPlayTime = (totalMinutes: number) => {
    if (!totalMinutes && totalMinutes !== 0) return "0m";
    const days = Math.floor(totalMinutes / (24 * 60));
    const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
    const minutes = Math.floor(totalMinutes % 60);

    return t("useravatar.stats.time_format", {
      days,
      hours,
      minutes,
    });
  };

  return (
    <Popover
      placement="bottom-end"
      showArrow
      backdrop="transparent"
      isOpen={open}
      onOpenChange={async (nextOpen: boolean) => {
        setOpen(nextOpen);
        if (!nextOpen) return;
        try {
          setRefreshing(true);
          const refreshed = await refreshSessionIfNeeded();
          if (refreshed) {
            setReloadNonce((v) => v + 1);
          }
        } catch (e) {
          console.error("[UserAvatar] XUserGetState error", e);
        } finally {
          setRefreshing(false);
        }
      }}
    >
      <PopoverTrigger>
        <div className="flex items-center gap-2 cursor-pointer transition-transform hover:scale-105 active:scale-95">
          <Avatar
            src={avatar}
            name={gamertag}
            size="sm"
            isBordered
            color="primary"
            className="ring-2 ring-primary-500/30"
          />
        </div>
      </PopoverTrigger>
      <PopoverContent className="p-1 bg-white dark:bg-zinc-900 border border-default-200/70 dark:border-zinc-700/60 shadow-2xl rounded-2xl">
        <div className="px-4 py-3 w-64">
          <div className="flex items-center justify-between mb-3">
            <Chip
              startContent={<FaXbox className="text-primary-600" />}
              variant="flat"
              color="primary"
              size="sm"
              className="bg-primary-100/70 dark:bg-primary-900/25 text-primary-700 dark:text-primary-400"
            >
              {t("useravatar.xbox_live")}
            </Chip>
            <Button
              isIconOnly
              variant="light"
              size="sm"
              aria-label={t("useravatar.refresh_session_aria")}
              isLoading={refreshing}
              onPress={async () => {
                setRefreshing(true);
                await refreshSessionIfNeeded(true);
                setReloadNonce((v) => v + 1);
                setRefreshing(false);
              }}
            >
              <FaSync size={14} className={refreshing ? "animate-spin" : ""} />
            </Button>
          </div>

          <User
            name={
              <span className="font-bold text-lg bg-gradient-to-r from-primary-600 to-primary-400 bg-clip-text text-transparent">
                {gamertag}
              </span>
            }
            description={
              <div className="flex flex-col gap-1">
                <span className="text-xs text-default-500 dark:text-zinc-400">
                  {t("useravatar.xuid", {
                    xuid,
                  })}
                </span>
              </div>
            }
            avatarProps={{
              src: avatar,
              size: "lg",
              isBordered: true,
              className: "w-14 h-14 bg-transparent ring-2 ring-primary-500",
            }}
            classNames={{
              base: "justify-start gap-4",
              name: "text-lg",
            }}
          />

          {stats && (
            <div className="mt-4 pt-3 border-t border-default-100 dark:border-white/10">
              <div className="flex flex-col gap-4">
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-blue-100/50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400">
                    <FaClock size={12} />
                  </div>
                  <div className="flex flex-col">
                    <span className="text-[10px] text-default-400 tracking-wider font-semibold whitespace-nowrap">
                      {t("useravatar.stats.minutes_played")}
                    </span>
                    <span className="text-xs font-bold text-default-700 dark:text-zinc-200">
                      {formatPlayTime(stats.minutesPlayed)}
                    </span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-orange-100/50 dark:bg-orange-900/20 text-orange-600 dark:text-orange-400">
                    <FaCube size={12} />
                  </div>
                  <div className="flex flex-col">
                    <span className="text-[10px] text-default-400 tracking-wider font-semibold whitespace-nowrap">
                      {t("useravatar.stats.blocks_broken")}
                    </span>
                    <span className="text-xs font-bold text-default-700 dark:text-zinc-200">
                      {stats.blockBroken?.toLocaleString()}
                    </span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-red-100/50 dark:bg-red-900/20 text-red-600 dark:text-red-400">
                    <FaSkull size={12} />
                  </div>
                  <div className="flex flex-col">
                    <span className="text-[10px] text-default-400 tracking-wider font-semibold whitespace-nowrap">
                      {t("useravatar.stats.mobs_defeated")}
                    </span>
                    <span className="text-xs font-bold text-default-700 dark:text-zinc-200">
                      {stats.mobKilled?.toLocaleString()}
                    </span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <div className="p-1.5 rounded-lg bg-purple-100/50 dark:bg-purple-900/20 text-purple-600 dark:text-purple-400">
                    <FaRoad size={12} />
                  </div>
                  <div className="flex flex-col">
                    <span className="text-[10px] text-default-400 tracking-wider font-semibold whitespace-nowrap">
                      {t("useravatar.stats.distance_travelled")}
                    </span>
                    <span className="text-xs font-bold text-default-700 dark:text-zinc-200">
                      {(stats.distanceTravelled / 1000).toFixed(1)} km
                    </span>
                  </div>
                </div>
              </div>
            </div>
          )}

          <div className="mt-3 pt-3 border-t border-default-100 flex justify-end" />
        </div>
      </PopoverContent>
    </Popover>
  );
};
