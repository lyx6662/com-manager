package buffer

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

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

// ReportProgress 补传进度
type ReportProgress struct {
	Total     int64     `json:"total"`
	Completed int64     `json:"completed"`
	Failed    int64     `json:"failed"`
	StartTime time.Time `json:"start_time"`
	Estimated time.Duration `json:"estimated"`
}

// OfflineBuffer 离线缓冲存储
type OfflineBuffer struct {
	db            *sql.DB
	log           *logger.Logger
	mu            sync.RWMutex
	retentionDays int
	dbPath        string
}

// NewOfflineBuffer 创建离线缓冲
func NewOfflineBuffer(dbPath string, retentionDays int, log *logger.Logger) (*OfflineBuffer, error) {
	buf := &OfflineBuffer{
		log:           log,
		retentionDays: retentionDays,
		dbPath:        dbPath,
	}

	if err := buf.initDB(); err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 启动定时清理
	go buf.cleanupLoop()

	return buf, nil
}

// initDB 初始化数据库
func (b *OfflineBuffer) initDB() error {
	// 确保数据库目录存在
	dir := filepath.Dir(b.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite3", b.dbPath)
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
	`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	b.log.Info("离线缓冲数据库初始化成功", "path", b.dbPath)
	return nil
}

// Store 写入离线数据
func (b *OfflineBuffer) Store(records []OfflineRecord) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	tx, err := b.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO offline_data (group_id, device_id, point_name, value, data_type, quality, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UnixMilli()
	for _, record := range records {
		if record.Timestamp == 0 {
			record.Timestamp = now
		}
		if record.CreatedAt == 0 {
			record.CreatedAt = now
		}

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
			return fmt.Errorf("插入记录失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	b.log.Debug("写入离线数据", "count", len(records))
	return nil
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

// cleanupLoop 定时清理循环
func (b *OfflineBuffer) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		b.Cleanup(b.retentionDays)
	}
}

// StoreDataPoints 存储数据点
func (b *OfflineBuffer) StoreDataPoints(groupID string, points []model.DataPoint) error {
	records := make([]OfflineRecord, 0, len(points))

	for _, point := range points {
		record := OfflineRecord{
			GroupID:   groupID,
			DeviceID:  point.DeviceID,
			PointName: point.Name,
			DataType:  string(point.DataType),
			Quality:   int(point.Quality),
			Timestamp: point.Timestamp.UnixMilli(),
		}

		// 序列化值
		// TODO: 根据数据类型序列化
		records = append(records, record)
	}

	return b.Store(records)
}

// Close 关闭数据库
func (b *OfflineBuffer) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
