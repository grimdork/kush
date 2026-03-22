#!/opt/homebrew/bin/kush
// network_examples.tengo -- Demonstrate scripting builtins for networking

// Note: This script exercises the built-in helpers exposed to Tengo scripts in kush:
// getenv, setenv, pr, printf, httpget, httppost, checkport, ping, dig

pr("--- Network builtins demo ---")

// Environment helpers
pr("KUSH_TEST before:", getenv("KUSH_TEST"))
setenv("KUSH_TEST", "network-demo")
pr("KUSH_TEST after:", getenv("KUSH_TEST"))

// HTTP GET
resp := httpget("https://grimdork.net")
pr("http_get returned:", resp)

// HTTP POST (simple form)
post := httppost("https://grimdork.net/form", "field=value")
pr("http_post returned:", post)

// Port check
open := checkport("grimdork.net", 443)
pr("port 443 open:", open)

// Ping (returns latency in ms or -1 on failure)
p := ping("grimdork.net")
if p == -1 {
    pr("ping failed")
} else {
    pr("ping latency (ms):", p)
}

// DNS lookup - structured object with helpers
d := dig("grimdork.net")
pr("ipv4 first:", d.ipv4.first())
pr("ipv4 all:", d.ipv4.all())
pr("ipv6 first:", d.ipv6.first())
pr("ipv6 all:", d.ipv6.all())

pr("--- demo complete ---")
