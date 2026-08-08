package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"ctfd-downloader/pkg/client"
	"ctfd-downloader/pkg/models"
	"ctfd-downloader/pkg/utils"

	"golang.org/x/time/rate"
)

type DownloadService struct {
	client     *client.CTFdClient
	filesystem *FileSystemService
	config     *DownloadConfig
	limiter    *rate.Limiter
	stats      *models.DownloadStats
	statsMutex sync.RWMutex
	progress   func(done, total int, result models.DownloadResult)
}

// SetProgressHook registers an optional callback fired once per finished
// challenge. When set, per-result logging is suppressed so a caller can render
// its own UI.
func (ds *DownloadService) SetProgressHook(fn func(done, total int, result models.DownloadResult)) {
	ds.progress = fn
}

// logf logs only when no progress hook owns the display, so log lines never
// corrupt a caller's live UI.
func (ds *DownloadService) logf(format string, args ...interface{}) {
	if ds.progress == nil {
		log.Printf(format, args...)
	}
}

type DownloadConfig struct {
	MaxWorkers     int
	RateLimit      int // requests per second
	RetryCount     int
	RetryDelay     time.Duration
	IncludeHints   bool
	IncludeSolves  bool
	SkipExisting   bool
	OverwriteFiles bool
	FileWorkers    int // separate worker pool for file downloads
}

func DefaultDownloadConfig() *DownloadConfig {
	return &DownloadConfig{
		MaxWorkers:     5,
		RateLimit:      10,
		RetryCount:     3,
		RetryDelay:     1 * time.Second,
		IncludeHints:   false,
		IncludeSolves:  false,
		SkipExisting:   true,
		OverwriteFiles: false,
		FileWorkers:    3,
	}
}

func NewDownloadService(ctfdClient *client.CTFdClient, filesystem *FileSystemService, config *DownloadConfig) *DownloadService {
	if config == nil {
		config = DefaultDownloadConfig()
	}

	limiter := rate.NewLimiter(rate.Limit(config.RateLimit), config.RateLimit*2)

	stats := &models.DownloadStats{
		StartTime: time.Now(),
	}

	return &DownloadService{
		client:     ctfdClient,
		filesystem: filesystem,
		config:     config,
		limiter:    limiter,
		stats:      stats,
	}
}

