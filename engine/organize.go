package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var copySuffix = regexp.MustCompile(`[ ]*[（(][0-9]+[）)]$`)

func fileNameScore(f File, names map[string]bool) int {
	base := filepath.Base(f.Path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	original := stem
	depth := 0
	for copySuffix.MatchString(stem) {
		stem = copySuffix.ReplaceAllString(stem, "")
		depth++
	}
	if stem != original && names[strings.ToLower(stem+ext)] {
		return depth
	}
	return 0
}
func rankFiles(files []File) string {
	names := map[string]bool{}
	for _, f := range files {
		names[strings.ToLower(filepath.Base(f.Path))] = true
	}
	sort.SliceStable(files, func(i, j int) bool {
		a, b := fileNameScore(files[i], names), fileNameScore(files[j], names)
		if a != b {
			return a < b
		}
		a, b = utf8.RuneCountInString(filepath.Base(files[i].Path)), utf8.RuneCountInString(filepath.Base(files[j].Path))
		if a != b {
			return a < b
		}
		return files[i].Path < files[j].Path
	})
	for _, f := range files {
		if fileNameScore(f, names) > 0 {
			return "存在对应编号副本，优先保留无副本编号的文件"
		}
	}
	return "无法确认原始来源，按名称简洁程度及路径稳定排序；可手动更换保留项"
}
func (e *Engine) activeGroups(report Report) []Group {
	groups := []Group{}
	for _, g := range report.Groups {
		files := []File{}
		for _, f := range g.Files {
			if !f.Moved {
				files = append(files, f)
			}
		}
		if len(files) > 1 {
			reason := rankFiles(files)
			groups = append(groups, Group{Hash: g.Hash, Files: files, Count: len(files), Keeper: files[0].ID, Reason: reason})
		}
	}
	return groups
}

type Selection struct {
	Scan string          `json:"scan"`
	IDs  map[string]bool `json:"ids"`
}

func (e *Engine) selection(report Report) Selection {
	var s Selection
	read(e.path("selection.json"), &s)
	if s.Scan != report.ID || s.IDs == nil {
		return Selection{Scan: report.ID, IDs: map[string]bool{}}
	}
	valid := map[string]bool{}
	for _, g := range e.activeGroups(report) {
		for _, f := range g.Files {
			if s.IDs[f.ID] {
				valid[f.ID] = true
			}
		}
	}
	s.IDs = valid
	return s
}
func (e *Engine) selectionSummary(report Report, s Selection) map[string]any {
	n := 0
	var bytes int64
	for _, g := range report.Groups {
		for _, f := range g.Files {
			if !f.Moved && s.IDs[f.ID] {
				n++
				bytes += f.Size
			}
		}
	}
	return map[string]any{"count": n, "bytes": bytes, "scan": report.ID, "revision": selectionRevision(s)}
}
func groupPage(groups []Group, page int) ([]Group, error) {
	if page < 0 || page > 8000 {
		return nil, errors.New("无效页码")
	}
	start := page * 25
	if start > len(groups) {
		start = len(groups)
	}
	end := start + 25
	if end > len(groups) {
		end = len(groups)
	}
	return groups[start:end], nil
}
func (e *Engine) selectFiles(r Request) (any, error) {
	var report Report
	if err := read(e.path("report.json"), &report); err != nil {
		return nil, err
	}
	if report.ID != r.Scan {
		return nil, errors.New("扫描结果已过期")
	}
	s := e.selection(report)
	groups := e.activeGroups(report)
	switch r.Mode {
	case "toggle-page", "toggle-all":
		if r.Mode == "toggle-page" {
			var err error
			groups, err = groupPage(groups, r.Page)
			if err != nil {
				return nil, err
			}
		}
		clear := duplicateState(groups, s) == "on"
		for _, g := range groups {
			if clear {
				for _, f := range g.Files {
					delete(s.IDs, f.ID)
				}
				continue
			}
			keeper := ""
			for _, f := range g.Files {
				if !s.IDs[f.ID] {
					keeper = f.ID
					break
				}
			}
			if keeper == "" {
				return nil, errors.New("保留项状态异常，请重新扫描")
			}
			for _, f := range g.Files {
				if f.ID != keeper {
					s.IDs[f.ID] = true
				}
			}
		}
	case "clear-all":
		s.IDs = map[string]bool{}
	case "suggest-all", "suggest-page", "clear-page":
		if r.Mode == "suggest-all" {
			s.IDs = map[string]bool{}
		} else {
			var err error
			groups, err = groupPage(groups, r.Page)
			if err != nil {
				return nil, err
			}
		}
		for _, g := range groups {
			for i, f := range g.Files {
				delete(s.IDs, f.ID)
				if r.Mode != "clear-page" && i > 0 {
					s.IDs[f.ID] = true
				}
			}
		}
	case "toggle":
		found := false
		for _, g := range groups {
			for _, f := range g.Files {
				if f.ID == r.ID {
					found = true
					if r.Checked {
						s.IDs[f.ID] = true
					} else {
						delete(s.IDs, f.ID)
					}
					n := 0
					for _, x := range g.Files {
						if s.IDs[x.ID] {
							n++
						}
					}
					if n >= len(g.Files) {
						return nil, errors.New("每组至少保留一份，请先取消另一份文件的勾选")
					}
					break
				}
			}
		}
		if !found {
			return nil, errors.New("文件已不存在或不属于当前结果")
		}
	default:
		return nil, errors.New("未知选择操作")
	}
	if err := save(e.path("selection.json"), s); err != nil {
		return nil, err
	}
	return e.selectionSummary(report, s), nil
}
func (e *Engine) directories(r Request) (any, error) {
	p := r.Root
	if p == "" {
		p = e.Storage
	}
	var err error
	p, err = e.checked(p)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	dirs := []map[string]string{}
	for _, d := range entries {
		if !d.IsDir() || d.Type()&os.ModeSymlink != 0 || strings.HasPrefix(d.Name(), ".") || d.Name() == "Android" {
			continue
		}
		child := filepath.Join(p, d.Name())
		if _, err = e.checked(child); err == nil {
			dirs = append(dirs, map[string]string{"name": d.Name(), "path": child})
		}
	}
	if r.Page < 0 || r.Page > 1000000 {
		return nil, errors.New("无效页码")
	}
	start := r.Page * 50
	if start > len(dirs) {
		start = len(dirs)
	}
	end := start + 50
	if end > len(dirs) {
		end = len(dirs)
	}
	parent := filepath.Dir(p)
	if p == e.Storage {
		parent = ""
	}
	return map[string]any{"path": p, "storage": e.Storage, "parent": parent, "items": dirs[start:end], "total": len(dirs)}, nil
}
func (e *Engine) groupsPage(r Request) (any, error) {
	var report Report
	if err := read(e.path("report.json"), &report); err != nil {
		if os.IsNotExist(err) {
			return map[string]any{"scan": "", "groups": []Group{}, "total": 0, "selection": map[string]any{"count": 0, "bytes": 0}}, nil
		}
		return nil, err
	}
	groups := e.activeGroups(report)
	s := e.selection(report)
	page, err := groupPage(groups, r.Page)
	if err != nil {
		return nil, err
	}
	allState, pageState := duplicateState(groups, s), duplicateState(page, s)
	allCap, pageCap := duplicateCapacity(groups), duplicateCapacity(page)
	for i := range page {
		for j := range page[i].Files {
			page[i].Files[j].Selected = s.IDs[page[i].Files[j].ID]
		}
		if len(page[i].Files) > 100 {
			page[i].Files = page[i].Files[:100]
		}
	}
	var reclaimable int64
	for _, g := range groups {
		reclaimable += int64(g.Count-1) * g.Files[0].Size
	}
	summary := e.selectionSummary(report, s)
	summary["allState"] = allState
	summary["pageState"] = pageState
	summary["allCapacity"] = allCap
	summary["pageCapacity"] = pageCap
	return map[string]any{"scan": report.ID, "groups": page, "total": len(groups), "root": report.Root, "selection": summary, "reclaimable": reclaimable}, nil
}
func duplicateCapacity(groups []Group) int {
	n := 0
	for _, g := range groups {
		n += g.Count - 1
	}
	return n
}
func duplicateState(groups []Group, s Selection) string {
	selected := 0
	for _, g := range groups {
		for _, f := range g.Files {
			if s.IDs[f.ID] {
				selected++
			}
		}
	}
	if selected == 0 {
		return "off"
	}
	if selected == duplicateCapacity(groups) {
		return "on"
	}
	return "partial"
}
