package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/lyx6662/com-manager/lib-modbus"
	"github.com/lyx6662/com-manager/lib-modbus/rtu"
	modbustcp "github.com/lyx6662/com-manager/lib-modbus/tcp"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

// DeviceCollector 单设备采集器接口（引用 lib-modbus 公共接口）
type DeviceCollector = modbus.DeviceCollector

// DeviceStatus 设备运行状态
type DeviceStatus struct {
	DeviceID   string    `json:"device_id"`
	Name       string    `json:"name"`
	Online     bool      `json:"online"`
	LastPoll   time.Time `json:"last_poll"`
	ErrorCount int       `json:"error_count"`
	LastError  string    `json:"last_error,omitempty"`
	PollCount  int64     `json:"poll_count"`
}

// Collector 采集调度器 — 管理所有设备的采集协程
type Collector struct {
	log      *logger.Logger
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	devices  map[string]*deviceWorker // deviceID -> worker
	statuses map[string]*DeviceStatus
	onData   func(deviceID string, points []model.DataPoint) // 数据回调
}

// deviceWorker 单设备采集工作协程
type deviceWorker struct {
	deviceID        string
	name            string
	collector       DeviceCollector
	mappings        []config.MappingRule
	slaveID         byte
	pollInterval    time.Duration
	timeout         time.Duration
	retry           int
	enabled         bool
	lastReadSuccess bool // 上次读取是否成功，用于减少重复日志
}

// NewCollector 创建采集调度器
func NewCollector(log *logger.Logger, onData func(deviceID string, points []model.DataPoint)) *Collector {
	return &Collector{
		log:      log,
		devices:  make(map[string]*deviceWorker),
		statuses: make(map[string]*DeviceStatus),
		onData:   onData,
	}
}

// AddSerialDevice 添加串口设备
func (c *Collector) AddSerialDevice(cfg config.SerialDeviceConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var collector DeviceCollector

	// 根据 protocol 字段选择协议
	switch cfg.Protocol {
	case "modbus_rtu", "":
		// 默认使用 Modbus RTU
		masterCfg := rtu.MasterConfig{
			DeviceID: cfg.ID,
			Port:     cfg.Port,
			BaudRate: cfg.BaudRate,
			DataBits: cfg.DataBits,
			StopBits: cfg.StopBits,
			Parity:   cfg.Parity,
			SlaveID:  byte(cfg.SlaveID),
			Timeout:  cfg.GetTimeoutDuration(),
			Retry:    cfg.Retry,
		}
		collector = rtu.NewMaster(masterCfg, c.log)

	default:
		c.log.Error("不支持的串口协议", "protocol", cfg.Protocol, "device_id", cfg.ID)
		return
	}

	c.devices[cfg.ID] = &deviceWorker{
		deviceID:        cfg.ID,
		name:            cfg.Name,
		collector:       collector,
		slaveID:         byte(cfg.SlaveID),
		pollInterval:    cfg.GetPollDuration(),
		timeout:         cfg.GetTimeoutDuration(),
		retry:           cfg.Retry,
		enabled:         cfg.Enabled,
		lastReadSuccess: true, // 初始状态为成功，第一次失败时会打印日志
	}

	c.statuses[cfg.ID] = &DeviceStatus{
		DeviceID: cfg.ID,
		Name:     cfg.Name,
	}
}

