package main

import (
	fatima "github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/lib"
	"github.com/fatima-go/fatima-core/runtime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type sample struct{}

func (*sample) Initialize() bool {
	rt := runtime.GetFatimaRuntime()
	return lib.RegisterCronJob(rt, "control.probe", func(_ string, r fatima.FatimaRuntime, args ...string) {
		path := filepath.Join(r.GetEnv().GetFolderGuide().GetDataFolder(), "requests.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			defer f.Close()
			f.WriteString(strings.Join(args, " ") + "\n")
		}
	}) == nil
}
func (*sample) Bootup()   {}
func (*sample) Goaway()   { time.Sleep(time.Second) }
func (*sample) Shutdown() { time.Sleep(time.Second) }
func main()               { rt := runtime.GetFatimaRuntime(); rt.Register(&sample{}); rt.Run() }
