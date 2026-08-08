package services

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ctfd-downloader/pkg/models"
	"ctfd-downloader/pkg/utils"

	"gopkg.in/yaml.v3"
)

type FileSystemService struct {
	baseDir string
}

func NewFileSystemService(baseDir string) *FileSystemService {
	return &FileSystemService{
		baseDir: baseDir,
	}
}

func (fs *FileSystemService) CreateChallengeDirectory(challenge *models.ChallengeDetailed) (string, error) {
	category := utils.SanitizeName(challenge.Category)
	challengeName := utils.SanitizeName(challenge.Name)

	challengeDir := filepath.Join(fs.baseDir, category, challengeName)

	err := os.MkdirAll(challengeDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create challenge directory %s: %w", challengeDir, err)
	}

	return challengeDir, nil
}

func (fs *FileSystemService) SaveChallengeMetadata(challenge *models.ChallengeDetailed, challengeDir string, fileInfos []models.FileInfo, hints []models.HintInfo, solves []models.SolveInfo) error {
	metadata := models.ChallengeMetadata{
		ID:             challenge.ID,
		Name:           challenge.Name,
		Description:    challenge.Description,
		Category:       challenge.Category,
		Value:          challenge.Value,
		Tags:           []string(challenge.Tags),
		Type:           challenge.Type,
		State:          challenge.State,
		Author:         challenge.Attribution,
		ConnectionInfo: challenge.ConnectionInfo,
		MaxAttempts:    challenge.MaxAttempts,
		Files:          fileInfos,
		Hints:          hints,
		Solves:         solves,
		DownloadedAt:   time.Now(),
	}

	metadata.Metadata = make(map[string]interface{})
	if challenge.Function != "" {
		metadata.Metadata["scoring_function"] = challenge.Function
	}
	if challenge.Initial != nil {
		metadata.Metadata["initial_value"] = *challenge.Initial
	}
	if challenge.Minimum != nil {
		metadata.Metadata["minimum_value"] = *challenge.Minimum
	}
	if challenge.Decay != nil {
		metadata.Metadata["decay"] = *challenge.Decay
	}
	if challenge.Logic != "" {
		metadata.Metadata["logic"] = challenge.Logic
	}

	yamlPath := filepath.Join(challengeDir, "challenge.yml")
	return fs.saveYAML(yamlPath, &metadata)
}

func (fs *FileSystemService) SaveChallengeREADME(challenge *models.ChallengeDetailed, challengeDir string, fileInfos []models.FileInfo) error {
	readmePath := filepath.Join(challengeDir, "README.md")

	content := fs.generateREADMEContent(challenge, fileInfos)

	err := os.WriteFile(readmePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to save README: %w", err)
	}

	return nil
}

// hasLaunchableInstance reports whether the challenge spins up an on-demand
// container (ctfd-whale and similar), which the description never mentions.
func hasLaunchableInstance(challenge *models.ChallengeDetailed) bool {
	if strings.Contains(strings.ToLower(challenge.Type), "docker") {
		return true
	}
	return strings.Contains(challenge.View, "Launch an instance") ||
		strings.Contains(challenge.View, "whale-panel")
}

// SaveChallengeView writes the server-rendered challenge HTML (CTFd's "view"
// field) so nothing shown in the challenge modal is lost, e.g. an on-demand
// instance launcher that isn't part of the description.
func (fs *FileSystemService) SaveChallengeView(challenge *models.ChallengeDetailed, challengeDir string) error {
	if strings.TrimSpace(challenge.View) == "" {
		return nil
	}
	viewPath := filepath.Join(challengeDir, "view.html")
	if err := os.WriteFile(viewPath, []byte(challenge.View), 0644); err != nil {
		return fmt.Errorf("failed to save view: %w", err)
	}
	return nil
}

