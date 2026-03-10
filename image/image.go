package image

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type ManifestList struct {
	SchemaVersion int                `json:"schemaVersion"`
	MediaType     string             `json:"mediaType"`
	Manifests     []ManifestListItem `json:"manifests"`
}

type Manifest struct {
	SchemaVersion int     `json:"schemaVersion"`
	MediaType     string  `json:"mediaType"`
	Config        Layer   `json:"config"`
	Layers        []Layer `json:"layers"`
}

type ManifestListItem struct {
	Digest    string   `json:"digest"`
	MediaType string   `json:"mediaType"`
	Platform  Platform `json:"platform"`
}

type Platform struct {
	Architectrue string `json:"architecture"`
	OS           string `json:"os"`
}

type Layer struct {
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
}

func getAuthToken(image string) (string, error) {
	url := fmt.Sprintf(
		"https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%s:pull",
		image,
	)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
	}

	// Stream response body directly into struct
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return result.Token, nil
}

// Get image manifest
func getManifest(image, tag, token string) (*Manifest, error) {

	// --->> Create and fetch request
	url := fmt.Sprintf(
		"https://registry-1.docker.io/v2/library/%s/manifests/%s",
		image, tag,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	} // <<---

	// --->> Debug the manifest response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	} // <<---

	// --->> Decode response into struct
	// If response is manifest list
	var manifestList ManifestList
	if err := json.Unmarshal(body, &manifestList); err == nil && len(manifestList.Manifests) > 0 {
		for _, m := range manifestList.Manifests {
			if m.Platform.Architectrue == "amd64" && m.Platform.OS == "linux" {
				return getManifestByDigest(image, m.Digest, token)
			}
		}
		return nil, fmt.Errorf("no amd64 manifest found")
	}

	// If response is regular manifest
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	} // <<---

	return &manifest, nil
}

// Get manifest by digest, when manifest returns manifest list
func getManifestByDigest(image, digest, token string) (*Manifest, error) {
	// --->> Create and fetch request
	url := fmt.Sprintf(
		"https://registry-1.docker.io/v2/library/%s/manifests/%s",
		image, digest,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest by digest: %w", err)
	}
	defer resp.Body.Close()
	// <<---

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	// --->> Decode response into struct
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}
	// <<---

	return &manifest, nil
}

func Pull(image, tag string) error {
	fmt.Printf("pulling %s:%s\n", image, tag)

	// --->> Get token
	token, err := getAuthToken(image)
	if err != nil {
		return fmt.Errorf("auth failed %w", err)
	}
	fmt.Println("authenticated")
	// <<---

	// --->> Get manifest
	manifest, err := getManifest(image, tag, token)
	if err != nil {
		return fmt.Errorf("failed to get manifest %w", err)
	}
	fmt.Printf("manifest fetched, %d layers\n", len(manifest.Layers))
	fmt.Println(manifest)
	// <<---

	// --->> Create directory to store image layers
	imageDir := fmt.Sprintf("/tmp/gocker/images/%s/%s", image, tag)
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return fmt.Errorf("failed to create img dir: %w", err)
	}
	// <<---

	// --->> Download each layer
	for i, layer := range manifest.Layers {
		fmt.Printf("downloading layer %d/%d\n", i+1, len(manifest.Layers))
		if err := downloadLayer(image, layer.Digest, token, imageDir); err != nil {
			return fmt.Errorf("failed to download layer %s: %w", layer.Digest, err)
		}
	}

	fmt.Printf("sucessfully pulled %s:%s\n", image, tag)
	return nil
	// <<---
}

func downloadLayer(image, digest, token, imageDir string) error {
	url := fmt.Sprintf(
		"https://registry-1.docker.io/v2/library/%s/blobs/%s",
		image, digest,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download layer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// File to store layer tarball
	layerPath := fmt.Sprintf("%s/%s.tar.gz", imageDir, digest[7:19])
	file, err := os.Create(layerPath)
	if err != nil {
		return fmt.Errorf("failed to create layer file: %w", err)
	}
	defer file.Close()

	// Stream response directly into file
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write layer: %w", err)
	}

	return nil
}
