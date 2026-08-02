package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"ctfd-downloader/pkg/client"
	"ctfd-downloader/pkg/models"
	"ctfd-downloader/pkg/services"
	"ctfd-downloader/pkg/utils"

	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"gopkg.in/yaml.v3"
)

const (
	version = "1.0.0"
	appName = "CTFd Challenge Downloader"
)

var (
	baseURL       = flag.String("url", "", "CTFd base URL (required)")
	token         = flag.String("token", "", "CTFd access token (required)")
	outputDir     = flag.String("output", "./challenges", "Output directory for challenges")
	configFile    = flag.String("config", "", "Configuration file path")
	workers       = flag.Int("workers", 5, "Number of concurrent workers")
	rateLimit     = flag.Int("rate-limit", 10, "Rate limit (requests per second)")
	retryCount    = flag.Int("retry", 3, "Number of retry attempts")
	retryDelay    = flag.String("retry-delay", "1s", "Delay between retries")
	includeHints  = flag.Bool("hints", false, "Include challenge hints")
	includeSolves = flag.Bool("solves", false, "Include challenge solves")
	skipExisting  = flag.Bool("skip-existing", true, "Skip existing challenges")
	overwrite     = flag.Bool("overwrite", false, "Overwrite existing files")
	verbose       = flag.Bool("verbose", false, "Enable verbose logging")
	showVersion   = flag.Bool("version", false, "Show version information")
	testConn      = flag.Bool("test", false, "Test connection and authentication")
	dryRun        = flag.Bool("dry-run", false, "Show what would be downloaded without actually downloading")
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s v%s\n", appName, version)
		os.Exit(0)
	}

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// use merged value so config-file retry_delay isn't ignored
	retryDelayDuration, _ := time.ParseDuration(config.RetryDelay)

	if *verbose {
		log.Printf("Configuration: %+v", config)
	}

	clientConfig := &client.ClientConfig{
		BaseURL:    config.BaseURL,
		Token:      config.Token,
		Timeout:    30 * time.Second,
		RateLimit:  config.RateLimit,
		RetryCount: config.RetryCount,
		RetryDelay: retryDelayDuration,
		Debug:      *verbose,
	}

	ctfdClient, err := client.NewCTFdClient(clientConfig)
	if err != nil {
		log.Fatalf("Failed to create CTFd client: %v", err)
	}

	if *testConn {
		log.Println("Testing connection...")
		if err := ctfdClient.TestConnection(); err != nil {
			log.Fatalf("Connection test failed: %v", err)
		}
		log.Println("Connection test successful!")
		return
	}

	filesystem := services.NewFileSystemService(config.OutputDir)

	downloadConfig := &services.DownloadConfig{
		MaxWorkers:     config.MaxWorkers,
		RateLimit:      config.RateLimit,
		RetryCount:     config.RetryCount,
		RetryDelay:     retryDelayDuration,
		IncludeHints:   config.IncludeHints,
		IncludeSolves:  config.IncludeSolves,
		SkipExisting:   *skipExisting,
		OverwriteFiles: *overwrite,
		FileWorkers:    3,
	}

	downloadService := services.NewDownloadService(ctfdClient, filesystem, downloadConfig)

	if *dryRun {
		if err := performDryRun(ctfdClient, config.OutputDir); err != nil {
			log.Fatalf("Dry run failed: %v", err)
		}
		return
	}

	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("CTFd Downloader  ·  %s  ·  %s\n\n", config.BaseURL, config.OutputDir)

	rows, err := runWithDashboard(ctx, downloadService)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("Cancelled.")
			os.Exit(130)
		}
		log.Fatalf("Download failed: %v", err)
	}

	stats := downloadService.GetStats()

	renderResultsTable(rows)
	renderStatsTable(stats)

	if len(stats.Errors) > 0 {
		fmt.Printf("\n%d error(s) during download:\n", len(stats.Errors))
		if *verbose {
			for i, errMsg := range stats.Errors {
				fmt.Printf("  %d. %s\n", i+1, errMsg)
			}
		} else {
			fmt.Println("  (run with -verbose to list them)")
		}
	}

	promptTarball(ctx, config.OutputDir)

	if stats.Failed > 0 {
		os.Exit(1)
	}
}

// promptTarball offers to bundle the whole output dir into a .tar.gz to share
// with teammates. It stays interruptible: a SIGINT (ctx cancel) aborts the wait.
func promptTarball(ctx context.Context, outputDir string) {
	fmt.Print("\nTarball all challenges to send to your team? [y/N] ")

	answerCh := make(chan string, 1)
	go func() {
		a, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answerCh <- a
	}()

	var answer string
	select {
	case answer = <-answerCh:
	case <-ctx.Done():
		fmt.Println()
		return
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
	default:
		return
	}

	name := fmt.Sprintf("%s-%s.tar.gz", filepath.Base(filepath.Clean(outputDir)), time.Now().Format("20060102-150405"))
	if err := createTarball(outputDir, name); err != nil {
		fmt.Printf("Failed to create tarball: %v\n", err)
		return
	}
	fmt.Printf("Wrote %s\n", name)
}

