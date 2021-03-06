package kiwi

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"unsafe"

	"github.com/securityclippy/kiwi/w32"
	"golang.org/x/sys/windows"
)

// ProcPlatAttribs contains platform specific fields to be
// embedded into the Process struct.
type ProcPlatAttribs struct {
	Handle w32.HANDLE
}

// PROCESS_ALL_ACCESS is the windows constant for full process access.
const PROCESS_ALL_ACCESS = w32.PROCESS_VM_READ | w32.PROCESS_VM_WRITE | w32.PROCESS_VM_OPERATION | w32.PROCESS_QUERY_INFORMATION

// GetProcessByPID returns the process with the given PID.
func GetProcessByPID(pid int) (Process, error) {
	hnd, ok := w32.OpenProcess(PROCESS_ALL_ACCESS, false, uint32(pid))
	if !ok {
		return Process{}, fmt.Errorf("OpenProcess %v: %w", pid, windows.GetLastError())
	}
	return Process{ProcPlatAttribs: ProcPlatAttribs{Handle: hnd}, PID: uint64(pid)}, nil
}

// getFileNameByPID returns a file name given a PID.
func getFileNameByPID(pid uint32) (string, error) {
	// Open process.
	hnd, ok := w32.OpenProcess(w32.PROCESS_QUERY_INFORMATION, false, pid)
	if !ok {
		return "", fmt.Errorf("OpenProcess %v: %w", pid, windows.GetLastError())
	}
	defer w32.CloseHandle(hnd)

	// Get file path.
	path, ok := w32.GetProcessImageFileName(hnd)
	if !ok {
		return "", fmt.Errorf("GetProcessImageFileName: %w", windows.GetLastError())
	}

	// Split file path to get file name.
	_, fileName := filepath.Split(path)
	return fileName, nil
}

// GetProcessByFileName returns the process with the given file name.
// If multiple processes have the same filename, the first process
// enumerated by this function is returned.
func GetProcessByFileName(fileName string) (Process, error) {
	// Read in process ids
	pidCount := 1024
	var PIDs []uint32
	var bytesRead uint32

	// Get the process ids, increasing the PIDs buffer each time if there isn't enough space.
	for i := 1; uint32(len(PIDs))*uint32(unsafe.Sizeof(uint32(0))) == bytesRead; i++ {
		PIDs = make([]uint32, pidCount*i)
		ok := w32.EnumProcesses(PIDs, uint32(len(PIDs))*uint32(unsafe.Sizeof(uint32(0))), &bytesRead)
		if !ok {
			return Process{}, fmt.Errorf("EnumProcesses: %w", windows.GetLastError())
		}
	}

	// Loop over PIDs,
	// (Divide bytesRead by sizeof(uint32) to get how many processes there are).
	for i := uint32(0); i < (bytesRead / 4); i++ {
		// Skip over the system process with PID 0.
		if PIDs[i] == 0 {
			continue
		}

		// Get the filename for this process
		curFileName, err := getFileNameByPID(PIDs[i])
		if err != nil {
			//return Process{}, fmt.Errorf("getFileNameByPID %v: %w", PIDs[i], err)
			continue
		}

		// Check if it is the process being searched for.
		if curFileName == fileName {
			hnd, ok := w32.OpenProcess(PROCESS_ALL_ACCESS, false, PIDs[i])
			if !ok {
				return Process{}, fmt.Errorf("OpenProcess %v: %w", PIDs[i], windows.GetLastError())
			}
			return Process{ProcPlatAttribs: ProcPlatAttribs{Handle: hnd}, PID: uint64(PIDs[i])}, nil
		}
	}

	// Couldn't find process, return an error.
	return Process{}, errors.New("couldn't find process with name " + fileName)
}


func (p *Process) PrintModules() {
	handle, err := syscall.OpenProcess(w32.PROCESS_QUERY_INFORMATION | w32.PROCESS_VM_READ, false, uint32(p.PID))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Process ID: %+v\n", p.PID)

	fmt.Println(handle)


	var cbNeeded uint32


	modules := make([]w32.HMODULE, 1024)


	mInfos := make([]w32.MODULEINFO, len(modules))
	if w32.EnumProcessModules(handle, modules, uint32(len(modules)), &cbNeeded) {


		enumMods:
		for k, v := range modules {


			SzModule := make([]uint16, w32.MAX_PATH+1)
			ok, fn := w32.GetModuleFileNameExA(syscall.Handle(p.Handle), syscall.Handle(v), SzModule)
			if ok {
				if strings.Contains(fn, "WowClassic.exe") {
					ok, info := w32.GetModuleInformation(syscall.Handle(p.Handle), v, mInfos[k])
					if ok {
						if info.BaseAddress() != nil {
							fmt.Printf("base address: %+v\n", info.BaseAddress())
							fmt.Printf("%+v\n", info)
							break enumMods
						}
					}

				}

			}


		}

	}
}


