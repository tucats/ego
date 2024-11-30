package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

const (
	topicsKey = "topics"
	introKey  = "introduction"
	debugKey  = "-d"
	helpKey   = "help"

	// Tag in the help file to introduce each topic. Note required trailing space.
	topicTag = ".topic "

	// Name of the help text file, located in the EGO_PATH location.
	helpFileName   = "help"
	helpFileSuffix = ".txt"
)

// help displays help text for a given help command line. The first token is usually
// the keyword "help", though if present this is skipped ovfer. The remaining strings
// are trimmed and convered to lower-case, and make into a single composite key that
// is separated by periods.
func help(userKeys []string) {
	keys := make([]string, 0)

	for n, key := range userKeys {
		key = strings.TrimSuffix(key, "\n")

		// Skip over the leading "help" token if found.
		if n == 0 && key == helpKey {
			continue
		}

		if len(strings.TrimSpace(key)) > 0 {
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 {
		keys = []string{introKey}
	}

	printHelp(keys)
}

func printHelp(keys []string) {
	var (
		path string
	)

	if libpath := settings.Get(defs.EgoLibPathSetting); libpath != "" {
		path = libpath
	} else {
		path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	language := os.Getenv("EGO_LANG")
	if language == "" {
		language = os.Getenv("LANG")
	}

	if len(language) > 2 {
		language = language[0:2]
	}

	// First, see if there is a help file with the current language
	// Not found, see if there is a help file for "en"
	// Not found, try to find the generic help file
	filename, b := findHelpContentByForLanguage(path, language)
	if b == nil {
		return
	}

	lines := strings.Split(string(b), "\n")
	topic := strings.TrimSpace(strings.Join(keys, "."))

	ui.Log(ui.AppLogger, "Using help file %s, language \"%s\"", filename, language)
	ui.Log(ui.AppLogger, "Using help key  %s", topic)

	// Trim any trailing spaces from each line in the array
	for i := 0; i < len(lines); i++ {
		for strings.HasSuffix(lines[i], " ") {
			lines[i] = strings.TrimSuffix(lines[i], " ")
		}
	}

	// If a subtopic, list it
	// If it is a sub=topic and we are doing the top-level topics
	// listing, then skip this entry.
	// Have we already put out a topic that starts the same way as this
	// string?
	// Have we put out the helpful heading yet?
	printTopicFromLines(topic, lines)
}

func printTopicFromLines(topic string, lines []string) {
	printing := false
	subtopicHeadings := false
	heading := "Additional topics:"

	if topic == topicsKey {
		printing = true
		topic = ""
		heading = "Help topics:"
	}

	previousTopics := map[string]bool{}

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "+--------+--------+-") {
			continue
		}

		if line == topicTag+topic {
			printing = true

			continue
		} else if printing && len(line) > 7 && line[0:len(topicTag)] == topicTag {
			if strings.HasPrefix(line, topicTag+topic) {
				if topic == "" && strings.Contains(line[1:], ".") {
					continue
				}

				topicUsed := false

				for k := range previousTopics {
					if strings.HasPrefix(line, k) {
						topicUsed = true

						break
					}
				}

				if !topicUsed {
					previousTopics[line] = true
				} else {
					continue
				}

				if !subtopicHeadings {
					fmt.Printf("\n%s\n", heading)

					subtopicHeadings = true
				}

				subtopic := strings.ReplaceAll(strings.TrimPrefix(line, topicTag), ".", " ")
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

	return
}

func findHelpContentByForLanguage(path string, language string) (string, []byte) {
	filename := filepath.Join(path, helpFileName+"_"+language+helpFileSuffix)

	b, err := os.ReadFile(filename)
	if err != nil {
		filename = filepath.Join(path, helpFileName+"_en"+helpFileSuffix)

		b, err = os.ReadFile(filename)
		if err != nil {
			filename = filepath.Join(path, helpFileName+helpFileSuffix)

			b, err = os.ReadFile(filename)
			if err != nil {
				fmt.Println("Help unavailable (unable to read help text file)")
				ui.Log(ui.AppLogger, "Help error: %v", err)

				return "", nil
			}
		}
	}

	return filename, b
}
