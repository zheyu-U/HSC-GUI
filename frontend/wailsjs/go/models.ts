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
	export class CourseChapter {
	    index: number;
	    id: string;
	    title: string;
	    videos: number;
	    videosPassed: number;
	    works: number;
	
	    static createFrom(source: any = {}) {
	        return new CourseChapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.id = source["id"];
	        this.title = source["title"];
	        this.videos = source["videos"];
	        this.videosPassed = source["videosPassed"];
	        this.works = source["works"];
	    }
	}
	export class CourseWork {
	    chapterIndex: number;
	    chapterId: string;
	    title: string;
	    workId: string;
	    workType: string;
	
	    static createFrom(source: any = {}) {
	        return new CourseWork(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterIndex = source["chapterIndex"];
	        this.chapterId = source["chapterId"];
	        this.title = source["title"];
	        this.workId = source["workId"];
	        this.workType = source["workType"];
	    }
	}
	export class CourseVideo {
	    chapterIndex: number;
	    chapterId: string;
	    title: string;
	    duration: number;
	    isPassed: boolean;
	    speedLimited: boolean;
	    playTime: number;
	
	    static createFrom(source: any = {}) {
	        return new CourseVideo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterIndex = source["chapterIndex"];
	        this.chapterId = source["chapterId"];
	        this.title = source["title"];
	        this.duration = source["duration"];
	        this.isPassed = source["isPassed"];
	        this.speedLimited = source["speedLimited"];
	        this.playTime = source["playTime"];
	    }
	}
	export class CourseSummary {
	    chapters: number;
	    videos: number;
	    videosPassed: number;
	    works: number;
	    durationHours: number;
	    speedLimited: number;
	    fetchedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new CourseSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapters = source["chapters"];
	        this.videos = source["videos"];
	        this.videosPassed = source["videosPassed"];
	        this.works = source["works"];
	        this.durationHours = source["durationHours"];
	        this.speedLimited = source["speedLimited"];
	        this.fetchedAt = source["fetchedAt"];
	    }
	}
	export class Target {
	    name: string;
	    courseId: string;
	    clazzId: string;
	    cpi: string;
	    enc: string;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.courseId = source["courseId"];
	        this.clazzId = source["clazzId"];
	        this.cpi = source["cpi"];
	        this.enc = source["enc"];
	    }
	}
	export class CourseDetail {
	    hasCourse: boolean;
	    stale: boolean;
	    target: Target;
	    summary: CourseSummary;
	    chapters: CourseChapter[];
	    videos: CourseVideo[];
	    works: CourseWork[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CourseDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasCourse = source["hasCourse"];
	        this.stale = source["stale"];
	        this.target = this.convertValues(source["target"], Target);
	        this.summary = this.convertValues(source["summary"], CourseSummary);
	        this.chapters = this.convertValues(source["chapters"], CourseChapter);
	        this.videos = this.convertValues(source["videos"], CourseVideo);
	        this.works = this.convertValues(source["works"], CourseWork);
	        this.message = source["message"];
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
	export class CourseItem {
	    index: number;
	    name: string;
	    teacher: string;
	    courseId: string;
	    clazzId: string;
	    cpi: string;
	
	    static createFrom(source: any = {}) {
	        return new CourseItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.teacher = source["teacher"];
	        this.courseId = source["courseId"];
	        this.clazzId = source["clazzId"];
	        this.cpi = source["cpi"];
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
	    accountTail: string;
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
	        this.accountTail = source["accountTail"];
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

