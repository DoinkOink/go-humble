package gohumble

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/feeds"
	log "github.com/sirupsen/logrus"
)

func Run() {
	wg := sync.WaitGroup{}
	for _, category := range []string{"books", "games", "software"} {
		wg.Add(1)
		go func(category string) {
			defer wg.Done()
			resp, err := http.Get(fmt.Sprintf("https://www.humblebundle.com/%s", category))
			if err != nil {
				log.WithFields(log.Fields{"status": resp.StatusCode}).Error(err)
			}
			defer resp.Body.Close()
			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				log.Error(err)
			}
			doc.Find("script#landingPage-json-data").Each(func(idx int, s *goquery.Selection) {
				node := s.Nodes[0]
				data := node.FirstChild.Data
				products, err := parseProducts([]byte(data), category)
				if err != nil {
					log.WithField("state", "parsing").Error(err)
				}
				feed, err := createFeed(products, category)
				if err != nil {
					log.WithField("state", "creating").Error(err)
				}
				if err := writeFeedToFile(feed, category); err != nil {
					log.WithField("state", "writing").Error(err)
				}
			})
		}(category)
	}
	wg.Wait()
}

func createFeed(products []Product, category string) (feeds.Feed, error) {
	feed := feeds.Feed{
		Title:       fmt.Sprintf("Go Humble! RSS %s", strings.ToTitle(category)),
		Link:        &feeds.Link{Href: "https://go-humble.github.io"},
		Description: fmt.Sprintf("Awesome RSS Feeds about HumbleBundle %s bundles!", category),
		Author:      &feeds.Author{Name: "Nils Müller", Email: "shimst3r+rss@gmail.com"},
		Created:     time.Now(),
	}
	feed.Items = make([]*feeds.Item, len(products))
	for idx, product := range products {
		// Need to add a Z in order to make it RFC3339 parseable.
		dt, err := time.Parse(time.RFC3339, product.StartDateDatetime+"Z")
		if err != nil {
			return feeds.Feed{}, err
		}
		feed.Items[idx] = &feeds.Item{
			Title:       product.TileShortName,
			Link:        &feeds.Link{Href: fmt.Sprintf("https://humblebundle.com%s", product.ProductURL)},
			Content:     product.DetailedMarketingBlurb,
			Created:     dt,
			Description: product.ShortMarketingBlurb,
		}
	}
	return feed, nil
}

func parseProducts(data []byte, category string) ([]Product, error) {
	switch category {
	case "books":
		var books BooksData
		err := json.Unmarshal([]byte(data), &books)
		if err != nil {
			return nil, err
		}
		return books.Data.Books.Mosaic[0].Products, nil
	case "games":
		var games GamesData
		err := json.Unmarshal([]byte(data), &games)
		if err != nil {
			return nil, err
		}
		return games.Data.Games.Mosaic[0].Products, nil
	case "software":
		var software SoftwareData
		err := json.Unmarshal([]byte(data), &software)
		if err != nil {
			return nil, err
		}
		return software.Data.Software.Mosaic[0].Products, nil
	default:
		return nil, fmt.Errorf("unknown category %s", category)
	}
}

func writeFeedToFile(feed feeds.Feed, category string) error {
	fileName := fmt.Sprintf("%s.rss", category)
	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	rss, err := feed.ToRss()
	if err != nil {
		return err
	}
	if _, err := w.WriteString(rss); err != nil {
		return err
	}
	return nil
}