// AddNetworkDevice 添加网口设备
func (c *Collector) AddNetworkDevice(cfg config.NetworkDeviceConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var collector DeviceCollector

	// 根据 protocol 字段选择协议
	switch cfg.Protocol {
	case "modbus_tcp", "":
		// 默认使用 Modbus TCP
		clientCfg := modbustcp.ClientConfig{
			DeviceID: cfg.ID,
			Host:     cfg.Host,
			Port:     cfg.Port,
			Timeout:  cfg.GetTimeoutDuration(),
			Retry:    cfg.Retry,
		}
		collector = modbustcp.NewClient(clientCfg, c.log)

	default:
		c.log.Error("不支持的网口协议", "protocol", cfg.Protocol, "device_id", cfg.ID)
		return
	}

	c.devices[cfg.ID] = &deviceWorker{
		deviceID:        cfg.ID,
		name:            cfg.Name,
		collector:       collector,
		slaveID:         byte(cfg.SlaveID),
		pollInterval:    cfg.GetPollDuration(),
		timeout:         cfg.GetTimeoutDuration(),
		retry:           cfg.Retry,
		enabled:         cfg.Enabled,
		lastReadSuccess: true, // 初始状态为成功，第一次失败时会打印日志
	}

	c.statuses[cfg.ID] = &DeviceStatus{
		DeviceID: cfg.ID,
		Name:     cfg.Name,
	}
}

// SetDeviceMappings 设置设备的点表映射
func (c *Collector) SetDeviceMappings(deviceID string, mappings []config.MappingRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if w, ok := c.devices[deviceID]; ok {
		w.mappings = mappings
	}
}

// ClearDevices 清空所有设备 (需在Stop后调用)
func (c *Collector) ClearDevices() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.devices = make(map[string]*deviceWorker)
	c.statuses = make(map[string]*DeviceStatus)
}

// Start 启动所有设备采集
func (c *Collector) Start() {
	c.ctx, c.cancel = context.WithCancel(context.Background())

	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, w := range c.devices {
		if !w.enabled {
			c.log.Info("设备已禁用，跳过", "device_id", w.deviceID)
			continue
		}

		c.wg.Add(1)
		go c.runWorker(w)
	}

	c.log.Info("采集调度器已启动", "device_count", len(c.devices))
}

// Stop 停止所有采集
func (c *Collector) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()

	// 断开所有设备连接
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, w := range c.devices {
		w.collector.Disconnect()
	}

	c.log.Info("采集调度器已停止")
}

// GetDeviceStatus 获取设备状态
func (c *Collector) GetDeviceStatus(deviceID string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if s, ok := c.statuses[deviceID]; ok {
		cp := *s
		return &cp
	}
	return nil
}

// GetAllDeviceStatus 获取所有设备状态
func (c *Collector) GetAllDeviceStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]interface{})
	for id, s := range c.statuses {
		cp := *s
		result[id] = &cp
	}
	return result
}

// IsDeviceOnline 检查设备是否在线
func (c *Collector) IsDeviceOnline(deviceID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if s, ok := c.statuses[deviceID]; ok {
		return s.Online
	}
	return false
}

// GetDevicePackets 获取设备的最近报文
func (c *Collector) GetDevicePackets(deviceID string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	w, ok := c.devices[deviceID]
	if !ok {
		return nil
	}
	return w.collector.GetRecentPackets()
}

// IsOnline 设备是否在线
func (s *DeviceStatus) IsOnline() bool {
	return s.Online
}

// GetLastPoll 获取最后轮询时间
func (s *DeviceStatus) GetLastPoll() interface{} {
	if s.LastPoll.IsZero() {
		return nil
	}
	return s.LastPoll
}

// GetErrorCount 获取错误次数
func (s *DeviceStatus) GetErrorCount() int {
	return s.ErrorCount
}

// GetLastError 获取最后错误信息
func (s *DeviceStatus) GetLastError() string {
	return s.LastError
}

// runWorker 运行单设备采集协程
func (c *Collector) runWorker(w *deviceWorker) {
	defer c.wg.Done()

	c.log.Info("设备采集协程启动",
		"device_id", w.deviceID,
		"poll_interval", w.pollInterval,
	)

	// 初始连接
	c.connectDevice(w)

	// 使用更短的 tick 间隔来检查重连时间
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.pollDevice(w)
		}
	}
}

