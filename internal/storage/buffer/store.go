package buffer

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

// PendingRecord 待补传记录 (用于API展示，值已反序列化)
type PendingRecord struct {
	ID        int64       `json:"id"`
	GroupID   string      `json:"group_id"`
	DeviceID  string      `json:"device_id"`
	PointName string      `json:"point_name"`
	Value     interface{} `json:"value"`
	DataType  string      `json:"data_type"`
	Quality   int         `json:"quality"`
	Timestamp int64       `json:"timestamp"`
}

// OfflineRecord 离线数据记录
type OfflineRecord struct {
	ID          int64     `json:"id"`
	GroupID     string    `json:"group_id"`
	DeviceID    string    `json:"device_id"`
	PointName   string    `json:"point_name"`
	Value       []byte    `json:"value"`
	DataType    string    `json:"data_type"`
	Quality     int       `json:"quality"`
	Timestamp   int64     `json:"timestamp"`
	CreatedAt   int64     `json:"created_at"`
	Transmitted bool      `json:"transmitted"`
	TransAt     int64     `json:"trans_at"`
}

// OfflineBuffer 离线缓冲存储
type OfflineBuffer struct {
	db            *sql.DB
	log           *logger.Logger
	mu            sync.RWMutex
	retentionDays int
	dbPath        string

	// 内存快照：仅保留每个数据点的最新值，刷盘时写入并清空
	snapshot    map[string]OfflineRecord // key: groupID|deviceID|pointName
	snapshotMu  sync.Mutex
	flushInterval time.Duration
	stopCh       chan struct{}

	// 刷盘条件回调：返回 true 时才执行刷盘，string 为原因说明
	shouldFlush func() (bool, string)
}

// NewOfflineBuffer 创建离线缓冲
func NewOfflineBuffer(dbPath string, retentionDays int, flushInterval time.Duration, log *logger.Logger) (*OfflineBuffer, error) {
	if flushInterval <= 0 {
		flushInterval = 10 * time.Minute
	}

	buf := &OfflineBuffer{
		log:           log,
		retentionDays: retentionDays,
		dbPath:        dbPath,
		flushInterval: flushInterval,
		stopCh:        make(chan struct{}),
		snapshot:      make(map[string]OfflineRecord),
	}


	if err := buf.initDB(); err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 启动定时清理
	go buf.cleanupLoop()

	// 启动定时刷盘
	go buf.flushLoop()

	log.Info("离线缓冲启动",
		"flush_interval", flushInterval,
		"retention_days", retentionDays,
	)

	return buf, nil
}

