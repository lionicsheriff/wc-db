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
	documentBase       = "."
	documentPath       = []string{"."}
	databasePath       = "./wc.db"
	headerFormat       = "Total: #{total} Today: #{today}#{goal}"
	goalFormat         = " Goal: #{target}(#{remaining})"
	itemFormat         = "#{path}: #{total} (#{today})"
	annotationPattern  = `#.*$`
	goal               = 0
	extensionBlacklist = []string{}
	extensionWhitelist = []string{}
)

func init() {
	flag.StringVar(&documentBase, "base", documentBase, "TODO")
	flag.StringVar(&documentBase, "b", documentBase, "TODO")

	flag.StringVar(&databasePath, "database", databasePath, "Path to database")
	flag.StringVar(&databasePath, "d", databasePath, "Path to database")

	flag.IntVar(&goal, "goal", goal, "TODO")
	flag.IntVar(&goal, "g", goal, "TODO")

	flag.StringVar(&annotationPattern, "annotation-pattern", annotationPattern, "TODO")
	flag.StringVar(&annotationPattern, "a", annotationPattern, "TODO")

	flag.StringVar(&headerFormat, "format-header", headerFormat, "TODO")
	flag.StringVar(&itemFormat, "format-item", itemFormat, "TODO")

}

func countAll(documents string, base string, db *sql.DB, annotationRegexp *regexp.Regexp, files DocumentMap) error {
	return filepath.Walk(documents, func(path string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		countFile(path, base, db, annotationRegexp, files)
		return nil
	})
}

func countFile(path string, base string, db *sql.DB, annotationRegexp *regexp.Regexp, files DocumentMap) (err error) {
	words := countWords(path, annotationRegexp)
	prev, err := getPreviousWordCount(db, path)
	if err != nil {
		return
	}
	yesterday, err := getPreviousDayWordCount(db, path)
	if err != nil {
		return
	}

	abs_path, err := filepath.Abs(path)
	if err != nil {
		return
	}

	rel_path, err := filepath.Rel(base, abs_path)
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

	basePath, _ := filepath.Abs(filepath.Dir(databasePath))

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
			countAll(path, basePath, db, annotationRegexp, files)
		} else {
			countFile(path, basePath, db, annotationRegexp, files)
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
