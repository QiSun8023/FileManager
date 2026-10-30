package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type File struct {
	Selected bool   `json:"selected"`
	ID       string `json:"id"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Moved    bool   `json:"moved"`
}
type Group struct {
	Keeper string `json:"keeper,omitempty"`
	Reason string `json:"reason,omitempty"`
	Hash   string `json:"hash"`
	Files  []File `json:"files"`
	Count  int    `json:"count,omitempty"`
}
type Report struct {
	ID     string  `json:"id"`
	Root   string  `json:"root"`
	Groups []Group `json:"groups"`
}
type Status struct {
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Moved   int    `json:"moved"`
	Job     string `json:"job,omitempty"`
	Phase   string `json:"phase"`
	Message string `json:"message"`
	Files   int    `json:"files"`
	Hashed  int    `json:"hashed"`
	Skipped int    `json:"skipped"`
	Groups  int    `json:"groups"`
	Bytes   int64  `json:"bytes"`
	Updated int64  `json:"updated"`
}
type Request struct {
	Watch      *WatchConfig `json:"watch,omitempty"`
	Decision   string       `json:"decision,omitempty"`
	Revision   string       `json:"revision"`
	Mode       string       `json:"mode"`
	ID         string       `json:"id"`
	Checked    bool         `json:"checked"`
	Job        string       `json:"job,omitempty"`
	RepairTime bool         `json:"repairTime"`
	RenameAAC  bool         `json:"renameAAC"`
	Zone       string       `json:"zone"`
	Root       string       `json:"root"`
	Min        int64        `json:"min"`
	Scan       string       `json:"scan"`
	IDs        []string     `json:"ids"`
	Page       int          `json:"page"`
}
type Trash struct {
	Selected bool   `json:"selected"`
	Modified *int64 `json:"modified,omitempty"`
	ID       string `json:"id"`
	Path     string `json:"path"`
	Hash     string `json:"hash"`
	Size     int64  `json:"size"`
	At       int64  `json:"at"`
}
type Engine struct {
	Data, Storage string
	status        Status
	last          time.Time
}

var cancelled = errors.New("扫描已停止")
var validID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func token() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func (e *Engine) path(name string) string { return filepath.Join(e.Data, name) }
func save(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
func read(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}
func (e *Engine) checked(p string) (string, error) {
	p = filepath.Clean(p)
	if !filepath.IsAbs(p) || !within(e.Storage, p) {
		return "", errors.New("只允许内部共享存储目录")
	}
	rel, _ := filepath.Rel(e.Storage, p)
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "." {
			continue
		}
		if part == "Android" || strings.HasPrefix(part, ".") {
			return "", errors.New("已排除 Android 和隐藏目录")
		}
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	if real != p {
		return "", errors.New("拒绝符号链接路径")
	}
	return p, nil
}
func (e *Engine) lock() (func(), error) {
	p := e.path("lock")
	err := os.Mkdir(p, 0700)
	if err != nil {
		var pid int
		b, _ := os.ReadFile(filepath.Join(p, "pid"))
		fmt.Sscanf(string(b), "%d", &pid)
		if pid > 0 && !alive(pid) {
			if err = os.Remove(filepath.Join(p, "pid")); err == nil {
				err = os.Remove(p)
			}
			if err == nil {
				err = os.Mkdir(p, 0700)
			}
		}
		if err != nil {
			return nil, errors.New("另一项操作正在进行，请稍后重试")
		}
	}
	if err = os.WriteFile(filepath.Join(p, "pid"), []byte(fmt.Sprint(os.Getpid())), 0600); err != nil {
		os.Remove(p)
		return nil, err
	}
	return func() { os.Remove(filepath.Join(p, "pid")); os.Remove(p) }, nil
}
func (e *Engine) update(force bool) error {
	if !force && time.Since(e.last) < time.Second {
		return nil
	}
	e.last = time.Now()
	e.status.Updated = time.Now().Unix()
	return save(e.path("status.json"), e.status)
}
func (e *Engine) checkCancel() error {
	if _, err := os.Stat(e.path("cancel")); err == nil {
		return cancelled
	}
	return nil
}
func (e *Engine) digest(path string, cancel bool) (string, fs.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return "", nil, err
	}
	if !before.Mode().IsRegular() {
		return "", nil, errors.New("不是普通文件")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return "", nil, errors.New("文件已变化")
	}
	h := sha256.New()
	buf := make([]byte, 256*1024)
	for {
		if cancel {
			if err = e.checkCancel(); err != nil {
				return "", nil, err
			}
			if err = e.update(false); err != nil {
				return "", nil, err
			}
		}
		n, re := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if re == io.EOF {
			break
		}
		if re != nil {
			return "", nil, re
		}
	}
	after, err := os.Lstat(path)
	end, ee := f.Stat()
	if err != nil || ee != nil || !os.SameFile(opened, after) || opened.Size() != end.Size() || opened.ModTime() != end.ModTime() || !after.Mode().IsRegular() {
		return "", nil, errors.New("读取期间文件已变化")
	}
	return hex.EncodeToString(h.Sum(nil)), end, nil
}
func (e *Engine) scan(r Request) error {
	e.status = Status{Phase: "scanning", Message: "正在枚举文件"}
	e.last = time.Time{}
	if err := e.update(true); err != nil {
		return err
	}
	root, err := e.checked(r.Root)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return errors.New("扫描目录不存在")
	}
	if r.Min < 1 {
		r.Min = 1
	}
	sizes := map[int64][]File{}
	count := 0
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if err := e.checkCancel(); err != nil {
			return err
		}
		if walkErr != nil {
			e.status.Skipped++
			return nil
		}
		if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "Android") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			e.status.Skipped++
			return nil
		}
		if d.IsDir() {
			return nil
		}
		i, er := d.Info()
		if er != nil {
			e.status.Skipped++
			return nil
		}
		if !i.Mode().IsRegular() || i.Size() < r.Min {
			return nil
		}
		count++
		if count > 200000 {
			return errors.New("超过 20 万文件上限，请缩小扫描目录")
		}
		e.status.Files++
		sizes[i.Size()] = append(sizes[i.Size()], File{ID: token(), Path: p, Size: i.Size(), Modified: i.ModTime().Unix()})
		return e.update(false)
	})
	if err != nil {
		return err
	}
	e.status.Message = "按大小筛选并计算 SHA-256"
	if err = e.update(true); err != nil {
		return err
	}
	hashed := map[string][]File{}
	for _, files := range sizes {
		if len(files) < 2 {
			continue
		}
		for _, f := range files {
			if _, err = e.checked(f.Path); err != nil {
				e.status.Skipped++
				continue
			}
			hash, inf, er := e.digest(f.Path, true)
			if errors.Is(er, cancelled) {
				return er
			}
			if er != nil || inf.Size() != f.Size {
				e.status.Skipped++
				continue
			}
			f.Modified = inf.ModTime().Unix()
			e.status.Hashed++
			hashed[hash] = append(hashed[hash], f)
		}
	}
	report := Report{ID: token(), Root: root, Groups: []Group{}}
	for h, files := range hashed {
		if len(files) < 2 {
			continue
		}
		rankFiles(files)
		report.Groups = append(report.Groups, Group{Hash: h, Files: files})
		e.status.Bytes += int64(len(files)-1) * files[0].Size
	}
	sort.Slice(report.Groups, func(i, j int) bool { return report.Groups[i].Files[0].Size > report.Groups[j].Files[0].Size })
	e.status.Groups = len(report.Groups)
	if err = save(e.path("report.json"), report); err != nil {
		return err
	}
	e.status.Phase = "done"
	e.status.Message = "扫描完成"
	return e.update(true)
}
func (e *Engine) trashRoot() string {
	return filepath.Join(e.Storage, "Android", "FileManager", "Trash")
}
func (e *Engine) trashBase() string { return filepath.Join(e.trashRoot(), "files") }
func (e *Engine) recordDir() string { return filepath.Join(e.trashRoot(), "records") }
func ensureDir(p string) error {
	p = filepath.Clean(p)
	if parent := filepath.Dir(p); parent != p {
		if err := ensureDir(parent); err != nil {
			return err
		}
	}
	if _, err := os.Lstat(p); os.IsNotExist(err) {
		if err = os.Mkdir(p, 0700); err != nil && !os.IsExist(err) {
			return err
		}
	} else if err != nil {
		return err
	}
	i, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return errors.New("目录不安全")
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil || real != p {
		return errors.New("目录包含链接")
	}
	return nil
}
func (e *Engine) payload(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", errors.New("无效回收项")
	}
	p := filepath.Join(e.trashBase(), id, "content")
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	if real != p {
		return "", errors.New("回收项路径包含链接")
	}
	i, err := os.Lstat(p)
	if err != nil || !i.Mode().IsRegular() {
		return "", errors.New("回收项文件不存在")
	}
	return p, nil
}
func (e *Engine) matches(f File, hash string) error { return e.matchesMode(f, hash, false) }
func (e *Engine) matchesMode(f File, hash string, background bool) error {
	if _, err := e.checked(f.Path); err != nil {
		return err
	}
	h, i, err := e.digest(f.Path, background)
	if err != nil {
		return err
	}
	if h != hash || i.Size() != f.Size {
		return errors.New("文件内容已改变，请重新扫描")
	}
	return nil
}
func (e *Engine) clean(r Request) (any, error) { return e.cleanFiles(r, false) }
func (e *Engine) cleanFiles(r Request, background bool) (result any, err error) {
	var report Report
	if err := read(e.path("report.json"), &report); err != nil {
		return nil, err
	}
	if report.ID != r.Scan {
		return nil, errors.New("扫描结果已过期")
	}
	if len(r.IDs) == 0 || (!background && len(r.IDs) > 500) {
		return nil, errors.New("每批选择 1–500 个文件")
	}
	selected := map[string]bool{}
	for _, id := range r.IDs {
		selected[id] = true
	}
	known := 0
	for _, g := range report.Groups {
		n, k := 0, 0
		for _, f := range g.Files {
			if !f.Moved {
				n++
				if selected[f.ID] {
					k++
					known++
				}
			}
		}
		if k > 0 && k >= n {
			return nil, errors.New("每组至少保留一份")
		}
	}
	if known != len(selected) {
		return nil, errors.New("包含无效或已清理文件")
	}
	if err := ensureDir(e.trashBase()); err != nil {
		return nil, err
	}
	if err := ensureDir(e.recordDir()); err != nil {
		return nil, err
	}
	moved := 0
	failures := []string{}
	failureCount := 0
	dirty := false
	sinceSave := 0
	failure := func(message string) {
		failureCount++
		if len(failures) < 100 {
			failures = append(failures, message)
		}
	}
	resultMap := func() map[string]any {
		return map[string]any{"moved": moved, "failures": failures, "failureCount": failureCount, "processed": e.status.Done}
	}
	defer func() {
		if dirty {
			if er := save(e.path("report.json"), report); err == nil && er != nil {
				err = er
			}
		}
	}()
	if background {
		e.status.Total = len(selected)
	}
	for gi := range report.Groups {
		g := &report.Groups[gi]
		var keeper *File
		for fi := range g.Files {
			f := &g.Files[fi]
			if !f.Moved && !selected[f.ID] {
				keeper = f
				break
			}
		}
		if keeper == nil {
			continue
		}
		for fi := range g.Files {
			f := &g.Files[fi]
			if f.Moved || !selected[f.ID] {
				continue
			}
			if background {
				if er := e.checkCancel(); er != nil {
					return resultMap(), er
				}
				e.status.Done++
				if er := e.update(false); er != nil {
					return resultMap(), er
				}
			}
			if er := e.matchesMode(*keeper, g.Hash, background); er != nil {
				if errors.Is(er, cancelled) {
					return resultMap(), er
				}
				failure(keeper.Path + ": 保留文件无法校验；已跳过")
				continue
			}
			if er := e.matchesMode(*f, g.Hash, background); er != nil {
				if errors.Is(er, cancelled) {
					return resultMap(), er
				}
				failure(f.Path + ": " + er.Error())
				continue
			}
			id := token()
			dir := filepath.Join(e.trashBase(), id)
			if err := os.Mkdir(dir, 0700); err != nil {
				return nil, err
			}
			current, statErr := os.Stat(f.Path)
			if statErr != nil {
				failure(f.Path + ": " + statErr.Error())
				os.Remove(dir)
				continue
			}
			mtime := current.ModTime().Unix()
			t := Trash{ID: id, Path: f.Path, Hash: g.Hash, Size: f.Size, At: time.Now().Unix(), Modified: &mtime}
			meta := filepath.Join(e.recordDir(), id+".json")
			if err := save(meta, t); err != nil {
				return nil, err
			}
			if err := os.Rename(f.Path, filepath.Join(dir, "content")); err != nil {
				os.Remove(meta)
				os.Remove(dir)
				failure(f.Path + ": " + err.Error())
				continue
			}
			f.Moved = true
			moved++
			dirty = true
			sinceSave++
			if background {
				e.status.Moved = moved
			}
			if !background || sinceSave >= 20 {
				if er := save(e.path("report.json"), report); er != nil {
					return resultMap(), er
				}
				dirty = false
				sinceSave = 0
			}
		}
	}
	return resultMap(), nil
}
func (e *Engine) trashList() ([]Trash, error) {
	out := []Trash{}
	entries, err := os.ReadDir(e.recordDir())
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		var t Trash
		if t, err = e.readTrash(strings.TrimSuffix(entry.Name(), ".json")); err == nil {
			if _, er := e.payload(t.ID); er == nil {
				out = append(out, t)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	return out, nil
}
func (e *Engine) trashAction(r Request, restore bool) (any, error) {
	return e.trashFiles(r, restore, false)
}
func (e *Engine) trashFiles(r Request, restore, background bool) (any, error) {
	if len(r.IDs) < 1 || (!background && len(r.IDs) > 500) {
		return nil, errors.New("每批选择 1–500 项")
	}
	done := 0
	failures := []string{}
	failureCount := 0
	failure := func(message string) {
		failureCount++
		if len(failures) < 100 {
			failures = append(failures, message)
		}
	}
	resultMap := func() map[string]any {
		return map[string]any{"done": done, "failures": failures, "failureCount": failureCount}
	}
	for _, id := range r.IDs {
		if background {
			if err := e.checkCancel(); err != nil {
				return resultMap(), err
			}
			e.status.Done++
			if err := e.update(false); err != nil {
				return resultMap(), err
			}
		}
		if !validID.MatchString(id) {
			return nil, errors.New("无效编号")
		}
		meta := filepath.Join(e.recordDir(), id+".json")
		var t Trash
		var err error
		if t, err = e.readTrash(id); err != nil {
			failure(id + ": 元数据不存在")
			continue
		}
		p, err := e.payload(id)
		if err != nil {
			failure(t.Path + ": " + err.Error())
			continue
		}
		if restore {
			parent := filepath.Dir(t.Path)
			if _, err = e.checked(parent); err != nil {
				failure(t.Path + ": 原目录不可用")
				continue
			}
			h, _, er := e.digest(p, background)
			if errors.Is(er, cancelled) {
				return resultMap(), er
			}
			if er != nil || h != t.Hash {
				failure(t.Path + ": 回收项校验失败")
				continue
			}
			if er = copyExclusive(p, t.Path, t.Hash); er != nil {
				failure(t.Path + ": " + er.Error())
				continue
			}
			if t.Modified != nil {
				if er = os.Chtimes(t.Path, time.Now(), time.Unix(*t.Modified, 0)); er != nil {
					os.Remove(t.Path)
					failure(t.Path + ": 修改时间恢复失败")
					continue
				}
			}
		}
		if err = os.Remove(p); err != nil {
			failure(t.Path + ": " + err.Error())
			continue
		}
		os.Remove(filepath.Dir(p))
		if err = os.Remove(meta); err != nil {
			return nil, err
		}
		done++
		if background {
			e.status.Moved = done
		}
	}
	return resultMap(), nil
}
func copyExclusive(src, dst, hash string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0660)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(dst)
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
func (e *Engine) dispatch(action string, r Request) (any, error) {
	if strings.HasPrefix(action, "watch-") || action == "helper-open" {
		return e.watchDispatch(action, r)
	}
	switch action {
	case "status":
		var s Status
		if read(e.path("status.json"), &s) != nil {
			s = Status{Phase: "idle", Message: "准备就绪"}
		}
		if s.Phase == "scanning" || s.Phase == "processing" {
			var pid int
			b, _ := os.ReadFile(e.path("lock/pid"))
			fmt.Sscanf(string(b), "%d", &pid)
			if pid > 0 && !alive(pid) {
				s.Phase = "error"
				s.Message = "上次任务被中断，请核对回收站并重新扫描"
			}
		}
		return s, nil
	case "groups":
		return e.groupsPage(r)
	case "directories":
		return e.directories(r)
	case "trash":
		unlock, err := e.lock()
		if err != nil {
			return nil, err
		}
		defer unlock()
		if err = e.migrateTrash(); err != nil {
			return nil, err
		}
		items, err := e.trashList()
		if err != nil {
			return nil, err
		}
		if r.Page < 0 || r.Page > 1000000 {
			return nil, errors.New("无效页码")
		}
		start := r.Page * 50
		if start > len(items) {
			start = len(items)
		}
		end := start + 50
		if end > len(items) {
			end = len(items)
		}
		var bytes int64
		for _, item := range items {
			bytes += item.Size
		}
		_, flat, err := e.flatItems("trash")
		if err != nil {
			return nil, err
		}
		sel := e.flatSelection("trash", "trash", flat)
		for i := range items {
			items[i].Selected = sel.IDs[items[i].ID]
		}
		return map[string]any{"scan": "trash", "items": items[start:end], "total": len(items), "bytes": bytes, "root": e.trashRoot(), "selection": flatSummary(flat, sel, r.Page)}, nil
	case "cancel":
		return nil, os.WriteFile(e.path("cancel"), []byte("1"), 0600)
	case "repairs":
		return e.repairPage(r)
	case "job-result":
		var v any
		path := "clean-result.json"
		if isItemJob(r.Job) {
			path = "item-result.json"
		}
		err := read(e.path(path), &v)
		return v, err
	}
	unlock, err := e.lock()
	if err != nil {
		return nil, err
	}
	if action == "start" {
		if r.Job != "" && r.Job != "repair" && r.Job != "clean" && !isItemJob(r.Job) {
			unlock()
			return nil, errors.New("未知扫描任务")
		}
		var er error
		if r.Job == "clean" {
			var report Report
			if er = read(e.path("report.json"), &report); er != nil {
				unlock()
				return nil, er
			}
			if report.ID != r.Scan {
				unlock()
				return nil, errors.New("扫描结果已过期")
			}
			sel := e.selection(report)
			if r.Revision != "" && r.Revision != selectionRevision(sel) {
				unlock()
				return nil, errors.New("选择已变化，请刷新后重新确认")
			}
			r.IDs = []string{}
			for id := range sel.IDs {
				r.IDs = append(r.IDs, id)
			}
			if len(r.IDs) == 0 {
				unlock()
				return nil, errors.New("尚未选择重复文件")
			}
			if er = save(e.path("clean-plan.json"), r); er != nil {
				unlock()
				return nil, er
			}
			r.IDs = nil
		} else if isItemJob(r.Job) {
			job := "trash"
			if r.Job == "repair-apply" {
				job = "repair"
			} else if er = e.migrateTrash(); er != nil {
				unlock()
				return nil, er
			}
			token, items, readErr := e.flatItems(job)
			if readErr != nil {
				unlock()
				return nil, readErr
			}
			if token == "" || r.Scan != token {
				unlock()
				return nil, errors.New("选择已过期，请刷新页面")
			}
			sel := e.flatSelection(job, token, items)
			if r.Revision != "" && r.Revision != selectionRevision(sel) {
				unlock()
				return nil, errors.New("选择已变化，请刷新后重新确认")
			}
			r.IDs = []string{}
			for id := range sel.IDs {
				r.IDs = append(r.IDs, id)
			}
			if len(r.IDs) == 0 {
				unlock()
				return nil, errors.New("尚未选择文件")
			}
			if er = save(e.path("item-plan.json"), r); er != nil {
				unlock()
				return nil, er
			}
			r.IDs = nil
		} else {
			root, checkErr := e.checked(r.Root)
			if checkErr != nil {
				unlock()
				return nil, checkErr
			}
			r.Root = root
		}
		os.Remove(e.path("cancel"))
		e.status = Status{Job: r.Job, Phase: "scanning", Message: "准备扫描"}
		if r.Job == "clean" || isItemJob(r.Job) {
			e.status.Phase = "processing"
			e.status.Message = "准备批量移入回收站"
		}
		if er = e.update(true); er != nil {
			unlock()
			return nil, er
		}
		b, _ := json.Marshal(r)
		self, er := os.Executable()
		if er != nil {
			unlock()
			return nil, er
		}
		cmd := exec.Command(self, "worker", base64.StdEncoding.EncodeToString(b))
		null, er := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if er != nil {
			unlock()
			return nil, er
		}
		defer null.Close()
		cmd.Stdin = null
		cmd.Stdout = null
		cmd.Stderr = null
		// The child waits for its PID to be committed before touching the scan state.
		if er = cmd.Start(); er != nil {
			unlock()
			return nil, er
		}
		if er = os.WriteFile(e.path("lock/pid"), []byte(fmt.Sprint(cmd.Process.Pid)), 0600); er != nil {
			cmd.Process.Kill()
			unlock()
			return nil, er
		}
		cmd.Process.Release()
		return map[string]bool{"started": true}, nil
	}
	defer unlock()
	switch action {
	case "select":
		return e.selectFiles(r)
	case "select-items":
		return e.selectItems(r)
	case "clean":
		if err = e.migrateTrash(); err != nil {
			return nil, err
		}
		return e.clean(r)
	case "restore":
		if err = e.migrateTrash(); err != nil {
			return nil, err
		}
		return e.trashAction(r, true)
	case "purge":
		if err = e.migrateTrash(); err != nil {
			return nil, err
		}
		return e.trashAction(r, false)
	case "repair-apply":
		return e.applyRepairs(r)
	}
	return nil, errors.New("未知操作")
}
func main() {
	if len(os.Args) == 3 && os.Args[1] == "info" {
		self, err := os.Executable()
		output(map[string]any{"version": version, "executable": self}, err)
		return
	}
	e := &Engine{Data: "/data/adb/dupeguard", Storage: "/storage/emulated/0"}
	if err := ensureDir(e.Data); err != nil {
		output(nil, err)
		return
	}
	real, err := filepath.EvalSymlinks(e.Storage)
	if err != nil {
		output(nil, err)
		return
	}
	e.Storage = real
	if len(os.Args) != 3 {
		output(nil, errors.New("需要操作名和 Base64 JSON 参数"))
		return
	}
	var r Request
	b, err := base64.StdEncoding.DecodeString(os.Args[2])
	if err == nil {
		err = json.Unmarshal(b, &r)
	}
	if err != nil {
		output(nil, err)
		return
	}
	if r.Root != "" {
		r.Root = strings.Replace(r.Root, "/sdcard", "/storage/emulated/0", 1)
		if within("/storage/emulated/0", r.Root) {
			rel, _ := filepath.Rel("/storage/emulated/0", r.Root)
			r.Root = filepath.Join(real, rel)
		}
	}
	if os.Args[1] == "watch-daemon" {
		output(nil, e.watchDaemon())
		return
	}
	if os.Args[1] == "worker" {
		for i := 0; i < 100; i++ {
			b, _ := os.ReadFile(e.path("lock/pid"))
			if string(b) == fmt.Sprint(os.Getpid()) {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		defer func() { os.Remove(e.path("lock/pid")); os.Remove(e.path("lock")) }()
		if isItemJob(r.Job) {
			err = e.runItemJob(r)
		} else if r.Job == "clean" {
			err = e.runClean(r)
		} else if r.Job == "repair" {
			err = e.previewRepairs(r)
		} else {
			err = e.scan(r)
		}
		if err != nil {
			e.status.Phase = "error"
			if errors.Is(err, cancelled) {
				e.status.Phase = "cancelled"
			}
			e.status.Message = err.Error()
			e.update(true)
		}
		return
	}
	result, err := e.dispatch(os.Args[1], r)
	output(result, err)
}
func output(result any, err error) {
	if err != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true, "data": result})
}