func (fs *FileSystemService) DownloadFile(fileURL, challengeDir string, downloader func(string, io.Writer) error) (models.FileInfo, error) {
	filename, err := utils.ExtractFilenameFromURL(fileURL)
	if err != nil {
		return models.FileInfo{}, fmt.Errorf("failed to extract filename from URL %s: %w", fileURL, err)
	}
	filename = utils.SanitizeName(filename) // guard against path traversal

	filePath := filepath.Join(challengeDir, filename)

	// temp file + rename: a failed re-download won't destroy a prior file
	tmp, err := os.CreateTemp(challengeDir, ".download-*")
	if err != nil {
		return models.FileInfo{}, fmt.Errorf("failed to create temp file in %s: %w", challengeDir, err)
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		if !success {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	// hash while writing (no re-read)
	hash := sha1.New()
	if err := downloader(fileURL, io.MultiWriter(tmp, hash)); err != nil {
		return models.FileInfo{}, fmt.Errorf("failed to download file: %w", err)
	}

	info, err := tmp.Stat()
	if err != nil {
		return models.FileInfo{}, fmt.Errorf("failed to get file info: %w", err)
	}
	size := info.Size()

	if err := tmp.Close(); err != nil {
		return models.FileInfo{}, fmt.Errorf("failed to flush file: %w", err)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return models.FileInfo{}, fmt.Errorf("failed to move file into place: %w", err)
	}
	success = true

	return models.FileInfo{
		Name: filename,
		URL:  fileURL,
		Path: filename,
		Size: size,
		SHA1: fmt.Sprintf("%x", hash.Sum(nil)),
	}, nil
}

func (fs *FileSystemService) CheckExistingChallenge(challenge *models.ChallengeDetailed) (*models.ChallengeMetadata, bool, error) {
	challengeDir, err := fs.getChallengeDirectory(challenge)
	if err != nil {
		return nil, false, err
	}

	yamlPath := filepath.Join(challengeDir, "challenge.yml")

	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		return nil, false, nil
	}

	metadata, err := fs.loadChallengeMetadata(yamlPath)
	if err != nil {
		return nil, true, fmt.Errorf("failed to load existing metadata: %w", err)
	}

	return metadata, true, nil
}

func (fs *FileSystemService) GetChallengeDirectory(challenge *models.ChallengeDetailed) (string, error) {
	return fs.getChallengeDirectory(challenge)
}

func (fs *FileSystemService) getChallengeDirectory(challenge *models.ChallengeDetailed) (string, error) {
	category := utils.SanitizeName(challenge.Category)
	challengeName := utils.SanitizeName(challenge.Name)

	return filepath.Join(fs.baseDir, category, challengeName), nil
}

func (fs *FileSystemService) loadChallengeMetadata(yamlPath string) (*models.ChallengeMetadata, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}

	var metadata models.ChallengeMetadata
	err = yaml.Unmarshal(data, &metadata)
	if err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (fs *FileSystemService) saveYAML(path string, data interface{}) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr // surface close errors
		}
	}()

	encoder := yaml.NewEncoder(file)
	if err = encoder.Encode(data); err != nil {
		encoder.Close()
		return err
	}
	return encoder.Close()
}

func (fs *FileSystemService) generateREADMEContent(challenge *models.ChallengeDetailed, fileInfos []models.FileInfo) string {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# %s\n\n", challenge.Name))
	content.WriteString(fmt.Sprintf("**Category:** %s  \n", challenge.Category))
	content.WriteString(fmt.Sprintf("**Points:** %d  \n", challenge.Value))
	content.WriteString(fmt.Sprintf("**Type:** %s  \n", challenge.Type))

	if len(challenge.Tags) > 0 {
		content.WriteString(fmt.Sprintf("**Tags:** %s  \n", strings.Join([]string(challenge.Tags), ", ")))
	}

	if challenge.Attribution != nil && *challenge.Attribution != "" {
		content.WriteString(fmt.Sprintf("**Author:** %s  \n", *challenge.Attribution))
	}

	content.WriteString("\n## Description\n\n")
	content.WriteString(challenge.Description)
	content.WriteString("\n\n")

	if challenge.ConnectionInfo != nil && *challenge.ConnectionInfo != "" {
		content.WriteString("## Connection Info\n\n")
		content.WriteString(*challenge.ConnectionInfo)
		content.WriteString("\n\n")
	}

	if len(fileInfos) > 0 {
		content.WriteString("## Files\n\n")
		for _, fileInfo := range fileInfos {
			content.WriteString(fmt.Sprintf("- [%s](./%s) (%s)\n", fileInfo.Name, fileInfo.Path, utils.FormatBytes(fileInfo.Size)))
		}
		content.WriteString("\n")
	}

	if hasLaunchableInstance(challenge) {
		content.WriteString("## Instance\n\n")
		content.WriteString("This challenge runs an on-demand instance you launch on the CTF site.\n\n")
	}

	if strings.TrimSpace(challenge.View) != "" {
		content.WriteString("Full rendered challenge (including any instance panel): [view.html](./view.html)\n\n")
	}

	if challenge.MaxAttempts > 0 {
		content.WriteString(fmt.Sprintf("**Max Attempts:** %d\n\n", challenge.MaxAttempts))
	}

	content.WriteString(fmt.Sprintf("*Downloaded on %s*\n", time.Now().Format("2006-01-02 15:04:05")))

	return content.String()
}
