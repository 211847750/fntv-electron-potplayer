import { dialog, IpcMainEvent } from 'electron';
import * as ply from '../../../modules/players';
import * as fn from '../../../modules/fn_api/api';
import * as fnConfig from '../../../modules/fn_config/config';
import { registerHandler } from '../core/ipcHandler';
import { registerAppHook } from '../core/appHook';
import * as log from '../../../modules/logger';
import * as os from 'os';
import * as fs from 'fs';
import { ItemListRequest } from '../../../modules/fn_api/types';
import { isTrusted } from '../../../modules/cert_trust';
import { checkLibraryPageUrl } from '../../common/utils';
import { getMainWindow } from '../../common/mainwin';
import { getProxySecret } from '../../common/proxy';
import { createProxyPlaybackUrl, registerPlaybackSession } from '../../common/proxySession';
import { getAccessCookieHeader } from '../../../modules/fn_api/accessGrant';

/**
* 媒体播放插件
* 处理视频播放相关功能
*/
interface PlayRequest {
    id: string;
    token: string;
    sourceIndex: number; // 可选，播放源
    routeType?: 'item' | 'season' | 'series' | 'live';
}

// 全局播放器实例引用
let currentPlayer: ply.BasePlayer | null = null;

// MPV播放器路径缓存
let cachedPlayerPath: string | null = null;
let cachedPotPlayerPath: string | null = null;

// 设置MPV播放器路径（用于覆盖默认路径）
export function setMpvPlayerPath(path: string | null): void {
    cachedPlayerPath = path;
}

// 设置PotPlayer播放器路径（用于覆盖默认路径）
export function setPotPlayerPath(path: string | null): void {
    cachedPotPlayerPath = path;
}

/**
 * 获取MPV播放器路径（带缓存）
 * @returns 播放器路径或undefined
 */
function getMpvPlayerPath(): string | undefined {
    // 如果已经缓存了路径，直接返回
    if (cachedPlayerPath) {
        return cachedPlayerPath;
    }

    const platform = os.platform();

    if (platform === 'win32') {
        // Windows 平台使用本地文件路径
        cachedPlayerPath = 'third_party\\fntv-mpv\\mpv.exe';
        return cachedPlayerPath;
    } else if (platform === 'darwin') {
        // macOS 常用安装路径
        const macPaths = [
            '/opt/homebrew/bin/mpv',  // Apple Silicon Mac (M1/M2)
            '/usr/local/bin/mpv',     // Intel Mac 或手动安装
            '/Applications/mpv.app/Contents/MacOS/mpv', // App bundle
        ];

        for (const path of macPaths) {
            if (fs.existsSync(path)) {
                cachedPlayerPath = path;
                log.info(`找到MPV播放器路径: ${path}`);
                return cachedPlayerPath;
            }
        }

        // 未找到mpv播放器
        dialog.showErrorBox('错误', 'macOS平台未找到mpv播放器，请使用Homebrew安装mpv后重试: brew install mpv');
        log.error('macOS平台未找到mpv播放器，请使用Homebrew安装mpv后重试: brew install mpv');
        return undefined;
    } else if (platform === 'linux') {
        // Linux 常用安装路径
        const linuxPaths = [
            '/usr/bin/mpv',           // 系统包管理器安装
            '/usr/local/bin/mpv',     // 手动编译安装
            '/snap/bin/mpv',          // Snap 包
            '/usr/games/mpv',         // 某些发行版
            '/opt/mpv/bin/mpv',       // 可选安装位置
        ];

        for (const path of linuxPaths) {
            if (fs.existsSync(path)) {
                cachedPlayerPath = path;
                log.info(`找到MPV播放器路径: ${path}`);
                return cachedPlayerPath;
            }
        }

        // 未找到mpv播放器
        dialog.showErrorBox('错误', 'Linux平台未找到mpv播放器，请安装mpv播放器后重试');
        log.error('Linux平台未找到mpv播放器，请安装mpv播放器后重试');
        return undefined;
    }

    return undefined;
}

