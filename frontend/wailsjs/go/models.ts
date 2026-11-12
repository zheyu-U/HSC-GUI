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
	export class CourseDoc {
	    chapterIndex: number;
	    chapterId: string;
	    title: string;
	    module: string;
	    isPassed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CourseDoc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterIndex = source["chapterIndex"];
	        this.chapterId = source["chapterId"];
	        this.title = source["title"];
	        this.module = source["module"];
	        this.isPassed = source["isPassed"];
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
	    objectId: string;
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
	        this.objectId = source["objectId"];
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
	    docs: number;
	    docsPassed: number;
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
	        this.docs = source["docs"];
	        this.docsPassed = source["docsPassed"];
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
	    docScanned: boolean;
	    target: Target;
	    summary: CourseSummary;
	    chapters: CourseChapter[];
	    videos: CourseVideo[];
	    works: CourseWork[];
	    docs: CourseDoc[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CourseDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasCourse = source["hasCourse"];
	        this.stale = source["stale"];
	        this.docScanned = source["docScanned"];
	        this.target = this.convertValues(source["target"], Target);
	        this.summary = this.convertValues(source["summary"], CourseSummary);
	        this.chapters = this.convertValues(source["chapters"], CourseChapter);
	        this.videos = this.convertValues(source["videos"], CourseVideo);
	        this.works = this.convertValues(source["works"], CourseWork);
	        this.docs = this.convertValues(source["docs"], CourseDoc);
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
	export class PlayStartResult {
	    running: boolean;
	    total: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PlayStartResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.total = source["total"];
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
	export class QuizBankInfo {
	    count: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizBankInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.message = source["message"];
	    }
	}
	export class QuizOption {
	    value: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.text = source["text"];
	    }
	}
	export class QuizQuestion {
	    qid: string;
	    typeLabel: string;
	    stem: string;
	    score: string;
	    options: QuizOption[];
	    blankCount: number;
	
	    static createFrom(source: any = {}) {
	        return new QuizQuestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.qid = source["qid"];
	        this.typeLabel = source["typeLabel"];
	        this.stem = source["stem"];
	        this.score = source["score"];
	        this.options = this.convertValues(source["options"], QuizOption);
	        this.blankCount = source["blankCount"];
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
	export class QuizDetailView {
	    workId: string;
	    title: string;
	    questions: QuizQuestion[];
	
	    static createFrom(source: any = {}) {
	        return new QuizDetailView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workId = source["workId"];
	        this.title = source["title"];
	        this.questions = this.convertValues(source["questions"], QuizQuestion);
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
	export class QuizListItem {
	    workId: string;
	    title: string;
	    chapterIndex: number;
	    chapterId: string;
	    questionCount: number;
	    fetched: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QuizListItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workId = source["workId"];
	        this.title = source["title"];
	        this.chapterIndex = source["chapterIndex"];
	        this.chapterId = source["chapterId"];
	        this.questionCount = source["questionCount"];
	        this.fetched = source["fetched"];
	    }
	}
	
	export class QuizOverview {
	    scanned: boolean;
	    fetchedCount: number;
	    questionCount: number;
	    items: QuizListItem[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scanned = source["scanned"];
	        this.fetchedCount = source["fetchedCount"];
	        this.questionCount = source["questionCount"];
	        this.items = this.convertValues(source["items"], QuizListItem);
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
	export class QuizPeekItem {
	    index: number;
	    title: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizPeekItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.title = source["title"];
	        this.status = source["status"];
	    }
	}
	
	export class QuizSettings {
	    enabled: boolean;
	    baseUrl: string;
	    apiKeyMasked: string;
	    apiKey: string;
	    model: string;
	    temperature: number;
	    maxTokens: number;
	    timeoutSeconds: number;
	    autoSubmit: boolean;
	    types: string[];
	    delayMs: number;
	    retries: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKeyMasked = source["apiKeyMasked"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	        this.temperature = source["temperature"];
	        this.maxTokens = source["maxTokens"];
	        this.timeoutSeconds = source["timeoutSeconds"];
	        this.autoSubmit = source["autoSubmit"];
	        this.types = source["types"];
	        this.delayMs = source["delayMs"];
	        this.retries = source["retries"];
	        this.message = source["message"];
	    }
	}
	export class QuizSolveResult {
	    total: number;
	    ok: number;
	    cached: number;
	    failed: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizSolveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.ok = source["ok"];
	        this.cached = source["cached"];
	        this.failed = source["failed"];
	        this.message = source["message"];
	    }
	}
	export class QuizSubmitResult {
	    ok: number;
	    skipped: number;
	    failed: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new QuizSubmitResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.message = source["message"];
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

