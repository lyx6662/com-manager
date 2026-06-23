package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("=== 设备1 测试 ===")
	fmt.Println("设备: 192.168.8.161:502")
	fmt.Println("从站ID: 200")
	fmt.Println("功能码: 03 (读保持寄存器)")
	fmt.Println("读取: 地址0开始，共11个寄存器\n")

	conn, err := net.DialTimeout("tcp", "192.168.8.161:502", 5*time.Second)
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()

	// 构建请求: 功能码03, 起始地址0, 数量11
	request := make([]byte, 12)
	request[0] = 0x00
	request[1] = 0x01
	request[2] = 0x00
	request[3] = 0x00
	request[4] = 0x00
	request[5] = 0x06
	request[6] = 200  // 从站ID
	request[7] = 0x03 // 功能码 03
	binary.BigEndian.PutUint16(request[8:10], 0)  // 起始地址 0
	binary.BigEndian.PutUint16(request[10:12], 11) // 数量 11

	fmt.Printf("发送报文: % X\n", request)

	_, err = conn.Write(request)
	if err != nil {
		fmt.Printf("发送失败: %v\n", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// 读取MBAP头 (6字节)
	mbap := make([]byte, 6)
	_, err = conn.Read(mbap)
	if err != nil {
		fmt.Printf("读取MBAP头失败: %v\n", err)
		return
	}
	fmt.Printf("MBAP头: % X\n", mbap)

	frameLength := int(mbap[4])<<8 | int(mbap[5])
	fmt.Printf("帧长度: %d\n", frameLength)

	// 读取响应体
	body := make([]byte, frameLength)
	_, err = conn.Read(body)
	if err != nil {
		fmt.Printf("读取响应体失败: %v\n", err)
		return
	}
	fmt.Printf("响应体: % X\n", body)

	// body[0]=UnitID, body[1]=FunctionCode, body[2]=ByteCount, body[3:]=Data
	unitID := body[0]
	funcCode := body[1]
	byteCount := int(body[2])
	fmt.Printf("UnitID: %d, 功能码: 0x%02X, 字节数: %d\n", unitID, funcCode, byteCount)

	if funcCode&0x80 != 0 {
		fmt.Printf("异常响应: 功能码=0x%02X, 异常码=0x%02X\n", funcCode, body[2])
		return
	}

	regs := make([]uint16, 11)
	for i := 0; i < 11 && i*2+3 < len(body); i++ {
		regs[i] = binary.BigEndian.Uint16(body[3+i*2 : 5+i*2])
	}

	fmt.Println("\n原始寄存器值:")
	for i, v := range regs {
		fmt.Printf("  寄存器[%d]: %d (0x%04X)\n", i, v, v)
	}

	fmt.Println("\n测试完成!")
}
