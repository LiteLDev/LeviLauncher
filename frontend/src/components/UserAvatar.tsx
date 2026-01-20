import React, { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { 
    Avatar, 
    Skeleton,
    Popover,
    PopoverTrigger,
    PopoverContent,
    User,
    Button,
    Chip,
    Tooltip,
} from "@heroui/react";
import { FaSync, FaXbox, FaClock, FaCube, FaSkull, FaRoad } from "react-icons/fa";
import * as minecraft from "bindings/github.com/liteldev/LeviLauncher/minecraft";

export const UserAvatar = () => {
    const { t, i18n } = useTranslation();
    const [gamertag, setGamertag] = useState("");
    const [xuid, setXuid] = useState("");
    const [avatar, setAvatar] = useState("");
    const [stats, setStats] = useState<any>(null);
    const [loading, setLoading] = useState(true);
    const [reloadNonce, setReloadNonce] = useState(0);
    const [refreshing, setRefreshing] = useState(false);
    const [open, setOpen] = useState(false);

    useEffect(() => {
        const fetchUser = async () => {
            try {
                if (!minecraft.GetLocalUserId) {
                    setLoading(false);
                    return;
                }

                const id = await minecraft.GetLocalUserId();
                if (!id) {
                    setGamertag("");
                    setXuid("");
                    setAvatar("");
                    return;
                }

                setXuid(id);
                const tag = await minecraft.GetLocalUserGamertag();
                if (!tag) {
                    setGamertag("");
                    return;
                }
                setGamertag(tag);
                
                try {
                    const getStats = (minecraft as any)?.GetAggregatedUserStatistics;
                    if (typeof getStats === "function") {
                        getStats(id).then((s: any) => {
                            if (s) setStats(s);
                        });
                    }
                } catch {}

                    void minecraft
                        .GetLocalUserGamerPicture(1)
                        .then((pic: any) => {
                            if (pic) setAvatar(`data:image/png;base64,${pic}`);
                        })
                        .catch((err: any) => {
                            console.error("[UserAvatar] GetLocalUserGamerPicture error", err);
                        });
            } catch (e) {
                console.error("[UserAvatar] fetchUser error", e);
            } finally {
                setLoading(false);
            }
        };
        fetchUser();
    }, [reloadNonce]);

    if (loading) {
        return <Skeleton className="flex rounded-full w-8 h-8 mr-2" />;
    }

    if (!gamertag) {
        return (
             <div className="flex items-center gap-2 mr-2">
                <Tooltip
                    content={t("useravatar.no_login_retry", {
                        defaultValue: "未检测到 Xbox 登录（点击重试）",
                    })}
                >
                    <Button 
                        isIconOnly 
                        variant="light" 
                        size="sm" 
                        onPress={() => {
                            setLoading(true);
                            setGamertag("");
                            setXuid("");
                            setAvatar("");
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
            defaultValue: `${days}d ${hours}h ${minutes}m`
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
                    const getState = (minecraft as any)?.XUserGetState;
                    if (typeof getState === "function") {
                        const state = await getState();
                        console.log("[UserAvatar] XUserGetState =>", state);
                        if (typeof state === "number" && state !== 0) {
                            setRefreshing(true);
                            try {
                                const reset = (minecraft as any)?.ResetSession;
                                if (typeof reset === "function") await reset();
                            } catch {}
                            setReloadNonce((v) => v + 1);
                            setRefreshing(false);
                        }
                    }
                } catch (e) {
                    console.error("[UserAvatar] XUserGetState error", e);
                }
            }}
        >
            <PopoverTrigger>
                <div className="flex items-center gap-2 mr-2 cursor-pointer transition-transform hover:scale-105 active:scale-95">
                    <Avatar
                        src={avatar}
                        name={gamertag}
                        size="sm"
                        isBordered
                        color="success"
                        className="ring-2 ring-emerald-500/30"
                    />
                </div>
            </PopoverTrigger>
            <PopoverContent className="p-1 bg-white dark:bg-zinc-900 border border-default-200/70 dark:border-zinc-700/60 shadow-2xl rounded-2xl">
                <div className="px-4 py-3 w-64">
                    <div className="flex items-center justify-between mb-3">
                        <Chip 
                            startContent={<FaXbox className="text-emerald-600" />} 
                            variant="flat" 
                            color="success" 
                            size="sm"
                            className="bg-emerald-100/70 dark:bg-emerald-900/25 text-emerald-700 dark:text-emerald-400"
                        >
                            {t("useravatar.xbox_live", { defaultValue: "Xbox Live" })}
                        </Chip>
                        <Button
                            isIconOnly
                            variant="light"
                            size="sm"
                            aria-label={t("useravatar.refresh_session_aria", {
                                defaultValue: "Refresh Xbox Session",
                            })}
                            isLoading={refreshing}
                            onPress={async () => {
                                setRefreshing(true);
                                try {
                                    const reset = (minecraft as any)?.ResetSession;
                                    if (typeof reset === "function") await reset();
                                } catch {}
                                setReloadNonce((v) => v + 1);
                                setRefreshing(false);
                            }}
                        >
                            <FaSync
                                size={14}
                                className={refreshing ? "animate-spin" : ""}
                            />
                        </Button>
                    </div>
                    
                    <User   
                        name={
                            <span className="font-bold text-lg bg-gradient-to-r from-emerald-600 to-teal-600 bg-clip-text text-transparent">
                                {gamertag}
                            </span>
                        }
                        description={
                            <div className="flex flex-col gap-1">
                                <span className="text-xs text-default-500">
                                    {t("useravatar.xuid", {
                                        defaultValue: "XUID: {{xuid}}",
                                        xuid,
                                    })}
                                </span>
                            </div>
                        }
                        avatarProps={{
                            src: avatar,
                            size: "lg",
                            isBordered: true,
                            color: "success",
                            className: "w-14 h-14"
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
                                            {t("useravatar.stats.minutes_played", { defaultValue: "游戏时间" })}
                                        </span>
                                        <span className="text-xs font-bold text-default-700 dark:text-default-300">
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
                                            {t("useravatar.stats.blocks_broken", { defaultValue: "打破的方块" })}
                                        </span>
                                        <span className="text-xs font-bold text-default-700 dark:text-default-300">
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
                                            {t("useravatar.stats.mobs_defeated", { defaultValue: "已击败的生物" })}
                                        </span>
                                        <span className="text-xs font-bold text-default-700 dark:text-default-300">
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
                                            {t("useravatar.stats.distance_travelled", { defaultValue: "行驶距离" })}
                                        </span>
                                        <span className="text-xs font-bold text-default-700 dark:text-default-300">
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
