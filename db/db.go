package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type Result struct {
	WPM       float64
	Accuracy  float64
	Correct   int
	Total     int
	Mistakes  int
	TimeTaken float64
}

type CharStat struct {
	Char      rune
	Correct   int
	Incorrect int
	Accuracy  float64
}

func getDB(dbFile string) (*sql.DB, error) {
	err := os.MkdirAll(filepath.Dir(dbFile), 0755)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, err
	}

	query := `CREATE TABLE IF NOT EXISTS stats (
		id INTEGER NOT NULL PRIMARY KEY,
		wpm REAL,
		accuracy REAL,
		correct INTEGER,
		total INTEGER,
		mistakes INTEGER,
		time REAL,
		created DEFAULT CURRENT_TIMESTAMP
	)`
	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	query = `CREATE TABLE IF NOT EXISTS chars (
		char INT PRIMARY KEY,
		correct REAL,
		incorrect REAL,
		accuracy REAL
	)`
	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func GetAll(dbFile string) ([]*Result, []*CharStat, error) {
	db, err := getDB(dbFile)
	if err != nil {
		return nil, nil, err
	}

	query := `SELECT wpm, accuracy, correct, total, mistakes, time FROM stats`
	rows, err := db.Query(query)
	if err != nil {
		return nil, nil, err
	}
	var results []*Result
	for rows.Next() {
		var result Result
		err := rows.Scan(&result.WPM, &result.Accuracy, &result.Correct, &result.Total, &result.Mistakes, &result.TimeTaken)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, &result)
	}

	query = `SELECT char, correct, incorrect, accuracy FROM chars ORDER BY accuracy DESC`
	rows, err = db.Query(query)
	if err != nil {
		return nil, nil, err
	}
	var charStats []*CharStat
	for rows.Next() {
		var charStat CharStat
		err := rows.Scan(&charStat.Char, &charStat.Correct, &charStat.Incorrect, &charStat.Accuracy)
		if err != nil {
			return nil, nil, err
		}
		charStats = append(charStats, &charStat)
	}

	return results, charStats, nil
}

func Save(result Result, charStats map[rune]CharStat, dbFile string) error {
	db, err := getDB(dbFile)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO stats(wpm, accuracy, correct, total, mistakes, time) VALUES($1, $2, $3, $4, $5, $6)`
	_, err = tx.Exec(query, result.WPM, result.Accuracy, result.Correct, result.Total, result.Mistakes, result.TimeTaken)
	if err != nil {
		return err
	}

	query = `INSERT INTO chars(char, correct, incorrect) VALUES($1, $2, $3) ON CONFLICT(char) DO UPDATE SET correct=correct+$2, incorrect=incorrect+$3`
	statment, err := tx.Prepare(query)
	if err != nil {
		return err
	}
	defer statment.Close()
	for i, v := range charStats {
		_, err = statment.Exec(i, v.Correct, v.Incorrect)
		if err != nil {
			return err
		}
	}

	query = `UPDATE chars SET accuracy=correct/(correct+incorrect)*100`
	_, err = tx.Exec(query)
	if err != nil {
		return err
	}

	tx.Commit()
	return nil
}
