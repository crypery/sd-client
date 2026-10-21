package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	width   = flag.Int("W", 256, "image width in pixels")
	height  = flag.Int("H", 256, "image height in pixels")
	prompt  = flag.String("p", "", "prompt")
	neg     = flag.String("n", "", "negative prompt")
	batch   = flag.Int("batch", 1, "number of images to generate (n)")
	format  = flag.String("format", "png", "output format: png or jpeg")
	quality = flag.Int("quality", 100, "output compression, 0-100")
	out     = flag.String("o", "", "output file path (default: generated/img_<timestamp>.png)")
	server  = flag.String("server", "127.0.0.1:8080", "stable-diffusion.cpp server host:port")
	timeout = flag.Duration("timeout", 10*time.Minute, "request timeout")
)

type imageRequest struct {
	Prompt            string `json:"prompt"`
	NegativePrompt    string `json:"negative_prompt,omitempty"`
	Size              string `json:"size"`
	N                 int    `json:"n"`
	OutputFormat      string `json:"output_format"`
	OutputCompression int    `json:"output_compression"`
}

type imageResponse struct {
	Created int64        `json:"created"`
	Data    []imageEntry `json:"data"`
}

type imageEntry struct {
	URL           string `json:"url"`
	B64JSON       string `json:"b64_json"`
	MimeType      string `json:"mime_type"`
	RevisedPrompt string `json:"revised_prompt"`
}

func sniffMime(data []byte) string {
	switch {
	case len(data) >= 4 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G':
		return "image/png"
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	case len(data) >= 6 && string(data[0:3]) == "GIF" && (string(data[3:5]) == "87a" || string(data[3:5]) == "89a"):
		return "image/gif"
	default:
		return ""
	}
}

func extFor(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpeg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func withIndex(p string, i int) string {
	dir := filepath.Dir(p)
	base := filepath.Base(p)
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return filepath.Join(dir, base+fmt.Sprintf("_%d", i+1))
	}
	return filepath.Join(dir, base[:dot]+fmt.Sprintf("_%d", i+1)+base[dot:])
}

func decodeEntry(e imageEntry, client *http.Client) ([]byte, string, error) {
	if e.B64JSON != "" {
		decoded, err := base64.StdEncoding.DecodeString(e.B64JSON)
		if err != nil {
			return nil, "", fmt.Errorf("decode b64_json: %w", err)
		}
		return decoded, e.MimeType, nil
	}

	u := e.URL
	if strings.HasPrefix(u, "data:") {
		comma := strings.Index(u, ",")
		if comma < 0 {
			return nil, "", fmt.Errorf("bad data url")
		}
		header := u[len("data:"):comma]
		parts := strings.Split(header, ";")
		mime := parts[0]
		payload := u[comma+1:]
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, "", fmt.Errorf("decode data url: %w", err)
		}
		return decoded, mime, nil
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func saveImage(data []byte, out, mime string, seq, total int) (string, error) {
	if out == "" {
		dir := "generated"
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		suffix := ""
		if total > 1 {
			suffix = fmt.Sprintf("_%d", seq+1)
		}
		out = filepath.Join(dir, "img_"+time.Now().Format("20060102_150405")+suffix+extFor(mime))
	}
	if !strings.Contains(filepath.Base(out), ".") {
		out += extFor(mime)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	flag.Parse()

	if *prompt == "" {
		fmt.Fprintln(os.Stderr, `error: -p "<prompt>" is required`)
		flag.Usage()
		os.Exit(1)
	}
	if *width <= 0 || *height <= 0 {
		fatalf("-W and -H must be positive numbers")
	}
	if *batch < 1 {
		fatalf("-batch must be >= 1")
	}
	if *format != "png" && *format != "jpeg" {
		fatalf("-format must be png or jpeg")
	}
	if *quality < 0 || *quality > 100 {
		fatalf("-quality must be in 0-100")
	}

	body, err := json.Marshal(imageRequest{
		Prompt:            *prompt,
		NegativePrompt:    *neg,
		Size:              fmt.Sprintf("%dx%d", *width, *height),
		N:                 *batch,
		OutputFormat:      *format,
		OutputCompression: *quality,
	})
	if err != nil {
		fatalf("%v", err)
	}

	client := &http.Client{Timeout: *timeout}
	url := "http://" + *server + "/v1/images/generations"
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		fatalf("read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		fatalf("server returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var ir imageResponse
	if err := json.Unmarshal(raw, &ir); err != nil {
		fatalf("parse response: %v", err)
	}
	if len(ir.Data) == 0 {
		fatalf("no images in response")
	}

	for i, e := range ir.Data {
		img, declared, err := decodeEntry(e, client)
		if err != nil {
			fatalf("%v", err)
		}
		mime := sniffMime(img)
		if mime == "" {
			mime = declared
		}
		if mime == "" {
			mime = "image/png"
		}
		outPath := *out
		if len(ir.Data) > 1 && outPath != "" {
			outPath = withIndex(outPath, i)
		}
		path, err := saveImage(img, outPath, mime, i, len(ir.Data))
		if err != nil {
			fatalf("save image: %v", err)
		}
		fmt.Println(path)
	}
}
