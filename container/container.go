package container

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

type Container struct {
	ID          string
	Image       string
	RootDir     string
	OverlayDirs OverlayDirs
}

type OverlayDirs struct {
	Image  string
	Upper  string
	Work   string
	Merged string
}

func New(image string) (*Container, error) {

	// Generate unique container ID
	id := generateID()

	rootDir := fmt.Sprintf("/tmp/gocker/containers/%s", id)

	dirs := OverlayDirs{
		Image:  filepath.Join(rootDir, "image"),  // unpacked layers
		Upper:  filepath.Join(rootDir, "upper"),  // writable layer
		Work:   filepath.Join(rootDir, "work"),   // overlayfs scratch
		Merged: filepath.Join(rootDir, "merged"), // final merged view
	}

	c := &Container{
		ID:          id,
		Image:       image,
		RootDir:     rootDir,
		OverlayDirs: dirs,
	}

	return c, nil
}

func (c *Container) Setup(imageDir string) error {

	dirs := c.OverlayDirs.Map()
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create dir %s: %w", dir, err)
		}
	}

	// --->> Unpack layers into /image
	if err := c.unpackLayers(imageDir); err != nil {
		return fmt.Errorf("failed to unpack layers: %w", err)
	} // <<---

	// --->> Mount overlayfs
	if err := c.mountOverlayfs(); err != nil {
		return fmt.Errorf("failed to mount overlay: %w", err)
	} // <<---

	return nil
}

func (c *Container) Run(command []string) error {
	// TODO
	cmd := exec.Command(command[0], command[1:]...)

	// Container's stdin, stdout, stderr to ours
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "/"

	// namespaces and chroot config
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS,
		Chroot: c.OverlayDirs.Merged,
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}
	return nil
}

func (d *OverlayDirs) Map() map[string]string {
	return map[string]string{
		"image":  d.Image,
		"upper":  d.Upper,
		"work":   d.Work,
		"merged": d.Merged,
	}
}

func (c *Container) unpackLayers(imageDir string) error {
	// --->> Find all layer tarballs in the img dir
	layers, err := filepath.Glob(filepath.Join(imageDir, "*.tar.gz"))
	if err != nil {
		return fmt.Errorf("failed to find layers: %w", err)
	}

	if len(layers) == 0 {
		return fmt.Errorf("no layers found in %s", imageDir)
	}

	// --->> Unpack each layer on top of previous one
	for _, layer := range layers {
		fmt.Printf("unpacking layers %s\n", filepath.Base(layer))
		cmd := exec.Command("tar", "-xzf", layer, "-C", c.OverlayDirs.Image)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to unpack layer%s: %w", layer, err)
		}
	} // <<---

	return nil
}

func (c *Container) mountOverlayfs() error {

	// lowerdir=image (read-only)
	// upperdir=upper (read-write)
	// workdir=work (scratch space)
	// merged (final view)
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
		c.OverlayDirs.Image,
		c.OverlayDirs.Upper,
		c.OverlayDirs.Work,
	)
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", opts, c.OverlayDirs.Merged)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to mount overlay: %w", err)
	}

	fmt.Printf("overlayfs mounted at %s\n", c.OverlayDirs.Merged)
	return nil
}

func generateID() string {
	b := make([]byte, 6) // 12 hex characters
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
