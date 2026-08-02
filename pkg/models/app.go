package models

import "time"

type Config struct {
	BaseURL       string `yaml:"base_url" json:"base_url"`
	Token         string `yaml:"token" json:"token"`
	OutputDir     string `yaml:"output_dir" json:"output_dir"`
	MaxWorkers    int    `yaml:"max_workers" json:"max_workers"`
	RateLimit     int    `yaml:"rate_limit" json:"rate_limit"`
	RetryCount    int    `yaml:"retry_count" json:"retry_count"`
	RetryDelay    string `yaml:"retry_delay" json:"retry_delay"`
	IncludeHints  bool   `yaml:"include_hints" json:"include_hints"`
	IncludeSolves bool   `yaml:"include_solves" json:"include_solves"`
}

func DefaultConfig() *Config {
	return &Config{
		OutputDir:     "./challenges",
		MaxWorkers:    5,
		RateLimit:     10,
		RetryCount:    3,
		RetryDelay:    "1s",
		IncludeHints:  false,
		IncludeSolves: false,
	}
}

type ChallengeMetadata struct {
	ID             int                    `yaml:"id" json:"id"`
	Name           string                 `yaml:"name" json:"name"`
	Description    string                 `yaml:"description" json:"description"`
	Category       string                 `yaml:"category" json:"category"`
	Value          int                    `yaml:"value" json:"value"`
	Tags           []string               `yaml:"tags" json:"tags"`
	Type           string                 `yaml:"type" json:"type"`
	State          string                 `yaml:"state" json:"state"`
	Author         *string                `yaml:"author,omitempty" json:"author,omitempty"`
	ConnectionInfo *string                `yaml:"connection_info,omitempty" json:"connection_info,omitempty"`
	MaxAttempts    int                    `yaml:"max_attempts" json:"max_attempts"`
	Files          []FileInfo             `yaml:"files" json:"files"`
	Hints          []HintInfo             `yaml:"hints,omitempty" json:"hints,omitempty"`
	Solves         []SolveInfo            `yaml:"solves,omitempty" json:"solves,omitempty"`
	DownloadedAt   time.Time              `yaml:"downloaded_at" json:"downloaded_at"`
	Metadata       map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type FileInfo struct {
	Name string `yaml:"name" json:"name"`
	URL  string `yaml:"url" json:"url"`
	Path string `yaml:"path" json:"path"`
	Size int64  `yaml:"size" json:"size"`
	SHA1 string `yaml:"sha1,omitempty" json:"sha1,omitempty"`
}

type HintInfo struct {
	ID      int     `yaml:"id" json:"id"`
	Title   string  `yaml:"title" json:"title"`
	Cost    int     `yaml:"cost" json:"cost"`
	Content *string `yaml:"content,omitempty" json:"content,omitempty"`
}

type SolveInfo struct {
	Team string    `yaml:"team" json:"team"`
	User string    `yaml:"user" json:"user"`
	Date time.Time `yaml:"date" json:"date"`
}

type DownloadStats struct {
	TotalChallenges int           `json:"total_challenges"`
	Downloaded      int           `json:"downloaded"`
	Skipped         int           `json:"skipped"`
	Failed          int           `json:"failed"`
	TotalFiles      int           `json:"total_files"`
	FilesDownloaded int           `json:"files_downloaded"`
	FilesFailed     int           `json:"files_failed"`
	TotalSize       int64         `json:"total_size"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	Duration        time.Duration `json:"duration"`
	Errors          []string      `json:"errors,omitempty"`
}

type DownloadResult struct {
	ChallengeID int      `json:"challenge_id"`
	Name        string   `json:"name"`
	Success     bool     `json:"success"`
	Skipped     bool     `json:"skipped"`
	Error       error    `json:"-"` // no useful JSON form
	Files       []string `json:"files"`
	OutputPath  string   `json:"output_path"`
}
