export namespace applog {
	
	export class Entry {
	    time: number;
	    message: string;
	    level: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.message = source["message"];
	        this.level = source["level"];
	    }
	}

}

export namespace connection {
	
	export class InternetCheck {
	    state: string;
	    ip: string;
	    country: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new InternetCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.ip = source["ip"];
	        this.country = source["country"];
	        this.message = source["message"];
	    }
	}
	export class Status {
	    state: string;
	    profileId: string;
	    mode: string;
	    error: string;
	    warning: string;
	    internet: InternetCheck;
	    reconnecting: boolean;
	    proxyAddresses: string[];
	    stats: core.Stats;
	    connectedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.profileId = source["profileId"];
	        this.mode = source["mode"];
	        this.error = source["error"];
	        this.warning = source["warning"];
	        this.internet = this.convertValues(source["internet"], InternetCheck);
	        this.reconnecting = source["reconnecting"];
	        this.proxyAddresses = source["proxyAddresses"];
	        this.stats = this.convertValues(source["stats"], core.Stats);
	        this.connectedAt = source["connectedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace core {
	
	export class Stats {
	    ActiveConnections: number;
	    BytesUp: number;
	    BytesDown: number;
	    LatencyMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ActiveConnections = source["ActiveConnections"];
	        this.BytesUp = source["BytesUp"];
	        this.BytesDown = source["BytesDown"];
	        this.LatencyMs = source["LatencyMs"];
	    }
	}

}

export namespace main {
	
	export class State {
	    version: string;
	    elevated: boolean;
	    settings: profile.Settings;
	    status: connection.Status;
	    pendingLink: string;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.elevated = source["elevated"];
	        this.settings = this.convertValues(source["settings"], profile.Settings);
	        this.status = this.convertValues(source["status"], connection.Status);
	        this.pendingLink = source["pendingLink"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace profile {
	
	export class Proxy {
	    port: number;
	    lan: boolean;
	    username: string;
	    password: string;
	    udp: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Proxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.lan = source["lan"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.udp = source["udp"];
	    }
	}
	export class Profile {
	    id: string;
	    name: string;
	    host: string;
	    dtlsPort: number;
	    wgPort: number;
	    localPort: number;
	    password: string;
	    hashes: string[];
	    workers: number;
	    mode: string;
	    proxy: Proxy;
	    streamOnly: boolean;
	    turnSni: string;
	    masque: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.dtlsPort = source["dtlsPort"];
	        this.wgPort = source["wgPort"];
	        this.localPort = source["localPort"];
	        this.password = source["password"];
	        this.hashes = source["hashes"];
	        this.workers = source["workers"];
	        this.mode = source["mode"];
	        this.proxy = this.convertValues(source["proxy"], Proxy);
	        this.streamOnly = source["streamOnly"];
	        this.turnSni = source["turnSni"];
	        this.masque = source["masque"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Routing {
	    mode: string;
	    exclude: string[];
	    include: string[];
	    bypassLan: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Routing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.exclude = source["exclude"];
	        this.include = source["include"];
	        this.bypassLan = source["bypassLan"];
	    }
	}
	export class Server {
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
	
	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.sshPort = source["sshPort"];
	        this.user = source["user"];
	        this.sshPassword = source["sshPassword"];
	        this.privateKey = source["privateKey"];
	        this.keyPassphrase = source["keyPassphrase"];
	        this.hostKey = source["hostKey"];
	        this.mainPassword = source["mainPassword"];
	        this.dtlsPort = source["dtlsPort"];
	        this.wgPort = source["wgPort"];
	        this.localPort = source["localPort"];
	        this.dns = source["dns"];
	    }
	}
	export class VK {
	    fingerprint: string;
	    clientIds: string;
	    customClientId: string;
	    customSecret: string;
	    vkCallsPreflight: boolean;
	    hashFallback: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VK(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fingerprint = source["fingerprint"];
	        this.clientIds = source["clientIds"];
	        this.customClientId = source["customClientId"];
	        this.customSecret = source["customSecret"];
	        this.vkCallsPreflight = source["vkCallsPreflight"];
	        this.hashFallback = source["hashFallback"];
	    }
	}
	export class Settings {
	    profiles: Profile[];
	    activeProfile: string;
	    servers: Server[];
	    routing: Routing;
	    vk: VK;
	    deviceId: string;
	    theme: string;
	    diagnostics: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profiles = this.convertValues(source["profiles"], Profile);
	        this.activeProfile = source["activeProfile"];
	        this.servers = this.convertValues(source["servers"], Server);
	        this.routing = this.convertValues(source["routing"], Routing);
	        this.vk = this.convertValues(source["vk"], VK);
	        this.deviceId = source["deviceId"];
	        this.theme = source["theme"];
	        this.diagnostics = source["diagnostics"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

