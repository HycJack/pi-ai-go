export namespace main {
	
	export class AIConfig {
	    provider: string;
	    model: string;
	    apiKey: string;
	    endpoint: string;
	    promptBase: string;
	    maxWords: number;
	    useTracing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.apiKey = source["apiKey"];
	        this.endpoint = source["endpoint"];
	        this.promptBase = source["promptBase"];
	        this.maxWords = source["maxWords"];
	        this.useTracing = source["useTracing"];
	    }
	}

}

