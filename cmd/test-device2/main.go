package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("=== 设备2 测试 ===")
	fmt.Println("设备: 192.168.8.162:50")
	fmt.Println("从站ID: 1")
	fmt.Println("功能码: 04 (读输入寄存器)")
	fmt.Println("读取: 地址0开始，共12个寄存器\n")

	conn, err := net.DialTimeout("tcp", "192.168.8.162:50", 5*time.Second)
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()

	// 构建请求: 功能码04, 起始地址0, 数量12
	request := make([]byte, 12)
	request[0] = 0x00
	request[1] = 0x01
	request[2] = 0x00
	request[3] = 0x00
	request[4] = 0x00
	request[5] = 0x06
	request[6] = 0x01  // 从站ID
	request[7] = 0x04  // 功能码 04
	binary.BigEndian.PutUint16(request[8:10], 0)   // 起始地址 0
	binary.BigEndian.PutUint16(request[10:12], 12) // 数量 12

	_, err = conn.Write(request)
	if err != nil {
		fmt.Printf("发送失败: %v\n", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// 读取响应头
	header := make([]byte, 7)
	_, err = conn.Read(header)
	if err != nil {
		fmt.Printf("读取响应头失败: %v\n", err)
		return
	}

	frameLength := int(header[4])<<8 | int(header[5])
	body := make([]byte, frameLength)
	_, err = conn.Read(body)
	if err != nil {
		fmt.Printf("读取响应体失败: %v\n", err)
		return
	}

	if body[0]&0x80 != 0 {
		fmt.Printf("异常响应: 功能码=0x%02X, 异常码=0x%02X\n", body[0], body[1])
		return
	}

	// 解析寄存器值
	byteCount := int(body[1])
	fmt.Printf("响应字节数: %d\n", byteCount)

	regs := make([]uint16, 12)
	for i := 0; i < 12 && i*2+2 < len(body); i++ {
		regs[i] = binary.BigEndian.Uint16(body[2+i*2 : 4+i*2])
	}

	fmt.Println("\n原始寄存器值:")
	for i, v := range regs {
		fmt.Printf("  寄存器[%d]: %d (0x%04X)\n", i, v, v)
	}

	// 按 DCBA 格式解析为 int32
	fmt.Println("\n按 DCBA 格式解析 (int32):")
	for i := 0; i+1 < len(regs); i += 2 {
		// DCBA: 交换寄存器顺序和字节顺序
		reg0 := ((regs[i+1] & 0xFF) << 8) | ((regs[i+1] >> 8) & 0xFF)
		reg1 := ((regs[i] & 0xFF) << 8) | ((regs[i] >> 8) & 0xFF)
		val := int32(uint32(reg0)<<16 | uint32(reg1))
		fmt.Printf("  地址 %d-%d: %d\n", i, i+1, val)
	}

	fmt.Println("\n测试完成!")
}
