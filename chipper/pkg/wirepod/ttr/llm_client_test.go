package wirepod_ttr

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestOpenRouterTransportAddsHeadersAndReasoningExclusion(t *testing.T) {
	transport := openRouterTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("HTTP-Referer"); got != "http://escapepod.local:8080" {
				t.Fatalf("HTTP-Referer = %q", got)
			}
			if got := req.Header.Get("X-OpenRouter-Title"); got != "WirePod Vector" {
				t.Fatalf("X-OpenRouter-Title = %q", got)
			}

			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatal(err)
			}

			var body map[string]any
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Fatal(err)
			}

			reasoning, ok := body["reasoning"].(map[string]any)
			if !ok {
				t.Fatalf("reasoning was not added: %#v", body["reasoning"])
			}
			if got := reasoning["effort"]; got != "low" {
				t.Fatalf("reasoning.effort = %#v", got)
			}
			if got := reasoning["exclude"]; got != true {
				t.Fatalf("reasoning.exclude = %#v", got)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://openrouter.ai/api/v1/chat/completions",
		strings.NewReader(`{"model":"openai/gpt-oss-120b:nitro"}`),
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestOpenRouterTransportPreservesExistingReasoning(t *testing.T) {
	transport := openRouterTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatal(err)
			}

			var body map[string]any
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Fatal(err)
			}

			reasoning, ok := body["reasoning"].(map[string]any)
			if !ok {
				t.Fatalf("reasoning was not preserved: %#v", body["reasoning"])
			}
			if got := reasoning["effort"]; got != "high" {
				t.Fatalf("reasoning.effort = %#v", got)
			}
			if got := reasoning["exclude"]; got != false {
				t.Fatalf("reasoning.exclude = %#v", got)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://openrouter.ai/api/v1/chat/completions",
		strings.NewReader(`{"model":"google/gemini-3.5-flash","reasoning":{"effort":"high","exclude":false}}`),
	)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}
