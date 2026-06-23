// 测试 IEC 61850 CGO 封装层
// 编译: go build -o test-iec61850.exe ./cmd/test-iec61850/
// 运行: test-iec61850.exe
package main

import (
	"fmt"
	"time"

	"github.com/lyx6662/com-manager/lib-iec61850"
)

func main() {
	fmt.Println("=== IEC 61850 CGO 封装层测试 ===")

	// 1. 创建数据模型
	fmt.Println("[1] 创建数据模型...")
	model := iec61850.CreateModel("TestModel")
	if model == nil {
		fmt.Println("ERROR: 创建模型失败")
		return
	}
	defer model.Destroy()

	// 2. 创建逻辑设备
	fmt.Println("[2] 创建逻辑设备 GRID_GATEWAY...")
	ld := model.AddLogicalDevice("GRID_GATEWAY")
	if ld == nil {
		fmt.Println("ERROR: 创建逻辑设备失败")
		return
	}

	// 3. 创建逻辑节点
	fmt.Println("[3] 创建逻辑节点 MMXU1...")
	ln := ld.AddLogicalNode("MMXU1")
	if ln == nil {
		fmt.Println("ERROR: 创建逻辑节点失败")
		return
	}

	// 4. 创建数据对象和数据属性
	fmt.Println("[4] 创建数据对象 TotW (总有功功率)...")
	totW := ln.AddDataObject("TotW")
	if totW == nil {
		fmt.Println("ERROR: 创建数据对象失败")
		return
	}

	// 创建 mag 子对象
	mag := totW.AddDataObject("mag")
	if mag == nil {
		fmt.Println("ERROR: 创建 mag 子对象失败")
		return
	}

	// 创建 f 属性 (浮点值, 功能约束 MX=测量值)
	daF := mag.AddDataAttribute("f",
		iec61850.TypeFloat32,
		iec61850.FCMeasurand,
		iec61850.TriggerDataChanged)
	if daF == nil {
		fmt.Println("ERROR: 创建 f 属性失败")
		return
	}

	// 注册到索引
	model.RegisterDA("GRID_GATEWAY/MMXU1.TotW.mag.f", daF)

	// 再创建一个状态量
	fmt.Println("[5] 创建数据对象 Mod (模式)...")
	mod := ln.AddDataObject("Mod")
	if mod == nil {
		fmt.Println("ERROR: 创建 Mod 数据对象失败")
		return
	}

	stVal := mod.AddDataAttribute("stVal",
		iec61850.TypeInt32,
		iec61850.FCStatus,
		iec61850.TriggerDataChanged)
	if stVal == nil {
		fmt.Println("ERROR: 创建 stVal 属性失败")
		return
	}
	model.RegisterDA("GRID_GATEWAY/MMXU1.Mod.stVal", stVal)

	// 6. 创建服务器
	fmt.Println("[6] 创建 IEC 61850 服务器...")
	srv := iec61850.NewServer(model, iec61850.ServerConfig{
		TCPPort: 10200,
	})
	if srv == nil {
		fmt.Println("ERROR: 创建服务器失败")
		return
	}
	defer srv.Destroy()

	// 7. 启动服务器
	fmt.Println("[7] 启动服务器 (端口 10200)...")
	if err := srv.Start(); err != nil {
		fmt.Printf("ERROR: 启动服务器失败: %v\n", err)
		return
	}
	defer srv.Stop()

	fmt.Printf("    服务器运行中: %v\n", srv.IsRunning())

	// 8. 更新数据
	fmt.Println("[8] 更新测量值...")
	srv.UpdateFloatByPath("GRID_GATEWAY/MMXU1.TotW.mag.f", 25.5)
	srv.UpdateInt32ByPath("GRID_GATEWAY/MMXU1.Mod.stVal", 1)

	// 读回验证
	val := srv.GetFloat(daF)
	fmt.Printf("    TotW.mag.f = %.2f\n", val)

	modVal := srv.GetInt32(stVal)
	fmt.Printf("    Mod.stVal = %d\n", modVal)

	// 9. 批量更新测试
	fmt.Println("[9] 批量更新测试...")
	srv.LockDataModel()
	for i := 0; i < 10; i++ {
		srv.UpdateFloatByPath("GRID_GATEWAY/MMXU1.TotW.mag.f", float32(i)*1.5)
	}
	srv.UnlockDataModel()

	val = srv.GetFloat(daF)
	fmt.Printf("    批量更新后 TotW.mag.f = %.2f\n", val)

	// 10. 等待连接
	fmt.Println("[10] 服务器运行中，等待客户端连接...")
	fmt.Println("     使用 IEDScout 或其他 61850 客户端连接 localhost:10200")
	fmt.Println("     按 Ctrl+C 退出")

	// 保持运行 60 秒，每秒检查连接数
	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)
		conns := srv.GetConnectionCount()
		if conns > 0 || i%10 == 0 {
			fmt.Printf("    [%ds] 运行中, 连接数: %d\n", i, conns)
		}
	}

	fmt.Println("=== 测试完成 ===")
}
