//go:build !online && !e2e

package parser

import "github.com/alexdupre/modules2tuple/v2/config"

func init() {
	config.Offline = true
}
