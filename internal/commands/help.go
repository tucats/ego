package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
)

const (
	topicsKey = "topics"
	introKey  = "introduction"
	helpKey   = "help"

	// Tag in the help file to introduce each topic. Note required trailing space.
	topicTag = ".topic "

	// Ruler string used in the text to help with alignment and formatting, which
	// should always be skipped when reading the help file.
	rulerString = "+--------+--------+-"

	// Name of the help text file, located in the EGO_PATH location.
	helpFileName   = "help"
	helpFileSuffix = ".txt"
)

// helpCommand examines the pending interactive input and reports whether the
// next line of it is a "help" command. When it is, the topic words on that
// line are returned along with everything that came after the line, which the
// caller must go on to process as ordinary Ego source.
//
// Returning the remainder separately is the whole point of this function.
// Input does not always arrive one line at a time. When Ego's input is a pipe
// rather than a console -- "ego run < script.ego", or a script piped in from
// another command -- the entire input is read into a single string before the
// run loop starts (see readSourceFromConsoleOrPipe). The help command used to
// be recognized by testing that whole string for a "help " prefix and then
// treating all of it as one command line, which had two consequences:
//
//   - The topic name swallowed the rest of the script. "help legal" followed
//     by more statements asked for a topic named "legal\nfmt.Println(...)",
//     which of course does not exist, so the user was told the topic could not
//     be found.
//
//   - Everything after the help command was then discarded without being run
//     or reported, because the caller replaced the whole input with an empty
//     string.
//
// Splitting at the first line ending fixes both. Note that the caller loops,
// so a script that opens with several help commands in a row has each of them
// handled in turn.
func helpCommand(text string) ([]string, string, bool) {
	var line, rest string

	// Split off the first line. strings.Cut looks for the separator and
	// reports whether it was there at all; when it was not, the input is a
	// single line with nothing following it.
	if before, after, found := strings.Cut(text, "\n"); found {
		line, rest = before, after
	} else {
		line, rest = text, ""
	}

	// A Windows line ending leaves a carriage return behind once the line
	// feed has been removed, and TrimSpace also disposes of any stray blanks
	// around the command itself.
	command := strings.ToLower(strings.TrimSpace(line))

	// The line has to be the word "help" on its own, or the word "help"
	// followed by the topic being asked about. Testing for "help" followed by
	// a space matters: without it, an Ego statement that merely begins with
	// those four letters, such as a call to a function named "helper", would
	// be mistaken for a request for documentation.
	if command != helpKey && !strings.HasPrefix(command, helpKey+" ") {
		return nil, "", false
	}

	// strings.Fields splits on any run of whitespace and discards empty
	// pieces, so "help  command   options" yields exactly three words.
	return strings.Fields(command), rest, true
}

// help displays help text for a given help command line. The first token is usually
// the keyword "help", though if present this is skipped over. The remaining strings
// are trimmed and converted to lower-case, and make into a single composite key that
// is separated by periods.
func help(userKeys []string) {
	keys := make([]string, 0)

	for n, key := range userKeys {
		// Strip the line ending from the last key on the line. TrimRight
		// removes any mixture of the two characters a line ending can be
		// made of, so this works whether the console handed back "topics\n"
		// or "topics\r\n".
		key = strings.TrimRight(key, "\r\n")

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

	language := os.Getenv(defs.EgoLangEnv)
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

	lines := splitLines(string(b))
	topic := strings.TrimSpace(strings.Join(keys, "."))

	ui.Log(ui.AppLogger, "app.help", ui.A{
		"path":     filename,
		"language": language})
	ui.Log(ui.AppLogger, "app.help.key", ui.A{
		"key": topic})

	// Trim any trailing spaces from each line in the array. The help text is
	// matched against exactly, so a stray space at the end of a ".topic" line
	// in the file would stop that topic from ever being found.
	for i := 0; i < len(lines); i++ {
		lines[i] = strings.TrimRight(lines[i], " ")
	}

	printTopicFromLines(topic, lines)
}

// splitLines breaks the text of a help file into individual lines, accepting
// any of the three line ending conventions that text files use.
//
// This matters because the help files are matched against exactly. A topic is
// located by testing whether a line is equal to ".topic introduction", so if
// the line the file actually contains is ".topic introduction" followed by a
// carriage return, no topic is ever found and every help request reports
// "Help topic not found".
//
// That is not a hypothetical. Git can be configured to convert text files to
// the local convention when they are checked out, and on Windows the
// installer's default setting does exactly that. A developer building Ego on
// Windows would get a lib directory whose help files end every line with a
// carriage return and a line feed, that archive would be packed into the
// executable, and help would then be broken for every user of that build.
// The repository's .gitattributes file now prevents the conversion, but this
// function makes the reader tolerant of it either way -- including for anyone
// who already has a converted copy installed.
//
// The three conventions are:
//
//	"\r\n"  Windows, and the internet's text protocols
//	"\n"    Unix, Linux, and macOS
//	"\r"    classic Mac OS, before Mac OS X
//
// Converting the first two forms into the third's separator and then doing a
// single split handles all three without needing to know which one was used,
// and without caring if a single file happens to mix them.
func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	return strings.Split(text, "\n")
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

	printing, shouldReturn := printOneTopic(lines, topic, printing, previousTopics, subtopicHeadings, heading)
	if shouldReturn {
		return
	}

	if !printing {
		fmt.Println("Help topic not found")
	}
}

func printOneTopic(lines []string, topic string, printing bool, previousTopics map[string]bool, subtopicHeadings bool, heading string) (bool, bool) {
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, rulerString) {
			continue
		}

		if line == topicTag+topic {
			printing = true

			continue
		}

		if printing && strings.HasPrefix(line, topicTag) {
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

			return false, true
		}

		if printing && !subtopicHeadings {
			fmt.Println(line)
		}
	}

	return printing, false
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
				ui.Log(ui.AppLogger, "app.help.error", ui.A{
					"error": err})

				return "", nil
			}
		}
	}

	return filename, b
}
