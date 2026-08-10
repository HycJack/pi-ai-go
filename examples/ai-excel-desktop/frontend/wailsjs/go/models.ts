export namespace main {
	
	export class ColumnStats {
	    name: string;
	    type: string;
	    count: number;
	    nullCount: number;
	    min: number;
	    max: number;
	    mean: number;
	    median: number;
	    stdDev: number;
	    unique: number;
	
	    static createFrom(source: any = {}) {
	        return new ColumnStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.count = source["count"];
	        this.nullCount = source["nullCount"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.mean = source["mean"];
	        this.median = source["median"];
	        this.stdDev = source["stdDev"];
	        this.unique = source["unique"];
	    }
	}
	export class FileInfo {
	    path: string;
	    name: string;
	    size: number;
	    sheets: string[];
	    selected: string;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.sheets = source["sheets"];
	        this.selected = source["selected"];
	    }
	}
	export class ModelInfo {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class SheetData {
	    name: string;
	    headers: string[];
	    rows: any[][];
	    types: Record<string, string>;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new SheetData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.headers = source["headers"];
	        this.rows = source["rows"];
	        this.types = source["types"];
	        this.total = source["total"];
	    }
	}

}

