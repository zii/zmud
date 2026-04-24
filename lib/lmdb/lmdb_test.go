package lmdb

import (
	"path/filepath"
	"testing"
)

func TestOpenClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 确保可以关闭
	if err := db.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestGetSet(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 测试 Set 和 Get
	err = db.Update(func(tx *Tx) error {
		return tx.Set("key1", "value1", nil)
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	var val string
	err = db.View(func(tx *Tx) error {
		var err error
		val, err = tx.Get("key1")
		return err
	})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("Get returned %q, want %q", val, "value1")
	}

	// 测试不存在的键
	err = db.View(func(tx *Tx) error {
		var err error
		val, err = tx.Get("nonexistent")
		return err
	})
	if err != nil {
		t.Fatalf("Get nonexistent failed: %v", err)
	}
	if val != "" {
		t.Errorf("Get nonexistent returned %q, want empty string", val)
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 先设置一个键
	err = db.Update(func(tx *Tx) error {
		return tx.Set("key1", "value1", nil)
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// 删除键
	err = db.Update(func(tx *Tx) error {
		tx.Delete("key1")
		return nil
	})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// 验证键已删除
	var val string
	err = db.View(func(tx *Tx) error {
		var err error
		val, err = tx.Get("key1")
		return err
	})
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if val != "" {
		t.Errorf("Get after delete returned %q, want empty string", val)
	}
}

func TestAscendKeys(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 设置一些测试数据
	err = db.Update(func(tx *Tx) error {
		tx.Set("prefix:key1", "value1", nil)
		tx.Set("prefix:key2", "value2", nil)
		tx.Set("other:key3", "value3", nil)
		tx.Set("prefix:key3", "value4", nil)
		return nil
	})
	if err != nil {
		t.Fatalf("Setup data failed: %v", err)
	}

	// 遍历前缀
	var count int
	var keys []string
	err = db.View(func(tx *Tx) error {
		return tx.AscendKeys("prefix:", func(key, value string) bool {
			count++
			keys = append(keys, key)
			return true
		})
	})
	if err != nil {
		t.Fatalf("AscendKeys failed: %v", err)
	}

	if count != 3 {
		t.Errorf("AscendKeys found %d keys, want 3", count)
	}

	// 验证键的顺序（应该按字典序）
	expected := []string{"prefix:key1", "prefix:key2", "prefix:key3"}
	for i, key := range keys {
		if i >= len(expected) {
			t.Errorf("Unexpected key: %s", key)
			continue
		}
		if key != expected[i] {
			t.Errorf("Key[%d] = %q, want %q", i, key, expected[i])
		}
	}
}

func TestReadOnlyTransaction(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 在只读事务中尝试写操作应该失败
	err = db.View(func(tx *Tx) error {
		return tx.Set("key1", "value1", nil)
	})
	if err != ErrReadOnlyTx {
		t.Errorf("Expected ErrReadOnlyTx, got %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 初始化数据
	err = db.Update(func(tx *Tx) error {
		return tx.Set("counter", "0", nil)
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// 简单并发测试（多个 goroutine 读取）
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			err := db.View(func(tx *Tx) error {
				val, err := tx.Get("counter")
				if err != nil {
					return err
				}
				if val != "0" {
					t.Errorf("Concurrent read got %s, want 0", val)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