// connectDevice 连接设备 (静默重连，不增加错误次数)
func (c *Collector) connectDevice(w *deviceWorker) {
	// 使用 goroutine 和 channel 实现超时，防止 serial.Open 阻塞
	type connectResult struct {
		err error
	}
	resultCh := make(chan connectResult, 1)

	go func() {
		err := w.collector.Connect()
		resultCh <- connectResult{err: err}
	}()

	// 等待连接结果或 context 取消
	select {
	case <-c.ctx.Done():
		// 程序退出，不等待连接结果
		return
	case result := <-resultCh:
		if result.err != nil {
			// 连接失败，更新最后轮询时间和错误信息
			c.mu.Lock()
			c.statuses[w.deviceID].Online = false
			c.statuses[w.deviceID].LastPoll = time.Now()
			c.statuses[w.deviceID].ErrorCount++
			c.statuses[w.deviceID].LastError = result.err.Error()
			c.mu.Unlock()
			return
		}
	}

	// 连接成功
	c.mu.Lock()
	c.statuses[w.deviceID].Online = true
	c.statuses[w.deviceID].LastError = ""
	c.mu.Unlock()
}

// registerRule 带原始索引的映射规则
type registerRule struct {
	index int
	rule  config.MappingRule
}

// registerRange 合并后的寄存器读取区间
type registerRange struct {
	sourceType string
	startAddr  uint16
	quantity   uint16
	rules      []registerRule
}

const (
	maxRegistersPerRead = 125 // Modbus 单次最大读取寄存器数
	mergeGapThreshold   = 10  // 相邻区间间隔 <= 此值时合并
)

// mergeRegisterRanges 将映射规则按源类型分组、排序、合并为连续读取区间
func mergeRegisterRanges(mappings []config.MappingRule) []registerRange {
	// 按 sourceType 分组（排除 coil 类型和 batch 类型）
	groups := make(map[string][]registerRule)
	for i, rule := range mappings {
		// batch 类型单独处理，不参与合并
		if rule.DataType == "int32_dcba_batch" || rule.DataType == "raw_batch" ||
			rule.DataType == "int32_dcba_batch_passthrough" || rule.DataType == "passthrough" {
			continue
		}
		// coil 类型不参与合并（读取方式不同）
		if rule.SourceType == "coil" {
			continue
		}
		groups[rule.SourceType] = append(groups[rule.SourceType], registerRule{index: i, rule: rule})
	}

	var ranges []registerRange

	for sourceType, rules := range groups {
		// 按起始寄存器地址排序
		sort.Slice(rules, func(i, j int) bool {
			return rules[i].rule.SourceRegister < rules[j].rule.SourceRegister
		})

		// 合并连续区间
		current := registerRange{
			sourceType: sourceType,
			startAddr:  rules[0].rule.SourceRegister,
			quantity:   uint16(rules[0].rule.GetRegisterCount()),
			rules:      []registerRule{rules[0]},
		}
		currentEnd := current.startAddr + current.quantity

		for _, r := range rules[1:] {
			regCount := uint16(r.rule.GetRegisterCount())
			ruleStart := r.rule.SourceRegister
			ruleEnd := ruleStart + regCount

			// 计算合并后的总数量
			newEnd := currentEnd
			if ruleEnd > newEnd {
				newEnd = ruleEnd
			}
			newQuantity := newEnd - current.startAddr

			// 判断是否可以合并：间隔 <= 阈值 且 不超过单次最大读取数
			gap := int(ruleStart) - int(currentEnd)
			if gap <= mergeGapThreshold && newQuantity <= maxRegistersPerRead {
				// 合并
				current.quantity = newQuantity
				current.rules = append(current.rules, r)
				currentEnd = newEnd
			} else {
				// 保存当前区间，开始新区间
				ranges = append(ranges, current)
				current = registerRange{
					sourceType: sourceType,
					startAddr:  ruleStart,
					quantity:   regCount,
					rules:      []registerRule{r},
				}
				currentEnd = ruleEnd
			}
		}
		ranges = append(ranges, current)
	}

	return ranges
}

