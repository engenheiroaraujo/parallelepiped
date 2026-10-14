package model

import "sort"

// UnresolvedGroup is the group of nodes created for links to missing notes.
const UnresolvedGroup = "~unresolved"

// GraphNode is one note (or missing link target) in the graph view.
type GraphNode struct {
	ID         string
	Title      string
	Group      string
	Degree     int
	Unresolved bool
}

// GraphEdge is a directed link from Source to Target, both node IDs.
type GraphEdge struct {
	Source string
	Target string
}

// Graph is the full note graph.
type Graph struct {
	Nodes []GraphNode
	Edges []GraphEdge
}

// NoteRef identifies an indexed note.
type NoteRef struct {
	Path   string
	Title  string
	Folder string
}

// UnresolvedLink is a link whose target matches no note.
type UnresolvedLink struct {
	Source    string
	TargetKey string
	// Target is the link text as written; it titles the missing note.
	Target string
}

// BuildGraph assembles nodes and edges, adds a node per missing target and
// counts each node's degree. Self links and duplicate edges are dropped.
func BuildGraph(notes []NoteRef, resolved []GraphEdge, unresolved []UnresolvedLink) Graph {
	nodes := make(map[string]*GraphNode, len(notes))
	order := make([]string, 0, len(notes))
	for _, note := range notes {
		nodes[note.Path] = &GraphNode{ID: note.Path, Title: note.Title, Group: note.Folder}
		order = append(order, note.Path)
	}

	seen := make(map[GraphEdge]bool)
	var edges []GraphEdge
	addEdge := func(edge GraphEdge) {
		if edge.Source == edge.Target || seen[edge] {
			return
		}
		source, okSource := nodes[edge.Source]
		target, okTarget := nodes[edge.Target]
		if !okSource || !okTarget {
			return
		}
		seen[edge] = true
		edges = append(edges, edge)
		source.Degree++
		target.Degree++
	}

	for _, edge := range resolved {
		addEdge(edge)
	}
	for _, link := range unresolved {
		id := UnresolvedGroup + "/" + link.TargetKey
		if _, exists := nodes[id]; !exists {
			title := link.Target
			if title == "" {
				title = link.TargetKey
			}
			nodes[id] = &GraphNode{ID: id, Title: title, Group: UnresolvedGroup, Unresolved: true}
			order = append(order, id)
		}
		addEdge(GraphEdge{Source: link.Source, Target: id})
	}

	graph := Graph{Nodes: make([]GraphNode, 0, len(order)), Edges: edges}
	for _, id := range order {
		graph.Nodes = append(graph.Nodes, *nodes[id])
	}
	sort.SliceStable(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	return graph
}
