package main

import (
	"encoding/json"
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
	"sync"
	"strings"
	"time"
)

var toqartBinary string
var buildTime string
var appLogger *Logger

func main() {
	toqartBinary = findToqartBinary()

	port := "5462"
	var logFmt, logOut string
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--port" && i+1 < len(os.Args) {
			port = os.Args[i+1]
			i++
		} else if os.Args[i] == "--toqart" && i+1 < len(os.Args) {
			toqartBinary = os.Args[i+1]
			i++
		} else if os.Args[i] == "--log" {
			logFmt = "plain"
		} else if strings.HasPrefix(os.Args[i], "--log=") {
			logFmt = os.Args[i][6:]
		} else if os.Args[i] == "--log-out" && i+1 < len(os.Args) {
			logOut = os.Args[i+1]
			i++
		}
	}

	if logFmt != "" {
		var err error
		appLogger, err = newLogger(logFmt, logOut)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create logger: %s\n", err)
			os.Exit(1)
		}
		defer appLogger.file.Close()
		fmt.Printf("[%s] Logging to %s [%s]\n", time.Now().Format("2006-01-02 15:04:05"), appLogger.file.Name(), logFmt)
	}

	http.HandleFunc("/", logMiddleware(handleIndex, "index page"))
	http.HandleFunc("/generate", logMiddleware(handleGenerate, "generate QR code"))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := ":" + port
	fmt.Printf("[%s] toQArt WebUI started at http://localhost%s\n", time.Now().Format("2006-01-02 15:04:05"), addr)
	fmt.Printf("[%s] toqart binary: %s\n", time.Now().Format("2006-01-02 15:04:05"), toqartBinary)
	if buildTime != "" {
		fmt.Printf("[%s] build time: %s\n", time.Now().Format("2006-01-02 15:04:05"), buildTime)
	}
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

func boolFlag(s string) string {
	if s == "true" {
		return "1"
	}
	return "0"
}

// ── Logger ──────────────────────────────────────────────

type Logger struct {
	mu     sync.Mutex
	format string
	file   *os.File
	w      io.Writer
}

func newLogger(format, outPath string) (*Logger, error) {
	if outPath == "" {
		now := time.Now()
		filename := fmt.Sprintf("%s.log", now.Format("20060102_150405"))
		outPath = filepath.Join("log", filename)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &Logger{
		format: format,
		file:   f,
		w:      io.MultiWriter(os.Stdout, f),
	}, nil
}

func (l *Logger) Print(method, ip, action string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().Format("2006-01-02 15:04:05")
	switch l.format {
	case "json":
		data, _ := json.Marshal(map[string]string{
			"time":   now,
			"method": method,
			"ip":     ip,
			"action": action,
		})
		fmt.Fprintln(l.w, string(data))
	default:
		fmt.Fprintf(l.w, "[%s] %s | %s | %s\n", now, method, ip, action)
	}
}

func getIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i >= 0 {
		ip = ip[:i]
	}
	return ip
}

func logMiddleware(next http.HandlerFunc, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if appLogger != nil {
			appLogger.Print(r.Method, getIP(r), action)
		}
		next(w, r)
	}
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
		writeError(w, http.StatusBadRequest, "Please select an image")
		return
	}
	defer file.Close()

	tmpDir, err := os.MkdirTemp("", "toqart-upload-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Server error")
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
		writeError(w, http.StatusInternalServerError, "Server error")
		return
	}
	io.Copy(dst, file)
	dst.Close()

	// ── Image pre-process & config ──
	mode := r.FormValue("mode")
	if mode == "invert" {
		if err := invertImage(inputPath); err != nil {
			writeError(w, http.StatusInternalServerError, "Image invert failed: "+err.Error())
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
	tintBrightness := atoi(r.FormValue("tint_brightness"))
	colorThreshold := atoi(r.FormValue("color_threshold"))

	useColor := "false"
	useColorTint := "false"
	invert := "false"
	switch mode {
	case "color":
		useColor = "true"
	case "tint":
		useColorTint = "true"
	case "invert":
		invert = "true"
	}

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
		if colorThreshold > 0 {
			args = append(args, "--color-threshold", strconv.Itoa(colorThreshold))
		}
	}
	if useColorTint == "true" {
		args = append(args, "--color-tint", "true")
		if tintBrightness > 0 {
			args = append(args, "--tint-brightness", strconv.Itoa(tintBrightness))
		}
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
			fmt.Sprintf("toqart execution failed:\n%s\n%s", string(output), err))
		return
	}

	resultFile, err := findFirstPNG(outDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			fmt.Sprintf("No QR code image found\nbinary output: %s\n", string(output)))
		return
	}

	// ── Post-process: scale ──
	if scale > 1 {
		resultFile, err = scaleImage(resultFile, scale)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Image scale failed")
			return
		}
	}

	w.Header().Set("Content-Type", "image/png")

	now := time.Now()
	flag := 0
	var val int
	if invert == "true" {
		flag = 4
	} else if useColor == "true" {
		flag = 2
		val = colorThreshold
	} else if useColorTint == "true" {
		flag = 1
		val = tintBrightness
	}
	fname := fmt.Sprintf("toqart_%02d%02d_%02d%02d%02d_%02d_%04d_%d%s_%04d.png",
		now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(),
		version, threshold, flag, boolFlag(usePattern), val,
	)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`inline; filename="%s"`, fname))
	http.ServeFile(w, r, resultFile)
}

// ── Util ─────────────────────────────────────────────────

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
