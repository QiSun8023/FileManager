//go:build linux || android

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func runWatchDaemon(e *Engine) error {
	// Only detach this daemon from a host app cgroup; never change global freezer settings.
	if os.Getuid() == 0 {
		if cg, er := os.ReadFile("/proc/self/cgroup"); er == nil && strings.Contains(string(cg), "0::/apps/") {
			if er = os.WriteFile("/sys/fs/cgroup/cgroup.procs", []byte(strconv.Itoa(os.Getpid())), 0600); er != nil {
				return fmt.Errorf("后台进程无法脱离宿主应用分组：%w", er)
			}
		}
	}
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
	os.WriteFile(d+"/start", []byte(watchProcessStart(os.Getpid())), 0600)
	defer os.Remove(d + "/pid")
	defer os.Remove(d + "/start")

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
		current, ce := os.Stat("/data/adb/modules/filemanager/bin/filemanager")
		own, oe := os.Stat("/proc/self/exe")
		if ce == nil && oe == nil && !os.SameFile(current, own) {
			return errWatchReplaced
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

func watchProcessStart(pid int) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return ""
	}
	end := strings.LastIndexByte(string(b), ')')
	if end < 0 {
		return ""
	}
	fields := strings.Fields(string(b)[end+1:])
	if len(fields) < 20 {
		return ""
	}
	return fields[19]
}
func watchDaemonAlive(e *Engine) bool {
	b, err := os.ReadFile(e.path("watch-daemon/pid"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return false
	}
	start, err := os.ReadFile(e.path("watch-daemon/start"))
	if err != nil || string(start) == "" || string(start) != watchProcessStart(pid) {
		return false
	}
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil || !(filepath.Clean(exe) == "/data/adb/modules/filemanager/bin/filemanager" || strings.HasPrefix(exe, "/data/adb/ksu/hot_install_trash/filemanager.") && strings.HasSuffix(exe, "/bin/filemanager")) {
		return false
	}
	cmd, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	return err == nil && strings.Contains(string(cmd), "\x00watch-daemon\x00")
}

func reexecWatch() error {
	return syscall.Exec("/data/adb/modules/filemanager/bin/filemanager", []string{"/data/adb/modules/filemanager/bin/filemanager", "watch-daemon", "e30="}, os.Environ())
}
