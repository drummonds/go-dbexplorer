package dbexplorer_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	dbexplorer "git.bytestone.uk/hum3/go-dbexplorer"
	_ "git.bytestone.uk/hum3/go-postgres"
)

// TestWASMSoakDataSize (moved from lofidb) measures how much data fits in a
// pglike :memory: DB and that the explorer still renders it
// when running under WASM (wasip1). It writes JSON-line metrics so the
// results can be plotted or compared across releases.
//
// Env vars:
//
//	WASM_SOAK_ROWS    target row count (default 100000)
//	WASM_SOAK_BATCH   rows per INSERT batch (default 1000)
//	WASM_SOAK_OUTPUT  JSON-lines output file (default: stdout via t.Log)
//
// Run native:
//
//	WASM_SOAK_ROWS=200000 go test -run TestWASMSoakDataSize -v -timeout 5m .
//
// Run under wasip1/wasm via wazero:
//
//	task test:wasm:soak WASM_SOAK_ROWS=200000
//
// The test does not fail on OOM — instead it records the last successful
// batch and how it terminated. Failures are reserved for unexpected errors.
func TestWASMSoakDataSize(t *testing.T) {
	target := envInt("WASM_SOAK_ROWS", 100_000)
	batch := envInt("WASM_SOAK_BATCH", 1000)
	output := os.Getenv("WASM_SOAK_OUTPUT")

	var emit func(metric)
	if output == "" {
		emit = func(m metric) { t.Log(m.line()) }
	} else {
		f, err := os.Create(output)
		if err != nil {
			t.Fatalf("open %s: %v", output, err)
		}
		t.Cleanup(func() { f.Close() })
		emit = func(m metric) {
			fmt.Fprintln(f, m.line())
			t.Log(m.line())
		}
	}

	db, err := sql.Open("pglike", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE rows (
		id INTEGER PRIMARY KEY,
		k TEXT NOT NULL,
		v TEXT NOT NULL,
		created_at TEXT
	)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	// The payload size matters more than column count for memory pressure.
	// 100 bytes/row × 100k rows = ~10 MiB raw; SQLite overhead and query
	// translation will inflate this in WASM.
	payload := strings.Repeat("x", 80)

	emit(metric{
		Stage:        "start",
		TargetRows:   target,
		BatchSize:    batch,
		WallSecs:     0,
		HeapAllocMB:  heapMB(),
		BatchLatency: 0,
	})

	start := time.Now()
	inserted := 0
	for inserted < target {
		n := batch
		if inserted+n > target {
			n = target - inserted
		}
		bStart := time.Now()
		err := insertBatch(db, inserted, n, payload)
		bDur := time.Since(bStart)
		if err != nil {
			emit(metric{
				Stage:        "error",
				TargetRows:   target,
				InsertedRows: inserted,
				WallSecs:     time.Since(start).Seconds(),
				HeapAllocMB:  heapMB(),
				BatchLatency: bDur.Seconds(),
				Err:          err.Error(),
			})
			t.Fatalf("insert at row %d: %v", inserted, err)
		}
		inserted += n

		// Snapshot every 10 batches to keep output manageable.
		if (inserted/batch)%10 == 0 {
			emit(metric{
				Stage:        "progress",
				TargetRows:   target,
				InsertedRows: inserted,
				WallSecs:     time.Since(start).Seconds(),
				HeapAllocMB:  heapMB(),
				BatchLatency: bDur.Seconds(),
			})
		}
	}

	// Verify the data is queryable end-to-end after insertion.
	var got int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM rows`).Scan(&got); err != nil {
		t.Fatalf("count: %v", err)
	}
	if int(got) != target {
		t.Errorf("count = %d, want %d", got, target)
	}

	// The explorer must still browse a table this size.
	ex := &dbexplorer.Explorer{DB: db}
	if idx := ex.IndexHTML(t.Context()); !strings.Contains(idx, "rows") {
		t.Errorf("IndexHTML does not list the rows table")
	}
	if page := ex.TableHTML(t.Context(), "rows", 1, "id", "asc", false); !strings.Contains(page, "k-0000000000") {
		t.Errorf("TableHTML page 1 does not show the first row")
	}

	emit(metric{
		Stage:        "done",
		TargetRows:   target,
		InsertedRows: inserted,
		WallSecs:     time.Since(start).Seconds(),
		HeapAllocMB:  heapMB(),
	})
}

// insertBatch writes n rows starting at id offset, using one parameterised
// statement per row inside a single transaction. Reusing one prepared
// statement keeps translation overhead out of the loop.
func insertBatch(db *sql.DB, offset, n int, payload string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO rows (id, k, v, created_at) VALUES ($1, $2, $3, $4)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	for i := range n {
		id := offset + i
		k := fmt.Sprintf("k-%010d", id)
		if _, err := stmt.Exec(id, k, payload, time.Now().Format(time.RFC3339)); err != nil {
			stmt.Close()
			tx.Rollback()
			return err
		}
	}
	stmt.Close()
	return tx.Commit()
}

// metric is one line in the JSON-lines output.
type metric struct {
	Stage        string  `json:"stage"`
	TargetRows   int     `json:"target_rows,omitempty"`
	InsertedRows int     `json:"inserted_rows,omitempty"`
	BatchSize    int     `json:"batch_size,omitempty"`
	WallSecs     float64 `json:"wall_secs"`
	HeapAllocMB  float64 `json:"heap_alloc_mb"`
	BatchLatency float64 `json:"batch_latency_secs,omitempty"`
	Err          string  `json:"err,omitempty"`
}

func (m metric) line() string {
	b, _ := json.Marshal(m)
	return string(b)
}

func heapMB() float64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return float64(ms.HeapAlloc) / 1024.0 / 1024.0
}

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
