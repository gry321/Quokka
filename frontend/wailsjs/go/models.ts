export namespace main {
	
	export class AppEntry {
	    name: string;
	    path: string;
	    icon: string;
	
	    static createFrom(source: any = {}) {
	        return new AppEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.icon = source["icon"];
	    }
	}
	export class PluginEntry {
	    name: string;
	    path: string;
	    icon: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new PluginEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.icon = source["icon"];
	        this.source = source["source"];
	    }
	}
	export class PluginInfo {
	    name: string;
	    path: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PluginInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.enabled = source["enabled"];
	    }
	}

}

