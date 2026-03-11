package container

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/juannio/farum/image"
)

type Container struct {
	ID          string
	Image       *image.Image
	RootDir     string
	OverlayDirs OverlayDirs
}

type OverlayDirs struct {
	Lower  string
	Upper  string
	Work   string
	Merged string
}

func New(image *image.Image) (*Container, error) {

	// Generate unique container ID
	id := generateID()

	rootDir := fmt.Sprintf("/tmp/gocker/containers/%s", id)

	c := &Container{
		ID:      id,
		Image:   image,
		RootDir: rootDir,
		OverlayDirs: OverlayDirs{
			Lower:  image.RootfsDir,                  // unpacked layers into /images
			Upper:  filepath.Join(rootDir, "upper"),  // writable layer
			Work:   filepath.Join(rootDir, "work"),   // overlayfs scratch
			Merged: filepath.Join(rootDir, "merged"), // final merged view
		},
	}

	return c, nil
}

func (c *Container) Setup() error {

	dirs := c.OverlayDirs.Map()
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create dir %s: %w", dir, err)
		}
	}

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
		"upper":  d.Upper,
		"work":   d.Work,
		"merged": d.Merged,
	}
}

func (c *Container) mountOverlayfs() error {

	// lowerdir=image (read-only)
	// upperdir=upper (read-write)
	// workdir=work (scratch space)
	// merged (final view)
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
		c.OverlayDirs.Lower,
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
