package main

import (
	"errors"
	"os"
)

type FlatItem struct {
	ID       string
	Size     int64
	Eligible bool
}

func (e *Engine) flatItems(job string) (string, []FlatItem, error) {
	items := []FlatItem{}
	switch job {
	case "repair":
		var plan RepairPlan
		if err := read(e.path("repair.json"), &plan); err != nil {
			if os.IsNotExist(err) {
				return "", items, nil
			}
			return "", nil, err
		}
		for _, x := range plan.Items {
			if !x.Done {
				items = append(items, FlatItem{x.ID, x.Size, x.Problem == ""})
			}
		}
		return plan.ID, items, nil
	case "trash":
		list, err := e.trashList()
		if err != nil {
			return "", nil, err
		}
		for _, x := range list {
			items = append(items, FlatItem{x.ID, x.Size, true})
		}
		return "trash", items, nil
	}
	return "", nil, errors.New("未知选择范围")
}
func (e *Engine) flatSelection(job, token string, items []FlatItem) Selection {
	var s Selection
	read(e.path(job+"-selection.json"), &s)
	if s.Scan != token || s.IDs == nil {
		s = Selection{Scan: token, IDs: map[string]bool{}}
	}
	valid := map[string]bool{}
	for _, x := range items {
		if x.Eligible && s.IDs[x.ID] {
			valid[x.ID] = true
		}
	}
	s.IDs = valid
	return s
}
func flatPage(items []FlatItem, page int) ([]FlatItem, error) {
	if page < 0 || page > 1000000 {
		return nil, errors.New("无效页码")
	}
	start := page * 50
	if start > len(items) {
		start = len(items)
	}
	end := start + 50
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], nil
}
func flatState(items []FlatItem, s Selection) (string, int) {
	cap, n := 0, 0
	for _, x := range items {
		if x.Eligible {
			cap++
			if s.IDs[x.ID] {
				n++
			}
		}
	}
	if n == 0 {
		return "off", cap
	}
	if n == cap {
		return "on", cap
	}
	return "partial", cap
}
func flatSummary(items []FlatItem, s Selection, page int) map[string]any {
	pageItems, _ := flatPage(items, page)
	allState, allCap := flatState(items, s)
	pageState, pageCap := flatState(pageItems, s)
	n := 0
	var bytes int64
	for _, x := range items {
		if s.IDs[x.ID] {
			n++
			bytes += x.Size
		}
	}
	return map[string]any{"count": n, "bytes": bytes, "allState": allState, "pageState": pageState, "allCapacity": allCap, "pageCapacity": pageCap, "revision": selectionRevision(s)}
}
func (e *Engine) selectItems(r Request) (any, error) {
	if r.Job == "trash" {
		if err := e.migrateTrash(); err != nil {
			return nil, err
		}
	}
	token, items, err := e.flatItems(r.Job)
	if err != nil {
		return nil, err
	}
	if token == "" || r.Scan != token {
		return nil, errors.New("选择结果已过期，请刷新页面")
	}
	s := e.flatSelection(r.Job, token, items)
	scope := items
	switch r.Mode {
	case "toggle-all", "toggle-page":
		if r.Mode == "toggle-page" {
			scope, err = flatPage(items, r.Page)
			if err != nil {
				return nil, err
			}
		}
		state, _ := flatState(scope, s)
		for _, x := range scope {
			if !x.Eligible {
				continue
			}
			if state == "on" {
				delete(s.IDs, x.ID)
			} else {
				s.IDs[x.ID] = true
			}
		}
	case "toggle":
		found := false
		for _, x := range items {
			if x.ID == r.ID && x.Eligible {
				found = true
				if r.Checked {
					s.IDs[x.ID] = true
				} else {
					delete(s.IDs, x.ID)
				}
				break
			}
		}
		if !found {
			return nil, errors.New("此文件不可选择，请刷新页面")
		}
	default:
		return nil, errors.New("未知选择操作")
	}
	if err = save(e.path(r.Job+"-selection.json"), s); err != nil {
		return nil, err
	}
	return flatSummary(items, s, r.Page), nil
}
