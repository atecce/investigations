package lyrics_net

import (
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/atecce/investigations/canvas"
	_ "github.com/mattn/go-sqlite3"
	"github.com/yhat/scrape"
	"golang.org/x/net/html"
)

type Investigator struct {
	URL string

	canvas    *canvas.Canvas
	wg        sync.WaitGroup
	caught_up bool
}

// should only return errors related to HTTP status codes.
// otherwise, it handles all it’s connection errors
// internally
func communicate(url string) (io.ReadCloser, error) {

	// never stop trying
	for {

		// get url
		resp, err := http.Get(url)

		// catch error
		if err != nil {
			log.Println(err)
			time.Sleep(time.Second)
			continue
		}

		// write status to output
		log.Println(url, resp.Status)

		// check status codes
		switch resp.StatusCode {

		// cases which are returned
		case http.StatusOK:
			return resp.Body, nil
		case http.StatusForbidden:
			return nil, errors.New("forbidden")
		case http.StatusNotFound:
			return nil, errors.New("not found")

		// cases which are retried
		case http.StatusServiceUnavailable:
			time.Sleep(10 * time.Minute)
		case http.StatusGatewayTimeout:
			time.Sleep(time.Minute)
		default:
			time.Sleep(time.Minute)
		}
	}
}

func inASCIIupper(start string) bool {
	for _, char := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		if string(char) == string(start[0]) {
			return true
		}
	}
	return false
}

func (investigator *Investigator) Investigate(start string) {

	// initiate db
	investigator.canvas = canvas.New("lyrics_net")

	// use specified start letter
	var expression string
	if inASCIIupper(start) {
		expression = "^/artists/[" + string(start[0]) + "-Z]$"
	} else {
		expression = "^/artists/[0A-Z]$"
	}

	// set regular expression for letter suburls
	letters, _ := regexp.Compile(expression)

	// set body
	b, err := communicate(investigator.URL)
	if err != nil {
		log.Println(investigator.URL, err)
		return
	}
	defer b.Close()

	// parse page
	root, err := html.Parse(b)
	if err != nil {
		log.Fatal(err)
	}

	// find letter urls
	letter_urls := scrape.FindAll(root, func(n *html.Node) bool {
		if n.Data == "a" {
			return letters.MatchString(scrape.Attr(n, "href"))
		}
		return false
	})

	// investigate letters
	for _, url := range letter_urls {
		letter_url := investigator.URL + scrape.Attr(url, "href") + "/99999"
		investigator.getArtists(start, letter_url)
	}
}

func (investigator *Investigator) getArtists(start, letter_url string) {

	// set caught up expression
	expression, _ := regexp.Compile("^" + start + ".*$")
	if start == "0" {
		investigator.caught_up = true
	}

	// set regular expression for letter suburls
	artists, _ := regexp.Compile("^artist/.*$")

	// set body
	b, err := communicate(letter_url)
	if err != nil {
		log.Println(letter_url, err)
		return
	}
	defer b.Close()

	// parse page
	root, err := html.Parse(b)
	if err != nil {
		log.Fatal(err)
	}

	// find artist urls
	artist_links := scrape.FindAll(root, func(n *html.Node) bool {
		if n.Parent != nil {
			return n.Parent.Data == "strong" && n.Data == "a"
		}
		return false
	})
	for _, link := range artist_links {

		artist_suburl := scrape.Attr(link, "href")

		if artists.MatchString(artist_suburl) {

			// concatenate artist url
			artist_url := investigator.URL + "/" + artist_suburl

			// extract artist name
			var artist_name string
			if link.FirstChild != nil {
				artist_name = link.FirstChild.Data
			}

			// check if caught up
			if expression.MatchString(artist_name) {
				investigator.caught_up = true
			}
			if !investigator.caught_up {
				continue
			}

			// parse the artist
			investigator.parseArtist(artist_url, artist_name)
		}
	}
}