function getPotPlayerPath(): string | undefined {
    if (cachedPotPlayerPath) {
        return cachedPotPlayerPath;
    }

    if (process.platform !== 'win32') {
        dialog.showErrorBox('错误', 'PotPlayer播放器仅支持Windows平台');
        log.error('PotPlayer播放器仅支持Windows平台');
        return undefined;
    }

    const configuredPath = fnConfig.getPotPlayerPath();
    if (configuredPath && fs.existsSync(configuredPath)) {
        cachedPotPlayerPath = configuredPath;
        return cachedPotPlayerPath;
    }

    const potPlayerPaths = [
        'C:\\Program Files\\DAUM\\PotPlayer\\PotPlayerMini64.exe',
        'C:\\Program Files\\DAUM\\PotPlayer\\PotPlayer64.exe',
        'C:\\Program Files (x86)\\DAUM\\PotPlayer\\PotPlayerMini.exe',
        'C:\\Program Files (x86)\\DAUM\\PotPlayer\\PotPlayer.exe'
    ];

    for (const path of potPlayerPaths) {
        if (fs.existsSync(path)) {
            cachedPotPlayerPath = path;
            log.info(`找到PotPlayer播放器路径: ${path}`);
            return cachedPotPlayerPath;
        }
    }

    dialog.showErrorBox('错误', '未找到PotPlayer播放器，请在托盘设置中指定PotPlayerMini64.exe路径');
    log.error('未找到PotPlayer播放器，请在托盘设置中指定PotPlayerMini64.exe路径');
    return undefined;
}


// 刷新窗口
async function refreshWindow(): Promise<void> {
    const currentURL = getMainWindow().webContents.getURL() || '';
    // 如果是资源库页面则不刷新
    if (checkLibraryPageUrl(currentURL)) {
        return;
    }

    log.info('刷新当前窗口');
    getMainWindow().webContents.reloadIgnoringCache();
}

/**
 * 创建播放器事件处理器
 * @param fnapi - API服务实例
 * @param itemGuid - 当前播放项的GUID
 * @returns 事件处理函数
 */
function eventHandler(fnapi: fn.ApiService) {
    return async (type: ply.EventType, data: ply.EventData) => {
        switch (type) {
            case ply.EventType.PROGRESS:
                const progressData = data as ply.PlayStatusData;

                if (progressData.itemGuid.length === 0) {
                    log.info("process itemguid is empty")
                    return;
                }

                // if (progressData.percentage > 90) {
                //     log.info('视频播放接近结束，更新状态...');
                //     await fnapi.setWatched(progressData.itemGuid);
                //     return;
                // }
                // 优先从缓存查询播放信息
                const resp = await fnapi.getPlayInfoCached(progressData.itemGuid);
                if (!resp.success || !resp.data) {
                    log.error('获取播放信息失败:', resp ? resp.message : '未知错误');
                    return;
                }

                const info = resp.data;

                const record: fn.PlayStatusData = {
                    item_guid: progressData.itemGuid,
                    media_guid: info.media_guid,
                    video_guid: info.video_guid,
                    audio_guid: info.audio_guid,
                    subtitle_guid: info.subtitle_guid,
                    play_link: new URL(fnapi.getVideoUrl(info.media_guid)).hostname,
                    ts: progressData.ts,
                    duration: progressData.duration,
                };

                log.info('播放进度更新:', record);

                await fnapi.recordPlayStatus(record);
                break;

            case ply.EventType.ERROR:
                const errorData = data as ply.PlayErrorData;
                log.error('MPV error:', String(errorData.message));
                break;

            case ply.EventType.EXIT:
                const event = data as ply.PlayExitData;
                if (event.code !== 0) {
                    log.error(`播放器异常退出 (code ${event.code})`);
                    await new Promise(resolve => setTimeout(resolve, 50));
                    await refreshWindow();
                    return;
                }

                if (event.status.itemGuid.length === 0) {
                    return;
                }

                log.info('MPV exited with code:', event.code);
                log.info('最后播放位置:', event.status);

                // if (event.status.percentage > 90) {
                //     log.info('视频播放接近结束，更新状态...');
                //     await fnapi.setWatched(event.status.itemGuid);
                // } else {
                // 优先从缓存查询播放信息
                {
                    const resp = await fnapi.getPlayInfoCached(event.status.itemGuid);
                    if (!resp.success || !resp.data) {
                        log.error('获取播放信息失败:', resp ? resp.message : '未知错误');
                        return;
                    }

                    const info = resp.data;

                    const record: fn.PlayStatusData = {
                        item_guid: event.status.itemGuid,
                        media_guid: info.media_guid,
                        video_guid: info.video_guid,
                        audio_guid: info.audio_guid,
                        subtitle_guid: info.subtitle_guid,
                        play_link: new URL(fnapi.getVideoUrl(info.media_guid)).hostname,
                        ts: event.status.ts,
                        duration: event.status.duration,
                    };

                    log.debug('记录播放状态start');
                    await fnapi.recordPlayStatus(record);
                    log.debug('记录播放状态end');
                }

                // 等待50ms
                await new Promise(resolve => setTimeout(resolve, 50));
                await refreshWindow();
                break;

            default:
                log.debug('收到播放器事件:', type);
                break;
        }
    };
}