// readRange 批量读取一段连续寄存器
func (c *Collector) readRange(w *deviceWorker, sourceType string, startAddr uint16, quantity uint16) ([]uint16, error) {
	slaveID := w.slaveID
	switch sourceType {
	case "holding":
		return w.collector.ReadHoldingRegisters(slaveID, startAddr, quantity)
	case "input":
		return w.collector.ReadInputRegisters(slaveID, startAddr, quantity)
	default:
		return nil, fmt.Errorf("不支持的源寄存器类型: %s", sourceType)
	}
}

// pollDevice 轮询设备数据
func (c *Collector) pollDevice(w *deviceWorker) {
	if !w.collector.IsConnected() {
		// 检查 context 是否已取消
		select {
		case <-c.ctx.Done():
			return
		default:
			c.connectDevice(w)
		}

		if !w.collector.IsConnected() {
			return
		}
	}

	points := make([]model.DataPoint, 0, len(w.mappings)*2)
	now := time.Now()

	// 1. 处理 batch 类型规则（已有批量读取逻辑）
	for _, rule := range w.mappings {
		if rule.DataType == "int32_dcba_batch" || rule.DataType == "raw_batch" ||
			rule.DataType == "int32_dcba_batch_passthrough" || rule.DataType == "passthrough" {
			batchPoints, err := c.readBatchPoints(w, rule, now)
			if err != nil {
				// 只在状态变化时打印错误日志
				if w.lastReadSuccess {
					c.log.Warn("批量读取数据点失败",
						"device_id", w.deviceID,
						"point", rule.Name,
						"error", err,
					)
					w.lastReadSuccess = false
				}
				c.mu.Lock()
				c.statuses[w.deviceID].ErrorCount++
				c.statuses[w.deviceID].LastError = err.Error()
				c.mu.Unlock()
				if !w.collector.IsConnected() {
					c.connectDevice(w)
				}
				continue
			}
			points = append(points, batchPoints...)
		}
	}

	// 2. 处理 coil 类型规则（不参与合并，逐个读取）
	for _, rule := range w.mappings {
		if rule.SourceType != "coil" {
			continue
		}
		pt, err := c.readPoint(w, rule, now)
		if err != nil {
			// 只在状态变化时打印错误日志
			if w.lastReadSuccess {
				c.log.Warn("读取线圈失败",
					"device_id", w.deviceID,
					"point", rule.Name,
					"error", err,
				)
				w.lastReadSuccess = false
			}
			points = append(points, model.DataPoint{
				DeviceID:  w.deviceID,
				Name:      rule.Name,
				Value:     nil,
				Quality:   model.QualityBad,
				Timestamp: now,
				DataType:  model.DataType(rule.DataType),
			})
			c.mu.Lock()
			c.statuses[w.deviceID].ErrorCount++
			c.statuses[w.deviceID].LastError = err.Error()
			c.mu.Unlock()
			if !w.collector.IsConnected() {
				c.connectDevice(w)
			}
			continue
		}
		// 读取成功，更新状态
		if !w.lastReadSuccess {
			c.log.Info("读取线圈成功",
				"device_id", w.deviceID,
				"point", rule.Name,
			)
			w.lastReadSuccess = true
		}
		points = append(points, pt)
	}

	// 3. 处理 holding/input 类型规则（合并连续区间批量读取）
	mergedRanges := mergeRegisterRanges(w.mappings)
	for _, rg := range mergedRanges {
		regs, err := c.readRange(w, rg.sourceType, rg.startAddr, rg.quantity)
		if err != nil {
			// 只在状态变化时打印错误日志（从成功变为失败）
			if w.lastReadSuccess {
				c.log.Warn("批量读取寄存器失败",
					"device_id", w.deviceID,
					"source_type", rg.sourceType,
					"start", rg.startAddr,
					"quantity", rg.quantity,
					"error", err,
				)
				w.lastReadSuccess = false
			}
			// 整个区间读取失败，该区间内所有点标记为错误
			for _, rr := range rg.rules {
				points = append(points, model.DataPoint{
					DeviceID:  w.deviceID,
					Name:      rr.rule.Name,
					Value:     nil,
					Quality:   model.QualityBad,
					Timestamp: now,
					DataType:  model.DataType(rr.rule.DataType),
				})
			}
			c.mu.Lock()
			c.statuses[w.deviceID].ErrorCount++
			c.statuses[w.deviceID].LastError = err.Error()
			c.mu.Unlock()
			if !w.collector.IsConnected() {
				c.connectDevice(w)
			}
			continue
		}
		// 读取成功，更新状态
		if !w.lastReadSuccess {
			c.log.Info("批量读取寄存器成功",
				"device_id", w.deviceID,
				"source_type", rg.sourceType,
				"start", rg.startAddr,
				"quantity", rg.quantity,
			)
			w.lastReadSuccess = true
		}

		// 从批量结果中提取每个规则对应的寄存器值
		for _, rr := range rg.rules {
			offset := int(rr.rule.SourceRegister - rg.startAddr)
			regCount := rr.rule.GetRegisterCount()

			if offset+regCount > len(regs) {
				c.log.Warn("寄存器偏移超出范围",
					"device_id", w.deviceID,
					"point", rr.rule.Name,
					"offset", offset,
					"regCount", regCount,
					"totalRegs", len(regs),
				)
				points = append(points, model.DataPoint{
					DeviceID:  w.deviceID,
					Name:      rr.rule.Name,
					Value:     nil,
					Quality:   model.QualityBad,
					Timestamp: now,
					DataType:  model.DataType(rr.rule.DataType),
				})
				continue
			}

			subRegs := regs[offset : offset+regCount]
			value, quality := c.convertRegisters(subRegs, rr.rule.DataType, rr.rule.ByteOrder)
			points = append(points, model.DataPoint{
				DeviceID:  w.deviceID,
				Name:      rr.rule.Name,
				Value:     value,
				Quality:   quality,
				Timestamp: now,
				DataType:  model.DataType(rr.rule.DataType),
			})
		}
	}

	// 更新状态
	c.mu.Lock()
	c.statuses[w.deviceID].LastPoll = now
	c.statuses[w.deviceID].PollCount++

	// 判断设备是否在线：至少有一个数据点读取成功
	hasGoodData := false
	goodCount := 0
	badCount := 0
	for _, pt := range points {
		if pt.Quality == model.QualityGood {
			hasGoodData = true
			goodCount++
		} else {
			badCount++
		}
	}
	c.statuses[w.deviceID].Online = hasGoodData

	c.mu.Unlock()

	// 采集结果日志（注释掉以减少日志量）
	// c.log.Info("采集完成",
	// 	"device_id", w.deviceID,
	// 	"total", len(points),
	// 	"good", goodCount,
	// 	"bad", badCount,
	// 	"online", hasGoodData,
	// )

	// 回调数据
	if len(points) > 0 && c.onData != nil {
		c.onData(w.deviceID, points)
	}
}

