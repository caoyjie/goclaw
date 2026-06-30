package mediaremote

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

var httpURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

func ExtractCandidates(toolName string, args map[string]any, raw string) []Candidate {
	name := strings.ToLower(toolName)
	if !strings.Contains(name, "comfy") && !strings.Contains(name, "get_output") {
		return nil
	}

	var root any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return extractURLsFromText(toolName, raw)
	}

	var out []Candidate
	walkJSON(root, func(m map[string]any) {
		if u := firstString(m, "url", "download_url", "href"); isHTTPURL(u) {
			out = append(out, Candidate{
				SourceURL: u,
				MimeType:  firstString(m, "mime_type", "content_type"),
				NameHint:  firstString(m, "filename", "name"),
				Prompt:    promptFromArgs(args),
				ToolName:  toolName,
			})
		}
		if dc, ok := m["download_command"].(map[string]any); ok {
			if u := firstString(dc, "url", "download_url", "href"); isHTTPURL(u) {
				out = append(out, Candidate{
					SourceURL: u,
					MimeType:  firstString(dc, "mime_type", "content_type"),
					NameHint:  firstString(dc, "filename", "name"),
					Prompt:    promptFromArgs(args),
					ToolName:  toolName,
				})
			}
		}
	})
	return dedupeCandidates(out)
}

func walkJSON(v any, visit func(map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		visit(x)
		for _, child := range x {
			walkJSON(child, visit)
		}
	case []any:
		for _, child := range x {
			walkJSON(child, visit)
		}
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func extractURLsFromText(toolName, raw string) []Candidate {
	matches := httpURLPattern.FindAllString(raw, -1)
	out := make([]Candidate, 0, len(matches))
	for _, match := range matches {
		if isHTTPURL(match) {
			out = append(out, Candidate{SourceURL: match, ToolName: toolName})
		}
	}
	return dedupeCandidates(out)
}

func dedupeCandidates(in []Candidate) []Candidate {
	seen := make(map[string]bool, len(in))
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		if c.SourceURL == "" || seen[c.SourceURL] {
			continue
		}
		seen[c.SourceURL] = true
		out = append(out, c)
	}
	return out
}

func promptFromArgs(args map[string]any) string {
	for _, key := range []string{"prompt", "text", "positive_prompt"} {
		if v, ok := args[key].(string); ok {
			return v
		}
	}
	return ""
}