func (ds *DownloadService) DownloadAllChallenges(ctx context.Context) (*models.DownloadStats, error) {
	ds.resetStats()

	if ds.progress == nil {
		log.Println("Fetching challenge list...")
	}

	challenges, err := ds.client.GetChallenges()
	if err != nil {
		ds.addError(fmt.Sprintf("Failed to fetch challenges: %v", err))
		return ds.getStats(), err
	}

	ds.setTotalChallenges(len(challenges))
	if ds.progress != nil {
		ds.progress(0, len(challenges), models.DownloadResult{}) // announce total before work starts
	} else {
		log.Printf("Found %d challenges", len(challenges))
	}

	challengeJobs := make(chan models.Challenge, len(challenges))
	results := make(chan models.DownloadResult, len(challenges))

	var wg sync.WaitGroup
	for i := 0; i < ds.config.MaxWorkers; i++ {
		wg.Add(1)
		go ds.challengeWorker(ctx, challengeJobs, results, &wg)
	}

	go func() {
		defer close(challengeJobs)
		for _, challenge := range challenges {
			select {
			case challengeJobs <- challenge:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var processedCount int
	for result := range results {
		processedCount++
		quiet := ds.progress != nil
		if result.Success {
			if result.Skipped {
				ds.incrementSkipped()
				if !quiet {
					log.Printf("[%d/%d] Skipped: %s", processedCount, len(challenges), result.Name)
				}
			} else {
				ds.incrementDownloaded()
				if !quiet {
					log.Printf("[%d/%d] Successfully downloaded: %s", processedCount, len(challenges), result.Name)
				}
			}
		} else {
			ds.incrementFailed()
			errorMsg := fmt.Sprintf("Failed to download %s: %v", result.Name, result.Error)
			ds.addError(errorMsg)
			if !quiet {
				log.Printf("[%d/%d] %s", processedCount, len(challenges), errorMsg)
			}
		}

		if ds.progress != nil {
			ds.progress(processedCount, len(challenges), result)
		}

		select {
		case <-ctx.Done():
			ds.logf("Download cancelled")
			return ds.getStats(), ctx.Err()
		default:
		}
	}

	ds.finalize()

	finalStats := ds.getStats()

	// The drain loop can end normally when workers stop on cancellation, so
	// report the cancellation explicitly rather than a bogus "success".
	if err := ctx.Err(); err != nil {
		return finalStats, err
	}

	if ds.progress == nil {
		log.Printf("Download completed: %d successful, %d skipped, %d failed, %d files (%s) in %v",
			finalStats.Downloaded, finalStats.Skipped, finalStats.Failed, finalStats.FilesDownloaded,
			formatBytes(finalStats.TotalSize), finalStats.Duration)
	}

	return finalStats, nil
}

func (ds *DownloadService) challengeWorker(ctx context.Context, jobs <-chan models.Challenge, results chan<- models.DownloadResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result := ds.processChallenge(ctx, job)

		select {
		case results <- result:
		case <-ctx.Done():
			return
		}
	}
}

func (ds *DownloadService) processChallenge(ctx context.Context, challenge models.Challenge) models.DownloadResult {
	result := models.DownloadResult{
		ChallengeID: challenge.ID,
		Name:        challenge.Name,
		Category:    challenge.Category,
		Success:     false,
	}

	var lastErr error
	for attempt := 0; attempt < ds.config.RetryCount; attempt++ {
		if attempt > 0 {
			ds.logf("Retrying challenge %s (attempt %d/%d)", challenge.Name, attempt+1, ds.config.RetryCount)
			select {
			case <-time.After(ds.config.RetryDelay):
			case <-ctx.Done():
				result.Error = ctx.Err()
				return result
			}
		}

		if err := ds.limiter.Wait(ctx); err != nil {
			result.Error = err
			return result
		}

		if err := ds.downloadChallenge(ctx, &challenge, &result); err != nil {
			lastErr = err
			continue
		}

		result.Success = true
		return result
	}

	result.Error = lastErr
	return result
}

func (ds *DownloadService) downloadChallenge(ctx context.Context, challenge *models.Challenge, result *models.DownloadResult) error {
	challengeDetail, err := ds.client.GetChallenge(challenge.ID)
	if err != nil {
		return fmt.Errorf("failed to get challenge details: %w", err)
	}

	if ds.config.SkipExisting {
		if existing, exists, err := ds.filesystem.CheckExistingChallenge(challengeDetail); err != nil {
			ds.logf("Warning: Could not check existing challenge %s: %v", challenge.Name, err)
		} else if exists {
			ds.logf("Skipping existing challenge: %s (downloaded %v)", challenge.Name, existing.DownloadedAt.Format("2006-01-02 15:04:05"))
			result.OutputPath = fmt.Sprintf("%s/%s", sanitizeName(challengeDetail.Category), sanitizeName(challengeDetail.Name))
			result.Skipped = true
			return nil
		}
	}

	challengeDir, err := ds.filesystem.CreateChallengeDirectory(challengeDetail)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	result.OutputPath = challengeDir

	// CTFd's own files, plus attachment links embedded in the description
	// (some CTFs list files there instead of in the files field).
	fileURLs := append([]string{}, challengeDetail.Files...)
	for _, u := range utils.ExtractAttachmentURLs(challengeDetail.Description) {
		if err := ds.limiter.Wait(ctx); err != nil {
			return err
		}
		// ponytail: 250MB cap on description links so a stray installer/tooling
		// link can't run away; add a -max-file-size flag if this bites real files.
		if size, err := ds.client.FileSize(u); err == nil && size > 250<<20 {
			ds.logf("Skipping large attachment (%s): %s", formatBytes(size), u)
			continue
		}
		fileURLs = append(fileURLs, u)
	}

	var fileInfos []models.FileInfo
	if len(fileURLs) > 0 {
		fileInfos, err = ds.downloadChallengeFiles(ctx, fileURLs, challengeDir)
		if err != nil {
			return fmt.Errorf("failed to download files: %w", err)
		}
		result.Files = make([]string, len(fileInfos))
		for i, fi := range fileInfos {
			result.Files[i] = fi.Name
		}
	}

	var hints []models.HintInfo
	var solves []models.SolveInfo

	if ds.config.IncludeHints && len(challengeDetail.Hints) > 0 {
		for _, hint := range challengeDetail.Hints {
			hintInfo := models.HintInfo{
				ID:      hint.ID,
				Title:   hint.Title,
				Cost:    hint.Cost,
				Content: hint.Content,
			}
			hints = append(hints, hintInfo)
		}
	}

	if ds.config.IncludeSolves {
		solvesData, err := ds.client.GetChallengeSolves(challenge.ID)
		if err != nil {
			ds.logf("Warning: Could not fetch solves for %s: %v", challenge.Name, err)
		} else {
			for _, solve := range solvesData {
				solveInfo := models.SolveInfo{
					Team: solve.Name,
					User: solve.Account,
					Date: solve.Date,
				}
				solves = append(solves, solveInfo)
			}
		}
	}

	if err := ds.filesystem.SaveChallengeMetadata(challengeDetail, challengeDir, fileInfos, hints, solves); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	if err := ds.filesystem.SaveChallengeREADME(challengeDetail, challengeDir, fileInfos); err != nil {
		return fmt.Errorf("failed to save README: %w", err)
	}

	if err := ds.filesystem.SaveChallengeView(challengeDetail, challengeDir); err != nil {
		return fmt.Errorf("failed to save view: %w", err)
	}

	ds.commitFileStats(len(fileURLs), fileInfos)

	return nil
}

func (ds *DownloadService) downloadChallengeFiles(ctx context.Context, fileURLs []string, challengeDir string) ([]models.FileInfo, error) {
	if len(fileURLs) == 0 {
		return nil, nil
	}

	fileJobs := make(chan string, len(fileURLs))
	fileResults := make(chan fileDownloadResult, len(fileURLs))

	var wg sync.WaitGroup
	workerCount := ds.config.FileWorkers
	if workerCount > len(fileURLs) {
		workerCount = len(fileURLs)
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go ds.fileWorker(ctx, fileJobs, fileResults, challengeDir, &wg)
	}

	go func() {
		defer close(fileJobs)
		for _, fileURL := range fileURLs {
			select {
			case fileJobs <- fileURL:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(fileResults)
	}()

	var fileInfos []models.FileInfo
	var errs []error

	// stats committed once by caller after success (avoids retry double-count)
	for result := range fileResults {
		if result.err != nil {
			errs = append(errs, result.err)
		} else {
			fileInfos = append(fileInfos, result.info)
		}
	}

	if len(errs) > 0 {
		return fileInfos, fmt.Errorf("some files failed to download: %v", errs)
	}

	return fileInfos, nil
}

type fileDownloadResult struct {
	info models.FileInfo
	err  error
}

func (ds *DownloadService) fileWorker(ctx context.Context, jobs <-chan string, results chan<- fileDownloadResult, challengeDir string, wg *sync.WaitGroup) {
	defer wg.Done()

	for fileURL := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fileInfo, err := ds.downloadFileWithRetry(ctx, fileURL, challengeDir)

		select {
		case results <- fileDownloadResult{info: fileInfo, err: err}:
		case <-ctx.Done():
			return
		}
	}
}

// downloadFileWithRetry fetches one file, returning one (info, err) per call.
func (ds *DownloadService) downloadFileWithRetry(ctx context.Context, fileURL, challengeDir string) (models.FileInfo, error) {
	var lastErr error
	var fileInfo models.FileInfo

	for attempt := 0; attempt < ds.config.RetryCount; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(ds.config.RetryDelay):
			case <-ctx.Done():
				return models.FileInfo{}, ctx.Err()
			}
		}

		if err := ds.limiter.Wait(ctx); err != nil {
			return models.FileInfo{}, err
		}

		fileInfo, lastErr = ds.filesystem.DownloadFile(fileURL, challengeDir, ds.client.DownloadFileToWriter)
		if lastErr == nil {
			return fileInfo, nil
		}
	}

	return models.FileInfo{}, fmt.Errorf("failed to download %s after %d attempts: %w", fileURL, ds.config.RetryCount, lastErr)
}

