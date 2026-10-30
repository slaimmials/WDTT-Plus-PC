// Package captcha shows the VK "I'm not a robot" page in a WebView2 window and
// captures the success token the page receives.
package captcha

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

//go:embed interceptor.js
var interceptor string

const tokenPrefix = "wdtt-captcha:"

const (
	wmDestroy  = 0x0002
	wmMove     = 0x0003
	wmSize     = 0x0005
	wmActivate = 0x0006
	wmClose    = 0x0010

	wsOverlappedWindow = 0x00CF0000
	swShow             = 5
	smCXScreen         = 0
	smCYScreen         = 1
	idiApplication     = 32512
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	registerClassEx  = user32.NewProc("RegisterClassExW")
	createWindowEx   = user32.NewProc("CreateWindowExW")
	defWindowProc    = user32.NewProc("DefWindowProcW")
	destroyWindow    = user32.NewProc("DestroyWindow")
	showWindow       = user32.NewProc("ShowWindow")
	updateWindow     = user32.NewProc("UpdateWindow")
	setForeground    = user32.NewProc("SetForegroundWindow")
	postMessage      = user32.NewProc("PostMessageW")
	postQuitMessage  = user32.NewProc("PostQuitMessage")
	getMessage       = user32.NewProc("GetMessageW")
	translateMessage = user32.NewProc("TranslateMessage")
	dispatchMessage  = user32.NewProc("DispatchMessageW")
	getSystemMetrics = user32.NewProc("GetSystemMetrics")
	loadIcon         = user32.NewProc("LoadIconW")
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

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
	private uint32
}

var (
	mu        sync.Mutex
	classOnce sync.Once
	classErr  error
	className = windows.StringToUTF16Ptr("WDTTPlusCaptcha")

	windowsMu sync.Mutex
	windowMap = map[uintptr]*window{}
)

type window struct {
	hwnd     uintptr
	chromium *edge.Chromium
}

// Solve opens redirectURI and blocks until the user passes the check, closes
// the window or ctx ends. dataDir keeps the WebView2 profile.
func Solve(ctx context.Context, redirectURI, dataDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", fmt.Errorf("папка данных WebView2: %w", err)
	}

	result := make(chan string, 1)
	opened := make(chan error, 1)
	done := make(chan struct{})
	var hwnd uintptr

	go func() {
		defer close(done)
		// The thread stays locked so that it exits with the goroutine and its
		// COM apartment does not leak to other goroutines.
		runtime.LockOSThread()

		w, err := open(dataDir, redirectURI, func(token string) {
			select {
			case result <- token:
			default:
			}
		})
		if err != nil {
			opened <- err
			return
		}
		hwnd = w.hwnd
		opened <- nil
		loop()
		forget(w.hwnd)
	}()

	if err := <-opened; err != nil {
		<-done
		return "", err
	}
	select {
	case token := <-result:
		closeWindow(hwnd)
		<-done
		return token, nil
	case <-done:
		select {
		case token := <-result:
			return token, nil
		default:
			return "", errors.New("окно проверки закрыто")
		}
	case <-ctx.Done():
		closeWindow(hwnd)
		<-done
		return "", ctx.Err()
	}
}

func open(dataDir, url string, solved func(string)) (w *window, err error) {
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil && err != syscall.Errno(windows.S_FALSE) {
		return nil, fmt.Errorf("COM: %w", err)
	}
	classOnce.Do(registerClass)
	if classErr != nil {
		return nil, classErr
	}

	w = &window{chromium: edge.NewChromium()}
	w.hwnd, err = createWindow("WDTT Plus — проверка VK", 440, 680)
	if err != nil {
		return nil, err
	}
	remember(w)

	var embedErr error
	c := w.chromium
	c.DataPath = dataDir
	c.SetErrorCallback(func(err error) {
		log.Printf("[CAPTCHA] WebView2: %v", err)
		embedErr = err
		_, _, _ = postQuitMessage.Call(0)
	})
	c.MessageCallback = func(message string, _ *edge.ICoreWebView2, _ *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		if token, ok := strings.CutPrefix(message, tokenPrefix); ok && token != "" {
			solved(token)
		}
	}

	if !embed(c, w.hwnd) || c.GetController() == nil {
		_, _, _ = destroyWindow.Call(w.hwnd)
		forget(w.hwnd)
		if embedErr == nil {
			embedErr = errors.New("не удалось запустить")
		}
		return nil, fmt.Errorf("окно WebView2: %w; установите или обновите Microsoft Edge WebView2 Runtime", embedErr)
	}

	if settings, err := c.GetSettings(); err == nil {
		_ = settings.PutAreDefaultContextMenusEnabled(false)
		_ = settings.PutAreDevToolsEnabled(false)
	}
	c.Resize()
	c.Init(interceptor)
	c.Navigate(url)
	_, _, _ = setForeground.Call(w.hwnd)
	return w, nil
}

// embed starts the browser. When the WebView2 environment fails, the library
// leaves the webview unset and panics right after, so the panic is expected.
func embed(c *edge.Chromium, hwnd uintptr) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return c.Embed(hwnd)
}

func registerClass() {
	var instance windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &instance); err != nil {
		classErr = err
		return
	}
	icon, _, _ := loadIcon.Call(0, idiApplication)
	wc := wndClassEx{
		wndProc:   windows.NewCallback(wndProc),
		instance:  instance,
		icon:      windows.Handle(icon),
		iconSm:    windows.Handle(icon),
		className: className,
	}
	wc.size = uint32(unsafe.Sizeof(wc))
	if r, _, err := registerClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		classErr = fmt.Errorf("RegisterClassEx: %w", err)
	}
}

func createWindow(title string, width, height int32) (uintptr, error) {
	screenW, _, _ := getSystemMetrics.Call(smCXScreen)
	screenH, _, _ := getSystemMetrics.Call(smCYScreen)
	x := (int32(screenW) - width) / 2
	y := (int32(screenH) - height) / 2
	hwnd, _, err := createWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(title))),
		wsOverlappedWindow,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowEx: %w", err)
	}
	_, _, _ = showWindow.Call(hwnd, swShow)
	_, _, _ = updateWindow.Call(hwnd)
	return hwnd, nil
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	windowsMu.Lock()
	w := windowMap[hwnd]
	windowsMu.Unlock()
	if w != nil && w.chromium.GetController() != nil {
		switch message {
		case wmMove:
			_ = w.chromium.NotifyParentWindowPositionChanged()
		case wmSize:
			w.chromium.Resize()
		case wmActivate:
			if wParam != 0 {
				w.chromium.Focus()
			}
		}
	}
	switch message {
	case wmClose:
		_, _, _ = destroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		_, _, _ = postQuitMessage.Call(0)
		return 0
	}
	r, _, _ := defWindowProc.Call(hwnd, message, wParam, lParam)
	return r
}

func loop() {
	var m msg
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		_, _, _ = translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		_, _, _ = dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func closeWindow(hwnd uintptr) {
	_, _, _ = postMessage.Call(hwnd, wmClose, 0, 0)
}

func remember(w *window) {
	windowsMu.Lock()
	windowMap[w.hwnd] = w
	windowsMu.Unlock()
}

func forget(hwnd uintptr) {
	windowsMu.Lock()
	delete(windowMap, hwnd)
	windowsMu.Unlock()
}
