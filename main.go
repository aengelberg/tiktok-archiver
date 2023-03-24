package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

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
	progress := widget.NewProgressBar()
	startBtn := widget.NewButton("Start Download", func() {
		err := downloadVideos(input.Text, progress)
		if err != nil {
			dialog.ShowError(err, w)
		}
	})

	content := container.NewVBox(
		input,
		startBtn,
		progress,
	)

	w.SetContent(content)
	w.Resize(fyne.NewSize(400, 150))
	w.ShowAndRun()
}

func downloadVideos(filepath string, progress *widget.ProgressBar) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	datePattern := regexp.MustCompile(`^Date:\s+(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`)
	linkPattern := regexp.MustCompile(`^Link:\s+(.+)`)

	var date, link string
	var links []string
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
				links = append(links, fmt.Sprintf("%s %s", date, link))
				date = ""
				link = ""
			}
		}
	}

	progress.Max = float64(len(links))
	progress.SetValue(0)

	var wg sync.WaitGroup
	failedLinks := make(chan string, len(links))

	for _, link := range links {
		wg.Add(1)
		go func(link string) {
			defer wg.Done()

			parts := strings.Split(link, " ")
			date, url := parts[0], parts[1]

			if err := downloadVideo(date, url); err != nil {
				failedLinks <- fmt.Sprintf("%s %s", date, url)
			}

			progress.SetValue(progress.Value + 1)
		}(link)
	}

	wg.Wait()
	close(failedLinks)

	var failed []string
	for link := range failedLinks {
		failed = append(failed, link)
	}

	if len(failed) > 0 {
		return fmt.Errorf("Failed to download the following videos:\n%s", strings.Join(failed, "\n"))
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
