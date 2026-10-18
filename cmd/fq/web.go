package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/barisgokdemir/FileQuery/internal/web"
)

// runWeb starts the browser interface and blocks until Ctrl+C or the
// console window is closed.
func runWeb(port int, openBrowser bool, stdout, stderr io.Writer) int {
	srv := web.New(version)
	ln, url, err := srv.Listen(port)
	if err != nil {
		fmt.Fprintf(stderr, "fq: arayüz başlatılamadı: %v\n", err)
		if port != 0 {
			fmt.Fprintln(stderr, "Port kullanımda olabilir; --port seçeneğini kaldırarak boş bir port seçtirebilirsiniz.")
		}
		return 1
	}

	fmt.Fprintf(stdout, "FileQuery %s arayüzü çalışıyor:\n\n  %s\n\n", version, url)
	if openBrowser {
		if err := web.OpenBrowser(url); err != nil {
			fmt.Fprintln(stdout, "Tarayıcı açılamadı; yukarıdaki adresi kopyalayıp tarayıcınıza yapıştırın.")
		} else {
			fmt.Fprintln(stdout, "Tarayıcı açıldı. Açılmadıysa yukarıdaki adresi kopyalayıp yapıştırın.")
		}
	}
	fmt.Fprintln(stdout, "Kapatmak için bu pencereyi kapatın ya da Ctrl+C'ye basın.")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := srv.Serve(ctx, ln); err != nil {
		fmt.Fprintln(stderr, "fq:", err)
		return 1
	}
	fmt.Fprintln(stdout, "\nArayüz kapatıldı.")
	return 0
}
