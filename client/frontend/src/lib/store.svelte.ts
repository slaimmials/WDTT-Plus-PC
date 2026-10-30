import { api, events, errorText } from './api';
import type { LogEntry, PingResult, Profile, Settings, Status } from './types';

const MAX_LOGS = 3000;

export type Toast = { id: number; text: string; kind: 'info' | 'error' };

export type Dialog = {
  title: string;
  message?: string;
  input?: { label: string; value: string; type?: 'text' | 'number' };
  confirmLabel: string;
  danger?: boolean;
  resolve: (value: string | null) => void;
};

class Store {
  ready = $state(false);
  version = $state('');
  elevated = $state(false);
  settings = $state<Settings>({
    profiles: [],
    activeProfile: '',
    servers: [],
    routing: { mode: 'all', exclude: [], include: [], bypassLan: false },
    vk: { fingerprint: 'firefox', clientIds: '', customClientId: '', customSecret: '', vkCallsPreflight: true, hashFallback: false },
    deviceId: '',
    theme: 'system',
    diagnostics: false,
  });
  status = $state<Status>({
    state: 'disconnected', profileId: '', mode: '', error: '', proxyAddresses: null,
    stats: { ActiveConnections: 0, BytesUp: 0, BytesDown: 0, LatencyMs: 0 }, connectedAt: 0,
  });
  speed = $state({ up: 0, down: 0 });
  logs = $state<LogEntry[]>([]);
  toasts = $state<Toast[]>([]);
  incomingLink = $state('');
  dialog = $state<Dialog | null>(null);
  pings = $state<Record<string, PingResult>>({});

  activeProfile = $derived<Profile | undefined>(
    this.settings.profiles.find((p) => p.id === this.settings.activeProfile) ?? this.settings.profiles[0],
  );

  private lastStats = { at: 0, up: 0, down: 0 };
  private toastId = 0;

  async init() {
    const state = await api.state();
    this.version = state.version;
    this.elevated = state.elevated;
    this.settings = state.settings;
    this.status = state.status;
    this.logs = await api.logs();
    if (state.pendingLink) this.incomingLink = state.pendingLink;

    events.settings((s) => (this.settings = s));
    events.status((s) => this.applyStatus(s));
    events.link((link) => (this.incomingLink = link));
    events.ping(({ profileId, ...result }) => (this.pings[profileId] = result));
    events.log((entry) => {
      this.logs.push(entry);
      if (this.logs.length > MAX_LOGS) this.logs.splice(0, this.logs.length - MAX_LOGS);
    });
    this.ready = true;
  }

  private applyStatus(s: Status) {
    const now = Date.now();
    if (s.state === 'connected' && this.lastStats.at > 0) {
      const seconds = (now - this.lastStats.at) / 1000;
      if (seconds > 0.2) {
        this.speed = {
          up: (s.stats.BytesUp - this.lastStats.up) / seconds,
          down: (s.stats.BytesDown - this.lastStats.down) / seconds,
        };
      }
    } else if (s.state !== 'connected') {
      this.speed = { up: 0, down: 0 };
    }
    this.lastStats = { at: now, up: s.stats.BytesUp, down: s.stats.BytesDown };
    if (s.error && s.error !== this.status.error) this.notify(s.error, 'error');
    this.status = s;
  }

  notify(text: string, kind: Toast['kind'] = 'info') {
    const id = ++this.toastId;
    this.toasts.push({ id, text, kind });
    setTimeout(() => (this.toasts = this.toasts.filter((t) => t.id !== id)), kind === 'error' ? 7000 : 3500);
  }

  confirm(title: string, message: string, confirmLabel = 'Подтвердить', danger = false): Promise<boolean> {
    return new Promise((resolve) => {
      this.dialog = { title, message, confirmLabel, danger, resolve: (v) => resolve(v !== null) };
    });
  }

  ask(title: string, label: string, value = '', type: 'text' | 'number' = 'text'): Promise<string | null> {
    return new Promise((resolve) => {
      this.dialog = { title, input: { label, value, type }, confirmLabel: 'Сохранить', resolve };
    });
  }

  // run executes an action and shows its error as a toast.
  async run<T>(action: () => Promise<T>, success?: string): Promise<T | undefined> {
    try {
      const result = await action();
      if (success) this.notify(success);
      return result;
    } catch (error) {
      this.notify(errorText(error), 'error');
      return undefined;
    }
  }
}

export const store = new Store();
