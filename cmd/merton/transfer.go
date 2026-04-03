package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/investigato/go-psrp/client"
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

func cleanWindowsPath(path string) string {
	// for downloading FROM windows, need to remove all '/' and replace with '\'
	cleanPath := strings.ReplaceAll(path, "/", "\\")
	return cleanPath
}

func buildDownloadPaths(cwd, localPath, remotePath string) (resolvedLocalPath string, resolvedRemotePath string, err error) {
	// does the remote path start with a drive letter followed by a colon?
	driveLetter := ""
	if len(remotePath) > 1 && remotePath[1] == ':' {
		driveLetter = remotePath[:2]
	} else // get it from the cwd if it exists
	if len(cwd) > 1 && cwd[1] == ':' {
		driveLetter = cwd[:2]
	}
	// get the rest of the path after the drive letter, if it exists EXCEPT for the last part which could be a file or directory, we will handle that separately

	// split the rest of the path along each separator
	restOfPath := remotePath
	if driveLetter != "" {
		restOfPath = remotePath[2:]
	}

	// split into parts
	parts := strings.FieldsFunc(restOfPath, func(r rune) bool {
		return r == '\\' || r == '/'
	})
	lastPart := ""
	if len(parts) > 0 {
		lastPart = parts[len(parts)-1]
	}
	// make sure lastPart has an extension, then build string
	if filepath.Ext(lastPart) != "" {
		resolvedRemotePath = filepath.Join(driveLetter, restOfPath)
	}
	// for downloading FROM windows, need to remove all '/' and replace with '\'
	resolvedRemotePath = cleanWindowsPath(resolvedRemotePath)

	// if no local path, use current directory + filename as local path
	if localPath == "" {
		currentDir, _ := os.Getwd()
		resolvedLocalPath = filepath.Join(currentDir, lastPart)
	} else {
		// if path is relative, convert to absolute path on the local linux machine
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			// get the absolute path to whatever what entered and that will be the final destination, even without an extension
			absPath, err := filepath.Abs(localPath)
			if err != nil {
				return "", "", fmt.Errorf("failed to get absolute path: %v", err)
			}
			resolvedLocalPath = absPath
		} else {
			// for windows, if the path is relative, we will just use it as is and let the windows shell resolve it, this is because windows has different rules for resolving relative paths and it can be confusing to try to replicate that logic in go, especially with drive letters and UNC paths
			resolvedLocalPath = localPath
		}

	}
	return resolvedLocalPath, resolvedRemotePath, nil
}

func buildUploadPaths(cwd, localPath, remotePath string) (resolvedLocalPath string, resolvedRemotePath string, err error) {
	// if remote path is not provided, use the filename in the current directory as the remote path
	// does the localPath exist, it can be absolute or relative
	if _, err := os.Stat(localPath); err != nil {
		return "", "", fmt.Errorf("local file does not exist: %v", err)
	}
	if remotePath == "" {
		currentDir := ""
		if len(cwd) > 0 {
			currentDir = strings.Trim(cwd, "[]")
		} else {
			currentDir = "."
		}
		remotePath = filepath.Join(currentDir, filepath.Base(localPath))

		remotePath = cleanWindowsPath(remotePath)
		return localPath, remotePath, nil
	}
	driveLetter := ""
	if len(remotePath) > 1 && remotePath[1] == ':' {
		driveLetter = remotePath[:2]
	} else // get it from the cwd if it exists
	if len(cwd) > 1 && cwd[1] == ':' {
		driveLetter = cwd[:2]
	}
	// get the rest of the path after the drive letter, if it exists
	restOfPath := remotePath
	if driveLetter != "" {
		restOfPath = remotePath[2:]
	}

	// if it looks like a directory, append the filename to it
	if strings.HasSuffix(restOfPath, "\\") || strings.HasSuffix(restOfPath, "/") || (restOfPath == "" || filepath.Ext(restOfPath) == "") {
		resolvedRemotePath = filepath.Join(driveLetter, restOfPath, filepath.Base(localPath))
	} else {
		// otherwise, just join the drive letter and the rest of the path
		resolvedRemotePath = filepath.Join(driveLetter, restOfPath)
	}
	// for downloading FROM windows, need to remove all '/' and replace with '\'
	resolvedRemotePath = cleanWindowsPath(resolvedRemotePath)
	return localPath, resolvedRemotePath, nil

}