// initDB 初始化数据库
func (b *OfflineBuffer) initDB() error {
	// 确保数据库目录存在
	dir := filepath.Dir(b.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", b.dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	b.db = db

	// 创建表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS offline_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id TEXT NOT NULL,
		device_id TEXT NOT NULL,
		point_name TEXT NOT NULL,
		value BLOB,
		data_type TEXT,
		quality INTEGER DEFAULT 0,
		timestamp INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		transmitted INTEGER DEFAULT 0,
		trans_at INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_group_time ON offline_data(group_id, timestamp);
	CREATE INDEX IF NOT EXISTS idx_transmitted ON offline_data(transmitted);
	CREATE INDEX IF NOT EXISTS idx_created_at ON offline_data(created_at);

	CREATE TABLE IF NOT EXISTS heartbeat (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		updated_at INTEGER NOT NULL
	);
	INSERT OR IGNORE INTO heartbeat (id, updated_at) VALUES (1, 0);
	`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	b.log.Info("离线缓冲数据库初始化成功", "path", b.dbPath)
	return nil
}

// StoreDataPoints 更新内存快照（仅保留每个数据点的最新值）
func (b *OfflineBuffer) StoreDataPoints(groupID string, points []model.DataPoint) {
	b.snapshotMu.Lock()
	defer b.snapshotMu.Unlock()

	now := time.Now().UnixMilli()
	for _, point := range points {
		key := groupID + "|" + point.DeviceID + "|" + point.Name
		record := OfflineRecord{
			GroupID:   groupID,
			DeviceID:  point.DeviceID,
			PointName: point.Name,
			DataType:  string(point.DataType),
			Quality:   int(point.Quality),
			Timestamp: point.Timestamp.UnixMilli(),
			CreatedAt: now,
		}
		record.Value = serializeValue(point.Value)
		b.snapshot[key] = record
	}
}

// Flush 将内存快照写入 SQLite 并清空快照
func (b *OfflineBuffer) Flush() int {
	b.snapshotMu.Lock()
	if len(b.snapshot) == 0 {
		b.snapshotMu.Unlock()
		return 0
	}

	// 取出快照数据并清空
	records := make([]OfflineRecord, 0, len(b.snapshot))
	for _, r := range b.snapshot {
		records = append(records, r)
	}
	b.snapshot = make(map[string]OfflineRecord)
	b.snapshotMu.Unlock()

	// 写入数据库
	b.mu.Lock()
	defer b.mu.Unlock()

	tx, err := b.db.Begin()
	if err != nil {
		b.log.Error("开始刷盘事务失败", "error", err)
		return 0
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO offline_data (group_id, device_id, point_name, value, data_type, quality, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		b.log.Error("准备刷盘语句失败", "error", err)
		return 0
	}
	defer stmt.Close()

	for _, record := range records {
		_, err := stmt.Exec(
			record.GroupID,
			record.DeviceID,
			record.PointName,
			record.Value,
			record.DataType,
			record.Quality,
			record.Timestamp,
			record.CreatedAt,
		)
		if err != nil {
			b.log.Error("刷盘插入记录失败", "error", err)
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		b.log.Error("刷盘提交事务失败", "error", err)
		return 0
	}

	b.log.Info("缓冲数据刷盘完成", "count", len(records))
	return len(records)
}

// flushLoop 定时刷盘循环 — 每分钟检查一次，仅在分钟整除10时刷盘
func (b *OfflineBuffer) flushLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			// 仅在分钟为0/10/20/30/40/50时刷盘
			if time.Now().Minute()%10 != 0 {
				continue
			}
			// 检查刷盘条件回调
			if b.shouldFlush != nil {
				ok, reason := b.shouldFlush()
				if !ok {
					b.log.Info("写入断点续传false", "原因", reason)
					continue
				}
				b.log.Info("写入断点续传true", "原因", reason)
			}
			b.Flush()
		}
	}
}

// cleanupLoop 定时清理循环
func (b *OfflineBuffer) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.Cleanup(b.retentionDays)
		}
	}
}

// SetShouldFlush 设置刷盘条件回调
func (b *OfflineBuffer) SetShouldFlush(fn func() (bool, string)) {
	b.shouldFlush = fn
}

// QueueSize 获取内存快照中待写入的数据量
func (b *OfflineBuffer) QueueSize() int {
	b.snapshotMu.Lock()
	defer b.snapshotMu.Unlock()
	return len(b.snapshot)
}

// LoadUntransmitted 查询未补传的数据
func (b *OfflineBuffer) LoadUntransmitted(groupID string, limit int) ([]OfflineRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	query := `
		SELECT id, group_id, device_id, point_name, value, data_type, quality, timestamp, created_at, transmitted, trans_at
		FROM offline_data
		WHERE group_id = ? AND transmitted = 0
		ORDER BY timestamp ASC
		LIMIT ?
	`

	rows, err := b.db.Query(query, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	records := make([]OfflineRecord, 0)
	for rows.Next() {
		var record OfflineRecord
		err := rows.Scan(
			&record.ID,
			&record.GroupID,
			&record.DeviceID,
			&record.PointName,
			&record.Value,
			&record.DataType,
			&record.Quality,
			&record.Timestamp,
			&record.CreatedAt,
			&record.Transmitted,
			&record.TransAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描记录失败: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// LoadPendingRecords 分页查询待补传记录 (值已反序列化)
func (b *OfflineBuffer) LoadPendingRecords(groupID string, page, pageSize int) ([]PendingRecord, int64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	// 查询总数
	var total int64
	err := b.db.QueryRow("SELECT COUNT(*) FROM offline_data WHERE group_id = ? AND transmitted = 0", groupID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("统计失败: %w", err)
	}

	// 分页查询
	query := `
		SELECT id, group_id, device_id, point_name, value, data_type, quality, timestamp
		FROM offline_data
		WHERE group_id = ? AND transmitted = 0
		ORDER BY timestamp ASC
		LIMIT ? OFFSET ?
	`

	rows, err := b.db.Query(query, groupID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	records := make([]PendingRecord, 0)
	for rows.Next() {
		var r PendingRecord
		var value []byte
		err := rows.Scan(&r.ID, &r.GroupID, &r.DeviceID, &r.PointName, &value, &r.DataType, &r.Quality, &r.Timestamp)
		if err != nil {
			return nil, 0, fmt.Errorf("扫描记录失败: %w", err)
		}
		r.Value = DeserializeValue(value, r.DataType)
		records = append(records, r)
	}

	return records, total, nil
}

// MarkTransmitted 标记已补传
func (b *OfflineBuffer) MarkTransmitted(ids []int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(ids) == 0 {
		return nil
	}

	tx, err := b.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UnixMilli()
	for _, id := range ids {
		_, err := tx.Exec("UPDATE offline_data SET transmitted = 1, trans_at = ? WHERE id = ?", now, id)
		if err != nil {
			return fmt.Errorf("更新记录失败: %w", err)
		}
	}

	return tx.Commit()
}

// CountUntransmitted 统计未补传数据量
func (b *OfflineBuffer) CountUntransmitted(groupID string) (int64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var count int64
	err := b.db.QueryRow("SELECT COUNT(*) FROM offline_data WHERE group_id = ? AND transmitted = 0", groupID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计失败: %w", err)
	}

	return count, nil
}

// EarliestUntransmitted 查询最早未补传的时间戳
func (b *OfflineBuffer) EarliestUntransmitted(groupID string) (time.Time, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var timestamp int64
	err := b.db.QueryRow("SELECT MIN(timestamp) FROM offline_data WHERE group_id = ? AND transmitted = 0", groupID).Scan(&timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("查询失败: %w", err)
	}

	if timestamp == 0 {
		return time.Time{}, nil
	}

	return time.UnixMilli(timestamp), nil
}

// Cleanup 清理过期数据
func (b *OfflineBuffer) Cleanup(retentionDays int) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()

	result, err := b.db.Exec("DELETE FROM offline_data WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("清理数据失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	b.log.Info("清理过期数据", "deleted", affected, "retention_days", retentionDays)

	return affected, nil
}

// Size 获取缓冲数据大小
func (b *OfflineBuffer) Size() (int64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var count int64
	err := b.db.QueryRow("SELECT COUNT(*) FROM offline_data").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计失败: %w", err)
	}

	return count, nil
}

// GetAllGroupIDs 获取所有分组ID
func (b *OfflineBuffer) GetAllGroupIDs() ([]string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	rows, err := b.db.Query("SELECT DISTINCT group_id FROM offline_data")
	if err != nil {
		return nil, fmt.Errorf("查询分组失败: %w", err)
	}
	defer rows.Close()

	var groupIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		groupIDs = append(groupIDs, id)
	}

	if groupIDs == nil {
		groupIDs = make([]string, 0)
	}
	return groupIDs, nil
}

// CountTransmitted 统计已补传数据量
func (b *OfflineBuffer) CountTransmitted(groupID string) (int64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var count int64
	err := b.db.QueryRow("SELECT COUNT(*) FROM offline_data WHERE group_id = ? AND transmitted = 1", groupID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计失败: %w", err)
	}

	return count, nil
}

// MarkAllTransmitted 将指定分组所有未补传数据标记为已补传 (用于清理冗余数据)
func (b *OfflineBuffer) MarkAllTransmitted(groupID string) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now().UnixMilli()
	result, err := b.db.Exec("UPDATE offline_data SET transmitted = 1, trans_at = ? WHERE group_id = ? AND transmitted = 0", now, groupID)
	if err != nil {
		return 0, fmt.Errorf("批量标记失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	b.log.Info("批量标记已补传", "group_id", groupID, "count", affected)
	return affected, nil
}

// PurgeTransmitted 清除所有已补传数据
func (b *OfflineBuffer) PurgeTransmitted() (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	result, err := b.db.Exec("DELETE FROM offline_data WHERE transmitted = 1")
	if err != nil {
		return 0, fmt.Errorf("清除已补传数据失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	b.log.Info("清除已补传数据", "count", affected)
	return affected, nil
}

// Close 关闭数据库 (先刷盘再关闭)
func (b *OfflineBuffer) Close() error {
	close(b.stopCh)

	// 关闭前刷盘
	count := b.Flush()
	if count > 0 {
		b.log.Info("关闭前刷盘完成", "count", count)
	}

	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

// GetDB 获取数据库连接 (供其他模块复用)
func (b *OfflineBuffer) GetDB() *sql.DB {
	return b.db
}

// UpdateHeartbeat 更新心跳时间戳
func (b *OfflineBuffer) UpdateHeartbeat() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now().UnixMilli()
	_, err := b.db.Exec("UPDATE heartbeat SET updated_at = ? WHERE id = 1", now)
	if err != nil {
		b.log.Error("更新心跳失败", "error", err)
	}
}

// ReadHeartbeat 读取上次心跳时间戳 (UnixMilli)
func (b *OfflineBuffer) ReadHeartbeat() (int64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var ts int64
	err := b.db.QueryRow("SELECT updated_at FROM heartbeat WHERE id = 1").Scan(&ts)
	if err != nil {
		return 0, fmt.Errorf("读取心跳失败: %w", err)
	}
	return ts, nil
}

// serializeValue 将数据点值序列化为字节
func serializeValue(val interface{}) []byte {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case float32:
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, math.Float32bits(v))
		return buf
	case float64:
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, math.Float64bits(v))
		return buf
	case int16:
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(v))
		return buf
	case uint16:
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, v)
		return buf
	case int32:
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(v))
		return buf
	case uint32:
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, v)
		return buf
	case bool:
		if v {
			return []byte{1}
		}
		return []byte{0}
	default:
		return nil
	}
}

// DeserializeValue 将字节反序列化为可读值
func DeserializeValue(data []byte, dataType string) interface{} {
	if len(data) == 0 {
		return nil
	}

	switch dataType {
	case "float32":
		if len(data) >= 4 {
			bits := binary.BigEndian.Uint32(data[:4])
			return math.Float32frombits(bits)
		}
	case "float64":
		if len(data) >= 8 {
			bits := binary.BigEndian.Uint64(data[:8])
			return math.Float64frombits(bits)
		}
	case "int16":
		if len(data) >= 2 {
			return int16(binary.BigEndian.Uint16(data[:2]))
		}
	case "uint16":
		if len(data) >= 2 {
			return binary.BigEndian.Uint16(data[:2])
		}
	case "int32":
		if len(data) >= 4 {
			return int32(binary.BigEndian.Uint32(data[:4]))
		}
	case "uint32":
		if len(data) >= 4 {
			return binary.BigEndian.Uint32(data[:4])
		}
	case "bool":
		return data[0] != 0
	}

	// 兜底：返回十六进制字符串
	return fmt.Sprintf("%x", data)
}
