package main

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type Video struct {
	Date string `json:"Date"`
	Link string `json:"Link"`
}

func parseArchive(archivePath string) ([]Video, error) {
	var videos []Video

	archive, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()

	scanner := bufio.NewScanner(archive)
	var currentVideo Video
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Date: ") {
			if currentVideo.Link != "" {
				videos = append(videos, currentVideo)
			}
			currentVideo = Video{Date: line[len("Date: "):]}
		} else if strings.HasPrefix(line, "Link: ") {
			currentVideo.Link = line[len("Link: "):]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if currentVideo.Link != "" {
		videos = append(videos, currentVideo)
	}

	return videos, nil
}

func downloadVideos(videos []Video) error {
	for _, video := range videos {
		err := downloadVideo(video)
		if err != nil {
			return err
		}
	}
	return nil
}

func downloadVideo(video Video) error {
	resp, err := http.Get(video.Link)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	filename := video.Date[:10] + "-" + strings.ReplaceAll(video.Date[11:], ":", "-") + ".mp4"
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("TikTok Downloader")
	input := widget.NewEntry()
	input.SetPlaceHolder("Enter path to TikTok archive")

	output := widget.NewLabel("")

	button := widget.NewButton("Download", func() {
		archivePath := input.Text
		if archivePath == "" {
			dialog.ShowError(errors.New("Archive path is empty"), myWindow)
			return
		}
		videos, err := parseArchive(archivePath)
		if err != nil {
			dialog.ShowError(err, myWindow)
			return
		}
		err = downloadVideos(videos)
		if err != nil {
			dialog.ShowError(err, myWindow)
			return
		}
		output.SetText("Download complete!")
	})

	container := fyne.NewContainerWithLayout(layout.NewVBoxLayout(), input, button, output)
	myWindow.SetContent(container)
	myWindow.Resize(fyne.NewSize(400, 200))
	myWindow.ShowAndRun()
}
