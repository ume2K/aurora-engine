// +build ignore

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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
	n := flag.Int("n", 1000, "Total number of requests")
	c := flag.Int("c", 50, "Number of concurrent workers")
	z := flag.String("z", "30s", "Duration of the load test (e.g. 30s, 1m)")
	flag.Parse()

	if err := checkHey(); err != nil {
		fatalf("hey not found in PATH. Install: go install github.com/rakyll/hey@latest")
	}

	step("Registering test user...")
	email := fmt.Sprintf("loadtest-%d@test.local", time.Now().UnixMilli())
	password := "testpass123"
	register(*base, email, password)
	success("User registered")

	step("Logging in...")
	token := login(*base, email, password)
	success("JWT obtained (%d chars)", len(token))

	endpoint := *base + "/api/videos?limit=20"
	step("Running hey: %d requests, %d workers, duration %s", *n, *c, *z)
	info("Target: GET %s (JWT-protected, DB read)", endpoint)

	args := []string{
		"-n", fmt.Sprintf("%d", *n),
		"-c", fmt.Sprintf("%d", *c),
		"-z", *z,
		"-H", "Authorization: Bearer " + token,
		"-H", "Accept: application/json",
		endpoint,
	}

	cmd := exec.Command("hey", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println()
	if err := cmd.Run(); err != nil {
		fatalf("hey exited with error: %v", err)
	}

	fmt.Println()
	success("Load test complete.")
}

func checkHey() error {
	_, err := exec.LookPath("hey")
	return err
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
