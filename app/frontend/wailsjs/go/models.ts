export namespace cachecleanup {
	
	export class Info {
	    totalBytes: number;
	    totalDirs: number;
	    orphanedDirs: number;
	    orphanedBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalBytes = source["totalBytes"];
	        this.totalDirs = source["totalDirs"];
	        this.orphanedDirs = source["orphanedDirs"];
	        this.orphanedBytes = source["orphanedBytes"];
	    }
	}
	export class Result {
	    mode: string;
	    removedDirs: number;
	    freedBytes: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.removedDirs = source["removedDirs"];
	        this.freedBytes = source["freedBytes"];
	        this.message = source["message"];
	    }
	}

}

export namespace dialog {
	
	export class FileDialogRequest {
	    title: string;
	    filterName: string;
	    filterGlob: string;
	    directory: boolean;
	    defaultDirectory: string;
	    defaultFilename: string;
	
	    static createFrom(source: any = {}) {
	        return new FileDialogRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.filterName = source["filterName"];
	        this.filterGlob = source["filterGlob"];
	        this.directory = source["directory"];
	        this.defaultDirectory = source["defaultDirectory"];
	        this.defaultFilename = source["defaultFilename"];
	    }
	}

}

export namespace execution {
	
	export class RemoteExecRequest {
	    toolId: string;
	    connId: string;
	    args: string;
	    pythonEnv: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteExecRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toolId = source["toolId"];
	        this.connId = source["connId"];
	        this.args = source["args"];
	        this.pythonEnv = source["pythonEnv"];
	    }
	}

}

export namespace exportpkg {
	
	export class ExportToolRequest {
	    toolId: string;
	    mode?: string;
	    targetOS?: string;
	    targetArch?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportToolRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toolId = source["toolId"];
	        this.mode = source["mode"];
	        this.targetOS = source["targetOS"];
	        this.targetArch = source["targetArch"];
	    }
	}
	export class ExportToolResult {
	    toolId: string;
	    toolName: string;
	    strategy: string;
	    mode: string;
	    filePath: string;
	    directory: string;
	    targetOS?: string;
	    targetArch?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportToolResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toolId = source["toolId"];
	        this.toolName = source["toolName"];
	        this.strategy = source["strategy"];
	        this.mode = source["mode"];
	        this.filePath = source["filePath"];
	        this.directory = source["directory"];
	        this.targetOS = source["targetOS"];
	        this.targetArch = source["targetArch"];
	    }
	}

}

export namespace gosettings {
	
