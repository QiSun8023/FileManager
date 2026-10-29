package main

import (
	"errors"
	"fmt"
)

func isItemJob(job string) bool { return job == "repair-apply" || job == "restore" || job == "purge" }
func (e *Engine) runItemJob(r Request) error {
	var plan Request
	if err := read(e.path("item-plan.json"), &plan); err != nil {
		return err
	}
	if plan.Job != r.Job || plan.Scan != r.Scan {
		return errors.New("任务计划已过期")
	}
	e.status = Status{Job: r.Job, Phase: "processing", Total: len(plan.IDs), Message: "正在处理跨页所选文件"}
	if err := e.update(true); err != nil {
		return err
	}
	var result any
	var err error
	if r.Job == "repair-apply" {
		result, err = e.applyRepairFiles(plan, true)
	} else {
		if er := e.migrateTrash(); er != nil {
			return er
		}
		result, err = e.trashFiles(plan, r.Job == "restore", true)
	}
	if result == nil {
		result = map[string]any{"done": e.status.Moved, "failures": []string{}, "failureCount": 0}
	}
	if er := save(e.path("item-result.json"), result); er != nil {
		return er
	}
	if errors.Is(err, cancelled) {
		e.status.Phase = "cancelled"
		e.status.Message = fmt.Sprintf("已停止，已完成 %d 项，其余所选文件未继续处理", e.status.Moved)
		return e.update(true)
	}
	if err != nil {
		return err
	}
	e.status.Phase = "done"
	e.status.Message = fmt.Sprintf("处理结束，已完成 %d 项", e.status.Moved)
	return e.update(true)
}
