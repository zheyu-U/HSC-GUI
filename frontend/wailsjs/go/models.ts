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

