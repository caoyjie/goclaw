package mediaremote

import "testing"

func TestExtractComfyCandidatesFromDownloadURL(t *testing.T) {
	raw := `{"images":[{"url":"http://comfy.local/view?filename=out.png&type=output","mime_type":"image/png"}]}`
	got := ExtractCandidates("comfy_get_output", nil, raw)
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	if got[0].SourceURL != "http://comfy.local/view?filename=out.png&type=output" {
		t.Fatalf("SourceURL = %q", got[0].SourceURL)
	}
	if got[0].MimeType != "image/png" {
		t.Fatalf("MimeType = %q", got[0].MimeType)
	}
}

func TestExtractComfyCandidatesFromDownloadCommand(t *testing.T) {
	raw := `{"download_command":{"method":"GET","url":"http://comfy.local/view?filename=abc.png&type=output"}}`
	got := ExtractCandidates("get_output", nil, raw)
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	if got[0].SourceURL == "" {
		t.Fatal("SourceURL is empty")
	}
}

func TestExtractIgnoresNonComfyTool(t *testing.T) {
	got := ExtractCandidates("read_file", nil, `{"url":"http://example.test/a.png"}`)
	if len(got) != 0 {
		t.Fatalf("candidates = %d, want 0", len(got))
	}
}
