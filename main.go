package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type UserData struct {
	Video struct {
		Videos struct {
			VideoList []struct {
				Date  string `json:"Date"`
				Link  string `json:"Link"`
				Likes string `json:"Likes"`
			} `json:"VideoList"`
		} `json:"Videos"`
	} `json:"Video"`
}

func downloadFile(ctx context.Context, url, filepath string, wc *WriteCounter) error {
	// Create a request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	// Get the data
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the temporary file
	tempFilePath := filepath + ".temp"
	out, err := os.Create(tempFilePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the content length for progress calculation
	contentLength, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return err
	}
	wc.ContentLength = contentLength

	// Write the body to the temporary file with context cancellation check
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			_ = os.Remove(tempFilePath) // Remove the temporary file
			return ctx.Err()
		default:
			n, err := resp.Body.Read(buf)
			if err != nil && err != io.EOF {
				return err
			}
			if n == 0 {
				break
			}
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			wc.Write(buf[:n])
		}
	}

	// Rename the temporary file to the real file
	err = os.Rename(tempFilePath, filepath)
	if err != nil {
		return err
	}

	return nil
}

type WriteCounter struct {
	Total         int64
	ContentLength int64
	ProgressBar   *widget.ProgressBar
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += int64(n)
	floatValue := float64(wc.Total) / float64(wc.ContentLength)
	if floatValue-wc.ProgressBar.Value > 0.01 || wc.Total == wc.ContentLength {
		wc.ProgressBar.SetValue(floatValue)
	}
	return n, nil
}

type VideoLink struct {
	Date string
	Link string
}

func sortLinksByDateDescending(links []VideoLink) {
	sort.Slice(links, func(i, j int) bool {
		return links[i].Date > links[j].Date
	})
}

func readAndParseFile(filePath string, fileType string) ([]VideoLink, error) {
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var links []VideoLink

	switch fileType {
	case "Posts.txt":
		lines := strings.Split(string(fileContent), "\n")
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			if strings.HasPrefix(line, "Date:") {
				date := strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
				i++
				if i < len(lines) && strings.HasPrefix(lines[i], "Link:") {
					link := strings.TrimSpace(strings.TrimPrefix(lines[i], "Link:"))
					links = append(links, VideoLink{Date: date, Link: link})
				}
			}
		}
	case "user_data.json":
		var userData UserData
		err := json.Unmarshal(fileContent, &userData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON file: %v", err)
		}

		for _, video := range userData.Video.Videos.VideoList {
			links = append(links, VideoLink{Date: video.Date, Link: video.Link})
		}
	default:
		return nil, fmt.Errorf("unsupported file type")
	}

	sortLinksByDateDescending(links)

	return links, nil
}

var downloadWg sync.WaitGroup

type DownloadItem struct {
	FileName    string
	FilePath    string
	ProgressBar *widget.ProgressBar
	StatusIcon  *widget.Icon
	Status      string // "queued", "in progress", "succeeded", or "failed"
}

