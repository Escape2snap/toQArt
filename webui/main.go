package main

import (
	"fmt"
	"html/template"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var toqartBinary string

func main() {
	toqartBinary = findToqartBinary()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/generate", handleGenerate)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := ":8080"
	fmt.Printf("toQArt WebUI 启动于 http://localhost%s\n", addr)
	fmt.Printf("toqart 二进制路径: %s\n", toqartBinary)
	if err := http.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}

// ── Helpers ──────────────────────────────────────────────

func findToqartBinary() string {
	candidates := []string{
		"../target/debug/toqart",
		"../target/release/toqart",
		"./target/debug/toqart",
		"./target/release/toqart",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// fallback
	return "../target/debug/toqart"
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, msg)
}

func invertImage(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	src, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return err
	}

	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b_, a := src.At(x, y).RGBA()
			dst.Set(x, y, color.RGBA{
				R: uint8(255 - r/257),
				G: uint8(255 - g/257),
				B: uint8(255 - b_/257),
				A: uint8(a / 257),
			})
		}
	}
	out, _ := os.Create(path)
	defer out.Close()
	return png.Encode(out, dst)
}

func scaleImage(path string, factor int) (string, error) {
	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	img, err := png.Decode(src)
	src.Close()
	if err != nil {
		return "", err
	}

	b := img.Bounds()
	w, h := b.Dx()*factor, b.Dy()*factor
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		sy := y / factor
		for x := 0; x < w; x++ {
			dst.Set(x, y, img.At(x/factor, sy))
		}
	}

	scaled, _ := os.Create(path + ".scaled")
	defer scaled.Close()
	return scaled.Name(), png.Encode(scaled, dst)
}

func findFirstPNG(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var best string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".png") {
			if best == "" || e.Name() > best {
				best = e.Name()
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("no PNG found in %s", dir)
	}
	return filepath.Join(dir, best), nil
}

func oddVersion(v int) int {
	if v > 0 && v%2 == 0 {
		return v + 1
	}
	return v
}

// ── Handlers ─────────────────────────────────────────────

func handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	tmpl.Execute(w, nil)
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	r.ParseMultipartForm(32 << 20)

	// ── Save uploaded file ──
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "请选择一张图片")
		return
	}
	defer file.Close()

	tmpDir, err := os.MkdirTemp("", "toqart-upload-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	defer os.RemoveAll(tmpDir)

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".png"
	}
	inputPath := filepath.Join(tmpDir, "input"+ext)
	dst, err := os.Create(inputPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	io.Copy(dst, file)
	dst.Close()

	// ── Image pre-process ──
	if r.FormValue("invert") == "true" {
		if err := invertImage(inputPath); err != nil {
			writeError(w, http.StatusInternalServerError, "图片反色失败: "+err.Error())
			return
		}
	}

	// ── Parse config ──
	content := r.FormValue("content")
	if content == "" {
		content = "Attention Is All You Need."
	}

	version := oddVersion(atoi(r.FormValue("qr_version")))
	threshold := atoi(r.FormValue("threshold"))
	usePattern := r.FormValue("use_pattern")
	xAspect := atoi(r.FormValue("x_aspect"))
	yAspect := atoi(r.FormValue("y_aspect"))
	padL := atoi(r.FormValue("pad_l"))
	padR := atoi(r.FormValue("pad_r"))
	scale := atoi(r.FormValue("scale"))
	useColor := r.FormValue("use_color")
	colorThreshold := atoi(r.FormValue("color_threshold"))

	// ── Build & run toqart ──
	outDir := filepath.Join(tmpDir, "output")
	args := []string{
		"--content", content,
		"--output-path", outDir,
	}
	if usePattern == "true" {
		args = append(args, "--use-pattern", "true")
	}
	if useColor == "true" {
		args = append(args, "--color", "true")
	}
	if useColor == "true" && colorThreshold > 0 {
		args = append(args, "--color-threshold", strconv.Itoa(colorThreshold))
	}
	if version > 0 {
		args = append(args, "--qr-version", strconv.Itoa(version))
	}
	if threshold >= 0 && threshold <= 255 {
		args = append(args, "--threshold", strconv.Itoa(threshold))
	}
	if xAspect > 0 {
		args = append(args, "--x-aspect", strconv.Itoa(xAspect))
	}
	if yAspect > 0 {
		args = append(args, "--y-aspect", strconv.Itoa(yAspect))
	}
	if padL >= 0 {
		args = append(args, "--pad-l", strconv.Itoa(padL))
	}
	if padR >= 0 {
		args = append(args, "--pad-r", strconv.Itoa(padR))
	}
	args = append(args, inputPath)

	output, err := exec.Command(toqartBinary, args...).CombinedOutput()
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			fmt.Sprintf("执行 toqart 失败:\n%s\n%s", string(output), err))
		return
	}

	resultFile, err := findFirstPNG(outDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			fmt.Sprintf("未找到生成的二维码图片\n二进制输出: %s\n", string(output)))
		return
	}

	// ── Post-process: scale ──
	if scale > 1 {
		resultFile, err = scaleImage(resultFile, scale)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "缩放图片失败")
			return
		}
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`inline; filename="toqart_%d.png"`, time.Now().Unix()))
	http.ServeFile(w, r, resultFile)
}

// ── Util ─────────────────────────────────────────────────

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
