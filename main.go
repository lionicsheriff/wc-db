package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Document struct {
	Path      string
	Words     int
	Prev      int
	Yesterday int
}
type DocumentMap map[string]Document

func (document *Document) Today() int {

	return document.Words - document.Yesterday
}

var (
	documentPath       = []string{"."}
	databasePath       = "./wc.db"
	headerFormat       = "Total: #{total} Today: #{today}#{goal}"
	goalFormat         = " Goal: #{target}(#{remaining})"
	itemFormat         = "#{path}: #{total} (#{today})"
	annotationPattern  = `#.*$`
	ignoreFilePattern  = ""
	goal               = 0
	extensionBlacklist = []string{}
	extensionWhitelist = []string{}
)

func init() {
	flag.Usage = func() {
		executable := filepath.Base(os.Args[0])
		fmt.Printf("Usage: %s [options] [path to documents...]:\n\n", executable)
		flag.PrintDefaults()
	}

	flag.StringVar(&databasePath, "database", databasePath, "Path to database")
	flag.StringVar(&databasePath, "d", databasePath, "alias for --database")

	flag.IntVar(&goal, "goal", goal, "Number of words for daily goal")
	flag.IntVar(&goal, "g", goal, "alias for --goal")

	flag.StringVar(&annotationPattern, "annotation-pattern", annotationPattern, "Regexp for lines that don't count towards the total")
	flag.StringVar(&annotationPattern, "a", annotationPattern, "alias for --anotation-pattern")

	flag.StringVar(&ignoreFilePattern, "ignore-file-pattern", ignoreFilePattern, "Regexp for file names to ignore")

	flag.StringVar(&headerFormat, "format-header", headerFormat, "Format for header line")
	flag.StringVar(&itemFormat, "format-item", itemFormat, "Format for item line")
}

func countAll(documents string, base string, db *sql.DB, annotationRegexp *regexp.Regexp, skipRegexp *regexp.Regexp, files DocumentMap) error {
	return filepath.Walk(documents, func(path string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		countFile(path, base, db, annotationRegexp, skipRegexp, files)
		return nil
	})
}

func countFile(path string, base string, db *sql.DB, annotationRegexp *regexp.Regexp, skipRegexp *regexp.Regexp, files DocumentMap) (err error) {
	if ignoreFilePattern != "" && skipRegexp.MatchString(path) {
		return
	}

	abs_path, err := filepath.Abs(path)
	if err != nil {
		return
	}

	if abs_path == databasePath {
		return
	}

	rel_path, err := filepath.Rel(base, abs_path)
	if err != nil {
		return
	}

	words := countWords(abs_path, annotationRegexp)

	prev, err := getPreviousWordCount(db, rel_path)
	if err != nil {
		return
	}

	yesterday, err := getPreviousDayWordCount(db, rel_path)
	if err != nil {
		return
	}

	if words != prev {
		addWordCount(db, rel_path, words)
	}

	files[rel_path] = Document{Path: rel_path, Words: words, Prev: prev, Yesterday: yesterday}
	return
}

func countWords(path string, annotationRegexp *regexp.Regexp) int {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		line = annotationRegexp.ReplaceAllString(line, "")
		count += len(strings.Fields(line))
	}
	if err := scanner.Err(); err != nil {
		log.Println("error reading ", path, ":", err)
	}
	return count
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) > 0 {
		documentPath = args
	}

	annotationRegexp, err := regexp.Compile(annotationPattern)
	if err != nil {
		log.Fatal("Bad annotation pattern")
	}

	ignoreFileRegexp, err := regexp.Compile(ignoreFilePattern)
	if err != nil {
		log.Fatal("Bad file ignore pattern")
	}

	databasePath, err = filepath.Abs(databasePath) // switch databasePath to an absolute path. Makes it easier to deal with.
	if err != nil {
		log.Fatal("Bad database path")
	}
	basePath := filepath.Dir(databasePath) // used to calculate the documents location relative to the database

	db, err := openDb(databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = updateSchema(db)
	if err != nil {
		log.Fatal(err)
	}

	var files = make(DocumentMap)
	for _, path := range documentPath {
		fileMode, err := os.Stat(path)
		if err != nil {
			log.Println(err)
		} else if fileMode.IsDir() {
			countAll(path, basePath, db, annotationRegexp, ignoreFileRegexp, files)
		} else {
			countFile(path, basePath, db, annotationRegexp, ignoreFileRegexp, files)
		}
	}

	if headerFormat != "" {
		today := 0
		total := 0
		for _, document := range files {
			today += document.Today()
			total += document.Words
		}
		headerOutput := headerFormat
		headerOutput = strings.Replace(headerOutput, "#{total}", strconv.Itoa(total), -1)
		headerOutput = strings.Replace(headerOutput, "#{today}", strconv.Itoa(today), -1)

		if goal > 0 {
			goalOutput := goalFormat
			goalOutput = strings.Replace(goalOutput, "#{target}", strconv.Itoa(goal), -1)
			goalOutput = strings.Replace(goalOutput, "#{remaining}", strconv.Itoa(goal-today), -1)
			headerOutput = strings.Replace(headerOutput, "#{goal}", goalOutput, -1)
		} else {
			headerOutput = strings.Replace(headerOutput, "#{goal}", "", -1)
		}
		fmt.Println(headerOutput)
	}

	if itemFormat != "" {
		for _, document := range files {
			itemOutput := itemFormat
			itemOutput = strings.Replace(itemOutput, "#{path}", document.Path, -1)
			itemOutput = strings.Replace(itemOutput, "#{total}", strconv.Itoa(document.Words), -1)
			itemOutput = strings.Replace(itemOutput, "#{prev}", strconv.Itoa(document.Prev), -1)
			itemOutput = strings.Replace(itemOutput, "#{today}", strconv.Itoa(document.Today()), -1)
			fmt.Println(itemOutput)
		}
	}
}
