//go:build integration
// +build integration

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func killAllProcesses(serverCmd, agentCmd *exec.Cmd) {
	log.Println("Forcibly terminating all processes...")

	// Windows specific: Use taskkill to forcefully terminate processes
	if runtime.GOOS == "windows" {
		// Kill all Go processes
		exec.Command("taskkill", "/F", "/IM", "go.exe", "/T").Run()
		exec.Command("taskkill", "/F", "/IM", "server.exe", "/T").Run()
		exec.Command("taskkill", "/F", "/IM", "agent.exe", "/T").Run()

		// Kill processes by port (for Windows)
		killProcessesOnPort(9090)
		killProcessesOnPort(9091)
	} else {
		// Unix/Linux/MacOS
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Signal(syscall.SIGKILL)
			serverCmd.Wait()
		}
		if agentCmd != nil && agentCmd.Process != nil {
			agentCmd.Process.Signal(syscall.SIGKILL)
			agentCmd.Wait()
		}
	}

	// Short delay to ensure processes are terminated
	time.Sleep(500 * time.Millisecond)
}

func killProcessesOnPort(port int) {
	if runtime.GOOS != "windows" {
		return
	}

	// Find and kill process using port on Windows
	cmd := exec.Command("netstat", "-ano")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf(":%d", port)) && strings.Contains(line, "LISTENING") {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				pid := parts[len(parts)-1]
				exec.Command("taskkill", "/F", "/PID", pid, "/T").Run()
			}
		}
	}
}

func main() {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	log.Println("🚀 Starting integration test...")

	// Use file storage for simplicity
	tempFile := os.TempDir() + "\\metrics_integration_test.json"
	os.Remove(tempFile)

	// Start server
	log.Println("Starting server...")
	serverCmd := exec.CommandContext(ctx, "go", "run", "./cmd/server/main.go",
		"-a=localhost:9090",
		"-i=30",
		"-f="+tempFile,
		"-r=true")

	serverCmd.Stdout = os.Stdout
	serverCmd.Stderr = os.Stderr

	if err := serverCmd.Start(); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}

	// Wait for server to start
	log.Println("Waiting for server to start...")
	time.Sleep(5 * time.Second)

	// Check if server is running
	resp, err := http.Get("http://localhost:9090/ping")
	if err != nil {
		killAllProcesses(serverCmd, nil)
		log.Fatalf("❌ Server not responding: %v", err)
	}
	resp.Body.Close()

	log.Println("✅ Server is responding")

	// Start agent
	log.Println("Starting agent...")
	agentCmd := exec.CommandContext(ctx, "go", "run", "./cmd/agent/main.go",
		"-a=localhost:9090",
		"-r=2",
		"-p=1")

	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	if err := agentCmd.Start(); err != nil {
		killAllProcesses(serverCmd, nil)
		log.Fatalf("❌ Failed to start agent: %v", err)
	}

	// Wait for metrics to be sent
	log.Println("Waiting for metrics to be sent...")
	time.Sleep(5 * time.Second)

	// Check main page
	log.Println("Checking main page...")
	resp, err = http.Get("http://localhost:9090/")
	if err != nil {
		killAllProcesses(serverCmd, agentCmd)
		log.Fatalf("❌ Failed to get main page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		killAllProcesses(serverCmd, agentCmd)
		log.Fatalf("❌ Unexpected status: %d", resp.StatusCode)
	}

	fmt.Println("\n✅ Integration test passed!")

	// Force kill ALL processes
	log.Println("Terminating all test processes...")
	killAllProcesses(serverCmd, agentCmd)

	log.Println("Test completed successfully!")
}
