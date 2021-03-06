
// CLI

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	golsp "github.com/ajaymt/golsp/core"
	// "fmt"
)

func main() {
	filename := "-"
	dirname := "."
	file := os.Stdin
	var args []string

	if len(os.Args) > 1 {
		filename = os.Args[1]
		args = os.Args[2:]
	}

	if filename != "-" {
		filename, _ = filepath.Abs(filename)
		dirname = filepath.Dir(filename)
		file, _ = os.Open(filename)
	}

	input, _ := ioutil.ReadAll(file)
	// fmt.Println(PrintST(golsp.MakeST(golsp.Tokenize(string(input)))))
	golsp.Run(dirname, filename, args, string(input))
}
