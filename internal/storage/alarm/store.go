package alarm

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lyx6662/com-manager/pkg/logger"
)

// Level 报警级别
type Level string

const (
	LevelInfo     Level = "info"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"
)

// Record 报警记录
type Record struct {
	ID        int64     `json:"id"`
	DeviceID  string    `json:"device_id"`
	PointName string    `json:"point_name"`
	Level     Level     `json:"level"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Timestamp time.Time `json:"timestamp"`
	Acked     bool      `json:"acked"`
	AckTime   time.Time `json:"ack_time,omitempty"`
	AckUser   string    `json:"ack_user,omitempty"`
}

// Stats 报警统计
type Stats struct {
	Total    int `json:"total"`
	Unacked  int `json:"unacked"`
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
}

// Store 报警存储
type Store struct {
	db  *sql.DB
	log *logger.Logger
}

// NewStore 创建报警存储
func NewStore(db *sql.DB, log *logger.Logger) (*Store, error) {
	s := &Store{db: db, log: log}
	if err := s.initTable(); err != nil {
		return nil, err
	}
	return s, nil
}

// initTable 创建报警表
func (s *Store) initTable() error {
	sql := `
	CREATE TABLE IF NOT EXISTS alarms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id TEXT NOT NULL,
		point_name TEXT NOT NULL,
		level TEXT NOT NULL DEFAULT 'warning',
		message TEXT,
		value REAL,
		threshold REAL,
		timestamp INTEGER NOT NULL,
		acked INTEGER DEFAULT 0,
		ack_time INTEGER DEFAULT 0,
		ack_user TEXT DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_alarm_device ON alarms(device_id);
	CREATE INDEX IF NOT EXISTS idx_alarm_acked ON alarms(acked);
	CREATE INDEX IF NOT EXISTS idx_alarm_time ON alarms(timestamp);
	CREATE INDEX IF NOT EXISTS idx_alarm_level ON alarms(level);
	`

	_, err := s.db.Exec(sql)
	if err != nil {
		return fmt.Errorf("创建报警表失败: %w", err)
	}

	s.log.Info("报警表初始化成功")
	return nil
}

// Create 创建报警
func (s *Store) Create(record *Record) error {
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	result, err := s.db.Exec(`
		INSERT INTO alarms (device_id, point_name, level, message, value, threshold, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, record.DeviceID, record.PointName, record.Level, record.Message,
		record.Value, record.Threshold, record.Timestamp.UnixMilli())

	if err != nil {
		return fmt.Errorf("创建报警失败: %w", err)
	}

	id, _ := result.LastInsertId()
	record.ID = id

	s.log.Warn("新报警",
		"id", id,
		"device", record.DeviceID,
		"point", record.PointName,
		"level", record.Level,
		"message", record.Message,
	)

	return nil
}

// List 查询报警列表
func (s *Store) List(limit int, unackedOnly bool) ([]Record, error) {
	query := `SELECT id, device_id, point_name, level, message, value, threshold, timestamp, acked, ack_time, ack_user FROM alarms`
	if unackedOnly {
		query += ` WHERE acked = 0`
	}
	query += ` ORDER BY timestamp DESC`

	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询报警失败: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var r Record
		var ts, ackTime int64
		err := rows.Scan(&r.ID, &r.DeviceID, &r.PointName, &r.Level, &r.Message,
			&r.Value, &r.Threshold, &ts, &r.Acked, &ackTime, &r.AckUser)
		if err != nil {
			continue
		}
		r.Timestamp = time.UnixMilli(ts)
		if ackTime > 0 {
			r.AckTime = time.UnixMilli(ackTime)
		}
		records = append(records, r)
	}

	if records == nil {
		records = make([]Record, 0)
	}
	return records, nil
}

// Ack 确认报警
func (s *Store) Ack(id int64, user string) error {
	now := time.Now().UnixMilli()
	result, err := s.db.Exec(`UPDATE alarms SET acked = 1, ack_time = ?, ack_user = ? WHERE id = ?`,
		now, user, id)
	if err != nil {
		return fmt.Errorf("确认报警失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("报警不存在: %d", id)
	}

	s.log.Info("报警已确认", "id", id, "user", user)
	return nil
}

// GetStats 获取报警统计
func (s *Store) GetStats() (*Stats, error) {
	stats := &Stats{}

	// 总数
	s.db.QueryRow(`SELECT COUNT(*) FROM alarms`).Scan(&stats.Total)
	// 未确认
	s.db.QueryRow(`SELECT COUNT(*) FROM alarms WHERE acked = 0`).Scan(&stats.Unacked)
	// 各级别未确认数
	s.db.QueryRow(`SELECT COUNT(*) FROM alarms WHERE acked = 0 AND level = 'critical'`).Scan(&stats.Critical)
	s.db.QueryRow(`SELECT COUNT(*) FROM alarms WHERE acked = 0 AND level = 'warning'`).Scan(&stats.Warning)
	s.db.QueryRow(`SELECT COUNT(*) FROM alarms WHERE acked = 0 AND level = 'info'`).Scan(&stats.Info)

	return stats, nil
}

// CheckDuplicate 检查是否有未确认的同类报警 (避免重复报警)
func (s *Store) CheckDuplicate(deviceID, pointName string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM alarms WHERE device_id = ? AND point_name = ? AND acked = 0`,
		deviceID, pointName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Cleanup 清理已确认的旧报警
func (s *Store) Cleanup(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()
	result, err := s.db.Exec(`DELETE FROM alarms WHERE acked = 1 AND ack_time < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
