// Package envutil provides small env-var helpers shared by lambada-web and lambada-mta.
package envutil

import "os"

// Or returns the named environment variable, or fallback if unset/empty.
func Or(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
