package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

var basePath, _ = filepath.Abs("./tests")

func deleteFile(file string) (err error) {
	err = os.Remove(file)

	if err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}

	return nil
}

func copyFile(src string, dst string) (err error) {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()
	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer df.Close()
	_, err = io.Copy(df, sf)
	return err
}

func Test_InitDb(t *testing.T) {
	dbPath := "tests/temp/initDb.db"
	_ = deleteFile(dbPath)

	db, err := openDb(dbPath)
	if err != nil {
		t.Error("could not create new database")
	}
	defer db.Close()

	err = updateSchema(db)
	if err != nil {
		t.Error("could not update schema")
	}
}

func Test_WordCount(t *testing.T) {

	annotationRegexp, _ := regexp.Compile("")
	count := countWords("tests/docs/250.txt", annotationRegexp)

	if count != 250 {
		t.Error(fmt.Sprintf("word count incorrect: expected 250, got %d", count))
	}

}

func Test_WordCountWithAnnotation(t *testing.T) {
	annotationRegexp, _ := regexp.Compile("#.*$")
	count := countWords("tests/docs/201_with_49_annotated.txt", annotationRegexp)

	if count != 201 {
		t.Error(fmt.Sprintf("word count incorrect: expected 201, got %d", count))
	}
}

func Test_PrevCount(t *testing.T) {
	dbPath := "tests/temp/prevCount.db"
	_ = deleteFile(dbPath)
	copyFile("tests/testBase.db", dbPath)
	db, _ := openDb(dbPath)
	defer db.Close()

	// docs/250.txt history
	// #|id|path            |words|timestamp
	// -+--+----------------+-----+---------
	// 2|2 |docs/250.txt    |50   |1
	// 3|3 |docs/250.txt    |100  |2
	// 4|4 |docs/250.txt    |150  |3
	// 5|5 |docs/250.txt    |200  |4

	// therefore the previous word count is *150*
	count, err := getPreviousWordCount(db, "docs/250.txt")
	if err != nil {
		t.Error(fmt.Sprintf("could not retrieve previous word count: %s", err))
	}
	if count != 150 {
		t.Error(fmt.Sprintf("prev count incorrect: expected 200, got %d", count))
	}

	// docs/201_with_49_annotated.txt history
	// |id|path                           |words|timestamp
	// -+--+------------------------------+-----+---------
	// 1|1 |docs/201_with_49_annotated.txt|201  |1

	// therefore the previous word count is 0
	count, err = getPreviousWordCount(db, "docs/201_with_49_annotated.txt")
	if err != nil {
		t.Error(fmt.Sprintf("could not retrieve previous word count: %s", err))
	}
	if count != 0 {
		t.Error(fmt.Sprintf("prev count incorrect: expected 0, got %d", count))
	}
}

func Test_AddWordCount(t *testing.T) {
	dbPath := "tests/temp/addWordCount.db"
	_ = deleteFile(dbPath)
	copyFile("tests/testBase.db", dbPath)
	db, _ := openDb(dbPath)
	defer db.Close()

	// docs/250.txt history
	// #|id|path            |words|timestamp
	// -+--+----------------+-----+---------
	// 2|2 |docs/250.txt    |50   |1
	// 3|3 |docs/250.txt    |100  |2
	// 4|4 |docs/250.txt    |150  |3
	// 5|5 |docs/250.txt    |200  |4

	err := addWordCount(db, "docs/250.txt", 250)
	// 6|5 |docs/250.txt    |250  |<now>
	// therefore the previous word count is *200*

	if err != nil {
		t.Error(fmt.Sprintf("could not add word count: %s", err))
	}

	count, err := getPreviousWordCount(db, "docs/250.txt")
	if err != nil {
		t.Error(fmt.Sprintf("could not retrieve previous word count: %s", err))
	}
	if count != 200 {
		t.Error(fmt.Sprintf("prev count incorrect: expected 200, got %d", count))
	}
}

func Test_PreviousDayWordCount(t *testing.T) {
	dbPath := "tests/temp/previousDayWordCount.db"
	_ = deleteFile(dbPath)
	copyFile("tests/testBase.db", dbPath)
	db, _ := openDb(dbPath)
	defer db.Close()

	// docs/250.txt history
	// #|id|path            |words|timestamp
	// -+--+----------------+-----+---------
	// 2|2 |docs/250.txt    |50   |1
	// 3|3 |docs/250.txt    |100  |2
	// 4|4 |docs/250.txt    |150  |3
	// 5|5 |docs/250.txt    |200  |4

	err := addWordCount(db, "docs/250.txt", 250)
	// 6|5 |docs/250.txt    |250  |<now>

	// we need to wait as timestamp + path has a unique constraint (to keep the db less cluttered)
	time.Sleep(1 * time.Second)
	err = addWordCount(db, "docs/250.txt", 300)
	// 7|5 |docs/250.txt    |300  |<now>

	// therefore the previous day's word count is *200*
	count, err := getPreviousDayWordCount(db, "docs/250.txt")
	if err != nil {
		t.Error(fmt.Sprintf("could not retrieve previous day's word count: %s", err))
	}
	if count != 200 {
		t.Error(fmt.Sprintf("prev day's count incorrect: expected 200, got %d", count))
	}

	// and the previous word count is *250*
	count, err = getPreviousWordCount(db, "docs/250.txt")
	if err != nil {
		t.Error(fmt.Sprintf("could not retrieve previous word count: %s", err))
	}
	if count != 250 {
		t.Error(fmt.Sprintf("prev count incorrect: expected 250, got %d", count))
	}
}

func Test_DocumentLoad(t *testing.T) {
	dbPath := "tests/temp/updateCount.db"
	_ = deleteFile(dbPath)
	copyFile("tests/testBase.db", dbPath)
	db, _ := openDb(dbPath)
	defer db.Close()
	_ = updateSchema(db)
	var files = make(DocumentMap)
	annotationRegexp, _ := regexp.Compile("")
	ignoreFileRegexp, _ := regexp.Compile("$^")

	err := countFile("tests/docs/250.txt", basePath, db, annotationRegexp, ignoreFileRegexp, files)
	if err != nil {
		t.Error(fmt.Sprintf("countFile failed: %s", err))
	}

	var doc = files["docs/250.txt"]
	count := doc.Words

	if count != 250 {
		t.Error(fmt.Sprintf("word count incorrect: expected 250, got %d", count))
	}
}
