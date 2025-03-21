package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func compile(path, source string) {
	messages := make(map[string]map[string]string)

	// Make a list of the files in the path directory that match the "messages_*.txt" pattern.
	files, err := os.ReadDir(path)
	if err != nil {
		panic(err)
	}

	// Compile the files in the path and create the internal dictionary and the digest value.
	compileFiles(files, path, messages)

	digest := digestValue(path)

	// See if the existing digest value matches the current value in the
	// digest file. If it matches, the file is up to date and we do not
	// rewrite it.
	doWrite := true

	if data, err := os.ReadFile(source); err == nil {
		// If the start of the source file contains the digest value, then
		// the file is up to date.
		if strings.HasPrefix(string(data), digest) {
			doWrite = false
		}
	}

	// If the digest was missing or did not match, rewrite the dictionary in memory
	// to the source file, with the associated digest header.
	if doWrite {
		writeMessageDictionary(source, messages, digest)
	}
}

func writeMessageDictionary(source string, messages map[string]map[string]string, digest string) {
	source, _ = filepath.Abs(source)

	// languageCount the number of languguages represented by the messages map.
	languageCount := 0
	for _, languages := range messages {
		if len(languages) > languageCount {
			languageCount = len(languages)
		}
	}

	fmt.Printf("Generating %s, with %d localized string values and %d languages\n", source, len(messages), languageCount)

	// Create the source file.
	file, err := os.Create(source)
	if err != nil {
		panic(err)
	}

	// Write the digest and the header.
	fmt.Fprintf(file, "%s", digest)
	fmt.Fprintf(file, "package i18n\n\n")
	fmt.Fprintf(file, "// Code generated by lang tool at %s; DO NOT EDIT.\n\n", time.Now().Format(time.RFC1123Z))

	fmt.Fprintf(file, "var messages = map[string]map[string]string{\n")

	// Make alphabetically sorted list of the keys.
	keys := make([]string, len(messages))
	index := 0

	for key := range messages {
		keys[index] = key
		index++
	}

	sort.Strings(keys)

	// Scan over the messages map and write each element to the string constant,
	// using the sorted key list.
	for _, key := range keys {
		m := messages[key]
		fmt.Fprintf(file, "\t%q: {\n", key)

		// Make alphabetically sorted list of the languages. While
		// building the list, if any of the languages are identical to
		// the English language, then they can be omitted from the
		// output map.
		var langs []string

		for lang := range m {
			if lang != "en" {
				if messages[key][lang] == messages[key]["en"] {
					continue
				}
			}

			langs = append(langs, lang)
		}

		sort.Strings(langs)

		// Scan over the languages and write each element to the string constant,
		// using the sorted language list.
		for _, lang := range langs {
			message := m[lang]
			fmt.Fprintf(file, "\t\t%q: %q,\n", lang, message)
		}

		fmt.Fprintf(file, "\t},\n")
	}

	fmt.Fprintf(file, "}\n")

	file.Close()
}

func compileFiles(files []os.DirEntry, path string, messages map[string]map[string]string) {
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if !file.Type().IsRegular() {
			continue
		}

		if !strings.HasPrefix(file.Name(), "messages_") {
			continue
		}

		if !strings.HasSuffix(file.Name(), ".txt") {
			continue
		}

		lang := strings.TrimPrefix(file.Name(), "messages_")
		lang = strings.TrimSuffix(lang, ".txt")

		name := filepath.Join(path, file.Name())
		compileFile(name, lang, messages)
	}
}

func compileFile(filename, language string, messages map[string]map[string]string) {
	var prefix string

	if logging {
		fmt.Printf("Compiling %s\n", filename)
	}

	// Read the file into a buffer and split into lines separated by '\n'.
	b, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	_ = addFileToDigest(filename, b)

	// Split the file contents into an array of strings
	lines := strings.Split(string(b), "\n")

	for lineNumber, line := range lines {
		// Skip comments.
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Skip blank lines.
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Is it a new prefix key?
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				panic(fmt.Sprintf("%s:%d: Malformed prefix line\n", filename, lineNumber))
			}

			prefix = line[1 : len(line)-1]

			continue
		}

		// Split the line into the key and the message.
		i := strings.Index(line, "=")
		if i < 0 {
			panic(fmt.Sprintf("%s:%d: Malformed line\n", filename, lineNumber))
		}

		key := line[:i]
		message := line[i+1:]

		if prefix != "" {
			key = prefix + "." + key
		}

		// Create a map for the key if it doesn't already exist.
		if _, ok := messages[key]; !ok {
			messages[key] = make(map[string]string)
		}

		// See if this combination already exists. If so, warn the user!
		if msgGroup, ok := messages[key]; ok {
			if _, ok := msgGroup[language]; ok {
				fmt.Printf("%s:%d: Duplicate message for key '%s' in language '%s'\n",
					filename, lineNumber, key, language)
			}
		}

		// Let's do some simplistic validation fo the message string.
		test_message := strings.ReplaceAll(message, "'{'", "")
		test_message = strings.ReplaceAll(test_message, "'}", "")

		if strings.Count(test_message, "{") != strings.Count(test_message, "}") {
			fmt.Printf("%s:%d: Unmatched braces in message '%s'\n",
				filename, lineNumber, key)
		}
		// Add the message to the map.
		messages[key][language] = message
	}
}