func (ds *DownloadService) resetStats() {
	ds.statsMutex.Lock()
	defer ds.statsMutex.Unlock()

	ds.stats = &models.DownloadStats{
		StartTime: time.Now(),
	}
}

func (ds *DownloadService) getStats() *models.DownloadStats {
	ds.statsMutex.RLock()
	defer ds.statsMutex.RUnlock()

	statsCopy := *ds.stats
	return &statsCopy
}

func (ds *DownloadService) incrementDownloaded() {
	ds.statsMutex.Lock()
	defer ds.statsMutex.Unlock()
	ds.stats.Downloaded++
}

func (ds *DownloadService) incrementSkipped() {
	ds.statsMutex.Lock()
	defer ds.statsMutex.Unlock()
	ds.stats.Skipped++
}

func (ds *DownloadService) incrementFailed() {
	ds.statsMutex.Lock()
	defer ds.statsMutex.Unlock()
	ds.stats.Failed++
}

func (ds *DownloadService) setTotalChallenges(n int) {
	ds.statsMutex.Lock()
	defer ds.statsMutex.Unlock()
	ds.stats.TotalChallenges = n
}

func (ds *DownloadService) finalize() {
	ds.statsMutex.Lock()
	defer ds.statsMutex.Unlock()
	ds.stats.EndTime = time.Now()
	ds.stats.Duration = ds.stats.EndTime.Sub(ds.stats.StartTime)
}

// commitFileStats records a challenge's file counts once, after it succeeds.
func (ds *DownloadService) commitFileStats(totalFiles int, fileInfos []models.FileInfo) {
	ds.statsMutex.Lock()
	defer ds.statsMutex.Unlock()
	ds.stats.TotalFiles += totalFiles
	ds.stats.FilesDownloaded += len(fileInfos)
	for _, fi := range fileInfos {
		ds.stats.TotalSize += fi.Size
	}
}

func (ds *DownloadService) addError(errorMsg string) {
	ds.statsMutex.Lock()
	defer ds.statsMutex.Unlock()
	ds.stats.Errors = append(ds.stats.Errors, errorMsg)
}

func (ds *DownloadService) GetStats() *models.DownloadStats {
	return ds.getStats()
}

func formatBytes(bytes int64) string {
	return utils.FormatBytes(bytes)
}

func sanitizeName(name string) string {
	return utils.SanitizeName(name)
}
