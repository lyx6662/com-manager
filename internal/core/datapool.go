package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

// DataChangeSubscriber 数据变更订阅者接口
type DataChangeSubscriber interface {
	// OnDataChanged 数据变更回调
	OnDataChanged(deviceID string, pointName string, entry *DataPointEntry)
}

// DataPointEntry 数据池中的数据条目
type DataPointEntry struct {
	DeviceID    string      `json:"device_id"`
	PointName   string      `json:"point_name"`
	Value       interface{} `json:"value"`
	Quality     model.Quality `json:"quality"`
	Timestamp   time.Time   `json:"timestamp"`
	DataType    model.DataType `json:"data_type"`
	UpdateCount uint64      `json:"update_count"`
}

// subscriberInfo 订阅者信息
type subscriberInfo struct {
	subscriber DataChangeSubscriber
	pointIDs   map[string]bool // 订阅的数据点ID集合 (deviceID.pointName)
	allDevices bool            // 是否订阅所有设备
}

// DataPool 统一数据共享池
type DataPool struct {
	mu          sync.RWMutex
	log         *logger.Logger
	data        map[string]*DataPointEntry     // key: "deviceID.pointName"
	subscribers map[string]*subscriberInfo      // key: subscriber ID (使用指针地址)
}

// NewDataPool 创建数据共享池
func NewDataPool(log *logger.Logger) *DataPool {
	return &DataPool{
		log:         log,
		data:        make(map[string]*DataPointEntry),
		subscribers: make(map[string]*subscriberInfo),
	}
}

// makeKey 生成数据点唯一键
func makeKey(deviceID, pointName string) string {
	return deviceID + "." + pointName
}

// RegisterDataPoint 注册数据点到池中
func (dp *DataPool) RegisterDataPoint(deviceID, pointName string, dataType model.DataType) {
	key := makeKey(deviceID, pointName)

	dp.mu.Lock()
	defer dp.mu.Unlock()

	if _, exists := dp.data[key]; exists {
		return
	}

	dp.data[key] = &DataPointEntry{
		DeviceID:  deviceID,
		PointName: pointName,
		Quality:   model.QualityDisconnected,
		DataType:  dataType,
		Timestamp: time.Now(),
	}
}

// UnregisterDataPoint 注销数据点
func (dp *DataPool) UnregisterDataPoint(deviceID, pointName string) {
	key := makeKey(deviceID, pointName)

	dp.mu.Lock()
	defer dp.mu.Unlock()
	delete(dp.data, key)
}

// UpdateData 更新单个数据点
func (dp *DataPool) UpdateData(deviceID, pointName string, value interface{}, quality model.Quality, dataType model.DataType) {
	key := makeKey(deviceID, pointName)

	dp.mu.Lock()
	entry, exists := dp.data[key]
	if !exists {
		entry = &DataPointEntry{
			DeviceID:  deviceID,
			PointName: pointName,
			DataType:  dataType,
		}
		dp.data[key] = entry
	}

	entry.Value = value
	entry.Quality = quality
	entry.Timestamp = time.Now()
	entry.UpdateCount++

	// 收集需要通知的订阅者
	var notifyList []DataChangeSubscriber
	for _, sub := range dp.subscribers {
		if sub.allDevices || sub.pointIDs[key] {
			notifyList = append(notifyList, sub.subscriber)
		}
	}
	dp.mu.Unlock()

	// 在锁外通知订阅者，避免死锁
	for _, sub := range notifyList {
		sub.OnDataChanged(deviceID, pointName, entry)
	}
}

// BatchUpdateData 批量更新同一设备的数据点
func (dp *DataPool) BatchUpdateData(deviceID string, points []model.DataPoint) {
	type notifyItem struct {
		subscriber DataChangeSubscriber
		deviceID   string
		pointName  string
		entry      *DataPointEntry
	}

	var notifyList []notifyItem

	dp.mu.Lock()
	for _, pt := range points {
		key := makeKey(deviceID, pt.Name)

		entry, exists := dp.data[key]
		if !exists {
			entry = &DataPointEntry{
				DeviceID:  deviceID,
				PointName: pt.Name,
				DataType:  pt.DataType,
			}
			dp.data[key] = entry
		}

		entry.Value = pt.Value
		entry.Quality = pt.Quality
		entry.Timestamp = pt.Timestamp
		entry.UpdateCount++

		// 收集需要通知的订阅者
		for _, sub := range dp.subscribers {
			if sub.allDevices || sub.pointIDs[key] {
				notifyList = append(notifyList, notifyItem{
					subscriber: sub.subscriber,
					deviceID:   deviceID,
					pointName:  pt.Name,
					entry:      entry,
				})
			}
		}
	}
	dp.mu.Unlock()

	// 在锁外通知订阅者
	for _, item := range notifyList {
		item.subscriber.OnDataChanged(item.deviceID, item.pointName, item.entry)
	}
}

// GetData 获取单个数据点
func (dp *DataPool) GetData(deviceID, pointName string) (*DataPointEntry, bool) {
	key := makeKey(deviceID, pointName)

	dp.mu.RLock()
	defer dp.mu.RUnlock()

	entry, exists := dp.data[key]
	if !exists {
		return nil, false
	}

	// 返回副本
	cp := *entry
	return &cp, true
}

// GetDeviceData 获取设备的所有数据
func (dp *DataPool) GetDeviceData(deviceID string) map[string]*DataPointEntry {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	result := make(map[string]*DataPointEntry)
	prefix := deviceID + "."
	for key, entry := range dp.data {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			cp := *entry
			result[entry.PointName] = &cp
		}
	}
	return result
}

// GetAllData 获取所有数据
func (dp *DataPool) GetAllData() map[string]*DataPointEntry {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	result := make(map[string]*DataPointEntry, len(dp.data))
	for key, entry := range dp.data {
		cp := *entry
		result[key] = &cp
	}
	return result
}

// Subscribe 订阅指定数据点的变更
func (dp *DataPool) Subscribe(subscriber DataChangeSubscriber, pointIDs []string) {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	id := fmt.Sprintf("%p", subscriber)
	info := &subscriberInfo{
		subscriber: subscriber,
		pointIDs:   make(map[string]bool),
	}
	for _, pid := range pointIDs {
		info.pointIDs[pid] = true
	}
	dp.subscribers[id] = info
}

// SubscribeAll 订阅所有数据点变更
func (dp *DataPool) SubscribeAll(subscriber DataChangeSubscriber) {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	id := fmt.Sprintf("%p", subscriber)
	dp.subscribers[id] = &subscriberInfo{
		subscriber: subscriber,
		allDevices: true,
	}
}

// Unsubscribe 取消订阅
func (dp *DataPool) Unsubscribe(subscriber DataChangeSubscriber) {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	id := fmt.Sprintf("%p", subscriber)
	delete(dp.subscribers, id)
}

// GetSubscriberCount 获取订阅者数量（用于调试）
func (dp *DataPool) GetSubscriberCount() int {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	return len(dp.subscribers)
}

// GetDataPointCount 获取数据点数量
func (dp *DataPool) GetDataPointCount() int {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	return len(dp.data)
}
