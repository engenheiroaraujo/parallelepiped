export namespace dto {
	
	export class BacklinkItem {
	    path: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new BacklinkItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.title = source["title"];
	    }
	}
	export class BacklinksResponse {
	    backlinks: BacklinkItem[];
	
	    static createFrom(source: any = {}) {
	        return new BacklinksResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backlinks = this.convertValues(source["backlinks"], BacklinkItem);
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
	export class GraphLink {
	    source: string;
	    target: string;
	
	    static createFrom(source: any = {}) {
	        return new GraphLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	    }
	}
	export class GraphNode {
	    id: string;
	    title: string;
	    group: string;
	    degree: number;
	    unresolved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GraphNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.group = source["group"];
	        this.degree = source["degree"];
	        this.unresolved = source["unresolved"];
	    }
	}
	export class GraphResponse {
	    nodes: GraphNode[];
	    links: GraphLink[];
	
	    static createFrom(source: any = {}) {
	        return new GraphResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], GraphNode);
	        this.links = this.convertValues(source["links"], GraphLink);
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
	export class ImportResultResponse {
	    index_note: string;
	    units: number;
	    created: number;
	    updated: number;
	    unchanged: number;
	    removed: number;
	    skipped: string[];
	    kept: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResultResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index_note = source["index_note"];
	        this.units = source["units"];
	        this.created = source["created"];
	        this.updated = source["updated"];
	        this.unchanged = source["unchanged"];
	        this.removed = source["removed"];
	        this.skipped = source["skipped"];
	        this.kept = source["kept"];
	    }
	}
	export class NoteResponse {
	    path: string;
	    title: string;
	    content: string;
	    modified_at: string;
	
	    static createFrom(source: any = {}) {
	        return new NoteResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.modified_at = source["modified_at"];
	    }
	}
	export class VaultInfo {
	    path: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new VaultInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	    }
	}
	export class RecentVaultsResponse {
	    vaults: VaultInfo[];
	
	    static createFrom(source: any = {}) {
	        return new RecentVaultsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vaults = this.convertValues(source["vaults"], VaultInfo);
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
	export class SearchHitItem {
	    path: string;
	    title: string;
	    snippet: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchHitItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.title = source["title"];
	        this.snippet = source["snippet"];
	    }
	}
	export class SearchResponse {
	    hits: SearchHitItem[];
	
	    static createFrom(source: any = {}) {
	        return new SearchResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hits = this.convertValues(source["hits"], SearchHitItem);
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
	export class TreeEntry {
	    name: string;
	    path: string;
	    is_dir: boolean;
	    children: TreeEntry[];
	
	    static createFrom(source: any = {}) {
	        return new TreeEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.is_dir = source["is_dir"];
	        this.children = this.convertValues(source["children"], TreeEntry);
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
	export class TreeResponse {
	    root: TreeEntry;
	
	    static createFrom(source: any = {}) {
	        return new TreeResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = this.convertValues(source["root"], TreeEntry);
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

