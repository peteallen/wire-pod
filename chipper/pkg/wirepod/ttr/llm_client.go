package wirepod_ttr

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/sashabaranov/go-openai"
)

const (
	openRouterBaseURL = "https://openrouter.ai/api/v1"
	togetherBaseURL   = "https://api.together.xyz/v1"
)

func newKnowledgeClient() *openai.Client {
	switch vars.APIConfig.Knowledge.Provider {
	case "together":
		if vars.APIConfig.Knowledge.Model == "" {
			vars.APIConfig.Knowledge.Model = "meta-llama/Llama-3-70b-chat-hf"
			vars.WriteConfigToDisk()
		}
		conf := openai.DefaultConfig(vars.APIConfig.Knowledge.Key)
		conf.BaseURL = togetherBaseURL
		return openai.NewClientWithConfig(conf)
	case "custom":
		conf := openai.DefaultConfig(vars.APIConfig.Knowledge.Key)
		conf.BaseURL = vars.APIConfig.Knowledge.Endpoint
		return openai.NewClientWithConfig(conf)
	case "openrouter":
		conf := openai.DefaultConfig(vars.APIConfig.Knowledge.Key)
		conf.BaseURL = openRouterBaseURL
		conf.HTTPClient = &http.Client{
			Transport: openRouterTransport{base: http.DefaultTransport},
		}
		return openai.NewClientWithConfig(conf)
	case "openai":
		return openai.NewClient(vars.APIConfig.Knowledge.Key)
	default:
		return openai.NewClient(vars.APIConfig.Knowledge.Key)
	}
}

type openRouterTransport struct {
	base http.RoundTripper
}

func (t openRouterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		t.base = http.DefaultTransport
	}

	req.Header.Set("HTTP-Referer", "http://escapepod.local:8080")
	req.Header.Set("X-OpenRouter-Title", "WirePod Vector")

	if req.Body != nil && strings.HasSuffix(req.URL.Path, "/chat/completions") {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()

		var payload map[string]any
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			return t.base.RoundTrip(req)
		}

		if _, exists := payload["reasoning"]; !exists {
			payload["reasoning"] = map[string]any{
				"effort":  "low",
				"exclude": true,
			}
		}

		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
	}

	return t.base.RoundTrip(req)
}
