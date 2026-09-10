package main

import (
	"github.com/fatima-go/fatima-core/runtime"
	"os"
	"path/filepath"
	"time"
)

// A real Fatima process used by the native and Linux smoke tests. Its drain
// and shutdown windows are deliberately visible in the deployment timeline.
type sample struct{}

func (*sample) Initialize() bool { return true }
func (*sample) Bootup() {
	_ = os.WriteFile(filepath.Join("..", "sample-started"), []byte(time.Now().Format(time.RFC3339)), 0644)
}
func (*sample) Goaway()   { time.Sleep(2 * time.Second) }
func (*sample) Shutdown() { time.Sleep(time.Second) }
func main()               { rt := runtime.GetFatimaRuntime(); rt.Register(&sample{}); rt.Run() }
