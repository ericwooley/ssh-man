export namespace bindings {

	export class CommandInitialState {
	    server: server.Server;
	    history: commandhistory.Entry[];

	    static createFrom(source: any = {}) {
	        return new CommandInitialState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = this.convertValues(source["server"], server.Server);
	        this.history = this.convertValues(source["history"], commandhistory.Entry);
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
	export class Diagnostics {
	    appDataPath: string;
	    databasePath: string;
	    version: string;

	    static createFrom(source: any = {}) {
	        return new Diagnostics(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appDataPath = source["appDataPath"];
	        this.databasePath = source["databasePath"];
	        this.version = source["version"];
	    }
	}
	export class ExplorerInitialState {
	    server: server.Server;

	    static createFrom(source: any = {}) {
	        return new ExplorerInitialState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = this.convertValues(source["server"], server.Server);
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
	export class SSHKeyOption {
	    name: string;
	    path: string;

	    static createFrom(source: any = {}) {
	        return new SSHKeyOption(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	    }
	}
	export class ServerWithConfigurations {
	    server: server.Server;
	    configurations: config.ConnectionConfiguration[];

	    static createFrom(source: any = {}) {
	        return new ServerWithConfigurations(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = this.convertValues(source["server"], server.Server);
	        this.configurations = this.convertValues(source["configurations"], config.ConnectionConfiguration);
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
	export class LoadInitialStateResult {
	    servers: ServerWithConfigurations[];
	    preferences: preferences.UserPreference;
	    sessions: any[];
	    sshKeys: SSHKeyOption[];
	    diagnostics: Diagnostics;
	    currentUsername?: string;
	    message?: string;
	    recoverable?: boolean;

	    static createFrom(source: any = {}) {
	        return new LoadInitialStateResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.servers = this.convertValues(source["servers"], ServerWithConfigurations);
	        this.preferences = this.convertValues(source["preferences"], preferences.UserPreference);
	        this.sessions = source["sessions"];
	        this.sshKeys = this.convertValues(source["sshKeys"], SSHKeyOption);
	        this.diagnostics = this.convertValues(source["diagnostics"], Diagnostics);
	        this.currentUsername = source["currentUsername"];
	        this.message = source["message"];
	        this.recoverable = source["recoverable"];
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
	export class PreviewInitialState {
	    server: server.Server;
	    remotePath: string;

	    static createFrom(source: any = {}) {
	        return new PreviewInitialState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = this.convertValues(source["server"], server.Server);
	        this.remotePath = source["remotePath"];
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

export namespace commandhistory {

	export class Entry {
	    id: string;
	    serverId: string;
	    command: string;
	    output: string;
	    exitCode: number;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    endedAt: any;
	    truncated?: boolean;
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.serverId = source["serverId"];
	        this.command = source["command"];
	        this.output = source["output"];
	        this.exitCode = source["exitCode"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.endedAt = this.convertValues(source["endedAt"], null);
	        this.truncated = source["truncated"];
	        this.error = source["error"];
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

export namespace commandrunner {

	export class Completion {
	    value: string;
	    name: string;
	    kind: string;

	    static createFrom(source: any = {}) {
	        return new Completion(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	    }
	}
	export class CompletionResult {
	    items: Completion[];

	    static createFrom(source: any = {}) {
	        return new CompletionResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Completion);
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
	export class ConnectResult {
	    connected: boolean;
	    needsPassphrase?: boolean;
	    homePath?: string;

	    static createFrom(source: any = {}) {
	        return new ConnectResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.needsPassphrase = source["needsPassphrase"];
	        this.homePath = source["homePath"];
	    }
	}

}

export namespace config {

	export class ConnectionConfiguration {
	    id: string;
	    serverId: string;
	    label: string;
	    connectionType: string;
	    localPort?: number;
	    remoteHost?: string;
	    remotePort?: number;
	    socksPort?: number;
	    autoReconnectEnabled: boolean;
	    startOnLaunch: boolean;
	    notes?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;

	    static createFrom(source: any = {}) {
	        return new ConnectionConfiguration(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.serverId = source["serverId"];
	        this.label = source["label"];
	        this.connectionType = source["connectionType"];
	        this.localPort = source["localPort"];
	        this.remoteHost = source["remoteHost"];
	        this.remotePort = source["remotePort"];
	        this.socksPort = source["socksPort"];
	        this.autoReconnectEnabled = source["autoReconnectEnabled"];
	        this.startOnLaunch = source["startOnLaunch"];
	        this.notes = source["notes"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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

export namespace preferences {

	export class BrowserAppearance {
	    icon?: string;
	    primaryColor?: string;

	    static createFrom(source: any = {}) {
	        return new BrowserAppearance(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.icon = source["icon"];
	        this.primaryColor = source["primaryColor"];
	    }
	}
	export class CustomBrowser {
	    id: string;
	    displayName: string;
	    launchReference: string;
	    engine: string;

	    static createFrom(source: any = {}) {
	        return new CustomBrowser(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.launchReference = source["launchReference"];
	        this.engine = source["engine"];
	    }
	}
	export class URLPortAssignment {
	    id: string;
	    port: number;
	    serverId: string;
	    browserId: string;

	    static createFrom(source: any = {}) {
	        return new URLPortAssignment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.port = source["port"];
	        this.serverId = source["serverId"];
	        this.browserId = source["browserId"];
	    }
	}
	export class URLRule {
	    id: string;
	    pattern: string;
	    action: string;
	    browserId?: string;
	    command?: string;

	    static createFrom(source: any = {}) {
	        return new URLRule(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.pattern = source["pattern"];
	        this.action = source["action"];
	        this.browserId = source["browserId"];
	        this.command = source["command"];
	    }
	}
	export class UserPreference {
	    theme: string;
	    lastSelectedServerId?: string;
	    browserSwitcherShortcut: string;
	    browserSwitcherBackwardShortcut: string;
	    browserAppearances: Record<string, BrowserAppearance>;
	    defaultBrowserId?: string;
	    proxyBrowserId?: string;
	    customBrowsers: CustomBrowser[];
	    urlRules: URLRule[];
	    urlPortAssignments: URLPortAssignment[];
	    // Go type: time
	    updatedAt: any;

	    static createFrom(source: any = {}) {
	        return new UserPreference(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.lastSelectedServerId = source["lastSelectedServerId"];
	        this.browserSwitcherShortcut = source["browserSwitcherShortcut"];
	        this.browserSwitcherBackwardShortcut = source["browserSwitcherBackwardShortcut"];
	        this.browserAppearances = this.convertValues(source["browserAppearances"], BrowserAppearance, true);
	        this.defaultBrowserId = source["defaultBrowserId"];
	        this.proxyBrowserId = source["proxyBrowserId"];
	        this.customBrowsers = this.convertValues(source["customBrowsers"], CustomBrowser);
	        this.urlRules = this.convertValues(source["urlRules"], URLRule);
	        this.urlPortAssignments = this.convertValues(source["urlPortAssignments"], URLPortAssignment);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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

export namespace previewwindow {

	export class State {
	    remotePath: string;
	    open: boolean;

	    static createFrom(source: any = {}) {
	        return new State(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remotePath = source["remotePath"];
	        this.open = source["open"];
	    }
	}

}

export namespace remote {

	export class ConnectResult {
	    connected: boolean;
	    needsPassphrase?: boolean;
	    homePath?: string;

	    static createFrom(source: any = {}) {
	        return new ConnectResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.needsPassphrase = source["needsPassphrase"];
	        this.homePath = source["homePath"];
	    }
	}
	export class Entry {
	    name: string;
	    path: string;
	    kind: string;
	    size: number;
	    // Go type: time
	    modifiedAt: any;
	    mode: string;
	    hidden: boolean;

	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.kind = source["kind"];
	        this.size = source["size"];
	        this.modifiedAt = this.convertValues(source["modifiedAt"], null);
	        this.mode = source["mode"];
	        this.hidden = source["hidden"];
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
	export class Directory {
	    path: string;
	    entries: Entry[];

	    static createFrom(source: any = {}) {
	        return new Directory(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.entries = this.convertValues(source["entries"], Entry);
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

	export class Preview {
	    path: string;
	    name: string;
	    kind: string;
	    mimeType: string;
	    content?: string;
	    size: number;
	    truncated?: boolean;
	    revision?: string;

	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.mimeType = source["mimeType"];
	        this.content = source["content"];
	        this.size = source["size"];
	        this.truncated = source["truncated"];
	        this.revision = source["revision"];
	    }
	}
	export class UploadFailure {
	    name: string;
	    code: string;

	    static createFrom(source: any = {}) {
	        return new UploadFailure(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.code = source["code"];
	    }
	}
	export class UploadResult {
	    uploaded: string[];
	    failures: UploadFailure[];

	    static createFrom(source: any = {}) {
	        return new UploadResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uploaded = source["uploaded"];
	        this.failures = this.convertValues(source["failures"], UploadFailure);
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

export namespace server {

	export class Server {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    socksPort: number;
	    username: string;
	    authMode: string;
	    keyReference?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;

	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.socksPort = source["socksPort"];
	        this.username = source["username"];
	        this.authMode = source["authMode"];
	        this.keyReference = source["keyReference"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
