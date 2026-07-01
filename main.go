package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// Windows API Structs
type MEMORYSTATUSEX struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	globalMemoryStatus = kernel32.NewProc("GlobalMemoryStatusEx")
	getDiskFreeSpace   = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func main() {
	for {
		clearScreen()

		// 1. Fetch Memory Stats via Win32 API
		var memStatus MEMORYSTATUSEX
		memStatus.Length = uint32(unsafe.Sizeof(memStatus))
		ret, _, _ := globalMemoryStatus.Call(uintptr(unsafe.Pointer(&memStatus)))
		if ret == 0 {
			fmt.Println("Error reading system memory counters.")
			return
		}

		// 2. Fetch Disk Space Stats via Win32 API
		cDrive, _ := syscall.UTF16PtrFromString("C:\\")
		var freeBytes, totalBytes, totalFreeBytes uint64
		ret, _, _ = getDiskFreeSpace.Call(
			uintptr(unsafe.Pointer(cDrive)),
			uintptr(unsafe.Pointer(&freeBytes)),
			uintptr(unsafe.Pointer(&totalBytes)),
			uintptr(unsafe.Pointer(&totalFreeBytes)),
		)
		if ret == 0 {
			fmt.Println("Error reading drive space counters.")
			return
		}

		// Unit Conversions (Bytes to GB)
		gb := float64(1024 * 1024 * 1024)
		totalMem := float64(memStatus.TotalPhys) / gb
		availMem := float64(memStatus.AvailPhys) / gb
		usedMem := totalMem - availMem

		totalDisk := float64(totalBytes) / gb
		freeDisk := float64(freeBytes) / gb
		usedDisk := totalDisk - freeDisk
		diskPercent := (usedDisk / totalDisk) * 100

		// Print Live Output Data Streams
		fmt.Println("=========================================")
		fmt.Println("       NATIVE WINDOWS METRIC ENGINE      ")
		fmt.Println("=========================================")
		fmt.Printf("Live Time: %s\n\n", time.Now().Format("15:04:05"))

		fmt.Println("--- RAM METRICS ---")
		fmt.Printf("Total System RAM:  %.2f GB\n", totalMem)
		fmt.Printf("Used RAM:          %.2f GB\n", usedMem)
		fmt.Printf("Available RAM:     %.2f GB\n", availMem)
		fmt.Printf("Memory Load:       %d%%\n\n", memStatus.MemoryLoad)

		fmt.Println("--- DRIVE SPACE (C:) ---")
		fmt.Printf("Total Drive Size:  %.2f GB\n", totalDisk)
		fmt.Printf("Used Drive Space:  %.2f GB\n", usedDisk)
		fmt.Printf("Free Space Left:   %.2f GB\n", freeDisk)
		fmt.Printf("Disk Space Load:   %.2f%%\n", diskPercent)
		fmt.Println("=========================================")

		time.Sleep(1 * time.Second)
	}
}
