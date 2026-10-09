package main

import (
	"fmt"
	"os"

	"thinhthan/internal/conformance/style"
)

func main() {
	for _, d := range style.CheckCSharpTree(os.Args[1]) {
		fmt.Println("STYLE:", d)
	}
}
