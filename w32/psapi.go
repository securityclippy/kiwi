package w32

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	psapi = syscall.NewLazyDLL("psapi.dll")

	// Process enumeration
	pEnumProcesses = psapi.NewProc("EnumProcesses")

	pEnumProcessModules = psapi.NewProc("EnumProcessModules")
	pGetModuleFileNameExA = psapi.NewProc("GetModuleFileNameExA")
	pGetModuleInformation = psapi.NewProc("GetModuleInformation")

	// Other
	pGetProcessImageFileName = psapi.NewProc("GetProcessImageFileNameA")
)

func EnumProcesses(pProcessIds []uint32, cb uint32, pBytesReturned *uint32) bool {
	ret, _, _ := pEnumProcesses.Call(uintptr(unsafe.Pointer(&pProcessIds[0])), uintptr(cb), uintptr(unsafe.Pointer(pBytesReturned)))
	return ret != 0
}

func EnumProcessModules(handle syscall.Handle, hMods []HMODULE, cb uint32, pBytesReturned *uint32) bool {
	ret, _, _ := pEnumProcessModules.Call(uintptr(handle), uintptr(unsafe.Pointer(&hMods[0])), uintptr(cb), uintptr(unsafe.Pointer(pBytesReturned)))
	return ret != 0
}

func GetModuleFileNameExA(hProcess syscall.Handle, hModule syscall.Handle, lpFilename []uint16) (bool, string) {
	lpFilenameByte := make([]byte, 2048)
	ret, _, _ := pGetModuleFileNameExA.Call(uintptr(hProcess), uintptr(hModule), uintptr(unsafe.Pointer(&lpFilenameByte[0])), uintptr(uint32(2048)))
	fmt.Println(string(lpFilenameByte[:ret]))
	return ret != 0, string(lpFilenameByte[:ret])
}

func szExeFileToString(ByteString []uint16) string {
	var End = 0

	for i, _ := range ByteString {
		if ByteString[i] == 0 {
			End = i
			break
		}
	}

	return syscall.UTF16ToString(ByteString[:End])
}

func GetModuleInformation(hProcess syscall.Handle, hmodule HMODULE, lpModInfo MODULEINFO) (bool, MODULEINFO) {
	fmt.Println("GetModuleInformation called")
	ret, _, _ := pGetModuleInformation.Call(uintptr(hProcess), uintptr(hmodule), uintptr(unsafe.Pointer(&lpModInfo)), unsafe.Sizeof(lpModInfo))

	//fmt.Printf("Module info: %+v\n", lpModInfo)
	return ret != 0, lpModInfo
}


func GetProcessImageFileName(hProcess HANDLE) (string, bool) {
	imageFileName := make([]byte, 2048)
	ret, _, _ := pGetProcessImageFileName.Call(uintptr(hProcess), uintptr(unsafe.Pointer(&imageFileName[0])), uintptr(len(imageFileName)))
	if ret != 0 {
		return string(imageFileName[:ret]), ret != 0
	} else {
		return "", ret != 0
	}
}
