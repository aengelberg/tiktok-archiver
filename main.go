package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
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

type download struct {
	name     binding.String
	progress binding.Float
	status   binding.String // "queued", "in progress", "succeeded", or "failed"
}

type downloadState struct {
	data   []download
	widget *widget.List
}

type appState struct {
	window         fyne.Window
	inputFile      binding.String
	outputDir      binding.String
	fileType       binding.String
	skipExisting   binding.Bool
	globalProgress binding.Float
	// Mutable state, to work around the limitations of Fyne's data binding.
	downloads *downloadState

	isDownloading binding.Bool
	cancelHook    *atomic.Value
	// Lock for the state transition between "not downloading" and "downloading". When this is locked, `cancelHook`
	// and `isDownloading` are being updated at the same time.
	lock sync.Mutex
}

func main() {
	a := app.NewWithID("com.aengelberg.ttdl")
	w := a.NewWindow("TikTok Video Downloader")

	appState := appState{
		window:         w,
		inputFile:      binding.BindPreferenceString("inputFile", a.Preferences()),
		outputDir:      binding.BindPreferenceString("outputDir", a.Preferences()),
		fileType:       binding.BindPreferenceString("fileType", a.Preferences()),
		skipExisting:   binding.NewBool(),
		globalProgress: binding.NewFloat(),
		downloads: &downloadState{
			data:   []download{},
			widget: nil,
		},

		isDownloading: binding.NewBool(),
		cancelHook:    &atomic.Value{},
		lock:          sync.Mutex{},
	}

	createUI(appState)

	w.ShowAndRun()
}

func createUI(appState appState) {
	// UI elements
	inputButton := widget.NewButton("Select Input File", nil)
	outputButton := widget.NewButton("Select Output Directory", nil)
	inputLabel := widget.NewLabelWithData(appState.inputFile)
	outputLabel := widget.NewLabelWithData(appState.outputDir)
	initialFileType, _ := appState.fileType.Get()
	fileTypeSelect := widget.NewSelect([]string{"Posts.txt", "user_data.json"}, func(fileType string) {
		appState.fileType.Set(fileType)
	})
	fileTypeSelect.SetSelected(initialFileType)
	downloadButton := widget.NewButton("Download", nil)
	cancelButton := widget.NewButton("Cancel", nil)
	appState.isDownloading.AddListener(binding.NewDataListener(func() {
		isDownloading, _ := appState.isDownloading.Get()
		if isDownloading {
			downloadButton.Disable()
			cancelButton.Enable()
		} else {
			downloadButton.Enable()
			cancelButton.Disable()
		}
	}))
	skipExistingCheckbox := widget.NewCheckWithData("Skip already-downloaded files", appState.skipExisting)
	appState.skipExisting.Set(true)
	progressBar := widget.NewProgressBarWithData(appState.globalProgress)

	// Create a container to hold individual download items
	downloadList := newDownloadListWidget(appState)
	appState.downloads.widget = downloadList
	scrollContainer := container.NewVScroll(downloadList)
	scrollContainer.SetMinSize(fyne.NewSize(400, 400))

	leftSide := container.NewVBox(
		container.NewHBox(inputButton, inputLabel),
		container.NewHBox(outputButton, outputLabel),
		fileTypeSelect,
		skipExistingCheckbox,
		downloadButton,
		cancelButton,
	)

	rightSide := container.NewBorder(
		container.NewVBox(
			progressBar,
			widget.NewLabel("Individual Downloads:"),
		),
		nil, nil, nil,
		scrollContainer,
	)

	content := container.NewBorder(
		nil, nil, nil,
		container.NewHBox(
			canvas.NewLine(color.White),
			rightSide,
		),
		leftSide,
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

func newDownloadListWidget(appState appState) *widget.List {
	return widget.NewList(
		func() int {
			return len(appState.downloads.data)
		},
		func() fyne.CanvasObject {
			statusIcon := widget.NewIcon(getStatusIcon("queued"))
			fileNameLabel := widget.NewLabel("")
			progressBar := widget.NewProgressBar()
			return container.NewHBox(statusIcon, fileNameLabel, progressBar)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			hbox := obj.(*fyne.Container)
			statusIcon := hbox.Objects[0].(*widget.Icon)
			fileNameLabel := hbox.Objects[1].(*widget.Label)
			progressBar := hbox.Objects[2].(*widget.ProgressBar)

			download := appState.downloads.data[id]

			download.status.AddListener(binding.NewDataListener(func() {
				status, _ := download.status.Get()
				statusIcon.SetResource(getStatusIcon(status))
			}))
			fileNameLabel.Bind(download.name)
			progressBar.Bind(download.progress)
		},
	)
}

func selectInputFile(appState appState) {
	fd := dialog.NewFileOpen(func(file fyne.URIReadCloser, err error) {
		if err == nil && file != nil {
			appState.inputFile.Set(file.URI().Path())
		}
	}, appState.window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".txt", ".json"}))
	fd.Show()
}

