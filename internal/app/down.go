package app

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/lm-lovett/intranet-penetration-cloudflare/internal/config"
)

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func Down() error {
	pid, err := config.ReadPID()
	if err != nil {
		return fmt.Errorf("cfpen is not running")
	}
	if !pidAlive(pid) {
		config.ClearRuntimeFiles()
		return fmt.Errorf("stale pid %d; cleaned up", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		_ = proc.Kill()
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			config.ClearRuntimeFiles()
			fmt.Println("stopped")
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	_ = proc.Kill()
	config.ClearRuntimeFiles()
	fmt.Println("killed")
	return nil
}
