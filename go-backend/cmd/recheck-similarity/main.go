package main

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type parseRow struct {
	uploadID int64
	simhash  int64
	taskID   int64
}

func hamming(a, b uint64) int {
	xor := a ^ b
	dist := 0
	for xor > 0 {
		dist++
		xor &= xor - 1
	}
	return dist
}

func main() {
	dbPath := filepath.Join(".", "data", "app.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT pr.upload_id, pr.simhash, u.task_id
		FROM parse_results pr
		JOIN uploads u ON pr.upload_id = u.id
		WHERE pr.simhash IS NOT NULL
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var all []parseRow
	for rows.Next() {
		var r parseRow
		if err := rows.Scan(&r.uploadID, &r.simhash, &r.taskID); err != nil {
			continue
		}
		all = append(all, r)
	}
	fmt.Printf("Total parse results with simhash: %d\n", len(all))

	// Group by task
	byTask := map[int64][]parseRow{}
	for _, r := range all {
		byTask[r.taskID] = append(byTask[r.taskID], r)
	}

	// Find pairs with hamming <= 3
	type pair struct {
		taskID  int64
		aID     int64
		bID     int64
		hamming int
		cosine  float64
	}
	var suspects []pair
	for tid, items := range byTask {
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				dist := hamming(uint64(items[i].simhash), uint64(items[j].simhash))
				if dist <= 3 {
					aID, bID := items[i].uploadID, items[j].uploadID
					if aID > bID {
						aID, bID = bID, aID
					}
					cosine := 1.0 - float64(dist)/64.0
					suspects = append(suspects, pair{tid, aID, bID, dist, cosine})
				}
			}
		}
	}

	fmt.Printf("Suspect pairs (hamming <= 3): %d\n", len(suspects))
	for i, s := range suspects {
		if i >= 10 {
			break
		}
		fmt.Printf("  task=%d a=%d b=%d hamming=%d cosine=%.4f\n", s.taskID, s.aID, s.bID, s.hamming, s.cosine)
	}

	// Also count pairs with hamming <= 5 and <= 8 for comparison
	count5, count8 := 0, 0
	for _, items := range byTask {
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				dist := hamming(uint64(items[i].simhash), uint64(items[j].simhash))
				if dist <= 5 {
					count5++
				}
				if dist <= 8 {
					count8++
				}
			}
		}
	}
	fmt.Printf("For comparison: hamming<=5: %d, hamming<=8: %d\n", count5, count8)

	// Insert suspect records
	if len(suspects) > 0 {
		stmt, err := db.Prepare(`INSERT INTO similarity_records (task_id, upload_a_id, upload_b_id, hamming_distance, cosine_similarity, state) VALUES (?, ?, ?, ?, ?, 'suspect')`)
		if err != nil {
			fmt.Fprintf(os.Stderr, "prepare: %v\n", err)
			os.Exit(1)
		}
		defer stmt.Close()

		inserted := 0
		for _, s := range suspects {
			cosine := math.Round(s.cosine*1e6) / 1e6
			if _, err := stmt.Exec(s.taskID, s.aID, s.bID, s.hamming, cosine); err == nil {
				inserted++
			}
		}
		fmt.Printf("Inserted %d similarity records\n", inserted)
	}
}
