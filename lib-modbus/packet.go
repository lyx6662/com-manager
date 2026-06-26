package modbus

import "time"

// PacketEntry 通信报文条目
type PacketEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Direction   string    `json:"direction"`   // "Tx" 或 "Rx"
	Data        string    `json:"data"`         // hex 字符串
	Length      int       `json:"length"`
	Description string    `json:"description"`  // 如 "FC=0x03 SlaveID=1"
}
