package main

import "os"

const (
	compileOp = iota
)

var (
	logging = false
)

func main() {
	operation := compileOp

	initDigest()

	path := "/tmp/"
	source := "messages.go"
	digest := "messages.checksum"

	for argc := 1; argc < len(os.Args); argc++ {
		arg := os.Args[argc]
		switch arg {
		case "-l", "--logging":
			logging = true

		case "-c", "--compile":
			operation = compileOp

		case "-d", "--digest":
			if argc == len(os.Args)-1 {
				panic("Missing digest file name\n")
			}

			digest = os.Args[argc+1]
			argc++

		case "-p", "--path":
			if argc == len(os.Args)-1 {
				panic("Missing path name\n")
			}

			path = os.Args[argc+1]
			argc++

		case "-s", "--source":
			if argc == len(os.Args)-1 {
				panic("Missing source file name\n")
			}

			source = os.Args[argc+1]
			argc++

		default:
			panic("Unrecognized argument: " + arg + "\n")
		}
	}

	switch operation {
	case compileOp:
		compile(path, source, digest)
	}
}
