package main

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	ledongpdf "github.com/ledongthuc/pdf"
)

const html = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<title>PDF/DOCX 解析效果测试</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: system-ui, -apple-system, sans-serif; background: #f5f5f5; padding: 24px; }
h1 { font-size: 20px; margin-bottom: 16px; color: #1a1a1a; }
.drop-zone {
  border: 2px dashed #ccc; border-radius: 12px; padding: 48px;
  text-align: center; background: #fff; cursor: pointer;
  transition: all 0.2s; margin-bottom: 24px;
}
.drop-zone:hover, .drop-zone.dragover { border-color: #4f46e5; background: #eef2ff; }
.drop-zone p { color: #666; font-size: 14px; }
.drop-zone .hint { font-size: 12px; color: #999; margin-top: 8px; }
.results { display: flex; flex-direction: column; gap: 16px; }
.result-card {
  background: #fff; border-radius: 8px; padding: 16px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.08);
}
.result-card h3 { font-size: 14px; color: #4f46e5; margin-bottom: 8px; }
.meta { font-size: 12px; color: #666; margin-bottom: 8px; display: flex; gap: 16px; }
.meta span { background: #f0f0f0; padding: 2px 8px; border-radius: 4px; }
.text-output {
  background: #fafafa; border: 1px solid #eee; border-radius: 6px;
  padding: 12px; font-size: 13px; line-height: 1.6; white-space: pre-wrap;
  max-height: 400px; overflow-y: auto; font-family: 'Noto Sans SC', system-ui, sans-serif;
}
.loading { text-align: center; padding: 24px; color: #666; }
.file-info { font-size: 13px; color: #333; margin-bottom: 12px; font-weight: 600; }
.error { color: #dc2626; background: #fef2f2; padding: 8px 12px; border-radius: 6px; font-size: 13px; }
</style>
</head>
<body>
<h1>📄 PDF / DOCX 解析效果测试工具</h1>
<div class="drop-zone" id="dropZone" onclick="document.getElementById('fileInput').click()">
  <p>🖱️ 拖拽文件到这里，或点击选择</p>
  <p class="hint">支持 .pdf .docx .odt .rtf</p>
  <input type="file" id="fileInput" accept=".pdf,.docx,.odt,.rtf" multiple hidden>
</div>
<div class="results" id="results"></div>

<script>
const dropZone = document.getElementById('dropZone');
const fileInput = document.getElementById('fileInput');
const results = document.getElementById('results');

['dragenter','dragover'].forEach(e => dropZone.addEventListener(e, ev => { ev.preventDefault(); dropZone.classList.add('dragover'); }));
['dragleave','drop'].forEach(e => dropZone.addEventListener(e, ev => { ev.preventDefault(); dropZone.classList.remove('dragover'); }));

dropZone.addEventListener('drop', ev => { handleFiles(ev.dataTransfer.files); });
fileInput.addEventListener('change', ev => { handleFiles(ev.target.files); });

async function handleFiles(files) {
  for (const file of files) {
    const card = document.createElement('div');
    card.className = 'result-card';
    card.innerHTML = '<div class="loading">⏳ 正在解析 ' + file.name + ' ...</div>';
    results.prepend(card);

    const formData = new FormData();
    formData.append('file', file);

    try {
      const resp = await fetch('/parse', { method: 'POST', body: formData });
      const data = await resp.json();
      let html = '<div class="file-info">📁 ' + data.filename + ' (' + data.size + ')</div>';
      for (const r of data.results) {
        html += '<h3>' + r.engine + '</h3>';
        html += '<div class="meta"><span>⏱️ ' + r.duration + '</span><span>📊 ' + r.chars + ' 字符</span><span>📝 ' + r.lines + ' 非空行</span></div>';
        if (r.error) {
          html += '<div class="error">❌ ' + r.error + '</div>';
        } else {
          html += '<div class="text-output">' + escapeHtml(r.preview) + '</div>';
        }
      }
      card.innerHTML = html;
    } catch (e) {
      card.innerHTML = '<div class="error">请求失败: ' + e.message + '</div>';
    }
  }
}

function escapeHtml(s) { return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }
</script>
</body>
</html>`

type ParseResult struct {
	Engine   string `json:"engine"`
	Duration string `json:"duration"`
	Chars    int    `json:"chars"`
	Lines    int    `json:"lines"`
	Preview  string `json:"preview"`
	Error    string `json:"error,omitempty"`
}

type Response struct {
	Filename string        `json:"filename"`
	Size     string        `json:"size"`
	Results  []ParseResult `json:"results"`
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})

	http.HandleFunc("/parse", handleParse)

	addr := "127.0.0.1:9876"
	fmt.Printf("🚀 PDF 解析测试工具已启动: http://%s\n", addr)
	fmt.Println("   拖拽 PDF/DOCX 文件到浏览器页面即可测试")
	fmt.Println("   按 Ctrl+C 退出")

	// 自动打开浏览器
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser("http://" + addr)
	}()

	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}

func handleParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "读取文件失败", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 保存到临时文件
	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "pdftest-*"+filepath.Ext(header.Filename))
	if err != nil {
		http.Error(w, "创建临时文件失败", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	io.Copy(tmpFile, file)
	tmpFile.Close()

	filePath := tmpFile.Name()
	ext := strings.ToLower(filepath.Ext(header.Filename))

	resp := Response{
		Filename: header.Filename,
		Size:     formatSize(header.Size),
	}

	switch ext {
	case ".pdf":
		// ledongthuc/pdf
		resp.Results = append(resp.Results, runLedong(filePath))

	case ".docx":
		// 自写 DOCX 解析（标准库 archive/zip + encoding/xml）
		resp.Results = append(resp.Results, runDocx(filePath))

	case ".odt", ".rtf":
		resp.Results = append(resp.Results, ParseResult{
			Engine: "暂不支持",
			Error:  fmt.Sprintf("当前仅支持 .pdf 和 .docx，收到: %s", ext),
		})

	default:
		resp.Results = append(resp.Results, ParseResult{
			Engine: "不支持",
			Error:  fmt.Sprintf("不支持的文件类型: %s", ext),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func runLedong(filePath string) ParseResult {
	start := time.Now()
	f, err := os.Open(filePath)
	if err != nil {
		return ParseResult{Engine: "ledongthuc/pdf", Error: err.Error()}
	}
	defer f.Close()

	info, _ := f.Stat()
	reader, err := ledongpdf.NewReader(f, info.Size())
	if err != nil {
		return ParseResult{Engine: "ledongthuc/pdf", Error: err.Error(), Duration: time.Since(start).String()}
	}

	var sb strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			sb.WriteString(fmt.Sprintf("[第%d页失败]\n", i))
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	text := sb.String()
	return makeResult("ledongthuc/pdf", text, time.Since(start))
}

func runDocx(filePath string) ParseResult {
	start := time.Now()
	text, err := extractDocxText(filePath)
	if err != nil {
		return ParseResult{Engine: "DOCX (stdlib zip+xml)", Error: err.Error(), Duration: time.Since(start).String()}
	}
	return makeResult("DOCX (stdlib zip+xml)", text, time.Since(start))
}

// extractDocxText 用标准库解析 DOCX（zip 内的 word/document.xml）
func extractDocxText(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("打开 zip 失败: %w", err)
	}
	defer r.Close()

	var docFile *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return "", fmt.Errorf("未找到 word/document.xml，可能不是有效的 .docx 文件")
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", fmt.Errorf("打开 document.xml 失败: %w", err)
	}
	defer rc.Close()

	// 解析 XML，提取所有 <w:t> 标签的文本
	decoder := xml.NewDecoder(rc)
	var sb strings.Builder
	var inText bool

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// <w:t> 或 <w:t xml:space="preserve">
			if t.Name.Local == "t" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				inText = true
			}
			// <w:p> 段落开始 — 在段落之间加换行
			if t.Name.Local == "p" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
			}
		case xml.EndElement:
			if t.Name.Local == "t" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				inText = false
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}

	return sb.String(), nil
}

func makeResult(engine, text string, dur time.Duration) ParseResult {
	runes := []rune(text)
	lines := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}

	preview := text
	if len(runes) > 2000 {
		preview = string(runes[:2000]) + fmt.Sprintf("\n\n... [截断，共 %d 字符]", len(runes))
	}

	return ParseResult{
		Engine:   engine,
		Duration: dur.String(),
		Chars:    len(runes),
		Lines:    lines,
		Preview:  preview,
	}
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
