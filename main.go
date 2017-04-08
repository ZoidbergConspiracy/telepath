package main

import (
	"os"

	"github.com/zoidbergconspiracy/telepath/common"
)

func main() {
	common.Run(os.Args[1:], false)
}