function isGuid(value: string | undefined): value is string {
    return !!value && /^[a-f0-9]{32}$/i.test(value);
}

function chooseSeasonEpisode(episodes: fn.PlayListItem[]): fn.PlayListItem | null {
    if (episodes.length === 0) {
        return null;
    }

    return episodes.find(episode => episode.ts > 0 && (episode.duration <= 0 || episode.ts < episode.duration * 0.95))
        ?? episodes.find(episode => episode.watched !== 1)
        ?? episodes[0];
}

async function getPlayInfo(fnapi: fn.ApiService, itemGuid: string): Promise<fn.PlayInfo | null> {
    try {
        const response = await fnapi.getPlayInfo(itemGuid);
        if (!response.success || !response.data) {
            log.error('获取播放信息失败:', response ? response.message : '未知错误');
            return null;
        }

        return response.data;
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        log.error('获取播放信息异常:', message);
        return null;
    }
}

async function getFirstPlayableItemFromList(fnapi: fn.ApiService, parentGuid: string, routeName: string): Promise<fn.PlayListItem | null> {
    const mediaList = await fnapi.getItemList({
        parent_guid: parentGuid,
        exclude_folder: 1,
        sort_column: 'sort_title',
        sort_type: 'ASC',
    });
    if (!mediaList.success || !mediaList.data?.list) {
        log.error(`${routeName}获取子项目列表失败:`, mediaList ? mediaList.message : '未知错误');
        return null;
    }

    const item = chooseSeasonEpisode(mediaList.data.list);
    if (!item) {
        log.warn(`${routeName}子项目列表为空:`, parentGuid);
        return null;
    }

    return item;
}

async function resolveCollectionPlayInfo(fnapi: fn.ApiService, collectionGuid: string, routeName: string): Promise<fn.PlayInfo | null> {
    const collectionInfo = await getPlayInfo(fnapi, collectionGuid);
    if (collectionInfo?.type === 'Episode' || collectionInfo?.type === 'Video') {
        log.info(`${routeName}直接解析为播放项:`, collectionGuid, '=>', collectionInfo.guid);
        return collectionInfo;
    }

    const playItemGuid = collectionInfo?.item?.play_item_guid;
    if (isGuid(playItemGuid) && playItemGuid !== collectionGuid) {
        log.info(`${routeName}解析到实际播放项:`, collectionGuid, '=>', playItemGuid);
        const playInfo = await getPlayInfo(fnapi, playItemGuid);
        if (playInfo) {
            return playInfo;
        }
    }

    const episodeList = await fnapi.getEpisodeList(collectionGuid);
    if (episodeList.success && episodeList.data) {
        const episode = chooseSeasonEpisode(episodeList.data);
        if (episode) {
            log.info(`${routeName}使用剧集列表解析播放项:`, collectionGuid, '=>', episode.guid);
            return getPlayInfo(fnapi, episode.guid);
        }

        log.warn(`${routeName}剧集列表为空，尝试子项目列表:`, collectionGuid);
    } else {
        log.error(`${routeName}获取剧集列表失败:`, episodeList ? episodeList.message : '未知错误');
    }

    const item = await getFirstPlayableItemFromList(fnapi, collectionGuid, routeName);
    return item ? getPlayInfo(fnapi, item.guid) : null;
}

function isLivePlayInfo(info: fn.PlayInfo): boolean {
    return info.type === 'LiveChannel' || info.item?.type === 'LiveChannel' || (info.live_channels?.length ?? 0) > 0;
}

function processLiveMedia(info: fn.PlayInfo): ply.PlayItem[] {
    const title = info.item?.title || '直播频道';
    const channels = (info.live_channels || []).filter(channel => channel.can_play !== 0 && channel.path.trim().length > 0);

    return channels.map((channel, index) => {
        const sourceName = channel.file_name?.trim() || `线路${index + 1}`;
        return {
            itemGuid: '',
            title: channels.length > 1 ? `${title} - ${sourceName}` : title,
            tvTitle: '',
            seasonNumber: 0,
            episodeNumber: 0,
            ts: 0,
            duration: 0,
            playLink: channel.path.trim(),
            isLive: true,
        };
    });
}

