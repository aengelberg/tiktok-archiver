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
	"fyne.io/fyne/v2/data/binding"
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
		}
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
	ProgressState binding.Float
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += int64(n)
	floatValue := float64(wc.Total) / float64(wc.ContentLength)
	wc.ProgressState.Set(floatValue)
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

type FileState struct {
	FileName string
	Progress float64
	Status   string // "queued", "in progress", "succeeded", or "failed"
}

type AppState struct {
	window         fyne.Window
	cancel         *context.CancelFunc
	inputFile      binding.String
	outputDir      binding.String
	fileType       binding.String
	skipExisting   binding.Bool
	globalProgress binding.Float
	fileStates     binding.UntypedList // each element is a binding.DataMap that mirrors FileState
}

func main() {
	a := app.New()
	w := a.NewWindow("TikTok Video Downloader")

	appState := AppState{
		window:         w,
		cancel:         nil,
		inputFile:      binding.NewString(),
		outputDir:      binding.NewString(),
		fileType:       binding.NewString(),
		skipExisting:   binding.NewBool(),
		globalProgress: binding.NewFloat(),
		fileStates:     binding.NewUntypedList(),
	}

	createUI(appState)

	w.ShowAndRun()
}

func createUI(appState AppState) {
	// UI elements
	inputButton := widget.NewButton("Select Input File", nil)
	outputButton := widget.NewButton("Select Output Directory", nil)
	inputLabel := widget.NewLabelWithData(appState.inputFile)
	outputLabel := widget.NewLabelWithData(appState.outputDir)
	fileTypeSelect := widget.NewSelect([]string{"Posts.txt", "user_data.json"}, func(fileType string) {
		appState.fileType.Set(fileType)
	})
	appState.fileType.AddListener(binding.NewDataListener(func() {
		fileType, _ := appState.fileType.Get()
		fileTypeSelect.SetSelected(fileType)
	}))
	downloadButton := widget.NewButton("Download", nil)
	cancelButton := widget.NewButton("Cancel", nil)
	skipExistingCheckbox := widget.NewCheckWithData("Skip already-downloaded files", appState.skipExisting)
	appState.skipExisting.Set(true)
	progressBar := widget.NewProgressBarWithData(appState.globalProgress)

	// Create a container to hold individual download items
	fileList := newFileListWidget(appState)
	scrollContainer := container.NewVScroll(fileList)
	scrollContainer.SetMinSize(fyne.NewSize(400, 400))

	content := container.NewVBox(
		container.NewHBox(inputButton, inputLabel),
		container.NewHBox(outputButton, outputLabel),
		fileTypeSelect,
		skipExistingCheckbox,
		downloadButton,
		cancelButton,
		progressBar,
		widget.NewLabel("Individual Downloads:"),
		scrollContainer, // Add the scroll container to the main UI
	)

	inputButton.OnTapped = func() {
		selectInputFile(appState)
	}
	outputButton.OnTapped = func() {
		selectOutputDir(appState)
	}
	downloadButton.OnTapped = func() {
		downloadFiles(appState)
	}
	cancelButton.OnTapped = func() {
		cancelDownloads(appState)
	}

	appState.window.SetContent(content)
}

func newFileListWidget(appState AppState) fyne.Widget {
	return widget.NewListWithData(appState.fileStates,
		func() fyne.CanvasObject {
			fileNameLabel := widget.NewLabel("")
			statusIcon := widget.NewIcon(nil)
			progressBar := widget.NewProgressBar()
			return container.NewHBox(statusIcon, fileNameLabel, progressBar)
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			hbox := obj.(*fyne.Container)
			hbox.RemoveAll()
			dataMap := item.(binding.DataMap)
			fileNameObj, _ := dataMap.GetItem("FileName")
			fileNameState := fileNameObj.(binding.String)
			progressObj, _ := dataMap.GetItem("Progress")
			progressState := progressObj.(binding.Float)
			statusObj, _ := dataMap.GetItem("Status")
			statusState := statusObj.(binding.String)

			icon := widget.NewIcon(nil)
			statusState.AddListener(binding.NewDataListener(func() {
				status, _ := statusState.Get()
				icon.SetResource(getStatusIcon(status))
			}))
			label := widget.NewLabelWithData(fileNameState)
			progressBar := widget.NewProgressBarWithData(progressState)
			hbox.Add(icon)
			hbox.Add(label)
			hbox.Add(progressBar)
		},
	)
}

