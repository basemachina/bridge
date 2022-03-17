package testlogr

import (
	"log"

	"github.com/go-logr/logr/funcr"
)

// Logger is used in only tests
var Logger = funcr.New(
	func(prefix, args string) {
		log.Println(prefix, args)
	},
	funcr.Options{},
)