async function recordLivePlayStatus(fnapi: fn.ApiService, info: fn.PlayInfo): Promise<void> {
    const itemGuid = info.guid || info.item?.guid;
    const mediaGuid = info.media_guid || info.live_channels?.[0]?.guid;
    if (!itemGuid || !mediaGuid) {
        log.warn('直播最近播放记录缺少必要字段:', { itemGuid, mediaGuid });
        return;
    }

    const response = await fnapi.recordPlayStatus({
        item_guid: itemGuid,
        media_guid: mediaGuid,
        ts: 0,
    });

    if (!response.success) {
        log.error('记录直播最近播放失败:', response.message);
        return;
    }

    log.info('记录直播最近播放成功:', itemGuid, mediaGuid);
}

async function resolvePlayInfo(fnapi: fn.ApiService, request: PlayRequest): Promise<fn.PlayInfo | null> {
    if (request.routeType === 'season') {
        return resolveCollectionPlayInfo(fnapi, request.id, '季页面');
    }

    if (request.routeType === 'series') {
        return resolveCollectionPlayInfo(fnapi, request.id, '剧集页面');
    }

    return getPlayInfo(fnapi, request.id);
}

// 处理播放事件
async function handlePlayMovie(_event: IpcMainEvent, request: PlayRequest): Promise<void> {
    const { id, sourceIndex } = request;
    // 检查是否已有播放器在播放
    if (currentPlayer && currentPlayer.isPlaying()) {
        log.warn('已有播放器在播放，无法重复播放');
        return;
    }

    log.info('收到播放请求:', id, 'routeType:', request.routeType ?? 'item', 'index:', sourceIndex);

    const config = fnConfig.readConfig();
    if (!config || !config.domain) {
        throw new Error('无法找到服务器地址配置');
    }

    const token = config.token || request.token;
    if (!token) {
        throw new Error('无法找到登录令牌');
    }
    const fnapi = new fn.ApiService(config.domain, token);

    const playInfo = await resolvePlayInfo(fnapi, request);
    if (!playInfo) {
        return;
    }

    log.info('获取播放信息成功:', playInfo);

    const type = playInfo.type;
    const parentGuid = playInfo.parent_guid;
    const itemGuid = playInfo.guid;

    const skipConfig = {
        introEndSec: playInfo.play_config?.skip_opening ?? undefined,
        outroDurationSec: playInfo.play_config?.skip_ending ?? undefined,
    };

    let playList: ply.PlayItem[] = [];
    const isLive = request.routeType === 'live' || isLivePlayInfo(playInfo);

    if (isLive) {
        log.info('当前为直播频道，使用 live_channels 生成播放列表');
        playList = processLiveMedia(playInfo);
        for (const mediaItem of playList) {
            log.info('添加直播源到播放列表:', mediaItem);
        }
    }
    else if (type === 'Episode' && parentGuid) {
        log.info('当前为剧集，尝试获取系列下的所有剧集进行播放');
        const episodeList = await fnapi.getEpisodeList(parentGuid);
        if (!episodeList.success || !episodeList.data) {
            log.error('获取剧集列表失败:', episodeList ? episodeList.message : '未知错误');
            return;
        }

        for (const episode of episodeList.data) {
            const mediaItem = processEpisodeMedia(episode, skipConfig);
            playList.push(mediaItem);
            log.info('添加剧集到播放列表:', mediaItem);
        }
    }
    else if (type === 'Video' && parentGuid) {
        log.info('当前为其他视频，添加到播放列表');
        const req: ItemListRequest = {
            parent_guid: parentGuid,
            exclude_folder: 1,
            sort_column: 'sort_title',
            sort_type: 'ASC',
        };

        const mediaList = await fnapi.getItemList(req);
        log.info('获取媒体列表响应:', mediaList);
        if (!mediaList.success || !mediaList.data || !mediaList.data.list) {
            log.error('获取媒体列表失败:', mediaList ? mediaList.message : '未知错误');
            return;
        }

        for (const media of mediaList.data.list) {
            const mediaItem = processEpisodeMedia(media, skipConfig);
            playList.push(mediaItem);
            log.info('添加剧集到播放列表:', mediaItem);
        }
    }
    else {
        const mediaItem = processSingleMedia(playInfo);
        playList.push(mediaItem);
        log.info('添加单集到播放列表:', mediaItem);
    }

    if (playList.length === 0) {
        log.warn('播放列表为空');
        return;
    }

    // 寻找当前播放的媒体在数组中的位置
    const currentIndex = playList.findIndex(item => item.itemGuid === itemGuid);
    const playableIndex = currentIndex >= 0 ? currentIndex : 0;

    if (!isLive) {
        if (currentIndex < 0 || playList.some(item => !item.itemGuid)) {
            throw new Error('播放列表缺少有效媒体标识');
        }
        if (!config.account) {
            throw new Error('无法找到登录账号');
        }

        const playbackSession = await registerPlaybackSession(getProxySecret(), {
            token: config.token || token,
            account: config.account,
            domain: config.domain,
            accessCookie: getAccessCookieHeader(config.domain),
            skipVerify: isTrusted(config.domain),
            useNasLocal: config.nasProxyEnabled === true,
            itemGuids: playList.map(item => item.itemGuid),
        });

        playList = playList.map(item => ({
            ...item,
            playLink: createProxyPlaybackUrl(playbackSession, item.itemGuid),
        }));

        if (sourceIndex > 0) {
            log.info(`使用指定的播放源索引: ${sourceIndex}`);
            playList[playableIndex].playLink = createProxyPlaybackUrl(
                playbackSession,
                playList[playableIndex].itemGuid,
                sourceIndex,
            );
        }
    }

    const playerPath = getPotPlayerPath();
    if (!playerPath) {
        log.error('无法找到播放器路径');
        return;
    }

    const extraArgs: string[] = [];

    let playConfig: ply.Config = {
        fnapi: fnapi,
        playerPath: playerPath,
        // headers: {
        //     Authorization: token,
        // },
        extraArgs,
        debug: true,
        onEvent: eventHandler(fnapi)
    };

    // 创建播放器实例
    const player = ply.PlayerFactory.createPlayer(ply.PlayerType.POTPLAYER, playConfig);

    // 保存全局引用
    currentPlayer = player;

    const started = await player.playList(playList, playableIndex);
    if (started && isLive) {
        await recordLivePlayStatus(fnapi, playInfo);
        await refreshWindow();
    }
}

