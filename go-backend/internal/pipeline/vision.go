// Package pipeline provides the evaluation pipeline orchestrator.
// VisionParser uses GLM-4V-Flash multimodal API to extract text from document page images.
package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// VisionParser parses documents via GLM-4V-Flash multimodal vision API.
type VisionParser struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	sem        chan struct{}
}

// NewVisionParser creates a VisionParser pointing at GLM API.
func NewVisionParser(apiKey string) *VisionParser {
	return &VisionParser{
		apiKey:     apiKey,
		baseURL:    "https://open.bigmodel.cn/api/paas/v4",
		model:      "glm-4v-flash",
		httpClient: &http.Client{Timeout: 180 * time.Second},
		sem:        make(chan struct{}, 6),
	}
}

// ParsePages sends all page images concurrently and returns merged markdown.
func (vp *VisionParser) ParsePages(ctx context.Context, dataURIs []string) (string, error) {
	n := len(dataURIs)
	type pageResult struct {
		idx  int
		text string
		err  error
	}
	ch := make(chan pageResult, n)
	var wg sync.WaitGroup

	for i, uri := range dataURIs {
		wg.Add(1)
		go func(idx int, imageURI string) {
			defer wg.Done()
			text, err := vp.parseOne(ctx, imageURI, idx+1, n)
			ch <- pageResult{idx, text, err}
		}(i, uri)
	}
	wg.Wait()
	close(ch)

	results := make([]string, n)
	var errs []string
	for r := range ch {
		if r.err != nil {
			errs = append(errs, fmt.Sprintf("page%d: %v", r.idx+1, r.err))
			results[r.idx] = ""
		} else {
			results[r.idx] = r.text
		}
	}

	var buf bytes.Buffer
	for i, text := range results {
		buf.WriteString(fmt.Sprintf("\n\n## 第 %d 页\n\n", i+1))
		if text == "" {
			buf.WriteString("⚠️ 解析失败")
		} else {
			buf.WriteString(text)
		}
	}

	merged := buf.String()
	if len(errs) > 0 {
		return merged, fmt.Errorf("vision: %d/%d failed: %s", len(errs), n, joinErrs(errs))
	}
	return merged, nil
}

func (vp *VisionParser) parseOne(ctx context.Context, imageURI string, pageNum, total int) (string, error) {
	select {
	case vp.sem <- struct{}{}:
		defer func() { <-vp.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	payload := map[string]any{
		"model": vp.model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "image_url", "image_url": map[string]string{"url": imageURI}},
					{"type": "text", "text": fmt.Sprintf("第%d/%d页，提取文字，Markdown", pageNum, total)},
				},
			},
		},
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*3) * time.Second)
		}

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequestWithContext(ctx, "POST", vp.baseURL+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+vp.apiKey)

		resp, err := vp.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 200 {
			var r struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(respBody, &r); err != nil {
				return "", fmt.Errorf("unmarshal: %w", err)
			}
			if len(r.Choices) > 0 && r.Choices[0].Message.Content != "" {
				return r.Choices[0].Message.Content, nil
			}
			return "", fmt.Errorf("empty response")
		}

		// Check for 1305 (model busy)
		var errResp struct {
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != nil && errResp.Error.Code == "1305" {
			lastErr = fmt.Errorf("model busy (1305)")
			continue
		}

		return "", fmt.Errorf("API HTTP %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}

	return "", fmt.Errorf("exhausted retries: %w", lastErr)
}

func joinErrs(errs []string) string {
	b := bytes.Buffer{}
	for i, e := range errs {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(e)
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}