package container

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
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

	rootDir := fmt.Sprintf("/tmp/farum/containers/%s", id)

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

	// Copy binary into container
	// Create paths
	binCurrentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get bin current path: %w", err)
	}
	destinationPath := filepath.Join(c.OverlayDirs.Merged, "tmp", "farum")

	if err := copyBin(binCurrentPath, destinationPath); err != nil {
		return fmt.Errorf("failed to copy bin from %s to %s: %w", binCurrentPath, destinationPath, err)
	}

	return nil
}

func (c *Container) Run(command []string) error {
	cmd := exec.Command("/tmp/farum", append([]string{"init", c.ID}, command...)...)

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
		Chroot: c.OverlayDirs.Merged, // TODO: pivot_root?
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
	/* cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", opts, c.OverlayDirs.Merged)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to mount overlay: %w", err)
	}*/

	if err := syscall.Mount("overlay", c.OverlayDirs.Merged, "overlay", 0, opts); err != nil {
		return fmt.Errorf("failed to mount overlay: %w", err)
	}

	return nil
}

func (c *Container) CleanUp() error {

	// TODO: try RemoveAll if Unmount fails ayway
	// Unmount overlayfs on /merged
	if err := syscall.Unmount(c.OverlayDirs.Merged, 0); err != nil {
		fmt.Printf("note: overlay unmount: %v\n", err)
	}

	// Remove container
	if err := os.RemoveAll(c.RootDir); err != nil {
		return fmt.Errorf("error removing container %s: %w", c.ID, err)
	}

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

func copyBin(src, dst string) error {

	srcBin, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcBin.Close()

	dstBin, err := os.Create(dst) //Creates bin, empty
	if err != nil {
		return err
	}
	defer dstBin.Close()

	//Copy src bin content into dst bin
	_, err = io.Copy(dstBin, srcBin)
	if err != nil {
		return err
	}

	if err := dstBin.Close(); err != nil {
		return err
	}

	if err := os.Chmod(dst, 0o755); err != nil {
		return err
	}

	return nil
}
