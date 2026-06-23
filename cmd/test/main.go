package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"
)

// Modbus TCP 请求 - 读保持寄存器
func readHoldingRegisters(conn net.Conn, slaveID byte, startAddr uint16, quantity uint16) ([]uint16, error) {
	// MBAP头 + 功能码 + 数据
	request := make([]byte, 12)
	// Transaction ID
	request[0] = 0x00
	request[1] = 0x01
	// Protocol ID
	request[2] = 0x00
	request[3] = 0x00
	// Length
	request[4] = 0x00
	request[5] = 0x06
	// Unit ID
	request[6] = slaveID
	// Function Code
	request[7] = 0x03
	// Start Address
	binary.BigEndian.PutUint16(request[8:10], startAddr)
	// Quantity
	binary.BigEndian.PutUint16(request[10:12], quantity)

	// 发送请求
	_, err := conn.Write(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	// 设置读超时
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// 读取响应头 (7字节)
	header := make([]byte, 7)
	_, err = conn.Read(header)
	if err != nil {
		return nil, fmt.Errorf("读取响应头失败: %w", err)
	}

	// 解析长度
	frameLength := int(header[4])<<8 | int(header[5])
	if frameLength < 2 {
		return nil, fmt.Errorf("响应长度异常: %d", frameLength)
	}

	// 读取响应体
	body := make([]byte, frameLength)
	_, err = conn.Read(body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 检查功能码
	if body[0]&0x80 != 0 {
		return nil, fmt.Errorf("异常响应: 功能码=0x%02X, 异常码=0x%02X", body[0], body[1])
	}

	// 解析寄存器值
	_ = int(body[1]) // byteCount
	regs := make([]uint16, quantity)
	for i := 0; i < int(quantity) && i*2+2 < len(body); i++ {
		regs[i] = binary.BigEndian.Uint16(body[2+i*2 : 4+i*2])
	}

	return regs, nil
}

// 将两个 uint16 转换为 float32 (大端)
func regsToFloat32(reg1, reg2 uint16) float32 {
	bits := uint32(reg1)<<16 | uint32(reg2)
	return math.Float32frombits(bits)
}

func main() {
	fmt.Println("=== Modbus TCP 测试客户端 ===")
	fmt.Println("连接 127.0.0.1:502 ...")

	conn, err := net.DialTimeout("tcp", "127.0.0.1:502", 5*time.Second)
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("连接成功!\n")

	// 测试读取设备1数据 (地址 0-10)
	fmt.Println("--- 设备1 数据 (地址 0-10) ---")
	regs, err := readHoldingRegisters(conn, 1, 0, 11)
	if err != nil {
		fmt.Printf("读取失败: %v\n", err)
	} else {
		fmt.Printf("原始寄存器值: %v\n", regs)

		// 解析 float32 数据
		if len(regs) >= 2 {
			val := regsToFloat32(regs[0], regs[1])
			fmt.Printf("地址 0-1 (float32): %.2f\n", val)
		}
		if len(regs) >= 4 {
			val := regsToFloat32(regs[2], regs[3])
			fmt.Printf("地址 2-3 (float32): %.2f\n", val)
		}
		if len(regs) >= 6 {
			val := regsToFloat32(regs[4], regs[5])
			fmt.Printf("地址 4-5 (float32): %.2f\n", val)
		}
	}

	fmt.Println()

	// 测试读取设备2数据 (地址 20-30)
	fmt.Println("--- 设备2 数据 (地址 20-30) ---")
	regs2, err := readHoldingRegisters(conn, 1, 20, 11)
	if err != nil {
		fmt.Printf("读取失败: %v\n", err)
	} else {
		fmt.Printf("原始寄存器值: %v\n", regs2)

		if len(regs2) >= 2 {
			val := regsToFloat32(regs2[0], regs2[1])
			fmt.Printf("地址 20-21 (float32): %.2f\n", val)
		}
		if len(regs2) >= 4 {
			val := regsToFloat32(regs2[2], regs2[3])
			fmt.Printf("地址 22-23 (float32): %.2f\n", val)
		}
	}

	fmt.Println("\n测试完成!")
}