// 处理当前播放的媒体信息
function processEpisodeMedia(info: fn.PlayListItem, skipConfig?: { introEndSec?: number; outroDurationSec?: number }): ply.PlayItem {
    return {
        itemGuid: info.guid,
        title: info.title,
        tvTitle: info.tv_title,
        seasonNumber: info.season_number,
        episodeNumber: info.episode_number,
        ts: info.ts,
        duration: info.duration,
        playLink: '',
        introEndSec: skipConfig?.introEndSec,
        outroDurationSec: skipConfig?.outroDurationSec,
    };
}

// 处理单个待播放媒体信息
function processSingleMedia(info: fn.PlayInfo): ply.PlayItem {
    return {
        itemGuid: info.guid,
        title: info.item.title,
        tvTitle: info.item.tv_title,
        seasonNumber: info.item.season_number,
        episodeNumber: info.item.episode_number,
        ts: info.ts,
        duration: info.item.duration,
        playLink: '',
        introEndSec: info.play_config?.skip_opening ?? undefined,
        outroDurationSec: info.play_config?.skip_ending ?? undefined,
    };
}

// 应用退出前清理播放器
function handleBeforeQuit(): void {
    if (currentPlayer) {
        log.info('应用退出前关闭播放器');
        currentPlayer.stop();
        currentPlayer = null;
    }

    // 清理播放器路径缓存
    cachedPlayerPath = null;
    cachedPotPlayerPath = null;
}

// 注册媒体播放处理器
function init(): void {
    // 从配置中读取MPV播放器路径并设置
    const configMpvPath = fnConfig.getMpvPlayerPath();
    if (configMpvPath) {
        setMpvPlayerPath(configMpvPath);
        log.info(`从配置中加载MPV播放器路径: ${configMpvPath}`);
    }

    const configPotPlayerPath = fnConfig.getPotPlayerPath();
    if (configPotPlayerPath) {
        setPotPlayerPath(configPotPlayerPath);
        log.info(`从配置中加载PotPlayer播放器路径: ${configPotPlayerPath}`);
    }

    registerHandler('play-movie', handlePlayMovie);
    registerAppHook('beforeQuit', handleBeforeQuit);
}

export {
    init
};