// GetModuleBase takes a module name as an argument. (e.g. "kernel32.dll")
// Returns the modules base address.
//
// (Mostly taken from genkman's gist: https://gist.github.com/henkman/3083408)
// TODO(Andoryuuta): Figure out possible licencing issues with this, or rewrite.
func (p *Process) GetModuleBase(moduleName string) (uintptr, error) {
	snap, ok := w32.CreateToolhelp32Snapshot(w32.TH32CS_SNAPMODULE32|w32.TH32CS_SNAPALL|w32.TH32CS_SNAPMODULE, uint32(p.PID))
	if !ok {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot: %w", windows.GetLastError())
	}

	fmt.Printf("Snap: %+v\n", snap)
	defer w32.CloseHandle(snap)

	var me32 w32.MODULEENTRY32
	me32.DwSize = uint32(unsafe.Sizeof(me32))

	// Get first module.
	if !w32.Module32First(snap, &me32) {
		return 0, fmt.Errorf("Module32First: %w", windows.GetLastError())
	}

	// Check first module.
	if syscall.UTF16ToString(me32.SzModule[:]) == moduleName {
		return uintptr(unsafe.Pointer(me32.ModBaseAddr)), nil
	}

	// Loop all modules remaining.
	for w32.Module32Next(snap, &me32) {
		// Check this module.
		if syscall.UTF16ToString(me32.SzModule[:]) == moduleName {
			return uintptr(unsafe.Pointer(me32.ModBaseAddr)), nil
		}
	}

	// Module couldn't be found.
	return 0, errors.New("couldn't find module")
}


func (p *Process) ListProcessModules() (uintptr, error) {
	snap, ok := w32.CreateToolhelp32Snapshot(w32.TH32CS_SNAPMODULE32|w32.TH32CS_SNAPALL|w32.TH32CS_SNAPMODULE, uint32(p.PID))
	if !ok {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot: %w", windows.GetLastError())
	}
	defer w32.CloseHandle(snap)

	var me32 w32.MODULEENTRY32
	me32.DwSize = uint32(unsafe.Sizeof(me32))
	fmt.Println(snap)

	// Get first module.
	//if !w32.Module32First(snap, &me32) {
		//return 0, fmt.Errorf("Module32First: %w", windows.GetLastError())
	//}

	// Check first module.
	//if syscall.UTF16ToString(me32.SzModule[:]) == moduleName {
		//return uintptr(unsafe.Pointer(me32.ModBaseAddr)), nil
	//}

	// Loop all modules remaining.
	for w32.Module32Next(snap, &me32) {
		fmt.Println(syscall.UTF16ToString(me32.SzModule[:]))
		// Check this module.
		//if syscall.UTF16ToString(me32.SzModule[:]) == moduleName {
			//return uintptr(unsafe.Pointer(me32.ModBaseAddr)), nil
		//}
	}

	// Module couldn't be found.
	//return 0, errors.New("couldn't find module")
	return 0, nil
}

// The platform specific read function.
func (p *Process) read(addr uintptr, ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	dataAddr := getDataAddr(v)
	dataSize := getDataSize(v)
	bytesRead, ok := w32.ReadProcessMemory(
		p.Handle,
		unsafe.Pointer(addr),
		unsafe.Pointer(dataAddr),
		dataSize,
	)
	if !ok || bytesRead != dataSize {
		return errors.New("error reading process memory")
	}
	return nil
}

// The platform specific write function.
func (p *Process) write(addr uintptr, ptr interface{}) error {
	v := reflect.ValueOf(ptr)
	dataAddr := getDataAddr(v)
	dataSize := getDataSize(v)
	bytesWritten, ok := w32.WriteProcessMemory(
		p.Handle,
		unsafe.Pointer(addr),
		unsafe.Pointer(dataAddr),
		dataSize,
	)
	if !ok || bytesWritten != dataSize {
		return errors.New("error writing process memory")
	}
	return nil
}
