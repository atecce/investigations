package main

import (
	"flag"
	"fmt"
	"lyrics_net"
	"os"
)

func main() {

	// touch status file
	f, err := os.Create("statuses.txt")

	// check error
	if err != nil {
		fmt.Println("Failed to create file:", err)
	}

	// close file
	f.Close()

	// set start flag
	verbose := flag.Bool("v", false, "Print the tree as it is parsed.")
	start := flag.String("s", "0", "Specify start artist of crawl.")
	flag.Parse()

	// start the investigation
	lyrics_net.Investigate(*verbose, *start)
}