// readPoint 读取单个数据点
func (c *Collector) readPoint(w *deviceWorker, rule config.MappingRule, now time.Time) (model.DataPoint, error) {
	regCount := rule.GetRegisterCount()
	slaveID := w.slaveID

	var value interface{}
	var quality model.Quality

	switch rule.SourceType {
	case "holding":
		regs, err := w.collector.ReadHoldingRegisters(slaveID, rule.SourceRegister, uint16(regCount))
		if err != nil {
			return model.DataPoint{}, err
		}
		value, quality = c.convertRegisters(regs, rule.DataType, rule.ByteOrder)

	case "input":
		regs, err := w.collector.ReadInputRegisters(slaveID, rule.SourceRegister, uint16(regCount))
		if err != nil {
			return model.DataPoint{}, err
		}
		value, quality = c.convertRegisters(regs, rule.DataType, rule.ByteOrder)

	case "coil":
		coils, err := w.collector.ReadCoils(slaveID, rule.SourceRegister, 1)
		if err != nil {
			return model.DataPoint{}, err
		}
		if len(coils) > 0 {
			value = coils[0]
			quality = model.QualityGood
		} else {
			quality = model.QualityBad
		}

	default:
		return model.DataPoint{}, fmt.Errorf("不支持的源寄存器类型: %s", rule.SourceType)
	}

	return model.DataPoint{
		DeviceID:  w.deviceID,
		Name:      rule.Name,
		Value:     value,
		Quality:   quality,
		Timestamp: now,
		DataType:  model.DataType(rule.DataType),
	}, nil
}