func createTarball(srcDir, outFile string) (err error) {
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	walkErr := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil || rel == "." {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(tw, src)
		return err
	})
	if walkErr != nil {
		tw.Close()
		gz.Close()
		return walkErr
	}
	if err := tw.Close(); err != nil {
		gz.Close()
		return err
	}
	return gz.Close()
}

// runWithDashboard drives the download while rendering a live progress bar and
// returns the per-challenge results plus the download error (if any).
func runWithDashboard(ctx context.Context, ds *services.DownloadService) ([]models.DownloadResult, error) {
	pw := progress.NewWriter()
	pw.SetAutoStop(false)
	pw.SetTrackerLength(30)
	pw.SetMessageLength(20)
	pw.SetNumTrackersExpected(1)
	pw.SetUpdateFrequency(80 * time.Millisecond)
	pw.SetStyle(progress.StyleBlocks)
	pw.Style().Colors = progress.StyleColorsExample
	pw.Style().Visibility.ETA = true
	pw.Style().Visibility.Value = true

	tracker := &progress.Tracker{Message: "Downloading", Units: progress.UnitsDefault}

	// Start rendering only once the total is known (first hook, done==0), so the
	// bar never shows "???" during the initial challenge-list fetch.
	var rows []models.DownloadResult // appended from the drain goroutine only
	started := false
	ds.SetProgressHook(func(done, total int, r models.DownloadResult) {
		if !started {
			started = true
			tracker.UpdateTotal(int64(total))
			pw.AppendTracker(tracker)
			go pw.Render()
		}
		if done == 0 {
			return
		}
		tracker.Increment(1)
		rows = append(rows, r)
	})

	_, err := ds.DownloadAllChallenges(ctx)

	if err != nil {
		tracker.MarkAsErrored()
	} else {
		tracker.MarkAsDone()
	}

	if started {
		time.Sleep(120 * time.Millisecond) // let the final frame flush
	}
	pw.Stop()
	for pw.IsRenderInProgress() {
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println()

	return rows, err
}

// styledTable returns a rounded table with bold cyan title, bold header and
// dimmed borders — the shared look for both summary tables.
func styledTable(title string) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	t.SetTitle(title)
	st := t.Style()
	st.Title.Colors = text.Colors{text.Bold, text.FgHiCyan}
	st.Title.Align = text.AlignCenter
	st.Color.Header = text.Colors{text.Bold, text.FgHiWhite}
	st.Color.Border = text.Colors{text.FgHiBlack}
	st.Color.Separator = text.Colors{text.FgHiBlack}
	return t
}

func renderResultsTable(rows []models.DownloadResult) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	t := styledTable("Challenges")
	t.AppendHeader(table.Row{"#", "Challenge", "Files", "Status"})
	for i, r := range rows {
		var status string
		switch {
		case !r.Success:
			status = text.Colors{text.FgHiRed, text.Bold}.Sprint("FAILED")
		case r.Skipped:
			status = text.Colors{text.FgHiYellow}.Sprint("SKIPPED")
		default:
			status = text.Colors{text.FgHiGreen}.Sprint("OK")
		}
		t.AppendRow(table.Row{i + 1, r.Name, len(r.Files), status})
	}
	t.Render()
}

func renderStatsTable(s *models.DownloadStats) {
	failColor := text.Colors{text.FgHiGreen}
	if s.Failed > 0 {
		failColor = text.Colors{text.FgHiRed, text.Bold}
	}

	t := styledTable("Summary")
	t.AppendRows([]table.Row{
		{"Total challenges", text.Colors{text.Bold}.Sprint(s.TotalChallenges)},
		{"Downloaded", text.Colors{text.FgHiGreen}.Sprint(s.Downloaded)},
		{"Skipped", text.Colors{text.FgHiYellow}.Sprint(s.Skipped)},
		{"Failed", failColor.Sprint(s.Failed)},
		{"Files downloaded", s.FilesDownloaded},
		{"Total size", text.Colors{text.FgHiCyan}.Sprint(utils.FormatBytes(s.TotalSize))},
		{"Duration", s.Duration.Round(time.Millisecond)},
	})
	if s.Duration > 0 && s.FilesDownloaded > 0 {
		avg := float64(s.TotalSize) / s.Duration.Seconds()
		t.AppendRow(table.Row{"Average speed", text.Colors{text.FgHiCyan}.Sprint(utils.FormatBytes(int64(avg)) + "/s")})
	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Colors: text.Colors{text.Bold}},
		{Number: 2, Align: text.AlignRight},
	})
	t.Render()
}

