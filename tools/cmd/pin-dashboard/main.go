// Command pin-dashboard regenerates the table of pins in the front door's README from the
// lock files in each repository's main branch.
//
//	go run ./cmd/pin-dashboard --readme ../README.md            rewrite the table
//	go run ./cmd/pin-dashboard --readme ../README.md --check    fail if it is out of date
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/ParkWardRR/cairn-driving-log-selfhosted/tools/pindash"
)

const owner = "ParkWardRR"

func get(repo, file string) []byte {
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", owner, repo, file))
	if err != nil {
		fmt.Fprintln(os.Stderr, "pin-dashboard:", err)
		os.Exit(2)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "pin-dashboard: %s/%s: %s\n", repo, file, resp.Status)
		os.Exit(2)
	}
	b, _ := io.ReadAll(resp.Body)
	return b
}

func main() {
	readme := flag.String("readme", "../README.md", "the document holding the pins block")
	check := flag.Bool("check", false, "do not write; exit 1 if the block is out of date")
	flag.Parse()

	var repos []pindash.Repo
	for _, r := range []struct{ name, display string }{
		{"cairn-vehicle-server", "cairn-vehicle-server"},
		{"cairn-vehicle-web-dashboard", "cairn-vehicle-web-dashboard"},
		{"cairn-esp32-device-firmware", "cairn-esp32-device-firmware"},
		{"cairn-ios-companion-app", "cairn-ios-companion-app"},
		{"cairn-modules", "cairn-modules"},
	} {
		repos = append(repos, pindash.Repo{
			Name: r.display, URL: "https://github.com/" + owner + "/" + r.name,
			Contracts: get(r.name, "contracts.lock"),
			Server:    get(r.name, "server.lock"),
			Interop:   get(r.name, "interop.lock"),
		})
	}
	table, err := pindash.Render(repos)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pin-dashboard:", err)
		os.Exit(2)
	}
	doc, err := os.ReadFile(*readme)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pin-dashboard:", err)
		os.Exit(2)
	}
	out, changed, err := pindash.Replace(string(doc), table)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pin-dashboard:", err)
		os.Exit(2)
	}
	switch {
	case *check && changed:
		fmt.Fprintln(os.Stderr, "pin-dashboard: the README's pins are out of date; run it without --check")
		os.Exit(1)
	case *check:
		fmt.Println("pin-dashboard: up to date")
	case changed:
		if err := os.WriteFile(*readme, []byte(out), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "pin-dashboard:", err)
			os.Exit(2)
		}
		fmt.Println("pin-dashboard: updated", *readme)
	default:
		fmt.Println("pin-dashboard: already up to date")
	}
}
