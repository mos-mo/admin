export namespace main {
	
	export class frameSnapshot {
	    agentId: string;
	    imageBase64: string;
	    isPreview: boolean;
	    timestamp: number;
	
	    static createFrom(source: any = {}) {
	        return new frameSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.agentId = source["agentId"];
	        this.imageBase64 = source["imageBase64"];
	        this.isPreview = source["isPreview"];
	        this.timestamp = source["timestamp"];
	    }
	}

}

