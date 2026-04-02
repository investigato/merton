package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"net/http"

	"github.com/investigato/go-psrp/client"
	"github.com/investigato/prompt"
	"github.com/schollz/progressbar/v3"
)

const httpThreshold = 1 * 1024 * 1024 // 1MB
func outboundIP(target string) (string, error) {
	conn, err := net.Dial("udp", target+":80")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = conn.Close()
	}()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

func (sh *MertonShell) Download(ctx context.Context, c *client.Client, pr *prompt.Prompt, line string) {
	if sh.shellType == WinRsShell {
		fmt.Println("Download command is not supported in WinRS shell")
		return
	}
	// remove the command part and just keep the arguments for processing
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		fmt.Println("Usage: download <remote_path> [local_path]")
		return
	}
	args := strings.Fields(parts[1])
	if len(args) < 1 {
		fmt.Println("Usage: download <remote_path> [local_path]")
		return
	}
	remotePath := args[0]
	if !filepath.IsAbs(remotePath) {
		cleanCWD := strings.Trim(sh.cmdCWD, "[]")
		cleanRemotePath := strings.Trim(remotePath, "[]")
		remotePath = filepath.Join(cleanCWD, cleanRemotePath)
		// for downloading FROM windows, need to remove all '/' and replace with '\'
		remotePath = strings.ReplaceAll(remotePath, "/", "\\")
	}
	localPath := ""
	if len(args) >= 2 {
		localPath = args[1]
	} else {
		// just assume current directory
		currentDir, err := os.Getwd()
		if err != nil {
			log.Printf("Failed to get current directory: %v", err)
			localPath = filepath.Base(strings.ReplaceAll(remotePath, "\\", "/"))
		} else {
			localPath = filepath.Join(currentDir, filepath.Base(strings.ReplaceAll(remotePath, "\\", "/")))
		}
	}
	bar := progressbar.NewOptions64(-1,
		progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s", remotePath)),
		progressbar.OptionShowBytes(true),
	)
	options := []client.FileTransferOption{
		client.WithProgressCallback(func(transferred, total int64) {
			bar.ChangeMax64(total)
			if err := bar.Set64(transferred); err != nil {
				log.Printf("Failed to update progress bar: %v", err)
			}
		}),
	}
	err := c.FetchFile(ctx, remotePath, localPath, options...)
	if progressErr := bar.Finish(); progressErr != nil {
		log.Printf("Failed to finish progress bar: %v", progressErr)
	}
	if err != nil {
		log.Printf("\nDownload failed: %v", err)
	} else {
		fmt.Printf("\nDownloaded %s to %s\n", remotePath, localPath)
	}
}

func (sh *MertonShell) Upload(ctx context.Context, c *client.Client, pr *prompt.Prompt, line string, hostname string, port int) {
	if sh.shellType == WinRsShell {
		fmt.Println("Upload command is not supported in WinRS shell")
		return
	}
	// remove the command part and just keep the arguments for processing
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		fmt.Println("Usage: upload <local_path> [remote_path]")
		return
	}
	args := strings.Fields(parts[1])
	if len(args) < 1 {
		fmt.Println("Usage: upload <local_path> [remote_path]")
		return
	}
	localPath := args[0]
	remotePath := ""
	if len(args) >= 2 {
		remotePath = args[1]
	} else {
		remotePath = localPath
	}

	info, statErr := os.Stat(localPath)
	if statErr != nil {
		log.Printf("Failed to stat local file: %v", statErr)
		return
	}
	// if file size is > threshold, TRY the http server method.
	if info.Size() > httpThreshold {
		fmt.Printf("File size is greater than %d bytes, trying HTTP server method...\n", httpThreshold)
		if _, err := serveAndNotify(ctx, c, localPath, remotePath, hostname, port); err != nil {
			log.Printf("HTTP server method failed: %v", err)
			fmt.Println("Falling back to CopyFile method...")
			// if error, use the copy file method as fallback.
			useCopyFile(ctx, c, localPath, remotePath)
		}
	} else {
		// if file size is < threshold, just use the copy file method
		useCopyFile(ctx, c, localPath, remotePath)
	}

}