func validatePaths(method string, paths []string, cwd string) (string, string, error) {
	switch method {
	case "download":
		if len(paths) < 1 {
			return "", "", fmt.Errorf("usage: download <remote_path> [local_path]")
		}
		remotePath := paths[0]
		localPath := ""
		if len(paths) >= 2 {
			localPath = paths[1]
		}
		return buildDownloadPaths(cwd, localPath, remotePath)
	case "upload":
		if len(paths) < 1 {
			return "", "", fmt.Errorf("usage: upload <local_path> [remote_path]")
		}
		localPath := paths[0]
		// localPath can be relative or absolute, make sure file exists before proceeding
		if _, err := os.Stat(localPath); err != nil {
			return "", "", fmt.Errorf("local file does not exist: %v", err)
		}
		remotePath := ""
		if len(paths) >= 2 {
			remotePath = paths[1]
		}
		return buildUploadPaths(cwd, localPath, remotePath)
	default:
		return "", "", fmt.Errorf("invalid method: %s", method)
	}
}

func validateAbsolutePath(path string) bool {
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		// to validate this path FROM linux, we need to remove drive letter and replace all '\' with '/' and check if the path exists, if it does, we can assume it's an absolute path in windows and return the original path, if it doesn't, we can assume it's a relative path and return an error
		cleanPath := strings.ReplaceAll(path[2:], "\\", "/")
		path = cleanPath
	}
	return filepath.IsAbs(path)
}
func Download(ctx context.Context, sh *MertonShell, c *client.Client, line string) {
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
	localPath, remotePath, err := validatePaths("download", args, sh.cmdCWD)
	if err != nil {
		log.Printf("Failed to validate paths: %v", err)
		return
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
	fetchErr := c.FetchFile(ctx, remotePath, localPath, options...)
	if progressErr := bar.Finish(); progressErr != nil {
		log.Printf("Failed to finish progress bar: %v", progressErr)
	}
	if fetchErr != nil {
		log.Printf("\nDownload failed: %v", fetchErr)
	} else {
		fmt.Printf("\nDownloaded %s to %s\n", remotePath, localPath)
	}
}

func Upload(ctx context.Context, sh *MertonShell, c *client.Client, line string, hostname string, port int) {
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
	localPath, remotePath, err := validatePaths("upload", args, sh.cmdCWD)
	if err != nil {
		log.Printf("Failed to validate paths: %v", err)
		return
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

	mux.HandleFunc("/"+filepath.Base(filePath), func(w http.ResponseWriter, _ *http.Request) {
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
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, _ *http.Request) {
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
		if err := server.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
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
	//goland:noinspection HttpUrlsUsage
	fileURL := fmt.Sprintf("http://%s:%d/%s", localIP, port,
		filepath.Base(localPath))
	_, err = fmt.Fprintf(os.Stderr, "Serving %s from %s\n", filepath.Base(localPath),
		fileURL)
	if err != nil {
		return "", err
	}

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
		// need to shut down the server if the command fails, otherwise it will keep running until the file is fully downloaded
		if //goland:noinspection HttpUrlsUsage
		resp, shutdownErr := http.Get(fmt.Sprintf("http://%s:%d/shutdown", localIP, port)); shutdownErr != nil {
			log.Printf("failed to shutdown server: %v", shutdownErr)
		} else {
			if closeErr := resp.Body.Close(); closeErr != nil {
				log.Printf("failed to close shutdown response body: %v", closeErr)
			}
		}

		return "", fmt.Errorf("failed to execute download command: %w", err)
	}

	if _, err := fmt.Fprintln(os.Stderr); err != nil {
		return "", fmt.Errorf("failed to write newline to stderr: %w", err)
	}
	return fileURL, nil
}
