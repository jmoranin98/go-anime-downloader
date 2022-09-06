package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
)

type episodesBlockResponse []interface{}

func getEpisodeDownloadVideoURL(episodeID string) (string, error) {
	response, err := http.Get("https://jkanime.net/ajax/download_episode/" + episodeID)
	if err != nil {
		return "", err
	}

	defer response.Body.Close()

	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	downloadURL := string(b)
	downloadURL = strings.ReplaceAll(downloadURL, `"`, "")
	downloadURL = strings.ReplaceAll(downloadURL, `\/`, "/")

	finalDownloadURL := "https://jkanime.net" + downloadURL

	return finalDownloadURL, nil
}

func downloadVideo(videoURL string, filePath string) error {
	outputFile, err := os.Create(filePath)
	if err != nil {
		return err
	}

	defer outputFile.Close()

	response, err := http.Get(videoURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	_, err = io.Copy(outputFile, response.Body)
	if err != nil {
		return err
	}

	return nil
}

func getEpisodeID(episodeURL string) (string, error) {
	var err error
	var episodeID string

	c := colly.NewCollector()

	c.OnHTML("div#guardar-capitulo", func(h *colly.HTMLElement) {
		episodeID = h.Attr("data-capitulo")
	})

	c.OnError(func(r *colly.Response, collyError error) {
		err = collyError
	})

	c.Visit(episodeURL)

	if err != nil {
		return "", err
	}

	return episodeID, nil
}

func getNumberOfEpisodes(animeURL string) (int, error) {
	var animeID string
	episodesBlocks := 0
	totalEpisodes := 0

	c := colly.NewCollector()

	c.OnHTML("div#guardar-anime", func(h *colly.HTMLElement) {
		animeID = h.Attr("data-anime")
	})

	c.OnHTML("a.numbers", func(h *colly.HTMLElement) {
		episodesBlocks += 1
	})

	c.Visit(animeURL)

	for i := 1; i <= episodesBlocks; i++ {
		numberOfEpisodes, err := getNumerOfEpisodesFromBlock("https://jkanime.net/ajax/pagination_episodes/" + animeID + "/" + strconv.Itoa(i))
		if err != nil {
			return 0, errors.New("cannot get total number of episodes")
		}

		totalEpisodes += numberOfEpisodes
	}

	return totalEpisodes, nil
}

func getNumerOfEpisodesFromBlock(requestURL string) (int, error) {
	var parsedResponse episodesBlockResponse
	response, err := http.Get(requestURL)
	if err != nil {
		return 0, err
	}

	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&parsedResponse)
	if err != nil {
		return 0, err
	}

	return len(parsedResponse), nil
}

func downloadAnimeEpisodes(animeURL string, numberOfEpisodes int, directory string, prefix string) {
	var wg sync.WaitGroup
	var m sync.Mutex

	episodesDownloaded := 0

	for i := 1; i <= numberOfEpisodes; i++ {
		wg.Add(1)
		episodeIndex := strconv.Itoa(i)

		go func(episodeIndex string) {
			episodeID, err := getEpisodeID(animeURL + "/" + episodeIndex)
			if err != nil {
				fmt.Printf("error getting id of episode %s\n", episodeIndex)
			}

			downloadURL, err := getEpisodeDownloadVideoURL(episodeID)
			if err != nil {
				fmt.Printf("cannot get download video url of episode %s\n", episodeIndex)
			}

			err = downloadVideo(downloadURL, fmt.Sprintf("%s/%s%s.mp4", directory, prefix, episodeIndex))
			if err != nil {
				fmt.Printf("error downloading episode %s\n", episodeIndex)
			}

			m.Lock()
			episodesDownloaded += 1
			fmt.Printf("finished episode %s. %d/%d completed\n", episodeIndex, episodesDownloaded, numberOfEpisodes)
			m.Unlock()
			wg.Done()
		}(episodeIndex)
	}

	wg.Wait()
}

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		fmt.Println("Not provided required arguments")
		os.Exit(2)
	}

	animeURL := args[0]
	if string(animeURL[len(animeURL)-1]) == "/" {
		animeURL = animeURL[:len(animeURL)-1]
	}

	directory, err := filepath.Abs(args[1])
	if err != nil {
		fmt.Printf("Error getting absolute path of the directory")
	}

	prefix := ""
	if len(args) >= 3 {
		prefix = args[2]
	}

	numberOfEpisodes, err := getNumberOfEpisodes(animeURL)
	if err != nil {
		os.Exit(1)
	}
	fmt.Println("Number of episodes", numberOfEpisodes)
	downloadAnimeEpisodes(animeURL, numberOfEpisodes, directory, prefix)
}
