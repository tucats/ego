package main

import "os"

const (
	compileOp = iota
)

func main() {
	operation := compileOp

	path := "/tmp/"
	source := "messages.go"

	for argc := 1; argc < len(os.Args); argc++ {
		arg := os.Args[argc]
		switch arg {
		case "-c", "--compile":
			operation = compileOp

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
		compile(path, source)
	}
}
