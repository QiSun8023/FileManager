package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const helperPackage = "io.github.qisun8023.filemanager.helper"

type WatchRule struct {
	ID         string   `json:"id"`
	Root       string   `json:"root"`
	Target     string   `json:"target"`
	Extensions []string `json:"extensions"`
	Recursive  bool     `json:"recursive"`
}
type WatchConfig struct {
	Enabled       bool        `json:"enabled"`
	StableSeconds int         `json:"stableSeconds"`
	Rules         []WatchRule `json:"rules"`
}
type WatchEntry struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Stable   int64  `json:"stable"`
	Baseline bool   `json:"baseline"`
	Item     string `json:"item,omitempty"`
}
type WatchItem struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Target   string `json:"target"`
	Rule     string `json:"rule"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Hash     string `json:"hash"`
	At       int64  `json:"at"`
	State    string `json:"state"`
	Error    string `json:"error,omitempty"`
	Verified bool   `json:"verified"`
}
type WatchState struct {
	Entries map[string]WatchEntry `json:"entries"`
	Items   map[string]WatchItem  `json:"items"`
	Updated int64                 `json:"updated"`
	Error   string                `json:"error,omitempty"`
}

func (e *Engine) watchConfig() WatchConfig {
	c := WatchConfig{StableSeconds: 60, Rules: []WatchRule{}}
	read(e.path("watch-config.json"), &c)
	return c
}
func (e *Engine) watchState() (WatchState, error) {
	s := WatchState{Entries: map[string]WatchEntry{}, Items: map[string]WatchItem{}}
	err := read(e.path("watch-state.json"), &s)
	if os.IsNotExist(err) {
		err = nil
	}
	if s.Entries == nil {
		s.Entries = map[string]WatchEntry{}
	}
	if s.Items == nil {
		s.Items = map[string]WatchItem{}
	}
	return s, err
}

var extensionPattern = regexp.MustCompile(`^\.[a-z0-9]{1,12}$`)

func (e *Engine) validateWatch(c WatchConfig) (WatchConfig, error) {
	if c.StableSeconds < 30 || c.StableSeconds > 3600 {
		return c, errors.New("稳定等待时间必须为 30–3600 秒")
	}
	if c.Enabled && len(c.Rules) == 0 {
		return c, errors.New("请至少添加一条监控规则")
	}
	if len(c.Rules) > 20 {
		return c, errors.New("最多 20 条监控规则")
	}
	ids := map[string]bool{}
	for i := range c.Rules {
		r := &c.Rules[i]
		var err error
		if r.Root, err = e.checked(r.Root); err != nil {
			return c, err
		}
		if r.Target, err = e.checked(r.Target); err != nil {
			return c, err
		}
		for _, p := range []string{r.Root, r.Target} {
			f, err := os.Stat(p)
			if err != nil || !f.IsDir() {
				return c, errors.New("请选择存在的目录")
			}
		}
		if r.ID == "" {
			r.ID = token()
		}
		if !validID.MatchString(r.ID) || ids[r.ID] {
			return c, errors.New("规则编号无效或重复")
		}
		ids[r.ID] = true
		if len(r.Extensions) == 0 || len(r.Extensions) > 30 {
			return c, errors.New("每条规则需要 1–30 种文件后缀")
		}
		seen := map[string]bool{}
		exts := []string{}
		for _, ext := range r.Extensions {
			ext = strings.ToLower(strings.TrimSpace(ext))
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			if !extensionPattern.MatchString(ext) {
				return c, errors.New("文件后缀格式无效")
			}
			if !seen[ext] {
				exts = append(exts, ext)
				seen[ext] = true
			}
		}
		r.Extensions = exts
	}
	for _, r := range c.Rules {
		for _, other := range c.Rules {
			if within(other.Root, r.Target) {
				return c, errors.New("目标目录不能位于任何监控目录内，以免反复提醒")
			}
		}
	}
	return c, nil
}
func watchKey(rule, path string) string { return rule + "|" + path }
func (e *Engine) watchFiles(c WatchConfig) (map[string]WatchEntry, error) {
	result := map[string]WatchEntry{}
	seenPaths := map[string]bool{}
	count := 0
	for _, r := range c.Rules {
		err := filepath.WalkDir(r.Root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p != r.Root && (d.IsDir() && (d.Name() == "Android" || strings.HasPrefix(d.Name(), ".")) || d.Type()&os.ModeSymlink != 0) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if p != r.Root && !r.Recursive {
					return filepath.SkipDir
				}
				return nil
			}
			count++
			if count > 100000 {
				return errors.New("监控文件超过 10 万，请缩小范围")
			}
			match := false
			for _, ext := range r.Extensions {
				if strings.EqualFold(filepath.Ext(p), ext) {
					match = true
					break
				}
			}
			if !match {
				return nil
			}
			info, er := d.Info()
			if er != nil {
				return er
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			if seenPaths[p] {
				return nil
			}
			seenPaths[p] = true
			result[watchKey(r.ID, p)] = WatchEntry{Path: p, Size: info.Size(), Modified: info.ModTime().UnixNano()}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("监控目录 %s：%w", r.Root, err)
		}
	}
	return result, nil
}
func (e *Engine) watchHash(path string) (string, error) {
	f, err := openWatchFile(e.Storage, path, os.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, &watchReader{Reader: f, e: e})
	return hex.EncodeToString(h.Sum(nil)), err
}
func (e *Engine) archiveComplete(path string) (bool, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".zip" && ext != ".apk" {
		return false, nil
	}
	file, err := openWatchFile(e.Storage, path, os.O_RDONLY, 0)
	if err != nil {
		return true, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return true, err
	}
	z, err := zip.NewReader(file, info.Size())
	if err != nil {
		return true, err
	}

	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return true, err
		}
		_, err = io.Copy(io.Discard, &watchReader{Reader: r, e: e})
		r.Close()
		if err != nil {
			return true, err
		}
	}
	return true, nil
}
func temporarySibling(path string) bool {
	for _, ext := range []string{".part", ".tmp", ".download", ".crdownload", ".aria2", ".!qB", ".td"} {
		if _, err := os.Lstat(path + ext); err == nil {
			return true
		}
	}
	return false
}

// Each successful directory pass is a snapshot; failed passes never erase the baseline.
func (e *Engine) watchTick(now time.Time) error {
	unlock, err := e.lock()
	if err != nil {
		return err
	}
	defer unlock()
	c := e.watchConfig()
	if !c.Enabled {
		return nil
	}
	s, err := e.watchState()
	if err != nil {
		return err
	}
	files, err := e.watchFiles(c)
	if err != nil {
		s.Error = err.Error()
		save(e.path("watch-state.json"), s)
		return err
	}
	rules := map[string]WatchRule{}
	for _, r := range c.Rules {
		rules[r.ID] = r
	}
	pendingCount := 0
	for _, item := range s.Items {
		if item.State == "pending" {
			pendingCount++
		}
	}
	for k, v := range files {
		old, exists := s.Entries[k]
		if !exists {
			v.Stable = now.Unix()
			s.Entries[k] = v
			continue
		}
		if old.Baseline {
			continue
		}
		if old.Size != v.Size || old.Modified != v.Modified {
			if old.Item != "" {
				item := s.Items[old.Item]
				item.State = "changed"
				s.Items[item.ID] = item
			}
			v.Stable = now.Unix()
			s.Entries[k] = v
			continue
		}
		if pendingCount >= 500 || old.Item != "" || now.Unix()-old.Stable < int64(c.StableSeconds) || v.Size == 0 || temporarySibling(v.Path) || watchWriting(v.Path) {
			continue
		}
		if err = safeRegular(v.Path); err != nil {
			continue
		}
		verified, er := e.archiveComplete(v.Path)
		if er != nil {
			continue
		}
		hash, er := e.watchHash(v.Path)
		if er != nil {
			continue
		}
		info, er := os.Stat(v.Path)
		if er != nil || info.Size() != v.Size || info.ModTime().UnixNano() != v.Modified {
			continue
		}
		rule := rules[strings.SplitN(k, "|", 2)[0]]
		id := token()
		pendingCount++
		s.Items[id] = WatchItem{ID: id, Path: v.Path, Target: rule.Target, Rule: rule.ID, Size: v.Size, Modified: v.Modified, Hash: hash, At: now.Unix(), State: "pending", Verified: verified}
		old.Item = id
		s.Entries[k] = old
	}
	for k, v := range s.Entries {
		if _, ok := files[k]; !ok {
			if v.Item != "" {
				item := s.Items[v.Item]
				if item.State == "pending" {
					item.State = "gone"
					s.Items[item.ID] = item
				}
			}
			delete(s.Entries, k)
		}
	}
	// Keep all active decisions, bound completed history so it does not grow forever.
	if len(s.Items) > 1000 {
		items := []WatchItem{}
		for _, v := range s.Items {
			if v.State != "pending" {
				items = append(items, v)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].At < items[j].At })
		for _, v := range items {
			if len(s.Items) <= 1000 {
				break
			}
			delete(s.Items, v.ID)
		}
	}
	s.Updated = now.Unix()
	s.Error = ""
	return save(e.path("watch-state.json"), s)
}
func (e *Engine) watchSave(c WatchConfig) (any, error) {
	var err error
	c, err = e.validateWatch(c)
	if err != nil {
		return nil, err
	}
	unlock, err := e.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	old := e.watchConfig()
	s, err := e.watchState()
	if err != nil {
		return nil, err
	}
	// New/re-enabled/reconfigured rules establish their baseline at the moment of saving.
	prior := map[string]WatchRule{}
	for _, r := range old.Rules {
		prior[r.ID] = r
	}
	files, err := e.watchFiles(c)
	if err != nil {
		return nil, err
	}
	valid := map[string]bool{}
	for _, r := range c.Rules {
		valid[r.ID] = true
		o, ok := prior[r.ID]
		same := ok && fmt.Sprint(o) == fmt.Sprint(r) && old.Enabled
		for k, v := range files {
			if strings.HasPrefix(k, r.ID+"|") && !same {
				v.Baseline = true
				s.Entries[k] = v
			}
		}
	}
	for k := range s.Entries {
		if !valid[strings.SplitN(k, "|", 2)[0]] {
			delete(s.Entries, k)
		}
	}
	for id, v := range s.Items {
		if v.State == "pending" {
			o, ok := prior[v.Rule]
			changed := !ok
			for _, r := range c.Rules {
				if r.ID == v.Rule {
					changed = changed || fmt.Sprint(o) != fmt.Sprint(r)
				}
			}
			if !valid[v.Rule] || changed || !old.Enabled {
				v.State = "cancelled"
				s.Items[id] = v
			}
		}
	}
	// Persist the baseline before enabling. A failure can suppress a reminder but cannot move a file.
	if err = save(e.path("watch-state.json"), s); err != nil {
		return nil, err
	}
	if err = save(e.path("watch-config.json"), c); err != nil {
		return nil, err
	}
	return c, nil
}
func (e *Engine) watchDecide(r Request) (any, error) {
	if !validID.MatchString(r.ID) || (r.Decision != "move" && r.Decision != "keep") {
		return nil, errors.New("无效待处理操作")
	}
	unlock, err := e.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	s, err := e.watchState()
	if err != nil {
		return nil, err
	}
	v, ok := s.Items[r.ID]
	if !ok || v.State != "pending" {
		return nil, errors.New("该提醒已处理或过期")
	}
	if r.Decision == "keep" {
		v.State = "kept"
		for key, entry := range s.Entries {
			if entry.Item == v.ID {
				entry.Baseline = true
				s.Entries[key] = entry
			}
		}
		s.Items[v.ID] = v
		return v, save(e.path("watch-state.json"), s)
	}
	c := e.watchConfig()
	if !c.Enabled {
		return nil, errors.New("监控已关闭，不能执行转移")
	}
	matched := false
	for _, rule := range c.Rules {
		if rule.ID == v.Rule && rule.Target == v.Target && within(rule.Root, v.Path) {
			matched = true
		}
	}
	if !matched {
		return nil, errors.New("规则已改变，请刷新")
	}
	if _, err = e.checked(v.Path); err != nil {
		return nil, err
	}
	if _, err = e.checked(v.Target); err != nil {
		return nil, err
	}
	if temporarySibling(v.Path) || watchWriting(v.Path) {
		return nil, errors.New("仍有下载临时记录，请稍后再试")
	}
	if err = safeRegular(v.Path); err != nil {
		return nil, err
	}
	source, err := openWatchFile(e.Storage, v.Path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	info, err := source.Stat()
	source.Close()
	if err != nil {
		return nil, err
	}
	hash, err := e.watchHash(v.Path)
	if err != nil {
		return nil, err
	}
	if hash != v.Hash || info.Size() != v.Size || info.ModTime().UnixNano() != v.Modified {
		return nil, errors.New("文件已变化，拒绝转移，请等待重新检测")
	}
	if _, err = e.archiveComplete(v.Path); err != nil {
		return nil, errors.New("压缩包不完整或校验失败")
	}
	dest := filepath.Join(v.Target, filepath.Base(v.Path))
	if err = e.watchCopy(v.Path, dest, v.Hash); err != nil {
		return nil, fmt.Errorf("未转移（目标同名时不会覆盖）：%w", err)
	}
	copied, err := os.Stat(dest)
	if err != nil {
		return nil, err
	}
	current, er := os.Stat(v.Path)
	again, he := e.watchHash(v.Path)
	if er != nil || he != nil || watchWriting(v.Path) || !os.SameFile(info, current) || current.Size() != v.Size || current.ModTime().UnixNano() != v.Modified || again != v.Hash {
		removeWatchFile(e.Storage, dest, copied)
		return nil, errors.New("复制期间源文件变化，源文件已保留")
	}
	// Journal the verified destination before deleting source. Recovery never retries an overwrite.
	v.State = "copied"
	v.Error = dest
	s.Items[v.ID] = v
	if err = save(e.path("watch-state.json"), s); err != nil {
		if check, er := os.Stat(dest); er == nil && os.SameFile(copied, check) {
			removeWatchFile(e.Storage, dest, copied)
		}
		return nil, err
	}
	if err = removeWatchFile(e.Storage, v.Path, info); err != nil {
		return nil, fmt.Errorf("目标已复制，但源文件删除失败，两份文件均保留：%w", err)
	}
	timeWatchFile(e.Storage, dest, info.ModTime())
	v.State = "moved"
	v.Error = ""
	s.Items[v.ID] = v
	return v, save(e.path("watch-state.json"), s)
}
func (e *Engine) startWatch() error {
	if runtime.GOOS != "android" {
		return errors.New("后台监控只能在 Android 上启动")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if !e.watchConfig().Enabled {
		return nil
	}
	if watchDaemonAlive(e) {
		return nil
	}
	cmd := exec.Command(self, "watch-daemon", "e30=")
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd.Stdin = f
	log, logErr := os.OpenFile(e.path("watch-daemon.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if logErr != nil {
		return logErr
	}
	defer log.Close()
	cmd.Stdout = log
	cmd.Stderr = log
	detachWatch(cmd)
	if err = cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

var errWatchReplaced = errors.New("watch executable replaced")

func (e *Engine) watchDaemon() error {
	err := runWatchDaemon(e)
	if errors.Is(err, errWatchReplaced) {
		return reexecWatch()
	}
	return err
}
func (e *Engine) watchDispatch(action string, r Request) (any, error) {
	switch action {
	case "watch-config":
		return e.watchConfig(), nil
	case "watch-save":
		if r.Watch == nil {
			return nil, errors.New("缺少监控配置")
		}
		v, err := e.watchSave(*r.Watch)
		if err == nil && r.Watch.Enabled {
			err = e.startWatch()
		}
		return v, err
	case "watch-status":
		s, err := e.watchState()
		if err != nil {
			return nil, err
		}
		items := []WatchItem{}
		history := []WatchItem{}
		for _, v := range s.Items {
			if v.State == "pending" {
				items = append(items, v)
			} else {
				history = append(history, v)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].At > items[j].At })
		sort.Slice(history, func(i, j int) bool { return history[i].At > history[j].At })
		if len(history) > 30 {
			history = history[:30]
		}
		return map[string]any{"enabled": e.watchConfig().Enabled, "running": watchDaemonAlive(e), "updated": s.Updated, "error": s.Error, "items": items, "history": history, "version": version, "helper": e.helperStatus()}, nil
	case "watch-decide":
		return e.watchDecide(r)
	case "watch-start":
		return nil, e.startWatch()
	case "helper-open":
		if runtime.GOOS != "android" {
			return nil, errors.New("只能在 Android 上打开辅助应用")
		}
		out, err := exec.Command("am", "start", "-n", helperPackage+"/.MainActivity").CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("辅助应用未能打开：%s", out)
		}
		return string(out), nil
	}
	return nil, errors.New("未知监控操作")
}

func (e *Engine) watchCopy(src, dst, hash string) (err error) {
	in, err := openWatchFile(e.Storage, src, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := openWatchFile(e.Storage, dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0660)
	if err != nil {
		return err
	}
	info, _ := out.Stat()
	ok := false
	defer func() {
		out.Close()
		if !ok && info != nil {
			removeWatchFile(e.Storage, dst, info)
		}
	}()
	h := sha256.New()
	if _, err = io.Copy(io.MultiWriter(out, h), in); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != hash {
		return errors.New("复制校验失败")
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func (e *Engine) helperStatus() string {
	b, err := os.ReadFile(e.path("helper-status"))
	if err != nil {
		return "尚未确认辅助应用安装状态"
	}
	return strings.TrimSpace(string(b))
}

// Allow uninstall/update to stop large archive/hash checks without deleting source files.
type watchReader struct {
	io.Reader
	e    *Engine
	next time.Time
}

func (r *watchReader) Read(b []byte) (int, error) {
	if time.Now().After(r.next) {
		r.next = time.Now().Add(time.Second)
		if _, err := os.Stat(r.e.path("watch-stop")); err == nil {
			return 0, errors.New("监控检查已停止")
		}
	}
	return r.Reader.Read(b)
}