func useCopyFile(ctx context.Context, c *client.Client, localPath, remotePath string) {
	bar := progressbar.NewOptions64(-1,
		progressbar.OptionSetDescription(fmt.Sprintf("Uploading %s", localPath)),
		progressbar.OptionShowBytes(true),
	)
	options := []client.FileTransferOption{
		client.WithProgressCallback(func(transferred, total int64) {
			bar.ChangeMax64(total)
			if err := bar.Set64(transferred); err != nil {
				log.Printf("Failed to update progress bar: %v", err)
			}
		}),
	}
	err := c.CopyFile(ctx, localPath, remotePath, options...)
	if progressErr := bar.Finish(); progressErr != nil {
		log.Printf("Failed to finish progress bar: %v", progressErr)
	}
	if err != nil {
		log.Printf("\nUpload failed: %v", err)
	} else {
		fmt.Printf("\nUploaded %s to %s\n", localPath, remotePath)
		return
	}
}

func startServer(hostIP string, port int, filePath string, fileSize int64, bar *progressbar.ProgressBar) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	reader := progressbar.NewReader(file, bar)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", hostIP, port),
		Handler: mux,
	}

	mux.HandleFunc("/"+filepath.Base(filePath), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(filePath)))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))

		if _, copyErr := io.Copy(w, &reader); copyErr != nil {
			log.Printf("failed to stream file: %v", copyErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			log.Printf("failed to close file: %v", closeErr)
		}

		go func() {
			if shutdownErr := server.Shutdown(context.Background()); shutdownErr != nil {
				log.Printf("failed to shutdown server: %v", shutdownErr)
			}
		}()
	})
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write([]byte("Shutting down server")); writeErr != nil {
			log.Printf("failed to write shutdown response: %v", writeErr)
		}
		go func() {
			if shutdownErr := server.Shutdown(context.Background()); shutdownErr != nil {
				log.Printf("failed to shutdown server: %v", shutdownErr)
			}
		}()
	})
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			log.Printf("failed to close file: %v", closeErr)
		}
		return fmt.Errorf("failed to start listener: %w", err)
	}

	go func() {
		if err := server.Serve(ln); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

func serveAndNotify(ctx context.Context, c *client.Client, localPath, remotePath,
	target string, port int) (string, error) {
	localIP, err := outboundIP(target)
	if err != nil {
		return "", fmt.Errorf("failed to determine outbound IP: %w", err)
	}

	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat local file: %w", err)
	}

	fileURL := fmt.Sprintf("http://%s:%d/%s", localIP, port,
		filepath.Base(localPath))
	fmt.Fprintf(os.Stderr, "Serving %s from %s\n", filepath.Base(localPath),
		fileURL)

	bar := progressbar.NewOptions64(fileInfo.Size(),
		progressbar.OptionSetDescription("Uploading"),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWriter(os.Stderr),
	)

	if err := startServer(localIP, port, localPath, fileInfo.Size(), bar); err !=
		nil {
		return "", fmt.Errorf("failed to start server: %w", err)
	}

	cmd := fmt.Sprintf("iwr '%s' -OutFile '%s'", fileURL, remotePath)
	if _, err := c.Execute(ctx, cmd); err != nil {
		// need to shutdown the server if the command fails, otherwise it will keep running until the file is fully downloaded
		if _, shutdownErr := http.Get(fmt.Sprintf("http://%s:%d/shutdown", localIP, port)); shutdownErr != nil {
			log.Printf("failed to shutdown server: %v", shutdownErr)
		}
		return "", fmt.Errorf("failed to execute download command: %w", err)
	}

	fmt.Fprintln(os.Stderr)
	return fileURL, nil
}
