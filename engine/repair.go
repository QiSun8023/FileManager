package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
)

type RepairItem struct {
	Selected   bool   `json:"selected"`
	ID         string `json:"id"`
	Path       string `json:"path"`
	Target     string `json:"target"`
	Size       int64  `json:"size"`
	ModifiedNS int64  `json:"modifiedNS"`
	OldTime    int64  `json:"oldTime"`
	NewTime    *int64 `json:"newTime,omitempty"`
	Hash       string `json:"hash"`
	Problem    string `json:"problem,omitempty"`
	Warning    string `json:"warning,omitempty"`
	Done       bool   `json:"done"`
}
type RepairPlan struct {
	ID        string       `json:"id"`
	Root      string       `json:"root"`
	Zone      string       `json:"zone"`
	Items     []RepairItem `json:"items"`
	Unmatched int          `json:"unmatched"`
}
type RepairLog struct {
	ID    string     `json:"id"`
	Item  RepairItem `json:"item"`
	At    int64      `json:"at"`
	State string     `json:"state"`
}

var filenameDate = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}[ _][0-9]{2}-[0-9]{2}-[0-9]{2}`)

// A filename must contain exactly one complete, valid timestamp.
func dateFromName(name string, loc *time.Location) (time.Time, error) {
	matches := filenameDate.FindAllStringIndex(name, -1)
	if len(matches) != 1 {
		return time.Time{}, errors.New("文件名中没有唯一日期时间")
	}
	m := matches[0]
	digit := func(b byte) bool { return b >= '0' && b <= '9' }
	if (m[0] > 0 && digit(name[m[0]-1])) || (m[1] < len(name) && digit(name[m[1]])) {
		return time.Time{}, errors.New("日期时间边界无效")
	}
	text := strings.ReplaceAll(name[m[0]:m[1]], "_", " ")
	t, err := time.ParseInLocation("2006-01-02 15-04-05", text, loc)
	if err != nil || t.Format("2006-01-02 15-04-05") != text || t.Year() < 1970 || t.Year() > 2099 {
		return time.Time{}, errors.New("日期无效或超出 1970–2099 年范围")
	}
	return t, nil
}
func aacTarget(path string) string {
	if strings.HasSuffix(strings.ToLower(filepath.Base(path)), ".m4a.aac") {
		return path[:len(path)-8] + ".aac"
	}
	return path
}
func (e *Engine) previewRepairs(r Request) error {
	e.status = Status{Job: "repair", Phase: "scanning", Message: "正在预览文件修复"}
	e.last = time.Time{}
	if err := e.update(true); err != nil {
		return err
	}
	if !r.RepairTime && !r.RenameAAC {
		return errors.New("请选择至少一项修复")
	}
	if r.Zone == "" {
		r.Zone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(r.Zone)
	if err != nil {
		return errors.New("时区无效，例如 Asia/Shanghai 或 UTC")
	}
	root, err := e.checked(r.Root)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return errors.New("扫描目录不存在")
	}
	plan := RepairPlan{ID: token(), Root: root, Zone: r.Zone, Items: []RepairItem{}}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if er := e.checkCancel(); er != nil {
			return er
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
		inf, er := d.Info()
		if er != nil || !inf.Mode().IsRegular() {
			e.status.Skipped++
			return nil
		}
		e.status.Files++
		if e.status.Files > 200000 {
			return errors.New("超过 20 万文件上限，请缩小修复目录")
		}
		item := RepairItem{ID: token(), Path: p, Target: p, Size: inf.Size(), OldTime: inf.ModTime().Unix(), ModifiedNS: inf.ModTime().UnixNano()}
		if r.RenameAAC {
			item.Target = aacTarget(p)
		}
		if r.RepairTime {
			dt, de := dateFromName(d.Name(), loc)
			if de != nil {
				item.Warning = de.Error()
				plan.Unmatched++
			} else if dt.UnixNano() != inf.ModTime().UnixNano() {
				ts := dt.Unix()
				item.NewTime = &ts
			}
		}
		if item.Target == p && item.NewTime == nil {
			return e.update(false)
		}
		if item.Target != p {
			if _, er = os.Lstat(item.Target); er == nil {
				item.Problem = "目标同名文件已存在，将跳过"
			} else if !os.IsNotExist(er) {
				item.Problem = "无法检查目标路径，将跳过"
			}
		}
		if _, er = e.checked(p); er != nil {
			e.status.Skipped++
			return nil
		}
		h, after, er := e.digest(p, true)
		if er != nil {
			if errors.Is(er, cancelled) {
				return er
			}
			e.status.Skipped++
			return nil
		}
		if after.Size() != item.Size || after.ModTime().UnixNano() != item.ModifiedNS {
			e.status.Skipped++
			return nil
		}
		item.Hash = h
		e.status.Hashed++
		plan.Items = append(plan.Items, item)
		e.status.Groups = len(plan.Items)
		return e.update(false)
	})
	if err != nil {
		return err
	}
	if err = save(e.path("repair.json"), plan); err != nil {
		return err
	}
	e.status.Phase = "done"
	e.status.Message = fmt.Sprintf("修复预览完成：%d 项；%d 个文件没有可用日期", len(plan.Items), plan.Unmatched)
	return e.update(true)
}
func (e *Engine) repairPage(r Request) (any, error) {
	var plan RepairPlan
	if err := read(e.path("repair.json"), &plan); err != nil {
		if os.IsNotExist(err) {
			return map[string]any{"scan": "", "items": []RepairItem{}, "total": 0, "zone": "Asia/Shanghai", "unmatched": 0}, nil
		}
		return nil, err
	}
	if r.Page < 0 || r.Page > 4000 {
		return nil, errors.New("无效页码")
	}
	items := []RepairItem{}
	for _, item := range plan.Items {
		if !item.Done {
			items = append(items, item)
		}
	}
	start := r.Page * 50
	if start > len(items) {
		start = len(items)
	}
	end := start + 50
	if end > len(items) {
		end = len(items)
	}
	token, flat, err := e.flatItems("repair")
	if err != nil {
		return nil, err
	}
	sel := e.flatSelection("repair", token, flat)
	for i := range items {
		items[i].Selected = sel.IDs[items[i].ID]
	}
	return map[string]any{"scan": plan.ID, "root": plan.Root, "zone": plan.Zone, "items": items[start:end], "total": len(items), "unmatched": plan.Unmatched, "selection": flatSummary(flat, sel, r.Page)}, nil
}
func (e *Engine) applyRepairs(r Request) (any, error) { return e.applyRepairFiles(r, false) }
func (e *Engine) applyRepairFiles(r Request, background bool) (result any, err error) {
	var plan RepairPlan
	if err := read(e.path("repair.json"), &plan); err != nil {
		return nil, err
	}
	if r.Scan != plan.ID {
		return nil, errors.New("修复预览已过期，请重新预览")
	}
	if len(r.IDs) < 1 || (!background && len(r.IDs) > 500) {
		return nil, errors.New("每批选择 1–500 项")
	}
	wanted := map[string]bool{}
	for _, id := range r.IDs {
		wanted[id] = true
	}
	known := 0
	for _, item := range plan.Items {
		if wanted[item.ID] && !item.Done {
			if item.Problem != "" {
				return nil, errors.New("包含无法修复的项目，请重新预览")
			}
			known++
		}
	}
	if known != len(wanted) {
		return nil, errors.New("包含无效或已修复项目")
	}
	if err := ensureDir(e.path("repairs")); err != nil {
		return nil, err
	}
	// Any operation that changes names makes the old duplicate-file view stale.
	if err := os.Remove(e.path("report.json")); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	done, renamed, timed := 0, 0, 0
	failures := []string{}
	failureCount := 0
	dirty := false
	sinceSave := 0
	resultMap := func() map[string]any {
		return map[string]any{"done": done, "renamed": renamed, "timed": timed, "failures": failures, "failureCount": failureCount}
	}
	defer func() {
		if dirty {
			if er := save(e.path("repair.json"), plan); err == nil && er != nil {
				err = er
			}
		}
	}()
	for i := range plan.Items {
		item := &plan.Items[i]
		if !wanted[item.ID] || item.Done {
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
		fail := func(err error) {
			failureCount++
			if len(failures) < 100 {
				failures = append(failures, item.Path+": "+err.Error())
			}
		}
		if _, err := e.checked(item.Path); err != nil {
			fail(err)
			continue
		}
		before, err := os.Lstat(item.Path)
		if err != nil {
			fail(err)
			continue
		}
		atime := accessTime(before)
		hash, inf, err := e.digest(item.Path, background)
		if errors.Is(err, cancelled) {
			return resultMap(), err
		}
		if err != nil {
			fail(err)
			continue
		}
		if hash != item.Hash || inf.Size() != item.Size || inf.ModTime().UnixNano() != item.ModifiedNS {
			fail(errors.New("文件已变化，请重新预览"))
			continue
		}
		if item.Target != item.Path {
			if aacTarget(item.Path) != item.Target || filepath.Dir(item.Target) != filepath.Dir(item.Path) {
				fail(errors.New("改名目标无效"))
				continue
			}
			if _, err = os.Lstat(item.Target); err == nil {
				fail(errors.New("目标同名文件已存在，已跳过"))
				continue
			} else if !os.IsNotExist(err) {
				fail(err)
				continue
			}
		}
		record := RepairLog{ID: token(), Item: *item, At: time.Now().Unix(), State: "pending"}
		journal := filepath.Join(e.path("repairs"), record.ID+".json")
		if err = save(journal, record); err != nil {
			return nil, err
		}
		changedName := item.Target != item.Path
		if changedName {
			if err = e.safeRename(item.Path, item.Target, item.Hash, inf, atime); err != nil {
				record.State = "failed"
				save(journal, record)
				fail(err)
				continue
			}
			renamed++
		}
		if item.NewTime != nil {
			desired := time.Unix(*item.NewTime, 0)
			err = os.Chtimes(item.Target, atime, desired)
			if err == nil {
				after, se := os.Stat(item.Target)
				if se != nil {
					err = se
				} else if after.ModTime().Unix() != *item.NewTime {
					err = errors.New("文件系统未保存指定的修改时间")
				}
			}
			if err != nil {
				// Keep the original name and timestamp when a combined operation fails.
				restoreTimeErr := os.Chtimes(item.Target, atime, inf.ModTime())
				var renameErr error
				if changedName {
					current, ce := os.Stat(item.Target)
					if ce == nil {
						renameErr = e.safeRename(item.Target, item.Path, item.Hash, current, atime)
					} else {
						renameErr = ce
					}
					if renameErr == nil {
						renamed--
					}
				}
				record.State = "failed"
				if restoreTimeErr != nil || renameErr != nil {
					record.State = "partial"
					err = fmt.Errorf("%v；回退未完成，请核对原文件及目标文件：%v %v", err, restoreTimeErr, renameErr)
				}
				save(journal, record)
				fail(err)
				continue
			}
			timed++
		}
		item.Done = true
		done++
		if background {
			e.status.Moved = done
		}
		record.State = "done"
		if err = save(journal, record); err != nil {
			return nil, err
		}
		dirty = true
		sinceSave++
		if !background || sinceSave >= 20 {
			if err = save(e.path("repair.json"), plan); err != nil {
				return resultMap(), err
			}
			dirty = false
			sinceSave = 0
		}
	}
	return resultMap(), nil
}

// Native rename is atomic and refuses overwrite. Some Android FUSE mounts do
// not support renameat2 flags; the fallback copies with O_EXCL and verifies
// content and metadata before removing the source. It needs temporary space.
func (e *Engine) safeRename(src, dst, hash string, original fs.FileInfo, atime time.Time) error {
	if _, err := e.checked(src); err != nil {
		return err
	}
	if _, err := e.checked(filepath.Dir(dst)); err != nil {
		return err
	}
	err := renameNoReplace(src, dst)
	if err == nil {
		return nil
	}
	if !unsupportedRename(err) {
		return err
	}
	return e.copyMove(src, dst, hash, original, atime)
}
func (e *Engine) copyMove(src, dst, hash string, original fs.FileInfo, atime time.Time) error {
	if _, err := e.checked(src); err != nil {
		return err
	}
	if _, err := e.checked(filepath.Dir(dst)); err != nil {
		return err
	}
	var err error
	if err = copyExclusive(src, dst, hash); err != nil {
		return err
	}
	cleanup := func() { os.Remove(dst) }
	if err = os.Chtimes(dst, atime, original.ModTime()); err != nil {
		cleanup()
		return err
	}
	if _, err = e.checked(src); err != nil {
		cleanup()
		return err
	}
	current, err := os.Lstat(src)
	if err != nil || !os.SameFile(original, current) || current.Size() != original.Size() || current.ModTime() != original.ModTime() {
		cleanup()
		return errors.New("复制期间原文件已变化，已保留原文件")
	}
	if err = os.Remove(src); err != nil {
		cleanup()
		return err
	}
	return nil
}