func selectOutputDir(appState appState) {
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

func downloadFiles(appState appState) {
	appState.lock.Lock()
	defer appState.lock.Unlock()
	if isDownloading, _ := appState.isDownloading.Get(); isDownloading {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	appState.cancelHook.Swap(cancel)
	appState.isDownloading.Set(true)
	go func() {
		inputFilePath, _ := appState.inputFile.Get()
		fileType, _ := appState.fileType.Get()
		outputDir, _ := appState.outputDir.Get()
		skipExisting, _ := appState.skipExisting.Get()
		// Read and parse the input file
		links, err := readAndParseFile(inputFilePath, fileType)
		if err != nil {
			dialog.ShowError(err, appState.window)
			return
		}

		type downloadableFile struct {
			download
			path string
			name string
		}

		initialDownloads := make([]download, len(links))
		downloadableFiles := make([]downloadableFile, len(links))

		for i, link := range links {
			fileName := fmt.Sprintf("%s.mp4", strings.Replace(strings.Replace(link.Date, " ", "-", -1), ":", "-", -1))
			filePath := filepath.Join(outputDir, fileName)
			file := download{
				name:     binding.NewString(),
				status:   binding.NewString(),
				progress: binding.NewFloat(),
			}
			file.status.Set("queued")
			file.name.Set(fileName)
			initialDownloads[i] = file
			downloadableFiles[i] = downloadableFile{
				download: file,
				name:     fileName,
				path:     filePath,
			}
		}

		appState.downloads.data = initialDownloads
		appState.downloads.widget.Refresh()

		workerPool := make(chan struct{}, 4)
		downloadWg := sync.WaitGroup{}
		downloadWg.Add(len(links))

		for i, _ := range links {
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
				file := downloadableFiles[i]
				link := links[i]
				defer func() { <-workerPool }() // Release the worker back to the pool
				defer downloadWg.Done()

				if skipExisting {
					if _, err := os.Stat(filePath); err == nil {
						fmt.Printf("%s already exists. Skipping...\n", fileName)
						file.status.Set("succeeded")
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
			}(i)
		}
		downloadWg.Wait()
		fmt.Printf("All downloads completed.\n")
		appState.lock.Lock()
		defer appState.lock.Unlock()
		if isDownloading, _ := appState.isDownloading.Get(); !isDownloading {
			return
		}
		appState.isDownloading.Set(false)
	}()
}

func cancelDownloads(appState appState) {
	appState.lock.Lock()
	defer appState.lock.Unlock()
	if isDownloading, _ := appState.isDownloading.Get(); !isDownloading {
		return
	}
	if cancel := appState.cancelHook.Load(); cancel != nil {
		cancel.(context.CancelFunc)()
	}
	appState.isDownloading.Set(false)
}
