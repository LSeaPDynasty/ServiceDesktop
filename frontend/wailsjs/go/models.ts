export namespace main {
	
	export class AppConfigDTO {
	    language: string;
	    autoStart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppConfigDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.autoStart = source["autoStart"];
	    }
	}
	export class ServiceDTO {
	    id: string;
	    name: string;
	    category: string;
	    installPath: string;
	    port: number;
	    pid: number;
	    status: number;
	    statusText: string;
	    args: string;
	    profiles: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.installPath = source["installPath"];
	        this.port = source["port"];
	        this.pid = source["pid"];
	        this.status = source["status"];
	        this.statusText = source["statusText"];
	        this.args = source["args"];
	        this.profiles = source["profiles"];
	        this.error = source["error"];
	    }
	}
	export class ServiceDetailDTO {
	    id: string;
	    name: string;
	    displayName: string;
	    category: string;
	    installPath: string;
	    startCmd: string;
	    stopCmd: string;
	    port: number;
	    logFile: string;
	    args: string;
	    envVars: string;
	    isTemplate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServiceDetailDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.category = source["category"];
	        this.installPath = source["installPath"];
	        this.startCmd = source["startCmd"];
	        this.stopCmd = source["stopCmd"];
	        this.port = source["port"];
	        this.logFile = source["logFile"];
	        this.args = source["args"];
	        this.envVars = source["envVars"];
	        this.isTemplate = source["isTemplate"];
	    }
	}
	export class StartResult {
	    success: boolean;
	    error?: string;
	    service?: ServiceDTO;
	
	    static createFrom(source: any = {}) {
	        return new StartResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.service = this.convertValues(source["service"], ServiceDTO);
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

export namespace services {
	
	export class PortCheckResult {
	    available: boolean;
	    port: number;
	    pid: number;
	    processName: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PortCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.port = source["port"];
	        this.pid = source["pid"];
	        this.processName = source["processName"];
	        this.message = source["message"];
	    }
	}

}