// readBatchPoints 批量读取数据点 (用于 int32_dcba_batch 和 raw_batch 类型)
func (c *Collector) readBatchPoints(w *deviceWorker, rule config.MappingRule, now time.Time) ([]model.DataPoint, error) {
	regCount := rule.GetRegisterCount()
	slaveID := w.slaveID

	// 批量读取寄存器
	regs, err := w.collector.ReadInputRegisters(slaveID, rule.SourceRegister, uint16(regCount))
	if err != nil {
		return nil, err
	}

	if len(regs) < regCount {
		return nil, fmt.Errorf("读取寄存器数量不足: 期望%d, 实际%d", regCount, len(regs))
	}

	points := make([]model.DataPoint, 0, regCount)

	if rule.DataType == "raw_batch" || rule.DataType == "passthrough" {
		// 直接透传原始寄存器值，每个寄存器一个uint16
		outputCount := regCount
		if rule.MaxPoints > 0 && outputCount > rule.MaxPoints {
			outputCount = rule.MaxPoints
		}
		for i := 0; i < outputCount; i++ {
			targetReg := rule.TargetRegister + uint16(i)
			points = append(points, model.DataPoint{
				DeviceID:  w.deviceID,
				Name:      fmt.Sprintf("%s-%d", rule.Name, i+1),
				Value:     regs[i],
				Quality:   model.QualityGood,
				Timestamp: now,
				DataType:  model.DataType("uint16"),
				Extra: map[string]interface{}{
					"target_register": targetReg,
					"raw_value":       true,
				},
			})
		}
	} else if rule.DataType == "int32_dcba_batch_passthrough" {
		// 直接透传原始寄存器对，不做任何字节序转换
		dataPointCount := regCount / 2
		if rule.MaxPoints > 0 && dataPointCount > rule.MaxPoints {
			dataPointCount = rule.MaxPoints
		}

		for i := 0; i < dataPointCount; i++ {
			offset := i * 2
			if offset+1 >= len(regs) {
				break
			}
			targetReg := rule.TargetRegister + uint16(i*2)
			points = append(points, model.DataPoint{
				DeviceID:  w.deviceID,
				Name:      fmt.Sprintf("%s-%d", rule.Name, i+1),
				Value:     []uint16{regs[offset], regs[offset+1]},
				Quality:   model.QualityGood,
				Timestamp: now,
				DataType:  model.DataType("uint16_pair"),
				Extra: map[string]interface{}{
					"target_register": targetReg,
					"raw_passthrough": true,
				},
			})
		}
	} else {
		// int32_dcba_batch: 每2个寄存器组成一个int32值
		dataPointCount := regCount / 2

		// 限制数据点数量
		if rule.MaxPoints > 0 && dataPointCount > rule.MaxPoints {
			dataPointCount = rule.MaxPoints
		}

		for i := 0; i < dataPointCount; i++ {
			offset := i * 2
			if offset+1 >= len(regs) {
				break
			}

			// DCBA 格式: 交换寄存器顺序和字节顺序
			reg0 := ((regs[offset+1] & 0xFF) << 8) | ((regs[offset+1] >> 8) & 0xFF)
			reg1 := ((regs[offset] & 0xFF) << 8) | ((regs[offset] >> 8) & 0xFF)
			val := int32(uint32(reg0)<<16 | uint32(reg1))

			// 计算目标寄存器地址
			targetReg := rule.TargetRegister + uint16(i*2)

			points = append(points, model.DataPoint{
				DeviceID:  w.deviceID,
				Name:      fmt.Sprintf("%s-%d", rule.Name, i+1),
				Value:     val,
				Quality:   model.QualityGood,
				Timestamp: now,
				DataType:  model.DataType("int32"),
				Extra: map[string]interface{}{
					"target_register": targetReg,
				},
			})
		}
	}

	return points, nil
}