	export class GoOfficialRelease {
	    version: string;
	    stable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GoOfficialRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.stable = source["stable"];
	    }
	}
	export class GoRuntimeDetails {
	    goroot: string;
	    gopath: string;
	    goos: string;
	    goarch: string;
	    goversion: string;
	
	    static createFrom(source: any = {}) {
	        return new GoRuntimeDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.goroot = source["goroot"];
	        this.gopath = source["gopath"];
	        this.goos = source["goos"];
	        this.goarch = source["goarch"];
	        this.goversion = source["goversion"];
	    }
	}
	export class GoToolchainCandidate {
	    path: string;
	    version: string;
	    source: string;
	    label: string;
	    detail: string;
	    error?: string;
	    valid: boolean;
	    selected: boolean;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GoToolchainCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.label = source["label"];
	        this.detail = source["detail"];
	        this.error = source["error"];
	        this.valid = source["valid"];
	        this.selected = source["selected"];
	        this.active = source["active"];
	    }
	}
	export class GoToolchainConfig {
	    selectedBinary: string;
	    knownBinaries: string[];
	    lastInstallDirectory: string;
	    disabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GoToolchainConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selectedBinary = source["selectedBinary"];
	        this.knownBinaries = source["knownBinaries"];
	        this.lastInstallDirectory = source["lastInstallDirectory"];
	        this.disabled = source["disabled"];
	    }
	}
	export class GoToolchainState {
	    config: GoToolchainConfig;
	    candidates: GoToolchainCandidate[];
	    hasUsableBinary: boolean;
	    activeBinary: string;
	    activeVersion: string;
	    activeSource: string;
	    runtimeDetails: GoRuntimeDetails;
	    statusMessage: string;
	    suggestedInstallDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new GoToolchainState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], GoToolchainConfig);
	        this.candidates = this.convertValues(source["candidates"], GoToolchainCandidate);
	        this.hasUsableBinary = source["hasUsableBinary"];
	        this.activeBinary = source["activeBinary"];
	        this.activeVersion = source["activeVersion"];
	        this.activeSource = source["activeSource"];
	        this.runtimeDetails = this.convertValues(source["runtimeDetails"], GoRuntimeDetails);
	        this.statusMessage = source["statusMessage"];
	        this.suggestedInstallDirectory = source["suggestedInstallDirectory"];
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
	export class GoToolchainTaskState {
	    kind: string;
	    status: string;
	    message: string;
	    detail?: string;
	    currentItem?: string;
	    currentSource?: string;
	    progressPercent: number;
	    step: number;
	    totalSteps: number;
	    version?: string;
	    directory?: string;
	    transferredBytes?: number;
	    totalBytes?: number;
	    transferSpeed?: string;
	    error?: string;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new GoToolchainTaskState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.detail = source["detail"];
	        this.currentItem = source["currentItem"];
	        this.currentSource = source["currentSource"];
	        this.progressPercent = source["progressPercent"];
	        this.step = source["step"];
	        this.totalSteps = source["totalSteps"];
	        this.version = source["version"];
	        this.directory = source["directory"];
	        this.transferredBytes = source["transferredBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.transferSpeed = source["transferSpeed"];
	        this.error = source["error"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class InstallGoToolchainRequest {
	    version: string;
	    directory: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallGoToolchainRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.directory = source["directory"];
	    }
	}

}

export namespace main {
	
	export class WindowState {
	    width: number;
	    height: number;
	    x: number;
	    y: number;
	    maximised: boolean;
	    fullscreen: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WindowState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.maximised = source["maximised"];
	        this.fullscreen = source["fullscreen"];
	    }
	}
	export class WorkbenchBootstrap {
	    appTitle: string;
	    platform: string;
	    hostStack: string[];
	    primaryFlow: string[];
	    moduleBoundaries: string[];
	    parameterModes: string[];
	    tools: toolspec.ToolManifest[];
	
	    static createFrom(source: any = {}) {
	        return new WorkbenchBootstrap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appTitle = source["appTitle"];
	        this.platform = source["platform"];
	        this.hostStack = source["hostStack"];
	        this.primaryFlow = source["primaryFlow"];
	        this.moduleBoundaries = source["moduleBoundaries"];
	        this.parameterModes = source["parameterModes"];
	        this.tools = this.convertValues(source["tools"], toolspec.ToolManifest);
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

export namespace pythonsettings {
	
	export class PythonDependencyStatus {
	    packageName: string;
	    moduleName: string;
	    installed: boolean;
	    version?: string;
	    error?: string;
	    requiredBy: string[];
	
	    static createFrom(source: any = {}) {
	        return new PythonDependencyStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packageName = source["packageName"];
	        this.moduleName = source["moduleName"];
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.error = source["error"];
	        this.requiredBy = source["requiredBy"];
	    }
	}
	export class PythonToolchainCandidate {
	    path: string;
	    version: string;
	    source: string;
	    label: string;
	    detail: string;
	    error?: string;
	    valid: boolean;
	    selected: boolean;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PythonToolchainCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.label = source["label"];
	        this.detail = source["detail"];
	        this.error = source["error"];
	        this.valid = source["valid"];
	        this.selected = source["selected"];
	        this.active = source["active"];
	    }
	}
	export class PythonToolchainConfig {
	    selectedBinary: string;
	    knownBinaries: string[];
	    disabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PythonToolchainConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selectedBinary = source["selectedBinary"];
	        this.knownBinaries = source["knownBinaries"];
	        this.disabled = source["disabled"];
	    }
	}
	export class PythonToolchainState {
	    config: PythonToolchainConfig;
	    candidates: PythonToolchainCandidate[];
	    hasUsableBaseBinary: boolean;
	    activeBaseBinary: string;
	    activeBaseVersion: string;
	    activeBaseSource: string;
	    hasUsableBinary: boolean;
	    activeBinary: string;
	    activeVersion: string;
	    activeSource: string;
	    pipAvailable: boolean;
	    dependenciesReady: boolean;
	    missingPackages: string[];
	    statusMessage: string;
	    dependencies: PythonDependencyStatus[];
	    dependencyToolCount: number;
	    dependencyTotalCount: number;
	    managedEnvDirectory: string;
	    needsRebuild: boolean;
	    managedBaseBinary: string;
	    managedBaseVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new PythonToolchainState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], PythonToolchainConfig);
	        this.candidates = this.convertValues(source["candidates"], PythonToolchainCandidate);
	        this.hasUsableBaseBinary = source["hasUsableBaseBinary"];
	        this.activeBaseBinary = source["activeBaseBinary"];
	        this.activeBaseVersion = source["activeBaseVersion"];
	        this.activeBaseSource = source["activeBaseSource"];
	        this.hasUsableBinary = source["hasUsableBinary"];
	        this.activeBinary = source["activeBinary"];
	        this.activeVersion = source["activeVersion"];
	        this.activeSource = source["activeSource"];
	        this.pipAvailable = source["pipAvailable"];
	        this.dependenciesReady = source["dependenciesReady"];
	        this.missingPackages = source["missingPackages"];
	        this.statusMessage = source["statusMessage"];
	        this.dependencies = this.convertValues(source["dependencies"], PythonDependencyStatus);
	        this.dependencyToolCount = source["dependencyToolCount"];
	        this.dependencyTotalCount = source["dependencyTotalCount"];
	        this.managedEnvDirectory = source["managedEnvDirectory"];
	        this.needsRebuild = source["needsRebuild"];
	        this.managedBaseBinary = source["managedBaseBinary"];
	        this.managedBaseVersion = source["managedBaseVersion"];
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
	export class PythonToolchainTaskState {
	    kind: string;
	    status: string;
	    message: string;
	    detail?: string;
	    currentItem?: string;
	    progressPercent: number;
	    step: number;
	    totalSteps: number;
	    baseBinary?: string;
	    environmentDirectory?: string;
	    error?: string;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new PythonToolchainTaskState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.detail = source["detail"];
	        this.currentItem = source["currentItem"];
	        this.progressPercent = source["progressPercent"];
	        this.step = source["step"];
	        this.totalSteps = source["totalSteps"];
	        this.baseBinary = source["baseBinary"];
	        this.environmentDirectory = source["environmentDirectory"];
	        this.error = source["error"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

export namespace rustsettings {
	
	export class InstallRustToolchainRequest {
	    rustVersion: string;
	    zigVersion: string;
	    directory: string;
	
	    static createFrom(source: any = {}) {
	        return new InstallRustToolchainRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rustVersion = source["rustVersion"];
	        this.zigVersion = source["zigVersion"];
	        this.directory = source["directory"];
	    }
	}
	export class RustOfficialRelease {
	    version: string;
	    stable: boolean;
	    channel: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RustOfficialRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.stable = source["stable"];
	        this.channel = source["channel"];
	    }
	}
	export class RustToolchainCandidate {
	    path: string;
	    version: string;
	    source: string;
	    label: string;
	    detail: string;
	    error?: string;
	    valid: boolean;
	    selected: boolean;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RustToolchainCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.label = source["label"];
	        this.detail = source["detail"];
	        this.error = source["error"];
	        this.valid = source["valid"];
	        this.selected = source["selected"];
	        this.active = source["active"];
	    }
	}
	export class RustToolchainConfig {
	    mode: string;
	    selectedRustRoot: string;
	    knownRustRoots: string[];
	    selectedZigBinary: string;
	    knownZigBinaries: string[];
	    lastInstallDirectory: string;
	    disabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RustToolchainConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.selectedRustRoot = source["selectedRustRoot"];
	        this.knownRustRoots = source["knownRustRoots"];
	        this.selectedZigBinary = source["selectedZigBinary"];
	        this.knownZigBinaries = source["knownZigBinaries"];
	        this.lastInstallDirectory = source["lastInstallDirectory"];
	        this.disabled = source["disabled"];
	    }
	}
	export class RustToolchainEnvironment {
	    rootDir: string;
	    version: string;
	    source: string;
	    label: string;
	    detail: string;
	    error?: string;
	    valid: boolean;
	    selected: boolean;
	    active: boolean;
	    cargoBinary?: string;
	    rustupBinary?: string;
	    rustcBinary?: string;
	    cargoZigbuildBinary?: string;
	    hasRustup: boolean;
	    hasCargoZigbuild: boolean;
	    canManageTargets: boolean;
	    managed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RustToolchainEnvironment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rootDir = source["rootDir"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.label = source["label"];
	        this.detail = source["detail"];
	        this.error = source["error"];
	        this.valid = source["valid"];
	        this.selected = source["selected"];
	        this.active = source["active"];
	        this.cargoBinary = source["cargoBinary"];
	        this.rustupBinary = source["rustupBinary"];
	        this.rustcBinary = source["rustcBinary"];
	        this.cargoZigbuildBinary = source["cargoZigbuildBinary"];
	        this.hasRustup = source["hasRustup"];
	        this.hasCargoZigbuild = source["hasCargoZigbuild"];
	        this.canManageTargets = source["canManageTargets"];
	        this.managed = source["managed"];
	    }
	}
	export class RustToolchainTargetStatus {
	    platformKey: string;
	    platformLabel: string;
	    targetTriple: string;
	    installed: boolean;
	    native: boolean;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new RustToolchainTargetStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platformKey = source["platformKey"];
	        this.platformLabel = source["platformLabel"];
	        this.targetTriple = source["targetTriple"];
	        this.installed = source["installed"];
	        this.native = source["native"];
	        this.note = source["note"];
	    }
	}
	export class RustToolchainState {
	    config: RustToolchainConfig;
	    rustCandidates: RustToolchainEnvironment[];
	    zigCandidates: RustToolchainCandidate[];
	    installedTargets: string[];
	    targetStatuses: RustToolchainTargetStatus[];
	    hasInstalledTargetInfo: boolean;
	    hasFullTargetCoverage: boolean;
	    targetStatusMessage: string;
	    cargoZigbuildStatusMessage: string;
	    hasUsableEnvironment: boolean;
	    hasUsableRust: boolean;
	    hasUsableCargo: boolean;
	    hasUsableRustup: boolean;
	    hasUsableZig: boolean;
	    hasUsableCargoZigbuild: boolean;
	    canManageTargets: boolean;
	    canManageCargoZigbuild: boolean;
	    activeRustRoot: string;
	    activeRustVersion: string;
	    activeRustSource: string;
	    activeRustManaged: boolean;
	    activeCargoBinary: string;
	    activeCargoVersion: string;
	    activeCargoSource: string;
	    activeRustupBinary: string;
	    activeRustupVersion: string;
	    activeRustupSource: string;
	    activeRustcBinary: string;
	    activeZigBinary: string;
	    activeZigVersion: string;
	    activeZigSource: string;
	    activeCargoZigbuildBinary: string;
	    activeCargoZigbuildVersion: string;
	    activeCargoZigbuildSource: string;
	    statusMessage: string;
	    suggestedInstallDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new RustToolchainState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], RustToolchainConfig);
	        this.rustCandidates = this.convertValues(source["rustCandidates"], RustToolchainEnvironment);
	        this.zigCandidates = this.convertValues(source["zigCandidates"], RustToolchainCandidate);
	        this.installedTargets = source["installedTargets"];
	        this.targetStatuses = this.convertValues(source["targetStatuses"], RustToolchainTargetStatus);
	        this.hasInstalledTargetInfo = source["hasInstalledTargetInfo"];
	        this.hasFullTargetCoverage = source["hasFullTargetCoverage"];
	        this.targetStatusMessage = source["targetStatusMessage"];
	        this.cargoZigbuildStatusMessage = source["cargoZigbuildStatusMessage"];
	        this.hasUsableEnvironment = source["hasUsableEnvironment"];
	        this.hasUsableRust = source["hasUsableRust"];
	        this.hasUsableCargo = source["hasUsableCargo"];
	        this.hasUsableRustup = source["hasUsableRustup"];
	        this.hasUsableZig = source["hasUsableZig"];
	        this.hasUsableCargoZigbuild = source["hasUsableCargoZigbuild"];
	        this.canManageTargets = source["canManageTargets"];
	        this.canManageCargoZigbuild = source["canManageCargoZigbuild"];
	        this.activeRustRoot = source["activeRustRoot"];
	        this.activeRustVersion = source["activeRustVersion"];
	        this.activeRustSource = source["activeRustSource"];
	        this.activeRustManaged = source["activeRustManaged"];
	        this.activeCargoBinary = source["activeCargoBinary"];
	        this.activeCargoVersion = source["activeCargoVersion"];
	        this.activeCargoSource = source["activeCargoSource"];
	        this.activeRustupBinary = source["activeRustupBinary"];
	        this.activeRustupVersion = source["activeRustupVersion"];
	        this.activeRustupSource = source["activeRustupSource"];
	        this.activeRustcBinary = source["activeRustcBinary"];
	        this.activeZigBinary = source["activeZigBinary"];
	        this.activeZigVersion = source["activeZigVersion"];
	        this.activeZigSource = source["activeZigSource"];
	        this.activeCargoZigbuildBinary = source["activeCargoZigbuildBinary"];
	        this.activeCargoZigbuildVersion = source["activeCargoZigbuildVersion"];
	        this.activeCargoZigbuildSource = source["activeCargoZigbuildSource"];
	        this.statusMessage = source["statusMessage"];
	        this.suggestedInstallDirectory = source["suggestedInstallDirectory"];
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
	
	export class RustToolchainTaskState {
	    kind: string;
	    status: string;
	    message: string;
	    detail?: string;
	    currentItem?: string;
	    currentSource?: string;
	    progressPercent: number;
	    step: number;
	    totalSteps: number;
	    rustVersion?: string;
	    zigVersion?: string;
	    directory?: string;
	    transferredBytes?: number;
	    totalBytes?: number;
	    transferSpeed?: string;
	    error?: string;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new RustToolchainTaskState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.detail = source["detail"];
	        this.currentItem = source["currentItem"];
	        this.currentSource = source["currentSource"];
	        this.progressPercent = source["progressPercent"];
	        this.step = source["step"];
	        this.totalSteps = source["totalSteps"];
	        this.rustVersion = source["rustVersion"];
	        this.zigVersion = source["zigVersion"];
	        this.directory = source["directory"];
	        this.transferredBytes = source["transferredBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.transferSpeed = source["transferSpeed"];
	        this.error = source["error"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ZigOfficialRelease {
	    version: string;
	    stable: boolean;
	    date?: string;
	
	    static createFrom(source: any = {}) {
	        return new ZigOfficialRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.stable = source["stable"];
	        this.date = source["date"];
	    }
	}

}

export namespace shared {
	
	export class ArtifactBatchEstimate {
	    totalCount: number;
	    cachedCount: number;
	    buildCount: number;
	    invalidCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ArtifactBatchEstimate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalCount = source["totalCount"];
	        this.cachedCount = source["cachedCount"];
	        this.buildCount = source["buildCount"];
	        this.invalidCount = source["invalidCount"];
	    }
	}
	export class ArtifactBatchItemResult {
	    key: string;
	    toolId: string;
	    toolName: string;
	    kind: string;
	    targetOS: string;
	    targetArch: string;
	    status: string;
	    message: string;
	    outputPath?: string;
	    cacheHit: boolean;
	    startedAt: number;
	    endedAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new ArtifactBatchItemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.toolId = source["toolId"];
	        this.toolName = source["toolName"];
	        this.kind = source["kind"];
	        this.targetOS = source["targetOS"];
	        this.targetArch = source["targetArch"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.outputPath = source["outputPath"];
	        this.cacheHit = source["cacheHit"];
	        this.startedAt = source["startedAt"];
	        this.endedAt = source["endedAt"];
	    }
	}
	export class ArtifactBatchSelection {
	    toolId: string;
	    targetOS: string;
	    targetArch: string;
	
	    static createFrom(source: any = {}) {
	        return new ArtifactBatchSelection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toolId = source["toolId"];
	        this.targetOS = source["targetOS"];
	        this.targetArch = source["targetArch"];
	    }
	}
	export class ArtifactBatchRequest {
	    mode: string;
	    exportRootDir?: string;
	    concurrency: number;
	    skipUnchanged: boolean;
	    preferCache: boolean;
	    forceRebuild: boolean;
	    continueOnError: boolean;
	    items: ArtifactBatchSelection[];
	
	    static createFrom(source: any = {}) {
	        return new ArtifactBatchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.exportRootDir = source["exportRootDir"];
	        this.concurrency = source["concurrency"];
	        this.skipUnchanged = source["skipUnchanged"];
	        this.preferCache = source["preferCache"];
	        this.forceRebuild = source["forceRebuild"];
	        this.continueOnError = source["continueOnError"];
	        this.items = this.convertValues(source["items"], ArtifactBatchSelection);
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
	
	export class ArtifactBatchTask {
	    id: string;
	    mode: string;
	    status: string;
	    exportRootDir?: string;
	    concurrency: number;
	    skipUnchanged: boolean;
	    preferCache: boolean;
	    forceRebuild: boolean;
	    continueOnError: boolean;
	    totalCount: number;
	    successCount: number;
	    errorCount: number;
	    cachedCount: number;
	    skippedCount: number;
	    startedAt: number;
	    endedAt?: number;
	    currentItem?: string;
	    exitMessage?: string;
	    items: ArtifactBatchItemResult[];
	
	    static createFrom(source: any = {}) {
	        return new ArtifactBatchTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.mode = source["mode"];
	        this.status = source["status"];
	        this.exportRootDir = source["exportRootDir"];
	        this.concurrency = source["concurrency"];
	        this.skipUnchanged = source["skipUnchanged"];
	        this.preferCache = source["preferCache"];
	        this.forceRebuild = source["forceRebuild"];
	        this.continueOnError = source["continueOnError"];
	        this.totalCount = source["totalCount"];
	        this.successCount = source["successCount"];
	        this.errorCount = source["errorCount"];
	        this.cachedCount = source["cachedCount"];
	        this.skippedCount = source["skippedCount"];
	        this.startedAt = source["startedAt"];
	        this.endedAt = source["endedAt"];
	        this.currentItem = source["currentItem"];
	        this.exitMessage = source["exitMessage"];
	        this.items = this.convertValues(source["items"], ArtifactBatchItemResult);
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
	export class DownloadTask {
	    id: string;
	    sourceTaskId: string;
	    toolId: string;
	    toolName: string;
	    status: string;
	    remoteResultPath: string;
	    remoteResultKind: string;
	    localPath?: string;
	    directory?: string;
	    message?: string;
	    downloadedBytes: number;
	    totalBytes: number;
	    progressPercent: number;
	    startedAt: number;
	    endedAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new DownloadTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sourceTaskId = source["sourceTaskId"];
	        this.toolId = source["toolId"];
	        this.toolName = source["toolName"];
	        this.status = source["status"];
	        this.remoteResultPath = source["remoteResultPath"];
	        this.remoteResultKind = source["remoteResultKind"];
	        this.localPath = source["localPath"];
	        this.directory = source["directory"];
	        this.message = source["message"];
	        this.downloadedBytes = source["downloadedBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.progressPercent = source["progressPercent"];
	        this.startedAt = source["startedAt"];
	        this.endedAt = source["endedAt"];
	    }
	}
	export class ExecutionRequest {
	    toolId: string;
	    args: string;
	    pythonEnv: string;
	
	    static createFrom(source: any = {}) {
	        return new ExecutionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toolId = source["toolId"];
	        this.args = source["args"];
	        this.pythonEnv = source["pythonEnv"];
	    }
	}
	export class ExecutionTask {
	    id: string;
	    toolId: string;
	    toolName: string;
	    status: string;
	    target: string;
	    remoteConnId?: string;
	    args: string;
	    pythonEnv?: string;
	    usage: string;
	    startedAt: number;
	    endedAt?: number;
	    exitMessage?: string;
	    remoteResultStatus?: string;
	    remoteResultPath?: string;
	    remoteResultKind?: string;
	    remoteResultMessage?: string;
	    remoteResultDownloadedPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExecutionTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.toolId = source["toolId"];
	        this.toolName = source["toolName"];
	        this.status = source["status"];
	        this.target = source["target"];
	        this.remoteConnId = source["remoteConnId"];
	        this.args = source["args"];
	        this.pythonEnv = source["pythonEnv"];
	        this.usage = source["usage"];
	        this.startedAt = source["startedAt"];
	        this.endedAt = source["endedAt"];
	        this.exitMessage = source["exitMessage"];
	        this.remoteResultStatus = source["remoteResultStatus"];
	        this.remoteResultPath = source["remoteResultPath"];
	        this.remoteResultKind = source["remoteResultKind"];
	        this.remoteResultMessage = source["remoteResultMessage"];
	        this.remoteResultDownloadedPath = source["remoteResultDownloadedPath"];
	    }
	}

}

export namespace ssh {
	
	export class Connection {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    user: string;
	    authMethod: string;
	    password?: string;
	    keyPath?: string;
	    description: string;
	    hostKeyFingerprint?: string;
	    hostKeyAlgorithm?: string;
	    lastUsedAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.authMethod = source["authMethod"];
	        this.password = source["password"];
	        this.keyPath = source["keyPath"];
	        this.description = source["description"];
	        this.hostKeyFingerprint = source["hostKeyFingerprint"];
	        this.hostKeyAlgorithm = source["hostKeyAlgorithm"];
	        this.lastUsedAt = source["lastUsedAt"];
	    }
	}
	export class TestResult {
	    success: boolean;
	    message: string;
	    acceptedFingerprint?: string;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.acceptedFingerprint = source["acceptedFingerprint"];
	    }
	}

}

export namespace toolspec {
	
	export class RemoteExecutionSpec {
	    strategy: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteExecutionSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategy = source["strategy"];
	    }
	}
	export class LocalExecutionSpec {
	    adapter: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalExecutionSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adapter = source["adapter"];
	    }
	}
	export class ExecutionSpec {
	    local: LocalExecutionSpec;
	    remote: RemoteExecutionSpec;
	
	    static createFrom(source: any = {}) {
	        return new ExecutionSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.local = this.convertValues(source["local"], LocalExecutionSpec);
	        this.remote = this.convertValues(source["remote"], RemoteExecutionSpec);
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
	export class ExportSpec {
	    strategy: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategy = source["strategy"];
	    }
	}
	
	export class ParameterCondition {
	    key: string;
	    equals?: any;
	
	    static createFrom(source: any = {}) {
	        return new ParameterCondition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.equals = source["equals"];
	    }
	}
	export class ParameterOption {
	    label: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new ParameterOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.value = source["value"];
	    }
	}
	export class ParameterVisibility {
	    all?: ParameterCondition[];
	    any?: ParameterCondition[];
	
	    static createFrom(source: any = {}) {
	        return new ParameterVisibility(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.all = this.convertValues(source["all"], ParameterCondition);
	        this.any = this.convertValues(source["any"], ParameterCondition);
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
	export class ParameterSpec {
	    key: string;
	    argKey?: string;
	    type: string;
	    label: string;
	    placeholder?: string;
	    required?: boolean;
	    default?: any;
	    help?: string;
	    options?: ParameterOption[];
	    group?: string;
	    pathMode?: string;
	    repeatable?: boolean;
	    emit?: boolean;
	    visibleWhen?: ParameterVisibility;
	
	    static createFrom(source: any = {}) {
	        return new ParameterSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.argKey = source["argKey"];
	        this.type = source["type"];
	        this.label = source["label"];
	        this.placeholder = source["placeholder"];
	        this.required = source["required"];
	        this.default = source["default"];
	        this.help = source["help"];
	        this.options = this.convertValues(source["options"], ParameterOption);
	        this.group = source["group"];
	        this.pathMode = source["pathMode"];
	        this.repeatable = source["repeatable"];
	        this.emit = source["emit"];
	        this.visibleWhen = this.convertValues(source["visibleWhen"], ParameterVisibility);
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
	
	
	export class SourceSpec {
	    entry: string;
	
	    static createFrom(source: any = {}) {
	        return new SourceSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entry = source["entry"];
	    }
	}
	export class ToolDocs {
	    summary: string;
	    usage: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolDocs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = source["summary"];
	        this.usage = source["usage"];
	    }
	}
	export class ToolManifest {
	    id: string;
	    name: string;
	    kind: string;
	    category: string[];
	    icon: string;
	    description: string;
	    docs: ToolDocs;
	    params: ParameterSpec[];
	    execution: ExecutionSpec;
	    export: ExportSpec;
	    source: SourceSpec;
	
	    static createFrom(source: any = {}) {
	        return new ToolManifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.category = source["category"];
	        this.icon = source["icon"];
	        this.description = source["description"];
	        this.docs = this.convertValues(source["docs"], ToolDocs);
	        this.params = this.convertValues(source["params"], ParameterSpec);
	        this.execution = this.convertValues(source["execution"], ExecutionSpec);
	        this.export = this.convertValues(source["export"], ExportSpec);
	        this.source = this.convertValues(source["source"], SourceSpec);
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

