package main

import (
	"fmt"
	"sort"
	"syscall"
	"time"
	"unsafe"
)

// Win32 Constants
const (
	TH32CS_SNAPPROCESS        = 0x00000002
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ           = 0x0010
)

// Win32 Structs for System Metrics
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

// Win32 Structs for Process Tracking
type PROCESSENTRY32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16 // Defined as MAX_PATH array size to prevent compilation truncation
}

type PROCESS_MEMORY_COUNTERS struct {
	CB                      uint32
	PageFaultCount          uint32
	PeakWorkingSetSize      uintptr
	WorkingSetSize          uintptr
	QuotaPeakWorkingSetSize uintptr
	QuotaWorkingSetSize     uintptr
	QuotaPeakPagedPoolSize  uintptr
	QuotaPagedPoolSize      uintptr
	PeakPagefileUsage       uintptr
	PagefileUsage           uintptr
}

type ProcessInfo struct {
	Name string
	PID  uint32
	RAM  float64
}

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	psapi              = syscall.NewLazyDLL("psapi.dll")
	globalMemoryStatus = kernel32.NewProc("GlobalMemoryStatusEx")
	getDiskFreeSpace   = kernel32.NewProc("GetDiskFreeSpaceExW")

	createToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	process32First           = kernel32.NewProc("Process32FirstW")
	process32Next            = kernel32.NewProc("Process32NextW")
	openProcess              = kernel32.NewProc("OpenProcess")
	getProcessMemoryInfo     = psapi.NewProc("GetProcessMemoryInfo")
)

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func getTopProcesses() ([]ProcessInfo, error) {
	snapshot, _, err := createToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if syscall.Handle(snapshot) == syscall.InvalidHandle {
		return nil, err
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	var entry PROCESSENTRY32
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := process32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("failed to read first process")
	}

	var processes []ProcessInfo
	gb := float64(1024 * 1024 * 1024)

	for {
		name := syscall.UTF16ToString(entry.ExeFile[:])
		pid := entry.ProcessID

		hProcess, _, _ := openProcess.Call(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, 0, uintptr(pid))
		if hProcess != 0 {
			var memCounters PROCESS_MEMORY_COUNTERS
			memCounters.CB = uint32(unsafe.Sizeof(memCounters))

			retMem, _, _ := getProcessMemoryInfo.Call(hProcess, uintptr(unsafe.Pointer(&memCounters)), uintptr(memCounters.CB))
			if retMem != 0 {
				ramGB := float64(memCounters.WorkingSetSize) / gb
				processes = append(processes, ProcessInfo{Name: name, PID: pid, RAM: ramGB})
			}
			syscall.CloseHandle(syscall.Handle(hProcess))
		}

		ret, _, _ = process32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	sort.Slice(processes, func(i, j int) bool {
		return processes[i].RAM > processes[j].RAM
	})

	if len(processes) > 15 {
		return processes[:15], nil
	}
	return processes, nil
}

func main() {
	gb := float64(1024 * 1024 * 1024)

	for {
		clearScreen()

		// 1. Fetch Global Memory Stats
		var memStatus MEMORYSTATUSEX
		memStatus.Length = uint32(unsafe.Sizeof(memStatus))
		ret, _, _ := globalMemoryStatus.Call(uintptr(unsafe.Pointer(&memStatus)))
		if ret == 0 {
			fmt.Println("Error reading system memory counters.")
			return
		}

		// 2. Fetch Disk Space Stats
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

		// 3. Fetch Top 15 Active Processes
		topProcs, err := getTopProcesses()
		if err != nil {
			fmt.Printf("Error reading process tree: %v\n", err)
			return
		}

		// Unit Conversions
		totalMem := float64(memStatus.TotalPhys) / gb
		availMem := float64(memStatus.AvailPhys) / gb
		usedMem := totalMem - availMem

		totalDisk := float64(totalBytes) / gb
		freeDisk := float64(freeBytes) / gb
		usedDisk := totalDisk - freeDisk
		diskPercent := (usedDisk / totalDisk) * 100

		// Render Dashboard Interface
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
		fmt.Printf("Disk Space Load:   %.2f%%\n\n", diskPercent)

		fmt.Println("--- TOP 15 PROCESSES BY RAM ---")
		for i, proc := range topProcs {
			fmt.Printf("[%d] %-20s (PID: %-6d) -> %.2f GB\n", i+1, proc.Name, proc.PID, proc.RAM)
		}
		fmt.Println("=========================================")

		time.Sleep(1 * time.Second)
	}
}
