package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Each container lifecycle metadata,
// metadata is used to display <farum ps> info (containers status)

type ContainerMetadata struct {
	Id          string    `json:"id"`
	Image       string    `json:"image"`
	CreatedAt   time.Time `json:"created_at"`
	UpTime      time.Time `json:"up_time"`
	FinalizedAt time.Time `json:"finalized_at"`
	Status      string    `json:"status"`
	Command     string    `json:"command"`
	FilePath    string    `json:"-"`
}

// all metadata store dir
var baseDir = "/tmp/farum/metadata"

func NewMetaData(c *Container, command []string) *ContainerMetadata {
	return &ContainerMetadata{
		Id:        c.ID,
		Image:     fmt.Sprintf("%s:%s", c.Image.Name, c.Image.Tag),
		CreatedAt: time.Now(),
		Status:    "RUNNING",
		Command:   strings.Join(command, " "),
		FilePath:  fmt.Sprintf("%s/%s.json", baseDir, c.ID),
	}
}

func (cm *ContainerMetadata) SaveMetadata() error {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to get metadata dir %s", err)
	}

	data, err := json.MarshalIndent(cm, "", "  ")
	if err != nil {
		return fmt.Errorf("Error occurred during marshalling: %w", err)
	}

	if err := os.WriteFile(cm.FilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write data into json file: %w", err)
	}
	return nil
}

// Update container status
// TODO: Implement <starting, stopped, running>
func (cm *ContainerMetadata) UpdateMetaData() error {
	cm.Status = "EXITED"
	if err := cm.SaveMetadata(); err != nil {
		return fmt.Errorf("failed to change status to %s: %w", cm.Status, err)
	}
	return nil
}

func ReadMetadata() ([]ContainerMetadata, error) {

	// --->> baseDir exists?
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("failed trying to read %s: %w", baseDir, err)

	} // <<---

	// --->> Read baseDir entries
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s entries: %w", baseDir, err)
	}

	var all []ContainerMetadata
	var errors []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue // skip subdirectories
		}

		// Only .json
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(baseDir, entry.Name())

		// 4. Read file
		data, err := os.ReadFile(filePath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Error reading %s: %w", filePath, err))
			continue
		}

		// 5. Unmarshal
		var content ContainerMetadata
		if err := json.Unmarshal(data, &content); err != nil {
			fmt.Println("Invalid JSON:", filePath, err)
			continue
		}

		// collect or process
		all = append(all, content)

	}

	return all, nil
}
