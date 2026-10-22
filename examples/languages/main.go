package main

import (
	"fmt"
	"os"

	"github.com/longafterx/locq"
)

func main() {
	result, err := locq.Analyze([]string{"."}, &locq.Options{
		IncludeLangs: []string{"Go", "Python"},
		Exclude:      []string{"{dist,node_modules,target}/"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "locq: %v\n", err)
		os.Exit(1)
	}

	for _, lang := range result.Languages {
		fmt.Println(lang)
	}
	fmt.Println(result.Total)
	fmt.Printf("%+v", result)
}
