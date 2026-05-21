package main

import (
	"fmt"
	"os"
)

func main() {
	// 1. Phân tích tham số dòng lệnh
	opts := ParseArgs(os.Args)

	// 2. Route đến các hành động xử lý Đọc/Ghi thông tin (Không đổi state hệ thống)
	if opts.Query {
		fmt.Println(GetCurrentMode())
		return
	} else if opts.CacheCreate {
		AssertRoot()
		CreateCache()
		return
	} else if opts.CacheDelete {
		AssertRoot()
		DeleteCache()
		return
	} else if opts.CacheQuery {
		ShowCache()
		return
	}

	// 3. Route đến các hành động thay đổi trạng thái phần cứng/hệ điều hành
	if opts.Switch != "" || opts.ResetSddm || opts.Reset {

		// Luôn thiết lập cache trước khi đổi mode
		SetupCacheAdapter()

		if opts.Switch != "" {
			AssertRoot()

			// Map dữ liệu CLI -> SwitchOptions của core switcher
			switchOpts := SwitchOptions{
				DisplayManager:   opts.Dm,
				ForceComp:        opts.ForceComp,
				CoolbitsValue:    opts.Coolbits,
				Rtd3Value:        opts.Rtd3,
				UseNvidiaCurrent: opts.UseNvidiaCurrent,
			}
			SwitchMode(opts.Switch, switchOpts)

		} else if opts.ResetSddm {
			AssertRoot()
			CreateFile(SddmXsetupPath, SddmXsetupContent, true)
			fmt.Println("Operation completed successfully")

		} else if opts.Reset {
			AssertRoot()
			Cleanup()
			DeleteCache()
			RebuildInitramfs()
			fmt.Println("Operation completed successfully")
		}
	}
}
