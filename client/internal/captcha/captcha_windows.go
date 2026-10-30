// Package captcha shows the VK "I'm not a robot" page in a WebView2 window and
// captures the success token the page receives.
package captcha

import (
	"context"
	_ "embed"
	"errors"
	"runtime"
	"sync"

	webview2 "github.com/jchv/go-webview2"
)

//go:embed interceptor.js
var interceptor string

var mu sync.Mutex

// Solve opens redirectURI and blocks until the user passes the check, closes
// the window or ctx ends. dataDir keeps the WebView2 profile.
func Solve(ctx context.Context, redirectURI, dataDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	result := make(chan string, 1)
	ready := make(chan webview2.WebView, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		view := webview2.NewWithOptions(webview2.WebViewOptions{
			DataPath:  dataDir,
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title:  "WDTT Plus — проверка VK",
				Width:  440,
				Height: 680,
				Center: true,
			},
		})
		if view == nil {
			ready <- nil
			return
		}
		_ = view.Bind("wdttCaptchaSolved", func(token string) {
			select {
			case result <- token:
			default:
			}
			view.Destroy()
		})
		view.Init(interceptor)
		view.Navigate(redirectURI)
		ready <- view
		view.Run()
	}()

	view := <-ready
	if view == nil {
		<-done
		return "", errors.New("не удалось открыть окно WebView2: установите Microsoft Edge WebView2 Runtime")
	}
	select {
	case token := <-result:
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
		view.Destroy()
		<-done
		return "", ctx.Err()
	}
}
