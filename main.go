package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("TikTok Video Downloader")

	input := widget.NewEntry()
	input.SetPlaceHolder("Enter the path to the text file")
	startBtn := widget.NewButton("Start Download", func() {
		err := downloadVideos(input.Text)
		if err != nil {
			dialog.ShowError(err, w)
		} else {
			dialog.ShowInformation("Success", "All videos downloaded successfully!", w)
		}
	})

	content := container.NewVBox(
		input,
		startBtn,
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(400, 100))
	w.ShowAndRun()
}

func downloadVideos(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	datePattern := regexp.MustCompile(`^Date:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`)
	linkPattern := regexp.MustCompile(`^Link:\s+(.+)`)

	var date, link string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)

		if dateMatch := datePattern.FindStringSubmatch(line); dateMatch != nil {
			date = strings.Replace(dateMatch[1], ":", "-", -1)
		} else if linkMatch := linkPattern.FindStringSubmatch(line); linkMatch != nil {
			link = linkMatch[1]
			if date != "" && link != "" {
				if err := downloadVideo(date, link); err != nil {
					return err
				}
				date = ""
				link = ""
			}
		}
	}

	return nil
}

func downloadVideo(date, link string) error {
	resp, err := http.Get(link)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download video: %s", resp.Status)
	}

	output, err := os.Create(fmt.Sprintf("%s.mp4", date))
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, resp.Body)
	return err
}
