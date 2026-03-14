// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

func main() {
	base := flag.String("base", "http://localhost", "Base URL of the Traefik gateway")
	videoPath := flag.String("video", "scripts/test.mp4", "Path to test video file")
	count := flag.Int("count", 5, "Number of videos to upload")
	flag.Parse()

	if _, err := os.Stat(*videoPath); os.IsNotExist(err) {
		fatalf("Test video not found: %s\nRun 'make test-video' first.", *videoPath)
	}

	step("Registering test user...")
	email := fmt.Sprintf("failover-%d@test.local", time.Now().UnixMilli())
	register(*base, email, "testpass123")

	step("Logging in...")
	token := login(*base, email, "testpass123")
	info("JWT obtained (%d chars)", len(token))

	step("Uploading %d videos...", *count)
	uploadStart := time.Now()
	for i := 1; i <= *count; i++ {
		upload(*base, token, *videoPath)
		info("  uploaded %d/%d", i, *count)
	}
	info("All uploads done in %s", time.Since(uploadStart).Round(time.Millisecond))

	step("Waiting 2s for workers to start processing...")
	time.Sleep(2 * time.Second)

	statuses := pollVideos(*base, token)
	printStatusSummary("Before kill", statuses)

	step("Stopping aurora-api-1...")
	killStart := time.Now()
	dockerStop("aurora-api-1")
	success("aurora-api-1 stopped")

	step("Polling video statuses (timeout 120s)...")
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		statuses = pollVideos(*base, token)
		printStatusSummary(fmt.Sprintf("  t+%ds", int(time.Since(killStart).Seconds())), statuses)

		if allTerminal(statuses) {
			break
		}
	}

	fmt.Println()
	step("=== FAILOVER REPORT ===")
	totalTime := time.Since(killStart).Round(time.Millisecond)
	ready := countStatus(statuses, "ready")
	failed := countStatus(statuses, "failed")
	processing := countStatus(statuses, "processing")
	uploaded := countStatus(statuses, "uploaded")

	if ready == len(statuses) {
		success("All %d videos recovered successfully in %s", ready, totalTime)
	} else {
		warn("Results: ready=%d, failed=%d, processing=%d, uploaded=%d (total=%d, elapsed=%s)",
			ready, failed, processing, uploaded, len(statuses), totalTime)
	}

	step("Restarting aurora-api-1...")
	dockerStart("aurora-api-1")
	success("aurora-api-1 restarted")

	fmt.Println()
	success("Failover demo complete.")
}

func register(base, email, password string) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := http.Post(base+"/api/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		fatalf("register request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		fatalf("register failed (status %d): %s", resp.StatusCode, string(b))
	}
}

func login(base, email, password string) string {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := http.Post(base+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		fatalf("login failed (status %d): %s", resp.StatusCode, string(b))
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	token, _ := result["token"].(string)
	if token == "" {
		fatalf("no token in login response")
	}
	return token
}

func upload(base, token, videoPath string) {
	file, err := os.Open(videoPath)
	if err != nil {
		fatalf("open video: %v", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filepath.Base(videoPath))
	if err != nil {
		fatalf("create form file: %v", err)
	}
	io.Copy(part, file)
	writer.Close()

	req, _ := http.NewRequest("POST", base+"/api/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		fatalf("upload failed (status %d): %s", resp.StatusCode, string(b))
	}
}

type videoResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type videosListResponse struct {
	Videos []videoResponse `json:"videos"`
}

func pollVideos(base, token string) []videoResponse {
	req, _ := http.NewRequest("GET", base+"/api/videos?limit=100", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		warn("poll failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var result videosListResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Videos
}

func allTerminal(videos []videoResponse) bool {
	for _, v := range videos {
		if v.Status != "ready" && v.Status != "failed" {
			return false
		}
	}
	return len(videos) > 0
}

func countStatus(videos []videoResponse, status string) int {
	n := 0
	for _, v := range videos {
		if v.Status == status {
			n++
		}
	}
	return n
}

func printStatusSummary(prefix string, videos []videoResponse) {
	counts := map[string]int{}
	for _, v := range videos {
		counts[v.Status]++
	}
	parts := ""
	for _, s := range []string{"uploaded", "processing", "ready", "failed"} {
		if c, ok := counts[s]; ok && c > 0 {
			parts += fmt.Sprintf(" %s=%d", s, c)
		}
	}
	info("%s:%s (total=%d)", prefix, parts, len(videos))
}

func dockerStop(container string) {
	cmd := exec.Command("docker", "stop", "-t", "1", container)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatalf("docker stop %s: %v", container, err)
	}
}

func dockerStart(container string) {
	cmd := exec.Command("docker", "start", container)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		warn("docker start %s: %v", container, err)
	}
}

func step(format string, args ...interface{}) {
	fmt.Printf("\n%s%s[STEP]%s %s\n", colorBold, colorCyan, colorReset, fmt.Sprintf(format, args...))
}

func info(format string, args ...interface{}) {
	fmt.Printf("%s[INFO]%s %s\n", colorCyan, colorReset, fmt.Sprintf(format, args...))
}

func success(format string, args ...interface{}) {
	fmt.Printf("%s[OK]%s %s\n", colorGreen, colorReset, fmt.Sprintf(format, args...))
}

func warn(format string, args ...interface{}) {
	fmt.Printf("%s[WARN]%s %s\n", colorYellow, colorReset, fmt.Sprintf(format, args...))
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s[FATAL]%s %s\n", colorRed, colorReset, fmt.Sprintf(format, args...))
	os.Exit(1)
}