func selectInputFile(appState AppState) {
	fd := dialog.NewFileOpen(func(file fyne.URIReadCloser, err error) {
		if err == nil && file != nil {
			appState.inputFile.Set(file.URI().Path())
		}
	}, appState.window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".txt", ".json"}))
	fd.Show()
}

func selectOutputDir(appState AppState) {
	fd := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
		if err == nil && dir != nil {
			appState.outputDir.Set(dir.Path())
		}
	}, appState.window)
	fd.Show()
}

func getStatusIcon(status string) fyne.Resource {
	switch status {
	case "queued":
		return theme.FileVideoIcon()
	case "in progress":
		return theme.DownloadIcon()
	case "succeeded":
		return theme.ConfirmIcon()
	case "failed":
		return theme.CancelIcon()
	}
	return nil
}

func downloadFiles(appState AppState) {
	go func() {
		filePath, _ := appState.inputFile.Get()
		fileType, _ := appState.fileType.Get()
		outputDir, _ := appState.outputDir.Get()
		skipExisting, _ := appState.skipExisting.Get()
		// Read and parse the input file
		links, err := readAndParseFile(filePath, fileType)
		if err != nil {
			dialog.ShowError(err, appState.window)
			return
		}

		type downloadableFile struct {
			state    binding.DataMap
			path     string
			name     string
			status   binding.String
			progress binding.Float
		}

		downloadableFiles := make([]downloadableFile, len(links))
		dataMaps := make([]interface{}, len(links))

		for i, link := range links {
			fileName := fmt.Sprintf("%s.mp4", strings.Replace(strings.Replace(link.Date, " ", "-", -1), ":", "-", -1))
			filePath := filepath.Join(outputDir, fileName)
			stateMap := binding.BindStruct(FileState{FileName: fileName, Progress: 0, Status: "queued"})
			statusObj, _ := stateMap.GetItem("Status")
			progressObj, _ := stateMap.GetItem("Progress")
			dataMaps[i] = stateMap
			downloadableFiles[i] = downloadableFile{
				state:    stateMap,
				name:     fileName,
				path:     filePath,
				status:   statusObj.(binding.String),
				progress: progressObj.(binding.Float),
			}
		}

		appState.fileStates.Set(dataMaps)

		workerPool := make(chan struct{}, 4)
		downloadWg := sync.WaitGroup{}

		ctx, cancel := context.WithCancel(context.Background())

		appState.cancel = &cancel

		for i, link := range links {
			file := downloadableFiles[i]
			filePath := file.path
			fileName := file.name

			workerPool <- struct{}{} // Acquire a worker from the pool

			appState.globalProgress.Set(float64(i) / float64(len(links)))

			select {
			case <-ctx.Done():
				fmt.Printf("Downloads cancelled.\n")
				return
			default:
			}

			go func(i int) {
				defer func() { <-workerPool }() // Release the worker back to the pool
				defer downloadWg.Done()

				if skipExisting {
					if _, err := os.Stat(filePath); err == nil {
						fmt.Printf("%s already exists. Skipping...\n", fileName)
						file.status.Set("skipped")
						file.progress.Set(1.0)
						return
					}
				}

				fmt.Printf("Downloading %s...\n", fileName)
				wc := &WriteCounter{
					ProgressState: file.progress,
				}
				file.status.Set("in progress")
				err := downloadFile(ctx, link.Link, filePath, wc)
				if err != nil {
					fmt.Printf("Failed to download %s: %v\n", fileName, err)
					file.status.Set("failed")
				} else {
					fmt.Printf("Downloaded %s successfully.\n", fileName)
					file.status.Set("succeeded")
				}
				downloadWg.Done()
			}(i)
		}
		downloadWg.Wait()
		fmt.Printf("All downloads completed.\n")
	}()
}

func cancelDownloads(appState AppState) {
	if appState.cancel != nil {
		(*appState.cancel)()
	}
}
