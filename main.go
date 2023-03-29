package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/skratchdot/open-golang/open"
)

var (
	logger      = log.New(os.Stdout, "", log.LstdFlags)
	logFilePath string
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
	window       fyne.Window
	inputFile    binding.String
	outputDir    binding.String
	fileType     binding.String
	skipExisting binding.Bool
	parallelism  binding.Float

	completed      binding.Int
	errors         binding.Int
	skipped        binding.Int
	total          binding.Int
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
	a := app.NewWithID("com.aengelberg.tiktok-archiver")
	w := a.NewWindow("TikTok Archiver")

	newLogger, err := createLogger()
	if err != nil {
		logger.Printf("Failed to create logger: %v", err)
	} else {
		logger = newLogger
	}

	logger.Printf("Starting TikTok Archiver\n")

	appState := appState{
		window:       w,
		inputFile:    binding.BindPreferenceString("inputFile", a.Preferences()),
		outputDir:    binding.BindPreferenceString("outputDir", a.Preferences()),
		fileType:     binding.BindPreferenceString("fileType", a.Preferences()),
		skipExisting: binding.NewBool(),
		parallelism:  binding.BindPreferenceFloat("parallelism", a.Preferences()),

		completed:      binding.NewInt(),
		errors:         binding.NewInt(),
		skipped:        binding.NewInt(),
		total:          binding.NewInt(),
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

func createLogger() (*log.Logger, error) {
	// Generate the filename for the log file.
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(configDir, "TikTok Archiver", "log", fmt.Sprintf("log-%s.txt", time.Now().Format("2006-01-02-15-04-05")))
	logger.Printf("Logging to %s", path)
	err = os.MkdirAll(filepath.Dir(path), 0777)
	if err != nil {
		return nil, err
	}
	// Open the log file for writing. Create it if it doesn't exist.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	logFilePath = path
	newLogger := log.New(io.MultiWriter(file, os.Stdout), "", log.Ldate|log.Ltime)
	return newLogger, nil
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
	parallelismSlider := widget.NewSliderWithData(1, 8, appState.parallelism)
	if initialParallelism, _ := appState.parallelism.Get(); initialParallelism == 0 {
		appState.parallelism.Set(8)
	}
	downloadButton := widget.NewButton("Download", nil)
	downloadButton.SetIcon(theme.DownloadIcon())
	cancelButton := widget.NewButton("Cancel", nil)
	cancelButton.SetIcon(theme.CancelIcon())
	logButton := widget.NewButton("Open Log", func() {
		openLog()
	})
	logButton.SetIcon(theme.DocumentIcon())
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
	skipExistingCheckbox := widget.NewCheckWithData("Skip already-downloaded videos", appState.skipExisting)
	appState.skipExisting.Set(true)
	progressBar := widget.NewProgressBarWithData(appState.globalProgress)

	// Create a container to hold individual download items
	downloadList := newDownloadListWidget(appState)
	appState.downloads.widget = downloadList
	scrollContainer := container.NewVScroll(downloadList)
	scrollContainer.SetMinSize(fyne.NewSize(400, 400))

	errorTracker := canvas.NewText("", color.RGBA{R: 255, A: 255})
	appState.errors.AddListener(binding.NewDataListener(func() {
		errors, _ := appState.errors.Get()
		if errors > 0 {
			errorTracker.Text = fmt.Sprintf("(%d errors)", errors)
		} else {
			errorTracker.Text = ""
		}
	}))

	skipTracker := canvas.NewText("", color.RGBA{R: 102, G: 153, B: 204, A: 255})
	appState.skipped.AddListener(binding.NewDataListener(func() {
		skipped, _ := appState.skipped.Get()
		if skipped > 0 {
			skipTracker.Text = fmt.Sprintf("(%d skipped)", skipped)
		} else {
			skipTracker.Text = ""
		}
	}))

	leftSide := container.NewBorder(
		nil, container.NewVBox(
			downloadButton,
			container.NewGridWithColumns(2, cancelButton, logButton),
		),
		nil, nil,
		container.NewVBox(
			container.NewHBox(inputButton, inputLabel),
			container.NewHBox(outputButton, outputLabel),
			container.NewHBox(widget.NewLabel("File type:"), fileTypeSelect),
			widget.NewAccordion(
				widget.NewAccordionItem("Advanced Options",
					container.NewVBox(
						skipExistingCheckbox,
						container.NewBorder(nil, nil, widget.NewLabel("Parallelism:"), nil,
							container.NewBorder(
								nil, nil, widget.NewLabel("1"), widget.NewLabel("16"),
								parallelismSlider,
							),
						),
					),
				),
			),
		),
	)

	rightSide := container.NewBorder(
		container.NewVBox(
			container.NewHBox(
				widget.NewLabel("Completed:"),
				widget.NewLabelWithData(binding.IntToString(appState.completed)),
				widget.NewLabel("/"),
				widget.NewLabelWithData(binding.IntToString(appState.total)),
				errorTracker,
				skipTracker,
			),
			progressBar,
			widget.NewLabel("Individual Downloads:"),
		),
		nil, nil, nil,
		scrollContainer,
	)

	content := container.NewHSplit(leftSide, rightSide)

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
		return theme.ErrorIcon()
	case "cancelled":
		return theme.CancelIcon()
	}
	return nil
}

// Quick and dirty serialization for incrementing counters
var incLock sync.Mutex

func inc(intState binding.Int) {
	incLock.Lock()
	defer incLock.Unlock()
	val, _ := intState.Get()
	intState.Set(val + 1)
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

		appState.completed.Set(0)
		appState.errors.Set(0)
		appState.skipped.Set(0)
		appState.total.Set(len(links))

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

		parallelismFloat, _ := appState.parallelism.Get()
		workerPool := make(chan struct{}, int64(parallelismFloat))
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
				logger.Printf("Downloads cancelled.\n")
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
						logger.Printf("%s already exists. Skipping...\n", fileName)
						file.status.Set("succeeded")
						file.progress.Set(1.0)
						inc(appState.completed)
						inc(appState.skipped)
						return
					}
				}

				logger.Printf("Downloading %s...\n", fileName)
				wc := &WriteCounter{
					ProgressState: file.progress,
				}
				file.status.Set("in progress")
				err := downloadFile(ctx, link.Link, filePath, wc)
				if err != nil {
					if err == context.Canceled {
						logger.Printf("Download of %s cancelled.\n", fileName)
						file.status.Set("cancelled")
						return
					}
					logger.Printf("Failed to download %s: %v\n", fileName, err)
					file.status.Set("failed")
					inc(appState.completed)
					inc(appState.errors)
				} else {
					logger.Printf("Downloaded %s successfully.\n", fileName)
					file.status.Set("succeeded")
					inc(appState.completed)
				}
			}(i)
		}
		downloadWg.Wait()
		logger.Printf("All downloads completed.\n")
		appState.globalProgress.Set(1.0)
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

func openLog() {
	if logFilePath == "" {
		return
	}
	open.Start(logFilePath)
}
