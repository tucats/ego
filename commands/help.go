package commands

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

type helpTopic struct {
	name      string
	subTopics []helpTopic
	text      []string
}

func help(keys []string) {
	if len(keys) == 1 {
		keys = []string{"introduction"}
	} else {
		keys = keys[1:]
	}

	cleanKeys := make([]string, 0)

	for _, key := range keys {
		if len(strings.TrimSpace(key)) > 0 {
			cleanKeys = append(cleanKeys, strings.TrimSuffix(key, "\n"))
		}
	}

	printHelp(cleanKeys)
}

func printHelp(keys []string) {
	fn := path.Join(settings.Get(defs.EgoPathSetting), "help.txt")

	b, err := ioutil.ReadFile(fn)
	if err != nil {
		fmt.Printf("Help unavailable; %v\n", err)
	}

	topic := strings.Join(keys, ".")
	lines := strings.Split(string(b), "\n")
	printing := false
	subtopicHeadings := false
	heading := "Additional topics:"

	if topic == "topics" {
		printing = true
		topic = ""
		heading = "Help topics:"
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}

		if line == ".topic "+topic {
			printing = true

			continue
		} else if printing && len(line) > 7 && line[0:7] == ".topic " {
			// If a subtopic, list it
			if strings.HasPrefix(line, ".topic "+topic) {
				// If it is a sub=topic and we are doing the top-level topics
				// listing, then skip this entry.
				if topic == "" && strings.Index(line[1:], ".") >= 0 {
					continue
				}

				// Have we put out the helpful heading yet?
				if !subtopicHeadings {
					fmt.Printf("\n%s\n", heading)
					subtopicHeadings = true
				}

				subtopic := strings.ReplaceAll(strings.TrimPrefix(line, ".topic "), ".", " ")
				fmt.Printf("  %s\n", subtopic)

				continue
			}

			if subtopicHeadings {
				fmt.Println()
			}

			return
		}

		if printing && !subtopicHeadings {
			fmt.Println(line)
		}
	}

	if !printing {
		fmt.Println("Help topic not found")
	}
}
