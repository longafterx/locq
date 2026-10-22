package main

import (
	"fmt"
	"os"

	"github.com/longafterx/locq"
)

func main() {
	result, err := locq.Analyze([]string{"."}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "locq: %v\n", err)
		os.Exit(1)
	}

	for _, item := range result.Files {
		fmt.Println(item)
	}
	fmt.Println(result.Total)
	fmt.Printf("%+v", result)
}
