// Command webapp starts the project-request web application.
//
//	go run ./cmd/webapp [-addr host:port] [-oauth-client path]
//
// If -addr is not given, the port defaults to $PORT (Cloud Run sets this),
// falling back to 8080. -oauth-client defaults to secrets/client_secret.json
// under the current directory (see webapp/oauth-client.md), so this is
// normally run from the repository root.
package main

import (
	"cmp"
	"flag"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
	"github.com/yusni/google-cloud-auto-provision-2/webapp"
)

func main() {
	defaultAddr := ":" + cmp.Or(os.Getenv("PORT"), "8080")
	addr := flag.String("addr", defaultAddr, "address to listen on")
	oauthClient := flag.String("oauth-client", "secrets/client_secret.json", "path to the downloaded OAuth client JSON (see webapp/oauth-client.md)")
	flag.Parse()

	cfg, err := webapp.LoadOAuthConfig(*oauthClient)
	if err != nil {
		log.Fatal(err)
	}
	// TODO: pass real dependencies, the organization and break-glass groups once they exist.
	projects := webapp.NewProjectCreationService(nil, nil, nil, nil, domain.Organization{}, domain.BreakGlassRequirement{})

	log.Printf("listening on http://%s", displayAddr(*addr))
	log.Fatal(http.ListenAndServe(*addr, webapp.Handler(cfg, projects)))
}

// displayAddr turns an address suitable for net.Listen, such as ":8080" or
// "0.0.0.0:8080", into one that can be pasted into a browser, filling in
// "localhost" when the address has no host.
func displayAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "localhost:" + port
	}
	return addr
}
