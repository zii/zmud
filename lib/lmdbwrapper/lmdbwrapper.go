package lmdbwrapper

import (
	"os"
	"strings"
	"sync"

	"github.com/PowerDNS/lmdb-go/lmdb"
	"github.com/tidwall/match"
)

// DB 包装 lmdb.Env 和 lmdb.Dbi，提供与 buntdb 兼容的接口
type DB struct {
	env   *lmdb.Env
	dbi   lmdb.DBI
	path  string
	mu    sync.RWMutex
	closed bool
}

// Tx 包装 lmdb.Txn，提供事务操作
type Tx struct {
	txn   *lmdb.Txn
	dbi   lmdb.DBI
	write bool // 是否为写事务
}

// Options 配置选项
type Options struct {
	MapSize  int64 // 内存映射大小（字节），默认10MB
	MaxDBs   int   // 最大数据库数量，默认1
	NoSync   bool  // 是否禁用同步写入
	ReadOnly bool  // 只读模式打开
}

// 默认配置
var defaultOptions = Options{
	MapSize:  10 * 1024 * 1024, // 10MB
	MaxDBs:   1,
	NoSync:   false,
	ReadOnly: false,
}

// Open 使用默认配置打开数据库
func Open(path string) (*DB, error) {
	return OpenWithOptions(path, nil)
}

// OpenWithOptions 使用指定配置打开数据库
func OpenWithOptions(path string, opts *Options) (*DB, error) {
	if opts == nil {
		opts = &defaultOptions
	}

	env, err := lmdb.NewEnv()
	if err != nil {
		return nil, err
	}

	// 设置配置
	if err := env.SetMapSize(opts.MapSize); err != nil {
		env.Close()
		return nil, err
	}
	if err := env.SetMaxDBs(opts.MaxDBs); err != nil {
		env.Close()
		return nil, err
	}

	// 设置打开标志
	flags := uint(0)
	if opts.NoSync {
		flags |= lmdb.NoSync
	}
	if opts.ReadOnly {
		flags |= lmdb.Readonly
	}

	// 确保目录存在（lmdb期望目录路径）
	if err := os.MkdirAll(path, 0755); err != nil {
		env.Close()
		return nil, err
	}

	// 打开环境
	if err := env.Open(path, flags, 0644); err != nil {
		env.Close()
		return nil, err
	}

	// 创建/打开数据库
	var dbi lmdb.DBI
	err = env.Update(func(txn *lmdb.Txn) error {
		dbi, err = txn.CreateDBI("main")
		return err
	})
	if err != nil {
		env.Close()
		return nil, err
	}

	return &DB{
		env:    env,
		dbi:    dbi,
		path:   path,
		closed: false,
	}, nil
}

// Close 关闭数据库
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	db.closed = true
	db.env.Close()
	return nil
}

// View 执行只读事务
func (db *DB) View(fn func(tx *Tx) error) error {
	db.mu.RLock()
	if db.closed {
		db.mu.RUnlock()
		return ErrDBClosed
	}
	db.mu.RUnlock()

	return db.env.View(func(lmdbTxn *lmdb.Txn) error {
		tx := &Tx{
			txn:   lmdbTxn,
			dbi:   db.dbi,
			write: false,
		}
		return fn(tx)
	})
}

// Update 执行读写事务
func (db *DB) Update(fn func(tx *Tx) error) error {
	db.mu.RLock()
	if db.closed {
		db.mu.RUnlock()
		return ErrDBClosed
	}
	db.mu.RUnlock()

	return db.env.Update(func(lmdbTxn *lmdb.Txn) error {
		tx := &Tx{
			txn:   lmdbTxn,
			dbi:   db.dbi,
			write: true,
		}
		return fn(tx)
	})
}

// Get 获取键对应的值，键不存在时返回空字符串
func (tx *Tx) Get(key string) (string, error) {
	val, err := tx.txn.Get(tx.dbi, []byte(key))
	if err != nil {
		if lmdb.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return string(val), nil
}

// Set 设置键值对，opts参数被忽略（与buntdb兼容）
func (tx *Tx) Set(key, value string, opts any) error {
	if !tx.write {
		return ErrReadOnlyTx
	}
	return tx.txn.Put(tx.dbi, []byte(key), []byte(value), 0)
}

// Delete 删除键
func (tx *Tx) Delete(key string) error {
	if !tx.write {
		return ErrReadOnlyTx
	}
	return tx.txn.Del(tx.dbi, []byte(key), nil)
}

// AscendKeys 遍历键，对每个匹配 pattern 的键值对调用 fn
// pattern 支持通配符 *（匹配任意字符序列），如 "alias:*"
// 如果 fn 返回 false 则停止遍历
func (tx *Tx) AscendKeys(pattern string, fn func(key, value string) bool) error {
	// 从 pattern 提取实际前缀（取第一个 * 之前的部分），用于游标定位
	prefix := pattern
	if idx := strings.Index(prefix, "*"); idx >= 0 {
		prefix = prefix[:idx]
	}

	cursor, err := tx.txn.OpenCursor(tx.dbi)
	if err != nil {
		return err
	}
	defer cursor.Close()

	// 定位到前缀起始位置
	k, v, err := cursor.Get([]byte(prefix), nil, lmdb.SetRange)
	if err != nil && !lmdb.IsNotFound(err) {
		return err
	}

	for {
		if lmdb.IsNotFound(err) {
			break
		}

		keyStr := string(k)
		// 超出前缀范围则停止
		if !strings.HasPrefix(keyStr, prefix) {
			break
		}

		// 用完整 pattern 做匹配（不含 * 时用前缀匹配）
		matched := strings.HasPrefix(keyStr, pattern)
		if strings.Contains(pattern, "*") {
			matched = match.Match(keyStr, pattern)
		}
		if matched {
			if !fn(keyStr, string(v)) {
				break
			}
		}

		// 移动到下一个键
		k, v, err = cursor.Get(nil, nil, lmdb.Next)
		if err != nil && !lmdb.IsNotFound(err) {
			return err
		}
	}

	return nil
}