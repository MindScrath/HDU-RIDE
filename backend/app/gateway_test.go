package app

import "testing"

func TestRewriteGatewayLocation(t *testing.T) {
	prefix := "/ide/s/ws-1"
	host := "rstudio-ws-1.hdu-ride.svc.cluster.local:8787"

	cases := map[string]string{
		"/auth-sign-in":                               "/ide/s/ws-1/auth-sign-in",
		"/ide/s/ws-1/unsupported_browser.htm":         "/ide/s/ws-1/unsupported_browser.htm",
		"http://" + host + "/unsupported_browser.htm": "/ide/s/ws-1/unsupported_browser.htm",
		"http://" + host + "/ide/s/ws-1/p/?x=1":       "/ide/s/ws-1/p/?x=1",
		"https://example.edu/keep":                    "https://example.edu/keep",
		"":                                            "",
	}
	for input, want := range cases {
		if got := rewriteGatewayLocation(input, prefix, host); got != want {
			t.Fatalf("%q => %q, want %q", input, got, want)
		}
	}
}
