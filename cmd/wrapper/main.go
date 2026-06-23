// 包装程序：监控 com-manager.exe 进程，崩溃时记录退出码并自动重启
// 用法: wrapper.exe [com-manager.exe路径]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	// 确定 com-manager.exe 路径
	exePath := "com-manager.exe"
	if len(os.Args) > 1 {
		exePath = os.Args[1]
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(exePath)
	if err != nil {
		fmt.Printf("获取路径失败: %v\n", err)
		os.Exit(1)
	}

	// 日志文件
	logDir := "logs"
	os.MkdirAll(logDir, 0755)
	crashLogPath := filepath.Join(logDir, "crash.log")

	fmt.Println("========================================")
	fmt.Println("  进程监控包装程序")
	fmt.Println("  目标:", absPath)
	fmt.Println("  崩溃日志:", crashLogPath)
	fmt.Println("========================================")

	restartCount := 0

	for {
		restartCount++
		fmt.Printf("\n[%s] 第 %d 次启动\n", time.Now().Format("2006-01-02 15:04:05"), restartCount)

		cmd := exec.Command(absPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = filepath.Dir(absPath)

		startTime := time.Now()

		if err := cmd.Start(); err != nil {
			fmt.Printf("启动失败: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		pid := cmd.Process.Pid
		fmt.Printf("进程已启动, PID: %d\n", pid)

		// 等待进程退出
		err := cmd.Wait()
		elapsed := time.Since(startTime)
		exitCode := 0

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}

		// 如果运行时间很短且异常退出，记录崩溃
		if exitCode != 0 {
			crashMsg := fmt.Sprintf("[%s] 进程崩溃! PID: %d, 退出码: %d, 运行时长: %v",
				time.Now().Format("2006-01-02 15:04:05"), pid, exitCode, elapsed.Round(time.Second))

			fmt.Println("\n" + crashMsg)

			// Windows 访问违规的退出码
			switch exitCode {
			case 3221225477: // 0xC0000005 - ACCESS_VIOLATION
				fmt.Println("原因: 内存访问违规 (ACCESS_VIOLATION) - C 库段错误")
			case 3221225725: // 0xC00000FD - STACK_OVERFLOW
				fmt.Println("原因: 栈溢出 (STACK_OVERFLOW)")
			case 3221225786: // 0xC000013A - CONTROL_C_EXIT
				fmt.Println("原因: 用户中断 (Ctrl+C)")
			default:
				fmt.Printf("原因: 未知错误 (退出码: %d / 0x%X)\n", exitCode, uint32(exitCode))
			}

			// 写入崩溃日志
			f, fErr := os.OpenFile(crashLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if fErr == nil {
				fmt.Fprintf(f, "%s\n", crashMsg)
				switch exitCode {
				case 3221225477:
					fmt.Fprintf(f, "  原因: 内存访问违规 (ACCESS_VIOLATION) - C 库段错误\n")
				case 3221225725:
					fmt.Fprintf(f, "  原因: 栈溢出 (STACK_OVERFLOW)\n")
				case 3221225786:
					fmt.Fprintf(f, "  原因: 用户中断 (Ctrl+C)\n")
				default:
					fmt.Fprintf(f, "  原因: 未知错误 (退出码: %d / 0x%X)\n", exitCode, uint32(exitCode))
				}
				f.Close()
			}
		} else {
			fmt.Printf("进程正常退出, 运行时长: %v\n", elapsed.Round(time.Second))
		}

		// 如果是用户中断，不重启
		if exitCode == 3221225786 || exitCode == 0 {
			fmt.Println("正常退出，不再重启")
			break
		}

		// 崩溃后等待再重启
		fmt.Printf("5 秒后重启...\n")
		time.Sleep(5 * time.Second)
	}
}
