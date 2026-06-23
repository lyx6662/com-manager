package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func readHoldingRegisters(addr string, slaveID byte, startAddr uint16, quantity uint16) ([]uint16, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}
	defer conn.Close()

	request := make([]byte, 12)
	request[0] = 0x00
	request[1] = 0x01
	request[2] = 0x00
	request[3] = 0x00
	request[4] = 0x00
	request[5] = 0x06
	request[6] = slaveID
	request[7] = 0x03
	binary.BigEndian.PutUint16(request[8:10], startAddr)
	binary.BigEndian.PutUint16(request[10:12], quantity)

	_, err = conn.Write(request)
	if err != nil {
		return nil, fmt.Errorf("发送失败: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	header := make([]byte, 7)
	_, err = conn.Read(header)
	if err != nil {
		return nil, fmt.Errorf("读取响应头失败: %w", err)
	}

	frameLength := int(header[4])<<8 | int(header[5])
	body := make([]byte, frameLength)
	_, err = conn.Read(body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	if body[0]&0x80 != 0 {
		return nil, fmt.Errorf("异常响应: 0x%02X", body[1])
	}

	regs := make([]uint16, quantity)
	for i := 0; i < int(quantity) && i*2+2 < len(body); i++ {
		regs[i] = binary.BigEndian.Uint16(body[2+i*2 : 4+i*2])
	}
	return regs, nil
}

func main() {
	fmt.Println("=== 整合输出数据读取 ===")
	fmt.Println("地址: 127.0.0.1:502, 从站ID=1, 功能码03\n")

	// 读取地址 0-16
	regs, err := readHoldingRegisters("127.0.0.1:502", 1, 0, 17)
	if err != nil {
		fmt.Printf("读取失败: %v\n", err)
		return
	}

	fmt.Println("寄存器值:")
	for i, v := range regs {
		fmt.Printf("  地址 %2d: %d\n", i, v)
	}

	fmt.Println("\n--- 设备1数据 (地址 0-10, uint16) ---")
	for i := 0; i <= 10 && i < len(regs); i++ {
		fmt.Printf("  地址 %2d: %d\n", i, regs[i])
	}

	fmt.Println("\n--- 设备2数据 (地址 11-16, int32 DCBA格式) ---")
	for i := 11; i+1 < len(regs); i += 2 {
		// 将两个 uint16 转换为 int32
		val := int32(uint32(regs[i])<<16 | uint32(regs[i+1]))
		fmt.Printf("  地址 %d-%d: %d\n", i, i+1, val)
	}
}
