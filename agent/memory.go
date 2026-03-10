package agent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Memory provides short-term (in-process) and long-term (SQLite) memory for the agent.
type Memory struct {
	db *sql.DB
}

// NewMemory initialises an in-memory SQLite DB for this session.
// Call NewMemoryOnDisk(path) for persistent long-term memory.
func NewMemory() *Memory {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic("crabpath/memory: " + err.Error())
	}
	m := &Memory{db: db}
	m.migrate()
	return m
}

// NewMemoryOnDisk opens (or creates) a persistent SQLite file.
func NewMemoryOnDisk(path string) (*Memory, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("crabpath/memory: open %s: %w", path, err)
	}
	m := &Memory{db: db}
	m.migrate()
	return m, nil
}

func (m *Memory) migrate() {
	_, _ = m.db.Exec(`CREATE TABLE IF NOT EXISTS paths (
		id TEXT PRIMARY KEY,
		goal TEXT NOT NULL,
		status TEXT NOT NULL,
		answer TEXT,
		steps_json TEXT,
		started_at INTEGER,
		ended_at INTEGER
	)`)

	_, _ = m.db.Exec(`CREATE TABLE IF NOT EXISTS facts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT UNIQUE NOT NULL,
		value TEXT NOT NULL,
		updated_at INTEGER
	)`)
}

// SavePath persists a completed CrabPath to long-term storage.
func (m *Memory) SavePath(p *CrabPath) error {
	stepsJSON, _ := json.Marshal(p.Steps)
	var endedAt *int64
	if p.EndedAt != nil {
		t := p.EndedAt.Unix()
		endedAt = &t
	}
	_, err := m.db.Exec(`INSERT OR REPLACE INTO paths
		(id, goal, status, answer, steps_json, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Goal, string(p.Status), p.Answer,
		string(stepsJSON), p.StartedAt.Unix(), endedAt,
	)
	return err
}

// ListPaths returns the most recent N paths from memory.
func (m *Memory) ListPaths(limit int) ([]CrabPath, error) {
	rows, err := m.db.Query(
		`SELECT id, goal, status, answer, started_at FROM paths ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []CrabPath
	for rows.Next() {
		var p CrabPath
		var ts int64
		if err := rows.Scan(&p.ID, &p.Goal, &p.Status, &p.Answer, &ts); err != nil {
			continue
		}
		p.StartedAt = time.Unix(ts, 0)
		paths = append(paths, p)
	}
	return paths, nil
}

// SetFact stores a key-value pair for long-term recall.
func (m *Memory) SetFact(key, value string) error {
	_, err := m.db.Exec(
		`INSERT OR REPLACE INTO facts (key, value, updated_at) VALUES (?, ?, ?)`,
		key, value, time.Now().Unix(),
	)
	return err
}

// GetFact retrieves a stored fact.
func (m *Memory) GetFact(key string) (string, bool, error) {
	var value string
	err := m.db.QueryRow(`SELECT value FROM facts WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return value, err == nil, err
}
