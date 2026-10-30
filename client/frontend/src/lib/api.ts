import * as App from './wailsjs/go/main/App';
import { EventsOn, ClipboardSetText, ClipboardGetText } from './wailsjs/runtime/runtime';
import type {
  AdminResponse, AppState, HashCheckResult, LogEntry, PingResult, Profile, Routing, Server, Settings, Status, VK,
} from './types';

// The generated bindings use model classes; the UI works with plain objects
// of the same shape, so values are passed through untyped.
const go = App as unknown as Record<string, (...args: unknown[]) => Promise<any>>;

export const api = {
  state: (): Promise<AppState> => go.GetState(),
  restartAsAdmin: (): Promise<void> => go.RestartAsAdmin(),

  parseLink: (text: string): Promise<Profile> => go.ParseLink(text),
  saveProfile: (p: Profile): Promise<Profile> => go.SaveProfile(p),
  deleteProfile: (id: string): Promise<void> => go.DeleteProfile(id),
  setActiveProfile: (id: string): Promise<void> => go.SetActiveProfile(id),
  profileLink: (id: string): Promise<string> => go.ProfileLink(id),
  normalizeHashes: (text: string): Promise<string[]> => go.NormalizeHashes(text),

  connect: (id: string): Promise<void> => go.Connect(id),
  disconnect: (): Promise<void> => go.Disconnect(),
  checkHashes: (hashes: string[]): Promise<void> => go.CheckHashes(hashes),
  pingProfiles: (ids: string[]): Promise<void> => go.PingProfiles(ids),

  saveRouting: (r: Routing): Promise<void> => go.SaveRouting(r),
  validateRule: (rule: string): Promise<string> => go.ValidateRule(rule),
  saveVK: (vk: VK): Promise<void> => go.SaveVK(vk),
  saveAppearance: (theme: string, diagnostics: boolean): Promise<void> => go.SaveAppearance(theme, diagnostics),

  logs: (): Promise<LogEntry[]> => go.GetLogs(),
  clearLogs: (): Promise<void> => go.ClearLogs(),
  openDataFolder: (): Promise<void> => go.OpenDataFolder(),

  saveServer: (s: Server): Promise<Server> => go.SaveServer(s),
  deleteServer: (id: string): Promise<void> => go.DeleteServer(id),
  admin: (serverId: string, ...args: string[]): Promise<AdminResponse> => go.AdminCall(serverId, args),
  restartServer: (id: string): Promise<void> => go.RestartServer(id),
  deployServer: (id: string, mode: 'install' | 'reset'): Promise<void> => go.DeployServer(id, mode),
  uninstallServer: (id: string): Promise<void> => go.UninstallServer(id),
  clientLink: (serverId: string, password: string, hash: string, name: string): Promise<string> =>
    go.ClientLink(serverId, password, hash, name),

  saveTextFile: (name: string, content: string): Promise<void> => go.SaveTextFile(name, content),
  readTextFile: (): Promise<string> => go.ReadTextFile(),

  copy: (text: string): Promise<boolean> => ClipboardSetText(text),
  paste: (): Promise<string> => ClipboardGetText(),
};

export const events = {
  status: (fn: (s: Status) => void) => EventsOn('status', fn),
  settings: (fn: (s: Settings) => void) => EventsOn('settings', fn),
  log: (fn: (e: LogEntry) => void) => EventsOn('log', fn),
  link: (fn: (link: string) => void) => EventsOn('link', fn),
  hashCheck: (fn: (e: { index?: number; result?: HashCheckResult; error?: string; done?: boolean }) => void) =>
    EventsOn('hash-check', fn),
  ping: (fn: (e: { profileId: string } & PingResult) => void) => EventsOn('ping', fn),
  deploy: (fn: (e: { serverId: string; progress?: number; step?: string; done?: boolean; error?: string }) => void) =>
    EventsOn('deploy', fn),
};

export function errorText(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error ?? 'Неизвестная ошибка');
}
