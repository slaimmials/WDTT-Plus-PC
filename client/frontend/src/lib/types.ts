export type Mode = 'vpn' | 'auto' | 'socks5' | 'http';
export type RoutingMode = 'all' | 'exclude' | 'include';
export type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'disconnecting';

export interface Proxy {
  port: number;
  lan: boolean;
  username: string;
  password: string;
  udp: boolean;
}

export interface Profile {
  id: string;
  name: string;
  host: string;
  dtlsPort: number;
  wgPort: number;
  localPort: number;
  password: string;
  hashes: string[];
  workers: number;
  mode: Mode;
  proxy: Proxy;
  streamOnly: boolean;
  turnSni: string;
  masque: boolean;
}

export interface Routing {
  mode: RoutingMode;
  exclude: string[];
  include: string[];
  bypassLan: boolean;
}

export interface VK {
  fingerprint: string;
  clientIds: string;
  customClientId: string;
  customSecret: string;
  vkCallsPreflight: boolean;
  hashFallback: boolean;
}

export interface Server {
  id: string;
  name: string;
  host: string;
  sshPort: number;
  user: string;
  sshPassword: string;
  privateKey: string;
  keyPassphrase: string;
  hostKey: string;
  mainPassword: string;
  dtlsPort: number;
  wgPort: number;
  localPort: number;
  dns: string;
}

export interface Settings {
  profiles: Profile[];
  activeProfile: string;
  servers: Server[];
  routing: Routing;
  vk: VK;
  deviceId: string;
  theme: 'system' | 'dark' | 'light';
  diagnostics: boolean;
}

export interface Stats {
  ActiveConnections: number;
  BytesUp: number;
  BytesDown: number;
  LatencyMs: number;
}

export interface PingResult {
  testing?: boolean;
  latencyMs?: number;
  error?: string;
}

export interface Status {
  state: ConnectionState;
  profileId: string;
  mode: Mode | '';
  error: string;
  proxyAddresses: string[] | null;
  stats: Stats;
  connectedAt: number;
}

export interface AppState {
  version: string;
  elevated: boolean;
  settings: Settings;
  status: Status;
  pendingLink: string;
}

export interface LogEntry {
  time: number;
  message: string;
  level: 'info' | 'warn' | 'error';
}

export type HashStatus =
  | 'ok' | 'auth_required' | 'captcha' | 'dead' | 'blocked' | 'full' | 'limited' | 'network' | 'error';

export interface HashCheckResult {
  hash: string;
  status: HashStatus;
  message: string;
}

export interface TrafficValue { down: number; up: number }
export interface TrafficPeriod { today: TrafficValue; week: TrafficValue; month: TrafficValue; all: TrafficValue }

export interface AdminClient {
  password: string;
  label?: string;
  vk_hash?: string;
  ports: string;
  status: string;
  expires_at?: number;
  down_bytes?: number;
  up_bytes?: number;
  device_id?: string;
  device_name?: string;
  device_ip?: string;
  device_last_seen?: number;
  traffic: TrafficPeriod;
}

export interface AdminServerInfo {
  wdtt_plus_version: string;
  public_ip?: string;
  effective_public_ip?: string;
  dns?: string;
  default_ports: string;
  max_passwords: number;
  password_count: number;
  active_password_count: number;
  device_count: number;
  expired_count: number;
  orphan_device_count: number;
  active_connections: number;
  cpu_percent: number;
  memory_percent: number;
  network_rx_mbps: number;
  network_tx_mbps: number;
  traffic: TrafficPeriod;
}

export interface BackupSummary {
  id: string;
  created_at: number;
  reason: string;
  server_version: string;
  password_count: number;
  device_count: number;
  size_bytes: number;
  valid: boolean;
  error?: string;
}

export interface BackupStatus {
  policy: { enabled: boolean; interval_hours: number; retention_count: number };
  next_run_at?: number;
  total_bytes: number;
  backups: BackupSummary[] | null;
}

export interface AdminResponse {
  ok: boolean;
  message?: string;
  server?: AdminServerInfo;
  password?: AdminClient;
  passwords?: AdminClient[];
  backup_status?: BackupStatus;
  backup_document?: unknown;
}
