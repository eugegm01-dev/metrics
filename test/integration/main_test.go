// test/integration/main_test.go
//go:build integration

package integration

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestIntegration(t *testing.T) {
	// Запуск сервера с PostgreSQL
	cmdServer := exec.Command("go", "run", "../../cmd/server/main.go",
		"-a=localhost:9090",
		"-d=postgres://postgres:password@localhost:5432/metrics?sslmode=disable")
	cmdServer.Stdout = os.Stdout
	cmdServer.Stderr = os.Stderr
	go func() {
		if err := cmdServer.Run(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()
	time.Sleep(3 * time.Second)

	// Запуск агента
	cmdAgent := exec.Command("go", "run", "../../cmd/agent/main.go",
		"-a=localhost:9090",
		"-r=2",
		"-p=1")
	cmdAgent.Stdout = os.Stdout
	cmdAgent.Stderr = os.Stderr
	go func() {
		if err := cmdAgent.Run(); err != nil {
			log.Printf("Agent error: %v", err)
		}
	}()

	// Проверка работы
	time.Sleep(5 * time.Second)
	resp, err := http.Get("http://localhost:9090/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	fmt.Println("Integration test passed!")
}
