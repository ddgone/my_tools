export namespace main {
	
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
	    args: string;
	    pythonEnv?: string;
	    usage: string;
	    startedAt: number;
	    endedAt?: number;
	    exitMessage?: string;
	
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
	        this.args = source["args"];
	        this.pythonEnv = source["pythonEnv"];
	        this.usage = source["usage"];
	        this.startedAt = source["startedAt"];
	        this.endedAt = source["endedAt"];
	        this.exitMessage = source["exitMessage"];
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
	    category: string;
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

