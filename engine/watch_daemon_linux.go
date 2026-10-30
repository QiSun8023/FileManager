//go:build linux || android

package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func runWatchDaemon(e *Engine) error {
	d := e.path("watch-daemon")
	if err := ensureDir(d); err != nil {
		return err
	}
	f, err := os.OpenFile(d+"/owner", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	os.WriteFile(d+"/pid", []byte(strconv.Itoa(os.Getpid())), 0600)
	defer os.Remove(d + "/pid")

	lastWake := time.Time{}
	for {
		if _, err = os.Stat("/data/adb/modules/filemanager/disable"); err == nil {
			return nil
		}
		if _, err = os.Stat("/data/adb/modules/filemanager/remove"); err == nil {
			return nil
		}
		if _, err = os.Stat(e.path("watch-stop")); err == nil {
			return nil
		}
		c := e.watchConfig()
		if !c.Enabled {
			return nil
		}
		e.watchTick(time.Now())
		s, er := e.watchState()
		pending := false
		if er == nil {
			for _, v := range s.Items {
				if v.State == "pending" {
					pending = true
					break
				}
			}
		}
		if pending && time.Since(lastWake) > time.Minute {
			exec.Command("am", "start-foreground-service", "-n", helperPackage+"/.MonitorService").Run()
			lastWake = time.Now()
		}
		time.Sleep(5 * time.Second)
		link, er := os.Readlink("/proc/self/exe")
		if er == nil && strings.HasSuffix(link, " (deleted)") {
			return nil
		}
	}
}

// An open writable descriptor is a reason to keep waiting, even if size is stable.
func watchWriting(path string) bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, p := range entries {
		if _, err := strconv.Atoi(p.Name()); err != nil {
			continue
		}
		base := "/proc/" + p.Name() + "/fd/"
		fds, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(base + fd.Name())
			if err != nil || link != path {
				continue
			}
			b, err := os.ReadFile("/proc/" + p.Name() + "/fdinfo/" + fd.Name())
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(line, "flags:") {
					fields := strings.Fields(line)
					if len(fields) > 1 {
						flags, er := strconv.ParseInt(fields[1], 8, 64)
						if er == nil && flags&syscall.O_ACCMODE != syscall.O_RDONLY {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func detachWatch(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
