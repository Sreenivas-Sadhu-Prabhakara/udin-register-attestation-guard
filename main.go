// Command udin-register-attestation-guard keeps a practising firm's register of
// attested documents: what was signed, for whom, by which partner, against
// which figures, and whether the UDIN for it exists yet.
//
// It is one binary with no dependencies. It serves a small interface on the
// loopback address and keeps everything in a plain JSON file on the same
// machine. It makes no outbound network connections of any kind: there is no
// telemetry, no update check, and no link to the UDIN portal or any other
// server.
//
// It does not generate UDINs. Only ICAI's portal issues a UDIN, and only the
// signing member can log in and generate one. This tool records the number and
// counts the days.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/sreenivas-sadhu-prabhakara/udin-register-attestation-guard/internal/udin"
)

//go:embed web
var webFiles embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on; loopback only unless you deliberately change it")
	data := flag.String("data", "./udin-register-data.json", "path to the local register file")
	flag.Parse()

	if err := run(*addr, *data); err != nil {
		log.Fatalf("udin-register-attestation-guard: %v", err)
	}
}

func run(addr, data string) error {
	store, err := OpenStore(data)
	if err != nil {
		return err
	}

	ui, err := fs.Sub(webFiles, "web")
	if err != nil {
		return fmt.Errorf("could not open the embedded interface: %w", err)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %w", addr, err)
	}

	server := &http.Server{
		Handler:           NewServer(store, ui),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	out := log.New(os.Stdout, "", 0)
	out.Printf("UDIN register and attestation guard")
	out.Printf("  open      http://%s", listener.Addr().String())
	out.Printf("  register  %s", store.Path())
	out.Printf("  window    %d days from signing, counting both the signing day and the day of generation",
		udin.DefaultGenerationWindowDays)
	out.Printf("  note      this tool does not generate UDINs; generate them at %s", udin.PortalURL)
	out.Printf("  offline   nothing is sent anywhere; stop with Ctrl-C")

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
