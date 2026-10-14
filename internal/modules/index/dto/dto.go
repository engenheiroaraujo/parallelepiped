// Package dto holds the index payloads exchanged with the frontend.
// Type names are unique across all dto packages because Wails generates the
// TypeScript models of every package into one namespace per package name.
package dto

import "parallelepiped/internal/modules/index/model"

type GraphNode struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Group      string `json:"group"`
	Degree     int    `json:"degree"`
	Unresolved bool   `json:"unresolved"`
}

type GraphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type GraphResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

type BacklinkItem struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

type BacklinksResponse struct {
	Backlinks []BacklinkItem `json:"backlinks"`
}

type SearchHitItem struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

type SearchResponse struct {
	Hits []SearchHitItem `json:"hits"`
}

func GraphFromModel(g model.Graph) GraphResponse {
	out := GraphResponse{
		Nodes: make([]GraphNode, 0, len(g.Nodes)),
		Links: make([]GraphLink, 0, len(g.Edges)),
	}
	for _, n := range g.Nodes {
		out.Nodes = append(out.Nodes, GraphNode{ID: n.ID, Title: n.Title, Group: n.Group, Degree: n.Degree, Unresolved: n.Unresolved})
	}
	for _, e := range g.Edges {
		out.Links = append(out.Links, GraphLink{Source: e.Source, Target: e.Target})
	}
	return out
}

func BacklinksFromModel(items []model.Backlink) BacklinksResponse {
	out := BacklinksResponse{Backlinks: make([]BacklinkItem, 0, len(items))}
	for _, b := range items {
		out.Backlinks = append(out.Backlinks, BacklinkItem{Path: b.Path, Title: b.Title})
	}
	return out
}

func SearchFromModel(hits []model.SearchHit) SearchResponse {
	out := SearchResponse{Hits: make([]SearchHitItem, 0, len(hits))}
	for _, h := range hits {
		out.Hits = append(out.Hits, SearchHitItem{Path: h.Path, Title: h.Title, Snippet: h.Snippet})
	}
	return out
}
