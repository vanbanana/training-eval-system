// Package vision converts documents to page images for GLM-4V-Flash parsing.
// All external tool calls are runtime-only, no CGO required.
package vision

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// PageImage holds a single rendered page.
type PageImage struct {
	PageNum int
	DataURI string // "data:image/png;base64,..."
}

var mu sync.Mutex // Word COM is single-threaded

// ConvertFile converts any supported document to page images.
// ext: file extension with dot (".doc", ".docx", ".pdf", ".png", etc.)
func ConvertFile(filePath, ext string) ([]PageImage, error) {
	ext = strings.ToLower(ext)
	absPath, _ := filepath.Abs(filePath)

	switch ext {
	case ".doc":
		return convertDoc(absPath)
	case ".docx":
		return convertDocx(absPath)
	case ".pdf":
		return renderPDF(absPath)
	case ".png", ".jpg", ".jpeg":
		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		return []PageImage{{PageNum: 1, DataURI: dataURI(data)}}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}
}

func convertDoc(path string) ([]PageImage, error) {
	if runtime.GOOS == "windows" {
		mu.Lock()
		defer mu.Unlock()
		imgs, err := wordComConvert(path)
		if err == nil {
			return imgs, nil
		}
	}
	return libreOfficeConvert(path)
}

func convertDocx(path string) ([]PageImage, error) {
	return libreOfficeConvert(path)
}

func wordComConvert(path string) ([]PageImage, error) {
	pdfPath := filepath.Join(os.TempDir(), fmt.Sprintf("glm_%d.pdf", os.Getpid()))
	cleanFn := func(s string) string { return strings.ReplaceAll(s, "'", "''") }

	ps := fmt.Sprintf(
		`$w=New-Object -ComObject Word.Application;$w.Visible=$false;$w.DisplayAlerts=0;`+
			`$d=$w.Documents.Open('%s');$d.SaveAs('%s',17);$d.Close();$w.Quit()`,
		cleanFn(path), cleanFn(pdfPath))

	if err := exec.Command("powershell", "-NoProfile", "-Command", ps).Run(); err != nil {
		return nil, fmt.Errorf("word com: %w", err)
	}
	defer os.Remove(pdfPath)

	return renderPDF(pdfPath)
}

func libreOfficeConvert(path string) ([]PageImage, error) {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("glm_lo_%d", os.Getpid()))
	os.MkdirAll(tmp, 0755)
	defer os.RemoveAll(tmp)

	binary := findBinary("soffice", "libreoffice")
	if binary == "" {
		return nil, fmt.Errorf("LibreOffice not found (install libreoffice)")
	}

	out, err := exec.Command(binary, "--headless", "--convert-to", "pdf",
		"--outdir", tmp, path).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("lo: %w\n%s", err, out)
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	pdfPath := filepath.Join(tmp, base+".pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		return nil, fmt.Errorf("lo: output not found")
	}

	return renderPDF(pdfPath)
}

func renderPDF(path string) ([]PageImage, error) {
	if imgs, err := gsRender(path); err == nil && len(imgs) > 0 {
		return imgs, nil
	}
	if imgs, err := popplerRender(path); err == nil && len(imgs) > 0 {
		return imgs, nil
	}
	return nil, fmt.Errorf("no PDF renderer (install ghostscript or poppler-utils)")
}

func gsRender(path string) ([]PageImage, error) {
	if findBinary("gs") == "" {
		return nil, fmt.Errorf("gs not found")
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("glm_gs_%d", os.Getpid()))
	os.MkdirAll(tmp, 0755)
	defer os.RemoveAll(tmp)

	cmd := exec.Command("gs", "-dQUIET", "-dSAFER", "-dBATCH", "-dNOPAUSE",
		"-sDEVICE=png16m", "-r150",
		"-sOutputFile="+filepath.Join(tmp, "p%d.png"), path)
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var imgs []PageImage
	for i := 1; ; i++ {
		data, err := os.ReadFile(filepath.Join(tmp, fmt.Sprintf("p%d.png", i)))
		if err != nil {
			break
		}
		imgs = append(imgs, PageImage{PageNum: i, DataURI: dataURI(data)})
	}
	return imgs, nil
}

func popplerRender(path string) ([]PageImage, error) {
	if findBinary("pdftoppm") == "" {
		return nil, fmt.Errorf("pdftoppm not found")
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("glm_pp_%d", os.Getpid()))
	os.MkdirAll(tmp, 0755)
	defer os.RemoveAll(tmp)

	cmd := exec.Command("pdftoppm", "-png", "-r", "150", path, filepath.Join(tmp, "p"))
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var imgs []PageImage
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(tmp, e.Name()))
		if data == nil {
			continue
		}
		var pg int
		fmt.Sscanf(e.Name(), "p-%d.png", &pg)
		if pg == 0 {
			fmt.Sscanf(e.Name(), "p%d.png", &pg)
		}
		if pg == 0 {
			pg = 1
		}
		imgs = append(imgs, PageImage{PageNum: pg, DataURI: dataURI(data)})
	}
	return imgs, nil
}

func dataURI(data []byte) string {
	b64 := base64.StdEncoding.EncodeToString(data)
	mime := "image/png"
	if len(data) > 2 && data[0] == 0xFF && data[1] == 0xD8 {
		mime = "image/jpeg"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, b64)
}

func findBinary(names ...string) string {
	for _, n := range names {
		if _, err := exec.LookPath(n); err == nil {
			return n
		}
	}
	return ""
}

// ToolsAvailable reports which external tools are installed.
func ToolsAvailable() map[string]bool {
	return map[string]bool{
		"libreoffice": findBinary("soffice", "libreoffice") != "",
		"ghostscript": findBinary("gs") != "",
		"poppler":     findBinary("pdftoppm") != "",
		"powershell":  runtime.GOOS == "windows",
	}
}