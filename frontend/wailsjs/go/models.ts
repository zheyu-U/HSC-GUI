export namespace hsc {
	
	export class CaptchaView {
	    image: string;
	    suggestedCode: string;
	    confidence: number;
	    count: number;
	    hint: string;
	
	    static createFrom(source: any = {}) {
	        return new CaptchaView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.image = source["image"];
	        this.suggestedCode = source["suggestedCode"];
	        this.confidence = source["confidence"];
	        this.count = source["count"];
	        this.hint = source["hint"];
	    }
	}
	export class CheckResult {
	    ok: boolean;
	    checked: boolean;
	    hasTarget: boolean;
	    chapters: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.checked = source["checked"];
	        this.hasTarget = source["hasTarget"];
	        this.chapters = source["chapters"];
	        this.message = source["message"];
	    }
	}
	export class LoginResult {
	    ok: boolean;
	    message: string;
	    credSaved: boolean;
	    need2fa: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LoginResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.credSaved = source["credSaved"];
	        this.need2fa = source["need2fa"];
	    }
	}
	export class LogoutResult {
	    removed: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogoutResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.removed = source["removed"];
	        this.message = source["message"];
	    }
	}
	export class QRLoginResult {
	    image: string;
	    expiresAt: number;
	
	    static createFrom(source: any = {}) {
	        return new QRLoginResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.image = source["image"];
	        this.expiresAt = source["expiresAt"];
	    }
	}
	export class SessionInfo {
	    loggedIn: boolean;
	    cookieCount: number;
	    account: string;
	    credSaved: boolean;
	    credEncrypted: boolean;
	    hasTarget: boolean;
	    stateDir: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loggedIn = source["loggedIn"];
	        this.cookieCount = source["cookieCount"];
	        this.account = source["account"];
	        this.credSaved = source["credSaved"];
	        this.credEncrypted = source["credEncrypted"];
	        this.hasTarget = source["hasTarget"];
	        this.stateDir = source["stateDir"];
	        this.message = source["message"];
	    }
	}

}

export namespace main {
	
	export class AppModeInfo {
	    mode: string;
	    debug: boolean;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new AppModeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.debug = source["debug"];
	        this.version = source["version"];
	    }
	}
	export class DebugEntry {
	    time: string;
	    kind: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DebugEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.kind = source["kind"];
	        this.message = source["message"];
	    }
	}
	export class DebugInfo {
	    mode: string;
	    stateDir: string;
	    cwd: string;
	    exePath: string;
	    goVersion: string;
	    os: string;
	    arch: string;
	    cookieCount: number;
	    loggedIn: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DebugInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.stateDir = source["stateDir"];
	        this.cwd = source["cwd"];
	        this.exePath = source["exePath"];
	        this.goVersion = source["goVersion"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.cookieCount = source["cookieCount"];
	        this.loggedIn = source["loggedIn"];
	    }
	}

}

