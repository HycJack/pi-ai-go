export namespace main {
	
	export class ModelInfo {
	    id: string;
	    name: string;
	    reasoning?: boolean;
	    thinkingLevelMap?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.reasoning = source["reasoning"];
	        this.thinkingLevelMap = source["thinkingLevelMap"];
	    }
	}

}

