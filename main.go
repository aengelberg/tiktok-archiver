package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type UserData struct {
	Video struct {
		Videos struct {
			VideoList []struct {
				Date  string `json:"Date"`
				Link  string `json:"Link"`
				Likes int    `json:"Likes"`
			} `json:"VideoList"`
		} `json:"Videos"`
	} `json:"Video"`
}

func downloadFile(url string, filepath string, progress *widget.ProgressBar, logOutput *widget.Entry) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	counter := &WriteCounter{ProgressBar: progress}
	_, err = io.Copy(out, io.TeeReader(resp.Body, counter))
	if err != nil {
		return err
	}

	return nil
}

type WriteCounter struct {
	Total       int64
	ProgressBar *widget.ProgressBar
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += int64(n)
	wc.ProgressBar.SetValue(float64(wc.Total) / float64(wc.ProgressBar.Max))
	return n, nil
}

type VideoLink struct {
	Date string
	Link string
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

	return links, nil
}

func main() {
	a := app.New()
	w := a.NewWindow("TikTok Video Downloader")

	// UI elements
	filePathInput := widget.NewEntry()
	filePathInput.SetPlaceHolder("Path to Posts.txt or user_data.json")
	fileTypeSelect := widget.NewSelect([]string{"Posts.txt", "user_data.json"}, nil)
	downloadButton := widget.NewButton("Download", nil)
	progressBar := widget.NewProgressBar()
	logOutput := widget.NewMultiLineEntry()

	content := container.NewVBox(
		filePathInput,
		fileTypeSelect,
		downloadButton,
		progressBar,
		widget.NewLabel("Log:"),
		logOutput,
	)
	w.SetContent(content)

	// Download button action
	downloadButton.OnTapped = func() {
		filePath := filePathInput.Text
		fileType := fileTypeSelect.Selected

		// Read and parse the input file
		links, err := readAndParseFile(filePath, fileType)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		// Download videos
		downloadPath, err := os.Getwd()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		progressBar.Min = 0
		progressBar.Max = float64(len(links))

		for i, link := range links {
			filename := fmt.Sprintf("%s.mp4", strings.Replace(link.Date, ":", "-", -1))
			filePath := filepath.Join(downloadPath, filename)

			logOutput.SetText(fmt.Sprintf("Downloading %s...\n", filename))
			err := downloadFile(link.Link, filePath, progressBar, logOutput)
			if err != nil {
				logOutput.SetText(fmt.Sprintf("Failed to download %s\n", filename))
			} else {
				logOutput.SetText(fmt.Sprintf("Downloaded %s\n", filename))
			}
			progressBar.SetValue(float64(i + 1))
		}

		logOutput.SetText("Finished downloading videos.")
	}

	w.ShowAndRun()
}
