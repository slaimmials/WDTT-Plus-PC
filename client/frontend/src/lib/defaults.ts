import type { Profile, Server } from './types';

export function newProfile(): Profile {
  return {
    id: '',
    name: '',
    host: '',
    dtlsPort: 56000,
    wgPort: 56001,
    localPort: 9000,
    password: '',
    hashes: [],
    workers: 24,
    mode: 'vpn',
    proxy: { port: 1080, lan: false, username: '', password: '', udp: true },
    streamOnly: false,
    turnSni: '',
    masque: false,
  };
}

export function newServer(): Server {
  return {
    id: '',
    name: '',
    host: '',
    sshPort: 22,
    user: 'root',
    sshPassword: '',
    privateKey: '',
    keyPassphrase: '',
    hostKey: '',
    mainPassword: '',
    dtlsPort: 56000,
    wgPort: 56001,
    localPort: 9000,
    dns: '1.1.1.1',
  };
}