func main() {
	a := app.New()
	w := a.NewWindow("TikTok Video Downloader")

	// UI elements
	inputButton := widget.NewButton("Select Input File", nil)
	outputButton := widget.NewButton("Select Output Directory", nil)
	inputLabel := widget.NewLabel("Input File:")
	outputLabel := widget.NewLabel("Output Directory:")
	fileTypeSelect := widget.NewSelect([]string{"Posts.txt", "user_data.json"}, nil)
	downloadButton := widget.NewButton("Download", nil)
	cancelButton := widget.NewButton("Cancel", nil)
	skipExistingCheckbox := widget.NewCheck("Skip already-downloaded files", nil)
	skipExistingCheckbox.SetChecked(true)
	progressBar := widget.NewProgressBar()
	logOutput := widget.NewMultiLineEntry()

	// Create a container to hold individual download items
	downloadItemsContainer := container.NewVBox()
	scrollContainer := container.NewVScroll(downloadItemsContainer)
	scrollContainer.SetMinSize(fyne.NewSize(400, 400))

	content := container.NewVBox(
		container.NewHBox(inputButton, inputLabel),
		container.NewHBox(outputButton, outputLabel),
		fileTypeSelect,
		skipExistingCheckbox,
		downloadButton,
		cancelButton,
		progressBar,
		widget.NewLabel("Log:"),
		logOutput,
		widget.NewLabel("Individual Downloads:"),
		scrollContainer, // Add the scroll container to the main UI
	)
	w.SetContent(content)

	// Input file button action
	inputButton.OnTapped = func() {
		fd := dialog.NewFileOpen(func(file fyne.URIReadCloser, err error) {
			if err == nil && file != nil {
				inputLabel.SetText(file.URI().Path())
			}
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".txt", ".json"}))
		fd.Show()
	}

	// Output directory button action
	outputButton.OnTapped = func() {
		fd := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
			if err == nil && dir != nil {
				outputLabel.SetText(dir.Path())
			}
		}, w)
		fd.Show()
	}

	// Download button action
	downloadButton.OnTapped = func() {
		go func() {
			filePath := inputLabel.Text
			fileType := fileTypeSelect.Selected
			outputDir := outputLabel.Text

			// Read and parse the input file
			links, err := readAndParseFile(filePath, fileType)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}

			downloadItems := make([]*DownloadItem, len(links))

			progressBar.Min = 0
			progressBar.Max = float64(len(links))

			workerPool := make(chan struct{}, 4)
			downloadWg.Add(len(links))

			for i, link := range links {
				filename := fmt.Sprintf("%s.mp4", strings.Replace(strings.Replace(link.Date, " ", "-", -1), ":", "-", -1))
				filePath := filepath.Join(outputDir, filename)
				// Create a DownloadItem for each file and add it to the downloadItemsContainer
				downloadItem := &DownloadItem{
					FileName:    filename,
					FilePath:    filePath,
					ProgressBar: widget.NewProgressBar(),
					StatusIcon:  widget.NewIcon(nil), // Set the initial icon to nil
					Status:      "queued",
				}
				downloadItemsContainer.Add(container.NewHBox(
					downloadItem.StatusIcon,
					widget.NewLabel(downloadItem.FileName),
					downloadItem.ProgressBar,
				))
				downloadItems[i] = downloadItem
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancelButton.OnTapped = func() {
				cancel()
			}

			for i, link := range links {
				downloadItem := downloadItems[i]
				filename := downloadItem.FileName
				filePath := downloadItem.FilePath

				workerPool <- struct{}{} // Acquire a worker from the pool
				progressBar.SetValue(float64(i + 1))

				select {
				case <-ctx.Done():
					logOutput.SetText(logOutput.Text + "Downloads canceled.\n")
					return
				default:
				}

				go func(i int, link VideoLink) {
					defer func() { <-workerPool }() // Release the worker back to the pool

					if skipExistingCheckbox.Checked {
						if _, err := os.Stat(filePath); err == nil {
							logOutput.SetText(logOutput.Text + fmt.Sprintf("%s already exists. Skipping...\n", filename))
							downloadItem.StatusIcon.SetResource(theme.ConfirmIcon())
							downloadItem.Status = "succeeded"
							downloadItem.ProgressBar.SetValue(1.0)
							downloadWg.Done()
							return
						}
					}

					logOutput.SetText(logOutput.Text + fmt.Sprintf("Downloading %s...\n", filename))

					wc := &WriteCounter{
						ProgressBar: downloadItem.ProgressBar,
					}

					downloadItem.Status = "in progress"
					downloadItem.StatusIcon.SetResource(theme.MediaPlayIcon())

					err := downloadFile(ctx, link.Link, filePath, wc)
					if err != nil {
						logOutput.SetText(logOutput.Text + fmt.Sprintf("Failed to download %s: %v\n", filename, err))
						downloadItem.StatusIcon.SetResource(theme.CancelIcon())
						downloadItem.Status = "failed"
					} else {
						logOutput.SetText(logOutput.Text + fmt.Sprintf("Downloaded %s successfully.\n", filename))
						downloadItem.StatusIcon.SetResource(theme.ConfirmIcon())
						downloadItem.Status = "succeeded"
					}
					downloadWg.Done()
				}(i, link)
			}

			downloadWg.Wait()
			logOutput.SetText(logOutput.Text + "All downloads completed.\n")
		}()
	}

	w.ShowAndRun()
}
