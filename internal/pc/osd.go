//go:build windows

package pc

import (
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The on-screen confirmation: a small always-on-top box near the clock
// ("✓ Sent to phone") for about 1.5 s. Windows toasts can be silenced by Focus
// Assist or turned off for PowerShell, so this is the feedback the user can
// always see after pressing the hotkey. It never takes the keyboard focus.

var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	gdi32                          = windows.NewLazySystemDLL("gdi32.dll")
	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procSetTimer                   = user32.NewProc("SetTimer")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procDrawTextW                  = user32.NewProc("DrawTextW")
	procFillRect                   = user32.NewProc("FillRect")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procSystemParametersInfoW      = user32.NewProc("SystemParametersInfoW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procGetDpiForSystem            = user32.NewProc("GetDpiForSystem")
	procCreateSolidBrush           = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW                = gdi32.NewProc("CreateFontW")
	procSelectObject               = gdi32.NewProc("SelectObject")
	procDeleteObject               = gdi32.NewProc("DeleteObject")
	procSetBkMode                  = gdi32.NewProc("SetBkMode")
	procSetTextColor               = gdi32.NewProc("SetTextColor")
)

const (
	wsPopup          = 0x80000000
	wsExTopmost      = 0x00000008
	wsExToolWindow   = 0x00000080
	wsExLayered      = 0x00080000
	wsExNoActivate   = 0x08000000
	wsExTransparent  = 0x00000020
	wmDestroy        = 0x0002
	wmPaint          = 0x000F
	wmTimer          = 0x0113
	swShowNoActivate = 4
	spiGetWorkArea   = 0x0030
	lwaAlpha         = 0x2
	dtCenter         = 0x1
	dtVCenter        = 0x4
	dtSingleLine     = 0x20
	dtNoPrefix       = 0x800
	dtRTLReading     = 0x20000
	bkTransparent    = 1
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type rect struct{ left, top, right, bottom int32 }

type paintStruct struct {
	hdc       windows.Handle
	erase     int32
	paint     rect
	restore   int32
	incUpdate int32
	reserved  [32]byte
}

type winMsg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

var (
	osdOnce    sync.Once
	osdClass   *uint16
	osdProc    uintptr
	osdBusy    atomic.Bool
	osdText    []uint16
	osdRTL     bool
	osdTextMu  sync.Mutex
	osdShownMu sync.Mutex
	osdShown   int // confirmations shown (tests)
)

// OSDLogf, when set, receives one line when the confirmation cannot be shown.
var OSDLogf func(format string, args ...any)

// OSDCount reports how many on-screen confirmations were shown.
func OSDCount() int {
	osdShownMu.Lock()
	defer osdShownMu.Unlock()
	return osdShown
}

func osdWndProc(hwnd windows.Handle, m uint32, wp, lp uintptr) uintptr {
	switch m {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		var rc rect
		procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
		br, _, _ := procCreateSolidBrush.Call(0x003DA316) // #16A34A as 0x00BBGGRR
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), br)
		procDeleteObject.Call(br)
		h := int32(rc.bottom-rc.top) * 2 / 5
		face, _ := windows.UTF16PtrFromString("Segoe UI")
		font, _, _ := procCreateFontW.Call(uintptr(-h), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
		old, _, _ := procSelectObject.Call(hdc, font)
		procSetBkMode.Call(hdc, bkTransparent)
		procSetTextColor.Call(hdc, 0x00FFFFFF)
		osdTextMu.Lock()
		text := append([]uint16(nil), osdText...)
		flags := uintptr(dtCenter | dtVCenter | dtSingleLine | dtNoPrefix)
		if osdRTL {
			flags |= dtRTLReading
		}
		osdTextMu.Unlock()
		if len(text) > 0 {
			procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)), uintptr(unsafe.Pointer(&rc)), flags)
		}
		procSelectObject.Call(hdc, old)
		procDeleteObject.Call(font)
		procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmTimer:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(m), wp, lp)
	return r
}

// ShowSent shows text (e.g. "✓ Sent to phone") in a small green box above
// the clock for 1.5 s without taking the focus. While one is visible, further
// calls are ignored (the beep and the tray tick still confirm them).
// COPYAPP_NO_OSD=1 turns it off.
func ShowSent(text string, rtl bool) {
	if os.Getenv("COPYAPP_NO_OSD") != "" {
		return
	}
	if !osdBusy.CompareAndSwap(false, true) {
		if OSDLogf != nil {
			OSDLogf("osd: previous confirmation still visible")
		}
		return
	}
	osdTextMu.Lock()
	osdText, _ = windows.UTF16FromString(text)
	osdText = osdText[:len(osdText)-1]
	osdRTL = rtl
	osdTextMu.Unlock()
	go func() {
		defer osdBusy.Store(false)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		osdOnce.Do(func() {
			osdClass, _ = windows.UTF16PtrFromString("CopyAppSentOSD")
			osdProc = windows.NewCallback(osdWndProc)
			var inst windows.Handle
			windows.GetModuleHandleEx(0, nil, &inst)
			wc := wndClassEx{wndProc: osdProc, instance: inst, className: osdClass}
			wc.size = uint32(unsafe.Sizeof(wc))
			procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		})
		dpi := uintptr(96)
		if procGetDpiForSystem.Find() == nil {
			if d, _, _ := procGetDpiForSystem.Call(); d > 0 {
				dpi = d
			}
		}
		w, h := int32(240*dpi/96), int32(48*dpi/96)
		margin := int32(12 * dpi / 96)
		var wa rect
		procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&wa)), 0)
		x, y := wa.right-w-margin, wa.bottom-h-margin
		if wa.right == 0 {
			x, y = 100, 100
		}
		hwnd, _, e := procCreateWindowExW.Call(
			wsExTopmost|wsExToolWindow|wsExLayered|wsExNoActivate|wsExTransparent,
			uintptr(unsafe.Pointer(osdClass)), uintptr(unsafe.Pointer(osdClass)), wsPopup,
			uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, 0, 0)
		if hwnd == 0 {
			if OSDLogf != nil {
				OSDLogf("osd: CreateWindowEx failed: %v", e)
			}
			return
		}
		procSetLayeredWindowAttributes.Call(hwnd, 0, 240, lwaAlpha)
		procShowWindow.Call(hwnd, swShowNoActivate)
		procSetTimer.Call(hwnd, 1, 1500, 0)
		if OSDLogf != nil {
			OSDLogf("osd: shown at %d,%d (%dx%d)", x, y, w, h)
		}
		osdShownMu.Lock()
		osdShown++
		osdShownMu.Unlock()
		var m winMsg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
}
