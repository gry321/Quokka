//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"sync"
	"syscall"
	"unsafe"
)

// Windows API structs for icon extraction
type shFileInfoW struct {
	hIcon         syscall.Handle
	iIcon         int32
	dwAttributes  uint32
	szDisplayName [260]uint16
	szTypeName    [80]uint16
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  syscall.Handle
	hbmColor syscall.Handle
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type bitmapInfo struct {
	bmiHeader bitmapInfoHeader
}

const (
	shgfiIcon        = 0x000000100
	shgfiLargeIcon   = 0x000000000
	shgfiUseFileAttr = 0x000000010
	biRGB            = 0
	dibRGBColors     = 0
)

var (
	shell32DLL              = syscall.NewLazyDLL("shell32.dll")
	procSHGetFileInfoW      = shell32DLL.NewProc("SHGetFileInfoW")
	procExtractAssociatedIcon = shell32DLL.NewProc("ExtractAssociatedIconW")

	gdi32DLL           = syscall.NewLazyDLL("gdi32.dll")
	procGetIconInfo    = syscall.NewLazyDLL("user32.dll").NewProc("GetIconInfo")
	procGetDIBits      = gdi32DLL.NewProc("GetDIBits")
	procDeleteObject   = gdi32DLL.NewProc("DeleteObject")
	procCreateCompatDC = gdi32DLL.NewProc("CreateCompatibleDC")
	procSelectObject   = gdi32DLL.NewProc("SelectObject")
	procDeleteDC       = gdi32DLL.NewProc("DeleteDC")
	procDestroyIcon    = syscall.NewLazyDLL("user32.dll").NewProc("DestroyIcon")
)

// iconCache stores base64-encoded icon PNGs keyed by file path.
var iconCache sync.Map

// GetFileIcon extracts the associated icon from an exe/file and returns a base64 PNG string.
func GetFileIcon(filePath string) string {
	if v, ok := iconCache.Load(filePath); ok {
		return v.(string)
	}

	b64 := extractIcon(filePath)
	if b64 != "" {
		iconCache.Store(filePath, b64)
	}
	return b64
}

func extractIcon(filePath string) string {
	pathPtr, err := syscall.UTF16PtrFromString(filePath)
	if err != nil {
		return ""
	}

	// Try SHGetFileInfo first (works for most file types including .lnk)
	var fi shFileInfoW
	ret, _, _ := procSHGetFileInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&fi)),
		unsafe.Sizeof(fi),
		shgfiIcon|shgfiLargeIcon|shgfiUseFileAttr,
	)

	hIcon := fi.hIcon
	if ret == 0 || hIcon == 0 {
		// Fallback: ExtractAssociatedIcon (better for .exe directly)
		var idx uint16
		h, _, _ := procExtractAssociatedIcon.Call(0, uintptr(unsafe.Pointer(pathPtr)), uintptr(unsafe.Pointer(&idx)))
		hIcon = syscall.Handle(h)
		if hIcon == 0 {
			return ""
		}
	}
	defer procDestroyIcon.Call(uintptr(hIcon))

	// Get icon info (color bitmap + mask bitmap)
	var ii iconInfo
	ret, _, _ = procGetIconInfo.Call(
		uintptr(hIcon),
		uintptr(unsafe.Pointer(&ii)),
	)
	if ret == 0 {
		return ""
	}
	defer procDeleteObject.Call(uintptr(ii.hbmMask))
	if ii.hbmColor != 0 {
		defer procDeleteObject.Call(uintptr(ii.hbmColor))
	}

	const iconSize = 32

	// Prepare DIB header for reading
	bi := bitmapInfo{
		bmiHeader: bitmapInfoHeader{
			biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
			biWidth:       iconSize,
			biHeight:      -iconSize, // top-down
			biPlanes:      1,
			biBitCount:    32,
			biCompression: biRGB,
		},
	}

	pixels := make([]byte, iconSize*iconSize*4)

	// Create a memory DC to select the bitmap into
	dc, _, _ := procCreateCompatDC.Call(0)
	if dc == 0 {
		return ""
	}
	defer procDeleteDC.Call(dc)

	bmpHandle := ii.hbmColor
	if bmpHandle == 0 {
		bmpHandle = ii.hbmMask
	}

	oldBmp, _, _ := procSelectObject.Call(dc, uintptr(bmpHandle))
	if oldBmp != 0 {
		defer procSelectObject.Call(dc, oldBmp)
	}

	// Read pixel data
	ret, _, _ = procGetDIBits.Call(
		dc,
		uintptr(bmpHandle),
		0, iconSize,
		uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
	)
	if ret == 0 {
		return ""
	}

	// Check if alpha channel is all zeros (no premultiplied alpha)
	hasAlpha := false
	for i := 3; i < len(pixels); i += 4 {
		if pixels[i] != 0 {
			hasAlpha = true
			break
		}
	}

	// If no alpha from color bitmap, use the mask bitmap to derive transparency
	if !hasAlpha {
		maskPixels := make([]byte, iconSize*iconSize*4)
		maskBI := bitmapInfo{
			bmiHeader: bitmapInfoHeader{
				biSize:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
				biWidth:       iconSize,
				biHeight:      -iconSize,
				biPlanes:      1,
				biBitCount:    32,
				biCompression: biRGB,
			},
		}
		oldMask, _, _ := procSelectObject.Call(dc, uintptr(ii.hbmMask))
		if oldMask != 0 {
			defer procSelectObject.Call(dc, oldMask)
		}
		ret, _, _ = procGetDIBits.Call(
			dc, uintptr(ii.hbmMask),
			0, iconSize,
			uintptr(unsafe.Pointer(&maskPixels[0])),
			uintptr(unsafe.Pointer(&maskBI)),
			dibRGBColors,
		)
		if ret != 0 {
			for i := 0; i < iconSize*iconSize; i++ {
				// Mask pixel: if color mask pixel is non-zero (white), make transparent
				if maskPixels[i*4] != 0 || maskPixels[i*4+1] != 0 || maskPixels[i*4+2] != 0 {
					pixels[i*4+3] = 0 // transparent
				} else {
					pixels[i*4+3] = 255 // opaque
				}
			}
		}
	}

	// Build NRGBA image (Windows BMP stores BGRA, we need RGBA)
	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			off := (y*iconSize + x) * 4
			b, g, r, a := pixels[off], pixels[off+1], pixels[off+2], pixels[off+3]
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: a})
		}
	}

	// Encode to PNG then base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
