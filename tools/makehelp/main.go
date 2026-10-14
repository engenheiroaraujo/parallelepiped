// Command makehelp prints the documented targets of a Makefile: every
// "target: ... ## description" line. It replaces the usual grep/awk one-liner
// so `make help` also works without a POSIX shell.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: makehelp <Makefile>")
		os.Exit(2)
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		target, rest, ok := strings.Cut(line, ":")
		if !ok || strings.ContainsAny(target, " \t=$") {
			continue
		}
		_, description, ok := strings.Cut(rest, "## ")
		if !ok {
			continue
		}
		fmt.Printf("  %-10s %s\n", target, strings.TrimSpace(description))
	}
}