func (investigator *Investigator) parseArtist(artist_url, artist_name string) {

	// initialize artist flag
	var artistAdded bool

	// set body
	b, err := communicate(artist_url)
	if err != nil {
		log.Println(artist_url, err)
		return
	}
	defer b.Close()

	// parse page
	z := html.NewTokenizer(b)
	for {
		switch z.Next() {

		// end of html document
		case html.ErrorToken:
			return

		// catch start tags
		case html.StartTagToken:

			// set token
			t := z.Token()

			// look for artist album labels
			if t.Data == "h3" {
				for _, a := range t.Attr {
					if a.Key == "class" && a.Val == "artist-album-label" {

						// add artist
						if !artistAdded {
							investigator.canvas.AddArtist(artist_name)
							artistAdded = true
						}

						// album links are next token
						var album_url string
						z.Next()
						for _, album_attribute := range z.Token().Attr {
							if album_attribute.Key == "href" {
								album_url = investigator.URL + album_attribute.Val
							}
						}

						// album titles are the next token
						z.Next()
						album_title := z.Token().Data

						// add album
						investigator.canvas.AddAlbum(artist_name, album_title)

						// parse album
						if dorothy := investigator.parseAlbum(album_url, album_title); dorothy {
							investigator.no_place(album_title, z)
						}
					}
				}
			}
		}
	}
}

func (investigator *Investigator) no_place(album_title string, z *html.Tokenizer) {

	// parse album from artist page
	for {
		z.Next()
		t := z.Token()
		switch t.Data {

		// check for finished album
		case "div":

			for _, a := range t.Attr {
				if a.Key == "class" && a.Val == "clearfix" {
					investigator.wg.Wait()
					return
				}
			}

		// check for song links
		case "strong":

			z.Next()

			for _, a := range z.Token().Attr {
				if a.Key == "href" {

					// concatenate the url
					song_url := investigator.URL + a.Val

					// next token is artist name
					z.Next()
					song_title := z.Token().Data

					// parse song
					investigator.wg.Add(1)
					go investigator.parseSong(song_url, song_title, album_title)
				}
			}
		}
	}
}

func (investigator *Investigator) parseAlbum(album_url, album_title string) bool {

	// set body
	b, err := communicate(album_url)
	if err != nil {
		log.Println(album_url, err)
		return false
	}
	defer b.Close()

	// parse page
	root, err := html.Parse(b)
	if err != nil {
		log.Fatal(err)
	}

	// check for home page
	if _, dorothy := scrape.Find(root, func(n *html.Node) bool {
		return n.Data == "body" && scrape.Attr(n, "id") == "s4-page-homepage"
	}); dorothy {
		return true
	}

	// find song links
	song_links := scrape.FindAll(root, func(n *html.Node) bool {
		if n.Parent != nil {
			return n.Parent.Data == "strong" && n.Data == "a"
		}
		return false
	})

	// return if no songs
	if len(song_links) == 0 {
		return true
	}

	// scrape links
	for _, link := range song_links {
		song_url := investigator.URL + scrape.Attr(link, "href")

		// title is first child
		var song_title string
		if link.FirstChild != nil {
			song_title = link.FirstChild.Data
		} else {
			panic(err)
		}

		// parse songs
		investigator.wg.Add(1)
		go investigator.parseSong(song_url, song_title, album_title)
	}

	// wait for songs
	investigator.wg.Wait()
	return false
}

func (investigator *Investigator) parseSong(song_url, song_title, album_title string) {

	// finish job at the end of function call
	defer investigator.wg.Done()

	// set body
	b, err := communicate(song_url)
	if err != nil {
		log.Println(song_url, err)
		return
	}
	defer b.Close()

	// parse page
	root, err := html.Parse(b)
	if err != nil {
		if operr, ok := err.(*net.OpError); ok {
			if operr.Err.Error() == syscall.ECONNRESET.Error() {
				investigator.wg.Add(1)
				investigator.parseSong(song_url, song_title, album_title)
				return
			}
		}
		panic(err)
	}

	// extract lyrics
	if lyrics_root, ok := scrape.Find(root, func(n *html.Node) bool {
		return n.Data == "pre" && scrape.Attr(n, "id") == "lyric-body-text"
	}); ok {
		lyrics := scrape.Text(lyrics_root)
		investigator.canvas.AddSong(album_title, song_title, lyrics)
	}
}