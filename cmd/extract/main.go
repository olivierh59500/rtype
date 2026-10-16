// Command extract recreates ignored original assets from the source ADF.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/olivierh59500/r-type/internal/source"
)

func main() {
	adf := flag.String("adf", "previous/R-Type (1989)(Electric Dreams).adf", "source Amiga disk")
	out := flag.String("out", "assets/runtime", "generated asset directory")
	flag.Parse()
	manifest, err := source.Extract(*adf, *out)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Extracted original title, %d animation frames, %d level maps, font, and TFMX audio to %s\n", len(manifest.Sprites), len(manifest.Levels), *out)
}
