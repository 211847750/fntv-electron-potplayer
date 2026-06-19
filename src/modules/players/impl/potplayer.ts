import { app } from 'electron';
import { ChildProcess, spawn } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
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

type BridgeEvent = {
    type?: string;
    hwnd?: number;
    posMs?: number;
    durMs?: number;
    status?: number;
    message?: string;
    index?: number;
    episodeId?: string;
};

type PlaylistMapping = {
    index: number;
    item: PlayItem;
    subtitlePath?: string;
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

function formatDplValue(value: string): string {
    return value.replace(/[\r\n]/g, ' ').trim();
}

function getStartSec(item: PlayItem): number {
    return Math.max(item.ts || 0, item.introEndSec || 0);
}

export class PotPlayer extends BasePlayer {
    private bridgeProcess: ChildProcess | null = null;
    private currentItem: PlayItem | null = null;
    private exitEmitted = false;
    private stdoutBuffer = '';
    private playlistMapping: PlaylistMapping[] = [];
    private currentIndex = 0;
    private introSeekDone = new Set<string>();
    private outroTriggeredEpisodeId: string | null = null;
    private enableSkipIntro = true;
    private enableSkipOutro = true;
    private loadedSubtitles = new Map<string, string>();  // itemGuid -> subtitlePath
    private episodeProgress = new Map<string, PlayStatusData>();  // itemGuid -> latest progress

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

        if (infos.length === 0) {
            this.emitError('播放列表为空');
            return false;
        }
        this.playlistMapping = infos.map((item, index) => ({
            index,
            item,
        }));

        this.currentIndex = pos;
        this.currentItem = infos[pos] || infos[0];
        this.exitEmitted = false;
        this.loadedSubtitles.clear();
        this.episodeProgress.clear();

        // 预下载当前剧集字幕
        this.downloadSubtitleForEpisode(this.currentItem);

        this.updateGlobalStatus(this.statusFromItem(this.currentItem));

        try {
            const playlistPath = await this.generatePlaylist(infos, pos);
            const bridgeArgs = this.createBridgeArgs(playlistPath, args || []);

            this.bridgeProcess = spawn(bridgePath, bridgeArgs, {
                detached: false,
                stdio: ['pipe', 'pipe', 'pipe'],
                windowsHide: true
            });

            this.bridgeProcess.stdin?.on('error', (err) => {
                log.debug('PotPlayer Bridge stdin error:', err.message);
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
        this.sendCommand({ command: 'stop' });

        setTimeout(() => {
            if (this.bridgeProcess) {
                try {
                    this.bridgeProcess.kill();
                } catch (error) {
                    const message = error instanceof Error ? error.message : String(error);
                    log.debug('停止PotPlayer Bridge失败:', message);
                }
            }
            this.handleExit(0);
        }, 500);
    }

    isPlaying(): boolean {
        return this.currentItem !== null && !this.exitEmitted;
    }

    private async generatePlaylist(infos: PlayItem[], targetIndex: number): Promise<string> {
        const tempDir = path.join(os.tmpdir(), 'fntv-playlists');
        if (!fs.existsSync(tempDir)) {
            fs.mkdirSync(tempDir, { recursive: true });
        }

        const targetItem = infos[targetIndex] || infos[0];
        const startSec = getStartSec(targetItem);
        const startMs = startSec * 1000;

        let content = '\uFEFFDAUMPLAYLIST\n';
        content += `playname=${targetItem.playLink}\n`;
        content += `playtime=${startMs}\n`;

        for (let i = 0; i < infos.length; i++) {
            const item = infos[i];
            const index = i + 1;
            content += `${index}*file*${item.playLink}\n`;
            content += `${index}*title*${formatDplValue(this.formatItemTitle(item))}\n`;
            content += `${index}*played*0\n`;
            if (item.duration > 0) {
                content += `${index}*duration2*${item.duration}\n`;
            }
            if (i === targetIndex && startSec > 0) {
                content += `${index}*start*${startSec}\n`;
            }
            const mapping = this.playlistMapping[i];
            if (mapping?.subtitlePath) {
                content += `${index}*subtitle*${mapping.subtitlePath}\n`;
            }
        }

        const playlistPath = path.join(tempDir, `playlist_${Date.now()}.dpl`);
        await fs.promises.writeFile(playlistPath, content, 'utf-8');

        log.debug(`生成DPL播放列表: ${playlistPath}`);
        return playlistPath;
    }

    private createBridgeArgs(playlistPath: string, extraArgs: string[]): string[] {
        const bridgeArgs = [
            'play',
            '--potplayer', this.config.playerPath,
            '--playlist', playlistPath,
            '--interval', BRIDGE_PROGRESS_INTERVAL
        ];

        for (const extraArg of extraArgs) {
            bridgeArgs.push('--arg', extraArg);
        }

        return bridgeArgs;
    }

    private sendCommand(command: Record<string, unknown>): void {
        if (!this.bridgeProcess?.stdin?.writable) {
            return;
        }

        try {
            const data = JSON.stringify(command) + '\n';
            this.bridgeProcess.stdin.write(data);
        } catch (error) {
            log.debug('发送命令失败:', error);
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
                this.loadSubtitleForEpisode(this.currentItem);
                break;
            case 'progress':
                this.handleProgress(event);
                break;
            case 'episodeChanged':
                this.handleEpisodeChanged(event);
                break;
            case 'exit':
            case 'closed':
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

    private handleEpisodeChanged(event: BridgeEvent): void {
        const targetIndex = this.resolveEpisodeIndex(event);
        if (targetIndex === null) {
            log.warn('handleEpisodeChanged: 无法解析目标剧集', event);
            return;
        }

        this.switchToEpisode(targetIndex);
    }

    private resolveEpisodeIndex(event: BridgeEvent): number | null {
        if (this.playlistMapping.length === 0) {
            return null;
        }

        if (typeof event.index === 'number' && event.index >= 0 && event.index < this.playlistMapping.length) {
            return event.index;
        }

        if (event.episodeId) {
            const index = this.findEpisodeIndex(event.episodeId);
            if (index !== null) {
                return index;
            }
        }

        switch (event.message) {
            case 'next':
                return Math.min(this.currentIndex + 1, this.playlistMapping.length - 1);
            case 'previous':
                return Math.max(this.currentIndex - 1, 0);
            case '':
            case undefined:
                return null;
            default:
                return this.matchEpisodeIndexByTitle(event.message);
        }
    }

    private findEpisodeIndex(itemGuid: string): number | null {
        const mapping = this.playlistMapping.find(item => item.item.itemGuid === itemGuid);
        return mapping ? mapping.index : null;
    }

    private matchEpisodeIndexByTitle(title: string): number | null {
        if (!title || this.playlistMapping.length === 0) {
            return null;
        }

        const normalizedTitle = title.toLowerCase().trim();
        for (const mapping of this.playlistMapping) {
            const itemTitle = this.formatItemTitle(mapping.item).toLowerCase().trim();
            if (itemTitle && (normalizedTitle.includes(itemTitle) || itemTitle.includes(normalizedTitle))) {
                log.debug(`标题匹配成功: "${title}" -> index=${mapping.index}`);
                return mapping.index;
            }
        }

        const urlMatch = title.match(/playvideo\/([^?\\/]+)/i);
        if (urlMatch) {
            const index = this.findEpisodeIndex(urlMatch[1]);
            if (index !== null) {
                log.debug(`标题URL匹配成功: itemGuid=${urlMatch[1]} -> index=${index}`);
                return index;
            }
        }

        const epMatch = title.toLowerCase().match(/s(\d+)e(\d+)/i);
        if (epMatch) {
            const seasonNum = parseInt(epMatch[1], 10);
            const episodeNum = parseInt(epMatch[2], 10);
            for (const mapping of this.playlistMapping) {
                const item = mapping.item;
                if (item.seasonNumber === seasonNum && item.episodeNumber === episodeNum) {
                    log.debug(`SxxExx匹配成功: S${seasonNum}E${episodeNum} -> index=${mapping.index}`);
                    return mapping.index;
                }
            }
        }

        return null;
    }

    private switchToEpisode(index: number): void {
        if (index === this.currentIndex) {
            return;
        }

        this.flushCurrentProgressBeforeSwitch();
        this.currentIndex = index;
        this.updateCurrentEpisode();
    }

    private flushCurrentProgressBeforeSwitch(): void {
        const status = this.getStatus();
        if (!this.currentItem || status.itemGuid !== this.currentItem.itemGuid || status.ts <= 0) {
            return;
        }

        const progressData: PlayStatusData = { ...status };
        this.recordEpisodeProgress(progressData);
        this.emitEvent(EventType.PROGRESS, progressData);
    }

    private formatItemTitle(item: PlayItem): string {
        if (item.tvTitle) {
            return `${item.tvTitle} - S${item.seasonNumber || 0}E${item.episodeNumber || 0}: ${item.title}`;
        }
        return item.title || item.itemGuid;
    }

    private statusFromItem(item: PlayItem): PlayStatusData {
        const progress = this.episodeProgress.get(item.itemGuid);
        if (progress) {
            return { ...progress };
        }

        const ts = item.ts || 0;
        const duration = item.duration || 0;
        return {
            itemGuid: item.itemGuid,
            ts,
            duration,
            percentage: duration > 0 ? Math.floor((ts / duration) * 100) : 0
        };
    }

    private recordEpisodeProgress(status: PlayStatusData): void {
        this.episodeProgress.set(status.itemGuid, { ...status });
        this.updateGlobalStatus(status);
    }

    private updateCurrentEpisode(): void {
        if (this.currentIndex >= 0 && this.currentIndex < this.playlistMapping.length) {
            this.currentItem = this.playlistMapping[this.currentIndex].item;
            this.outroTriggeredEpisodeId = null;

            log.info(`切集: index=${this.currentIndex}, episodeId=${this.currentItem.itemGuid}`);

            this.updateGlobalStatus(this.statusFromItem(this.currentItem));

            this.applyIntroSeek();
            this.loadSubtitleForEpisode(this.currentItem);
        }
    }

    private applyIntroSeek(): void {
        if (!this.currentItem || !this.enableSkipIntro) {
            return;
        }

        const episodeId = this.currentItem.itemGuid;
        if (this.introSeekDone.has(episodeId)) {
            return;
        }

        const historySec = this.statusFromItem(this.currentItem).ts;
        const introEndSec = this.currentItem.introEndSec || 0;
        const targetSec = Math.max(historySec, introEndSec);

        if (targetSec > 0) {
            log.info(`跳过片头: episodeId=${episodeId}, targetSec=${targetSec}`);
            this.sendCommand({ command: 'seek', posMs: targetSec * 1000 });
            this.introSeekDone.add(episodeId);
        }
    }

    private async downloadSubtitleForEpisode(item: PlayItem | null): Promise<void> {
        if (!item || !item.itemGuid || this.loadedSubtitles.has(item.itemGuid)) {
            return;
        }
        try {
            const fnapi = this.getFnApi();
            const subs = await fnapi.getSubtitle(item.itemGuid);
            if (subs.length === 0) {
                return;
            }
            const paths = await fnapi.downloadSubtitle(subs);
            if (paths.length > 0) {
                const idx = this.playlistMapping.findIndex(m => m.item.itemGuid === item.itemGuid);
                if (idx >= 0) {
                    this.playlistMapping[idx].subtitlePath = paths[0];
                }
                this.loadedSubtitles.set(item.itemGuid, paths[0]);
                log.info(`字幕已下载: ${item.itemGuid} -> ${paths[0]}`);
            }
        } catch (error) {
            log.debug('下载字幕失败:', error);
        }
    }

    private loadSubtitleForEpisode(item: PlayItem | null): void {
        if (!item) {
            return;
        }
        const subPath = this.loadedSubtitles.get(item.itemGuid);
        if (subPath) {
            this.sendCommand({ command: 'loadSubtitle', path: subPath });
            log.info(`发送字幕到Bridge: ${subPath}`);
        } else {
            // 字幕可能还在下载，等待完成后发送
            this.downloadSubtitleForEpisode(item).then(() => {
                const p = this.loadedSubtitles.get(item.itemGuid);
                if (p) {
                    this.sendCommand({ command: 'loadSubtitle', path: p });
                }
            });
        }
    }


    private checkOutroSkip(posMs: number, durMs: number, duration: number): boolean {
        if (!this.currentItem || !this.enableSkipOutro) {
            return false;
        }

        if (!durMs || durMs <= 0) {
            return false;
        }

        const episodeId = this.currentItem.itemGuid;
        if (this.outroTriggeredEpisodeId === episodeId) {
            return true;
        }

        const currentSec = posMs / 1000;
        const durationSec = durMs / 1000;
        const outroDurationSec = this.currentItem.outroDurationSec || 0;
        const remainingSec = durationSec - currentSec;
        const shouldSkip = outroDurationSec > 0 && remainingSec <= outroDurationSec;

        if (shouldSkip) {
            log.info(`跳过片尾: episodeId=${episodeId}, posSec=${currentSec}, durSec=${durationSec}, outroDurationSec=${outroDurationSec}`);
            this.outroTriggeredEpisodeId = episodeId;
            this.markCurrentEpisodeCompleted(duration);
            this.goNextEpisode();
            return true;
        }

        return false;
    }

    private markCurrentEpisodeCompleted(duration: number): void {
        if (!this.currentItem || duration <= 0) {
            return;
        }

        const progressData: PlayStatusData = {
            itemGuid: this.currentItem.itemGuid,
            ts: duration,
            duration,
            percentage: 100
        };

        this.recordEpisodeProgress(progressData);
        this.emitEvent(EventType.PROGRESS, progressData);
    }

    private goNextEpisode(): void {
        if (this.currentIndex >= this.playlistMapping.length - 1) {
            log.info('已是最后一集，无法跳转');
            return;
        }

        log.info(`自动下一集: ${this.currentIndex} -> ${this.currentIndex + 1}`);
        this.sendCommand({ command: 'next' });
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

        if (this.checkOutroSkip(currentMs, totalMs, duration)) {
            return;
        }

        const progressData: PlayStatusData = {
            itemGuid: this.currentItem.itemGuid,
            ts,
            duration,
            percentage
        };

        this.recordEpisodeProgress(progressData);
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
