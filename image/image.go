package image

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

type Image struct {
	Name      string
	Tag       string
	ImageDir  string
	RootfsDir string
}

func New(name, tag string) *Image {
	imageDir := fmt.Sprintf("/tmp/gocker/images/%s/%s", name, tag)
	var image *Image = &Image{
		Name:      name,
		Tag:       tag,
		ImageDir:  imageDir,
		RootfsDir: filepath.Join(imageDir, "rootfs"),
	}
	return image
}

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

// Pulls manifest and layers tarballs, unpacks into rootfs
func (img *Image) Pull() error {
	fmt.Printf("pulling %s:%s\n", img.Name, img.Tag)

	// --->> Get token
	token, err := getAuthToken(img.Name)
	if err != nil {
		return fmt.Errorf("auth failed %w", err)
	}
	fmt.Println("authenticated")
	// <<---

	// --->> Get manifest
	manifest, err := getManifest(img.Name, img.Tag, token)
	if err != nil {
		return fmt.Errorf("failed to get manifest %w", err)
	}
	fmt.Printf("manifest fetched, %d layers\n", len(manifest.Layers))
	fmt.Println(manifest)
	// <<---

	// --->> Create directory to store image layers
	if err := os.MkdirAll(img.ImageDir, 0755); err != nil {
		return fmt.Errorf("failed to create img dir: %w", err)
	}
	// <<---

	// --->> Download each layer
	for i, layer := range manifest.Layers {
		fmt.Printf("downloading layer %d/%d\n", i+1, len(manifest.Layers))
		if err := downloadLayer(img.Name, layer.Digest, token, img.ImageDir); err != nil {
			return fmt.Errorf("failed to download layer %s: %w", layer.Digest, err)
		}
	} // <<---

	// --->> Unpack layers in /<IMAGE>/<TAG>/rootfs
	if err := unpackLayers(img.ImageDir, img.RootfsDir); err != nil {
		return fmt.Errorf("failed to unpack layers: %w", err)
	}

	fmt.Printf("sucessfully pulled %s:%s\n", img.Name, img.Tag)
	return nil

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

func unpackLayers(imageDir string, rootfs string) error {
	// --->> Find all layer tarballs in the img dir
	layers, err := filepath.Glob(filepath.Join(imageDir, "*.tar.gz"))
	if err != nil {
		return fmt.Errorf("failed to find layers: %w", err)
	}

	if len(layers) == 0 {
		return fmt.Errorf("no layers found in %s", imageDir)
	} // <<---

	// --->> Create rootfs dir, where tarballs will be unpacked
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		return fmt.Errorf("failed to create rootfs dir %s: %w", rootfs, err)
	} // <<---

	// --->> Unpack each layer on rootfs top of previous one
	for _, layer := range layers {
		fmt.Printf("unpacking layers %s\n", filepath.Base(layer))
		cmd := exec.Command("tar", "-xzf", layer, "-C", rootfs)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to unpack layer%s: %w", layer, err)
		}
	} // <<---

	return nil
}
