package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

func selectionRevision(s Selection) string {
	ids := make([]string, 0, len(s.IDs))
	for id, on := range s.IDs {
		if on {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(s.Scan + "\n" + strings.Join(ids, "\n")))
	return hex.EncodeToString(sum[:])
}
