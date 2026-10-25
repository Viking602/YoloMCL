//go:build !windows

package launcher

func watchInventoryChanges(string) inventoryChangeWatch { return nil }
