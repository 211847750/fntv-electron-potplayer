import { app } from 'electron';
import { ChildProcess, spawn } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import {
    BasePlayer,
    Config,
    EventType,
    PlayErrorData,
    PlayExitData,
    PlayItem,
    PlayStatusData,
    PlayerType
} from '../types';
import { PlayerFactory } from '../factory';
import log from '../../logger';

const BRIDGE_PROGRESS_INTERVAL = '1s';

function formatTitle(info: PlayItem): string {
    if (info.tvTitle) {
        return `${info.tvTitle || 'noTVTitle'} - S${info.seasonNumber || '0'}E${info.episodeNumber || '0'}: ${info.title || 'noTitle'}`;
    }

    return info.title || info.itemGuid;
}

type BridgeEvent = {
    type?: string;
    hwnd?: number;
    posMs?: number;
    durMs?: number;
    status?: number;
    message?: string;
};

function getBridgePath(): string {
    if (!app.isPackaged) {
        return process.platform === 'win32'
            ? '.\\third_party\\potplayer-bridge\\potbridge.exe'
            : './third_party/potplayer-bridge/potbridge.exe';
    }

    const appPath = app.getAppPath();
    const appRootPath = path.dirname(path.dirname(appPath));
    return path.join(appRootPath, 'third_party', 'potplayer-bridge', 'potbridge.exe');
}

export class PotPlayer extends BasePlayer {
    private bridgeProcess: ChildProcess | null = null;
    private currentItem: PlayItem | null = null;
    private exitEmitted = false;
    private stdoutBuffer = '';

    constructor(config: Config) {
        super(config);
    }

    async playList(infos: PlayItem[], pos: number, args?: string[]): Promise<boolean> {
        if (process.platform !== 'win32') {
            this.emitError('PotPlayer仅支持Windows平台');
            return false;
        }

        if (!this.config.playerPath) {
            this.emitError('未找到PotPlayer播放器路径');
            return false;
        }

        const bridgePath = getBridgePath();
        if (!fs.existsSync(bridgePath)) {
            this.emitError(`PotPlayer Bridge不存在: ${bridgePath}`);
            return false;
        }

        const currentItem = infos[pos] || infos[0];
        if (!currentItem) {
            this.emitError('播放列表为空');
            return false;
        }

        this.currentItem = currentItem;
        this.exitEmitted = false;
        this.updateGlobalStatus({
            itemGuid: currentItem.itemGuid,
            ts: currentItem.ts || 0,
            duration: currentItem.duration || 0,
            percentage: currentItem.duration > 0 ? Math.floor((currentItem.ts / currentItem.duration) * 100) : 0
        });

        try {
            const subtitlePaths = await this.loadSubtitlePaths(currentItem.itemGuid);
            const bridgeArgs = this.createBridgeArgs(currentItem, subtitlePaths, args || []);
            this.bridgeProcess = spawn(bridgePath, bridgeArgs, {
                detached: false,
                stdio: ['ignore', 'pipe', 'pipe'],
                windowsHide: true
            });

            this.bridgeProcess.stdout?.on('data', data => this.handleBridgeStdout(String(data)));
            this.bridgeProcess.stderr?.on('data', data => {
                const message = String(data).trim();
                if (message) {
                    log.debug('PotPlayer Bridge stderr:', message);
                }
            });

            this.bridgeProcess.once('error', (error: Error) => {
                this.emitError(`PotPlayer Bridge启动失败: ${error.message}`);
                this.handleExit(1);
            });

            this.bridgeProcess.once('exit', (code: number | null) => {
                log.debug('PotPlayer Bridge已退出:', code);
                this.bridgeProcess = null;
                if (!this.exitEmitted && code !== 0) {
                    this.handleExit(code ?? 1);
                }
            });

            log.info('PotPlayer Bridge已启动:', bridgePath, bridgeArgs);
            return true;
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            this.emitError(`PotPlayer播放失败: ${message}`);
            return false;
        }
    }

