package main

import (
	"errors"
	"fmt"
)

func (e *Engine) runClean(r Request) error {
	var plan Request
	if err := read(e.path("clean-plan.json"), &plan); err != nil {
		return err
	}
	if plan.Scan != r.Scan {
		return errors.New("批量清理计划已过期")
	}
	e.status = Status{Job: "clean", Phase: "processing", Message: "检查回收站并迁移旧数据", Total: len(plan.IDs)}
	if err := e.update(true); err != nil {
		return err
	}
	if err := e.migrateTrash(); err != nil {
		return err
	}
	e.status.Message = "重新校验并移入回收站"
	e.update(true)
	result, err := e.cleanFiles(plan, true)
	if result == nil {
		result = map[string]any{"moved": e.status.Moved, "failures": []string{}, "failureCount": 0}
	}
	if er := save(e.path("clean-result.json"), result); er != nil {
		return er
	}
	if errors.Is(err, cancelled) {
		e.status.Phase = "cancelled"
		e.status.Message = fmt.Sprintf("已停止，已移入回收站 %d 项；其余文件保留", e.status.Moved)
		return e.update(true)
	}
	if err != nil {
		return err
	}
	e.status.Phase = "done"
	e.status.Message = fmt.Sprintf("批量处理结束，已移入回收站 %d 项", e.status.Moved)
	return e.update(true)
}