// convertRegisters 将寄存器值转换为指定数据类型 (byteOrder 为空时使用默认大端)
func (c *Collector) convertRegisters(regs []uint16, dataType string, byteOrder ...string) (interface{}, model.Quality) {
	if len(regs) == 0 {
		return nil, model.QualityBad
	}

	// 提取字节序参数 (可选)
	bo := ""
	if len(byteOrder) > 0 {
		bo = byteOrder[0]
	}

	switch dataType {
	case "float32":
		if len(regs) < 2 {
			return nil, model.QualityBad
		}
		bits := decodeUint32WithByteOrder(regs, bo)
		return math.Float32frombits(bits), model.QualityGood

	case "float32_dcba":
		// DCBA 格式: 字节顺序反转 (兼容旧配置)
		if len(regs) < 2 {
			return nil, model.QualityBad
		}
		bits := decodeUint32WithByteOrder(regs, "DCBA")
		return math.Float32frombits(bits), model.QualityGood

	case "int32":
		if len(regs) < 2 {
			return nil, model.QualityBad
		}
		bits := decodeUint32WithByteOrder(regs, bo)
		return int32(bits), model.QualityGood

	case "int32_dcba":
		// DCBA 格式: 字节顺序反转 (兼容旧配置)
		if len(regs) < 2 {
			return nil, model.QualityBad
		}
		bits := decodeUint32WithByteOrder(regs, "DCBA")
		return int32(bits), model.QualityGood

	case "int32_dcba_passthrough":
		// 直接透传原始寄存器对，不做字节序转换
		if len(regs) < 2 {
			return nil, model.QualityBad
		}
		return []uint16{regs[0], regs[1]}, model.QualityGood

	case "uint32":
		if len(regs) < 2 {
			return nil, model.QualityBad
		}
		return decodeUint32WithByteOrder(regs, bo), model.QualityGood

	case "uint32_dcba":
		// DCBA 格式: 字节顺序反转 (兼容旧配置)
		if len(regs) < 2 {
			return nil, model.QualityBad
		}
		return decodeUint32WithByteOrder(regs, "DCBA"), model.QualityGood

	case "int16":
		return int16(regs[0]), model.QualityGood

	case "uint16":
		return regs[0], model.QualityGood

	case "passthrough":
		// 直接透传所有原始寄存器值，不做任何转换
		result := make([]uint16, len(regs))
		copy(result, regs)
		return result, model.QualityGood

	default:
		return regs[0], model.QualityGood
	}
}

// decodeUint32WithByteOrder 按指定字节序从2个寄存器值解码uint32
// ABCD: 大端 (默认), BADC: 字交换, CDAB: 字节交换, DCBA: 小端
func decodeUint32WithByteOrder(regs []uint16, byteOrder string) uint32 {
	switch byteOrder {
	case "BADC":
		// 字交换: 低16位在前，高16位在后
		return uint32(regs[1])<<16 | uint32(regs[0])
	case "CDAB":
		// 字节交换: 保持寄存器顺序，每个寄存器内字节反转
		reg0 := ((regs[0] & 0xFF) << 8) | ((regs[0] >> 8) & 0xFF)
		reg1 := ((regs[1] & 0xFF) << 8) | ((regs[1] >> 8) & 0xFF)
		return uint32(reg0)<<16 | uint32(reg1)
	case "DCBA":
		// 小端: 寄存器交换 + 每个寄存器内字节反转
		reg0 := ((regs[1] & 0xFF) << 8) | ((regs[1] >> 8) & 0xFF)
		reg1 := ((regs[0] & 0xFF) << 8) | ((regs[0] >> 8) & 0xFF)
		return uint32(reg0)<<16 | uint32(reg1)
	default:
		// ABCD 大端 (默认)
		return uint32(regs[0])<<16 | uint32(regs[1])
	}
}
