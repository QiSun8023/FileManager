package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (e *Engine) prepareTrash() error {
	for _, p := range []string{e.trashBase(), e.recordDir()} {
		if err := ensureDir(p); err != nil {
			return fmt.Errorf("回收站目录不可用：%s：%w", p, err)
		}
		probe := filepath.Join(p, ".write-test-"+token())
		f, err := os.OpenFile(probe, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("回收站目录不可写：%s：%w", p, err)
		}
		f.Close()
		if err = os.Remove(probe); err != nil {
			return err
		}
	}
	return nil
}
func safeRegular(p string) error {
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return err
	}
	i, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if real != p || !i.Mode().IsRegular() {
		return errors.New("记录或文件路径不安全")
	}
	return nil
}
func (e *Engine) readTrash(id string) (Trash, error) {
	var t Trash
	if !validID.MatchString(id) {
		return t, errors.New("无效回收项编号")
	}
	p := filepath.Join(e.recordDir(), id+".json")
	if err := safeRegular(p); err != nil {
		return t, err
	}
	if err := read(p, &t); err != nil {
		return t, err
	}
	if t.ID != id || len(t.Hash) != 64 || t.Size < 0 {
		return t, errors.New("恢复记录格式无效")
	}
	return t, nil
}
func (e *Engine) migrateTrash() error {
	if err := e.prepareTrash(); err != nil {
		return err
	}
	oldDir := e.path("trash")
	entries, err := os.ReadDir(oldDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if real, er := filepath.EvalSymlinks(oldDir); er != nil || real != oldDir {
		return errors.New("旧恢复记录目录不安全，已保留原数据")
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !validID.MatchString(id) {
			continue
		}
		oldMeta := filepath.Join(oldDir, entry.Name())
		if err = safeRegular(oldMeta); err != nil {
			return err
		}
		var t Trash
		if err = read(oldMeta, &t); err != nil {
			return fmt.Errorf("旧恢复记录无法读取，已保留：%s", oldMeta)
		}
		if t.ID != id || len(t.Hash) != 64 {
			return fmt.Errorf("旧恢复记录不匹配，已保留：%s", oldMeta)
		}
		source := filepath.Join(e.Storage, ".dupeguard-trash", id, "content")
		destDir := filepath.Join(e.trashBase(), id)
		dest := filepath.Join(destDir, "content")
		if err = ensureDir(destDir); err != nil {
			return err
		}
		_, sourceErr := os.Lstat(source)
		_, destErr := os.Lstat(dest)
		if os.IsNotExist(destErr) {
			if sourceErr != nil {
				return fmt.Errorf("旧回收文件不可用，记录已保留：%s", id)
			}
			if err = safeRegular(source); err != nil {
				return err
			}
			if err = copyExclusive(source, dest, t.Hash); err != nil {
				return fmt.Errorf("迁移失败，旧文件及记录保留：%s：%w", id, err)
			}
		} else if destErr != nil {
			return destErr
		}
		if err = safeRegular(dest); err != nil {
			return err
		}
		hash, inf, er := e.digest(dest, false)
		if er != nil || hash != t.Hash || inf.Size() != t.Size {
			return fmt.Errorf("新回收文件校验失败，旧数据保留：%s", id)
		}
		meta := filepath.Join(e.recordDir(), id+".json")
		if _, er = os.Lstat(meta); er == nil {
			current, re := e.readTrash(id)
			if re != nil || current.Path != t.Path || current.Hash != t.Hash || current.Size != t.Size {
				return fmt.Errorf("恢复记录冲突，旧数据保留：%s", id)
			}
		} else if !os.IsNotExist(er) {
			return er
		} else if er = save(meta, t); er != nil {
			return er
		}
		if sourceErr == nil {
			if err = safeRegular(source); err != nil {
				return err
			}
			hash, inf, er = e.digest(source, false)
			if er != nil || hash != t.Hash || inf.Size() != t.Size {
				return fmt.Errorf("旧回收文件已变化，保留两处数据：%s", id)
			}
			if err = os.Remove(source); err != nil {
				return err
			}
			os.Remove(filepath.Dir(source))
		} else if !os.IsNotExist(sourceErr) {
			return sourceErr
		}
		if err = os.Remove(oldMeta); err != nil {
			return err
		}
	}
	os.Remove(oldDir)
	os.Remove(filepath.Join(e.Storage, ".dupeguard-trash"))
	return nil
}