func loadConfig() (*models.Config, error) {
	config := models.DefaultConfig()

	if *configFile != "" {
		fileConfig, err := loadConfigFile(*configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
		mergeConfigs(config, fileConfig)
	}

	// flags override config only when explicitly set
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["url"] {
		config.BaseURL = *baseURL
	}
	if set["token"] {
		config.Token = *token
	}
	if set["output"] {
		config.OutputDir = *outputDir
	}
	if set["workers"] {
		config.MaxWorkers = *workers
	}
	if set["rate-limit"] {
		config.RateLimit = *rateLimit
	}
	if set["retry"] {
		config.RetryCount = *retryCount
	}
	if set["retry-delay"] {
		config.RetryDelay = *retryDelay
	}
	if set["hints"] {
		config.IncludeHints = *includeHints
	}
	if set["solves"] {
		config.IncludeSolves = *includeSolves
	}

	if config.BaseURL == "" {
		if envURL := os.Getenv("CTFD_URL"); envURL != "" {
			config.BaseURL = envURL
		}
	}
	if config.Token == "" {
		if envToken := os.Getenv("CTFD_TOKEN"); envToken != "" {
			config.Token = envToken
		}
	}

	return config, nil
}

func loadConfigFile(filename string) (*models.Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func mergeConfigs(base, override *models.Config) {
	if override.BaseURL != "" {
		base.BaseURL = override.BaseURL
	}
	if override.Token != "" {
		base.Token = override.Token
	}
	if override.OutputDir != "" {
		base.OutputDir = override.OutputDir
	}
	if override.MaxWorkers != 0 {
		base.MaxWorkers = override.MaxWorkers
	}
	if override.RateLimit != 0 {
		base.RateLimit = override.RateLimit
	}
	if override.RetryCount != 0 {
		base.RetryCount = override.RetryCount
	}
	if override.RetryDelay != "" {
		base.RetryDelay = override.RetryDelay
	}
	if override.IncludeHints {
		base.IncludeHints = override.IncludeHints
	}
	if override.IncludeSolves {
		base.IncludeSolves = override.IncludeSolves
	}
}

func validateConfig(config *models.Config) error {
	if config.BaseURL == "" {
		return fmt.Errorf("CTFd base URL is required (use -url flag or CTFD_URL environment variable)")
	}
	if config.Token == "" {
		return fmt.Errorf("CTFd access token is required (use -token flag or CTFD_TOKEN environment variable)")
	}
	if config.MaxWorkers < 1 {
		return fmt.Errorf("number of workers must be at least 1")
	}
	if config.RateLimit < 1 {
		return fmt.Errorf("rate limit must be at least 1")
	}
	if config.RetryCount < 0 {
		return fmt.Errorf("retry count cannot be negative")
	}

	if _, err := time.ParseDuration(config.RetryDelay); err != nil {
		return fmt.Errorf("invalid retry delay format: %w", err)
	}

	return nil
}

func performDryRun(ctfdClient *client.CTFdClient, outputDir string) error {
	log.Println("Performing dry run...")

	challenges, err := ctfdClient.GetChallenges()
	if err != nil {
		return fmt.Errorf("failed to fetch challenges: %w", err)
	}

	fmt.Printf("\nFound %d challenges:\n\n", len(challenges))

	categoryCounts := make(map[string]int)
	totalFiles := 0

	for i, challenge := range challenges {
		detailed, err := ctfdClient.GetChallenge(challenge.ID)
		if err != nil {
			log.Printf("Warning: Could not get details for challenge %s: %v", challenge.Name, err)
			continue
		}

		fileCount := len(detailed.Files)
		totalFiles += fileCount

		categoryCounts[challenge.Category]++

		fmt.Printf("%d. %s (%s) - %d points", i+1, challenge.Name, challenge.Category, challenge.Value)
		if fileCount > 0 {
			fmt.Printf(" [%d files]", fileCount)
		}
		if challenge.SolvedByMe {
			fmt.Printf(" ✓")
		}
		fmt.Println()
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("Total challenges: %d\n", len(challenges))
	fmt.Printf("Total files: %d\n", totalFiles)
	fmt.Printf("\nBy category:\n")

	for category, count := range categoryCounts {
		fmt.Printf("  %s: %d\n", category, count)
	}

	fmt.Printf("\nDirectory structure that would be created:\n")
	fmt.Printf("%s/\n", outputDir)
	for category := range categoryCounts {
		fmt.Printf("├── %s/\n", utils.SanitizeName(category))
	}

	return nil
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s v%s\n\n", appName, version)
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Downloads challenges from a CTFd platform with concurrent processing.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -url https://ctf.example.com -token ctfd_abc123def456\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -config config.yml -workers 10 -rate-limit 15\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -url https://ctf.example.com -token $CTFD_TOKEN -test\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  CTFD_URL    - CTFd base URL\n")
		fmt.Fprintf(os.Stderr, "  CTFD_TOKEN  - CTFd access token\n")
	}
}
