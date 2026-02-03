package exporter

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jsherman999/openclaw_keyspider/internal/store"
)

type GraphExport struct {
	Hosts []store.Host `json:"hosts"`
	Edges []store.Edge `json:"edges"`
}

func ExportGraphJSON(ctx context.Context, st *store.Store, limit int) ([]byte, string, error) {
	hosts, err := st.ListHosts(ctx, limit)
	if err != nil {
		return nil, "", err
	}
	edges, err := st.ListEdges(ctx, limit)
	if err != nil {
		return nil, "", err
	}
	b, err := json.MarshalIndent(GraphExport{Hosts: hosts, Edges: edges}, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return b, "application/json", nil
}

func ExportGraphCSV(ctx context.Context, st *store.Store, limit int) ([]byte, string, error) {
	edges, err := st.ListEdges(ctx, limit)
	if err != nil {
		return nil, "", err
	}
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)
	_ = w.Write([]string{"src_label", "dest_host_id", "evidence_type", "confidence", "first_seen", "last_seen"})
	for _, e := range edges {
		_ = w.Write([]string{e.SrcLabel, fmt.Sprintf("%d", e.DestHostID), e.Evidence, fmt.Sprintf("%d", e.Confidence), e.FirstSeen.Format("2006-01-02T15:04:05Z07:00"), e.LastSeen.Format("2006-01-02T15:04:05Z07:00")})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "text/csv", nil
}

func ExportGraphGraphML(ctx context.Context, st *store.Store, limit int) ([]byte, string, error) {
	hosts, err := st.ListHosts(ctx, limit)
	if err != nil {
		return nil, "", err
	}
	edges, err := st.ListEdges(ctx, limit)
	if err != nil {
		return nil, "", err
	}

	// Minimal GraphML.
	// Node ids use host:<id> for hosts and src:<label> for source labels that are not hosts.
	nodes := map[string]bool{}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>\n`)
	sb.WriteString(`<graphml xmlns="http://graphml.graphdrawing.org/xmlns">\n`)
	sb.WriteString(`<graph id="keyspider" edgedefault="directed">\n`)

	for _, h := range hosts {
		id := fmt.Sprintf("host:%d", h.ID)
		nodes[id] = true
		sb.WriteString(fmt.Sprintf(`<node id="%s"><data key="hostname">%s</data></node>\n`, xmlEscape(id), xmlEscape(h.Hostname)))
	}

	for _, e := range edges {
		src := "src:" + e.SrcLabel
		// if src_host_id known, prefer host node
		if e.SrcHostID != nil {
			src = fmt.Sprintf("host:%d", *e.SrcHostID)
		}
		dst := fmt.Sprintf("host:%d", e.DestHostID)
		if !nodes[src] {
			nodes[src] = true
			sb.WriteString(fmt.Sprintf(`<node id="%s"><data key="label">%s</data></node>\n`, xmlEscape(src), xmlEscape(e.SrcLabel)))
		}
		// dst host node should exist; but be defensive
		if !nodes[dst] {
			nodes[dst] = true
			sb.WriteString(fmt.Sprintf(`<node id="%s"><data key="label">%s</data></node>\n`, xmlEscape(dst), xmlEscape(dst)))
		}
		sb.WriteString(fmt.Sprintf(`<edge source="%s" target="%s"><data key="evidence">%s</data><data key="confidence">%d</data></edge>\n`, xmlEscape(src), xmlEscape(dst), xmlEscape(e.Evidence), e.Confidence))
	}

	sb.WriteString(`</graph>\n</graphml>\n`)
	return []byte(sb.String()), "application/graphml+xml", nil
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