    stop(): void {
        if (this.bridgeProcess) {
            try {
                this.bridgeProcess.kill();
            } catch (error) {
                const message = error instanceof Error ? error.message : String(error);
                log.debug('停止PotPlayer Bridge失败:', message);
            }
        }

        this.handleExit(0);
    }

    isPlaying(): boolean {
        return this.currentItem !== null && !this.exitEmitted;
    }

    private createBridgeArgs(info: PlayItem, subtitlePaths: string[], extraArgs: string[]): string[] {
        const bridgeArgs = [
            'play',
            '--potplayer', this.config.playerPath,
            '--url', info.playLink,
            '--title', formatTitle(info),
            '--interval', BRIDGE_PROGRESS_INTERVAL
        ];

        if (info.ts > 0) {
            bridgeArgs.push('--seek', String(info.ts));
        }

        const firstSubtitlePath = subtitlePaths[0];
        if (firstSubtitlePath) {
            bridgeArgs.push('--sub', firstSubtitlePath);
        }

        for (const extraArg of extraArgs) {
            bridgeArgs.push('--arg', extraArg);
        }

        return bridgeArgs;
    }

    private async loadSubtitlePaths(itemGuid: string): Promise<string[]> {
        try {
            const fnapi = this.getFnApi();
            const subtitles = await fnapi.getSubtitle(itemGuid);
            return await fnapi.downloadSubtitle(subtitles);
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            log.debug('PotPlayer字幕加载准备失败:', message);
            return [];
        }
    }

    private handleBridgeStdout(chunk: string): void {
        this.stdoutBuffer += chunk;
        const lines = this.stdoutBuffer.split(/\r?\n/);
        this.stdoutBuffer = lines.pop() || '';

        for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed) {
                continue;
            }

            try {
                this.handleBridgeEvent(JSON.parse(trimmed) as BridgeEvent);
            } catch (error) {
                const message = error instanceof Error ? error.message : String(error);
                log.debug('PotPlayer Bridge事件解析失败:', message, trimmed);
            }
        }
    }

    private handleBridgeEvent(event: BridgeEvent): void {
        if (this.config.debug) {
            log.debug('PotPlayer Bridge事件:', event);
        }

        switch (event.type) {
            case 'ready':
                log.info('PotPlayer窗口已绑定:', event.hwnd);
                break;
            case 'progress':
                this.handleProgress(event);
                break;
            case 'exit':
                this.handleProgress(event);
                this.handleExit(0);
                break;
            case 'error':
                this.emitError(event.message || 'PotPlayer Bridge错误');
                break;
            default:
                log.debug('未知PotPlayer Bridge事件:', event);
                break;
        }
    }

    private handleProgress(event: BridgeEvent): void {
        if (!this.currentItem || this.exitEmitted) {
            return;
        }

        const currentMs = Number(event.posMs || 0);
        const totalMs = Number(event.durMs || 0);
        const duration = totalMs > 0 ? Math.floor(totalMs / 1000) : this.currentItem.duration;
        const ts = currentMs > 0 ? Math.floor(currentMs / 1000) : this.getStatus().ts;
        const percentage = duration > 0 ? Math.min(100, Math.floor((ts / duration) * 100)) : 0;

        const progressData: PlayStatusData = {
            itemGuid: this.currentItem.itemGuid,
            ts,
            duration,
            percentage
        };

        this.updateGlobalStatus(progressData);
        this.emitEvent(EventType.PROGRESS, progressData);
    }

    private handleExit(code: number): void {
        if (this.exitEmitted) {
            return;
        }

        this.exitEmitted = true;

        const event: PlayExitData = {
            code,
            status: this.getStatus()
        };

        this.emitEvent(EventType.EXIT, event);
        this.bridgeProcess = null;
        this.currentItem = null;
        this.stdoutBuffer = '';
    }

    private emitError(message: string): void {
        log.error(message);
        const errorEvent: PlayErrorData = { message };
        this.emitEvent(EventType.ERROR, errorEvent);
    }
}

PlayerFactory.registerPlayer(PlayerType.POTPLAYER, PotPlayer);
