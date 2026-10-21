package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	width   = flag.Int("W", 256, "image width in pixels (sizes sent to the server must be multiples of 64 and are rounded up; the smaller generation dimension is at least 256px)")
	height  = flag.Int("H", 256, "image height in pixels (sizes sent to the server must be multiples of 64 and are rounded up; the smaller generation dimension is at least 256px)")
	prompt  = flag.String("p", "", "prompt")
	neg     = flag.String("n", "", "negative prompt")
	batch   = flag.Int("batch", 1, "number of images to generate (n)")
	format  = flag.String("format", "png", "output format: png or jpeg")
	quality = flag.Int("quality", 100, "output compression, 0-100")
	out     = flag.String("o", "", "output file path (default: generated/img_<timestamp>.png)")
	server  = flag.String("server", "127.0.0.1:8080", "stable-diffusion.cpp server host:port")
	timeout = flag.Duration("timeout", 10*time.Minute, "request timeout")
)

const (
	minGenDimension = 256
	sizeStep        = 64
	lanczosA        = 3
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

func ceilToStep(v, step int) int {
	if v%step == 0 {
		return v
	}
	return (v/step + 1) * step
}

func generationSize(w, h int) (genW, genH int, changed bool) {
	origW, origH := w, h
	if w < minGenDimension || h < minGenDimension {
		scale := math.Max(
			float64(minGenDimension)/float64(w),
			float64(minGenDimension)/float64(h),
		)
		w = int(math.Ceil(float64(w) * scale))
		h = int(math.Ceil(float64(h) * scale))
	}
	genW, genH = ceilToStep(w, sizeStep), ceilToStep(h, sizeStep)
	changed = genW != origW || genH != origH
	return genW, genH, changed
}

func decodeImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func encodeImage(img image.Image, format string, quality int) ([]byte, string, error) {
	var buf bytes.Buffer
	if format == "jpeg" {
		b := img.Bounds()
		dst := image.NewRGBA(b)
		draw.Draw(dst, b, image.NewUniform(color.White), image.Point{}, draw.Over)
		draw.Draw(dst, b, img, b.Min, draw.Over)
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	}
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}

func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	px := math.Pi * x
	return math.Sin(px) / px
}

func lanczos(t float64) float64 {
	if math.Abs(t) >= lanczosA {
		return 0
	}
	return sinc(t) * sinc(t/lanczosA)
}

type sampleWeight struct {
	index  int
	weight float64
}

func sampleWeights(center float64, size int) []sampleWeight {
	var out []sampleWeight
	lo := int(math.Ceil(center - float64(lanczosA)))
	hi := int(math.Floor(center + float64(lanczosA)))
	for i := lo; i <= hi; i++ {
		if i < 0 || i >= size {
			continue
		}
		w := lanczos(float64(i) - center)
		if w != 0 {
			out = append(out, sampleWeight{i, w})
		}
	}
	return out
}

func clampByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

func resizeLanczos(src image.Image, dw, dh int) *image.NRGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	s := make([]color.NRGBA, sw*sh)
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			r, g, b, a := src.At(sb.Min.X+x, sb.Min.Y+y).RGBA()
			s[y*sw+x] = color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
		}
	}
	xwAll := make([][]sampleWeight, dw)
	for dx := 0; dx < dw; dx++ {
		cx := (float64(dx) + 0.5) * float64(sw) / float64(dw)
		xwAll[dx] = sampleWeights(cx, sw)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for dy := 0; dy < dh; dy++ {
		cy := (float64(dy) + 0.5) * float64(sh) / float64(dh)
		yw := sampleWeights(cy, sh)
		for dx := 0; dx < dw; dx++ {
			var r, g, b, a, total float64
			for _, wy := range yw {
				row := wy.index * sw
				for _, wx := range xwAll[dx] {
					w := wy.weight * wx.weight
					p := s[row+wx.index]
					pa := float64(p.A)
					r += w * float64(p.R) * pa / 255
					g += w * float64(p.G) * pa / 255
					b += w * float64(p.B) * pa / 255
					a += w * pa
					total += w
				}
			}
			if total > 0 {
				r /= total
				g /= total
				b /= total
				a /= total
			}
			dst.SetNRGBA(dx, dy, color.NRGBA{
				R: clampByte(r),
				G: clampByte(g),
				B: clampByte(b),
				A: clampByte(a),
			})
		}
	}
	return dst
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

	genW, genH, sizeChanged := generationSize(*width, *height)

	body, err := json.Marshal(imageRequest{
		Prompt:            *prompt,
		NegativePrompt:    *neg,
		Size:              fmt.Sprintf("%dx%d", genW, genH),
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

	if sizeChanged {
		fmt.Printf("note: requested size %dx%d, generated at %dx%d, saving at %dx%d\n",
			*width, *height, genW, genH, *width, *height)
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
		if decoded, derr := decodeImage(img); derr != nil {
			if sizeChanged {
				fmt.Fprintf(os.Stderr, "warning: cannot decode image for resize: %v (saving as-is)\n", derr)
			}
		} else if b := decoded.Bounds(); b.Dx() != *width || b.Dy() != *height {
			resized := resizeLanczos(decoded, *width, *height)
			data, newMime, eerr := encodeImage(resized, *format, *quality)
			if eerr != nil {
				fatalf("re-encode image: %v", eerr)
			}
			img = data
			mime = newMime
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
