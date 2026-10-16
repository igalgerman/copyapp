//go:build copyapp_test_loopback

package main

// Temporary integration-test copies must never open a LAN listener. Keeping this
// in a separate build also preserves the scope when testing self-install/update.
const httpsBindHost = "127.0.0.1"
