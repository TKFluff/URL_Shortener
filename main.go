package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	// "database/sql"

	"github.com/go-sql-driver/mysql"
)

var urls = make(map[string]string)
var serverLoc = "3030"
var homeURL = fmt.Sprintf("http://localhost:%s", serverLoc)
var db *sql.DB

// Database fields to query.
// The key will be the long url for the db. The longURL can possibly show up for multiple shorturls.
type URL struct {
	ShortURL       string
	LongURL        string
	ExpirationDate int64
	Views          int64
}

// Need to test connection still. Updated w/ root password. dbname is subject to change.
// Once connection is made with db, Maybe can decide on things like adding a timestamp that'll determine
// expiration date should we want urls to not be permanently held. Can look into LRU cache as well.
const (
	username = "root"
	password = "Aeiyuyaeae1!"
	hostname = "127.0.0.1:3306"
	dbname   = "URLShortener"
	//Maybe don't need this? Need to figure out connections.
	tablename = "ShortURL"
)

/*
* Need a main page for user to input url
Need a page after that url gets inputted that shows original url and now new url.
Lastly, need a page to process that shortened url and "spit out" original url
*/
func main() {
	http.HandleFunc("/", handleMain)
	http.HandleFunc("/shorturl", handleNewURL)
	http.HandleFunc("/short/", handleRedirect)
	fmt.Println("URL Shortener is running on :" + serverLoc)
	http.ListenAndServe(":"+serverLoc, nil)

	cfg := mysql.Config{
		User:   username,
		Passwd: password,
		Net:    "tcp",
		Addr:   hostname,
		DBName: dbname,
	}
	// Get a database handle.
	var err error

	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	fmt.Println("Connected!")
}

func handleMain(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		http.Redirect(w, r, "/shorturl", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html")

	//personal comments to keep track of things for me.
	//title is what the tab is called.
	//body is what shows up on the page
	//form creates a form and the method determines what kind of http call it is.
	fmt.Fprint(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Simple URL Shortener</title>
		</head>
		<body>
			<h2>URL Shortener</h2>
			<form id="urlForm" method="post" action="/shorturl">
				<div class="field" id="url"
					<label for="url">Original URL:</label>
					<input type="url" name="url" placeholder="Enter a URL" required">
				</div>
				<div class="field" id="urlOption"
					<label for="urlOption">How would you like the url to be generated?</label>
					<select id="urlOption" name="urlOption" onchange = "shortURLVisibility()">
					<option>Auto-Generate</option>
					<option>User Input</option>
					</select>
				</div>
				<div class="field" id="shortenedURL" style="display: none;"
					<label for="shortenedURL"> Shortened URL: http://localhost:3030/short/ </label>
					<input type="text" name="shortenedURL" placeholder="Enter a key for the new url">
				</div>
				<p>
					<input type="submit" value="Shorten">
				</p>

				<script>
					function shortURLVisibility() {
						var shortenedURL = document.getElementById("shortenedURL");
						var visibility = shortenedURL.style.display;
						if (visibility == "none")  {
							shortenedURL.style.display = "block";
						}
						else {
							shortenedURL.style.display = "none";
						}
					}
					</script>
			</form>
		</body>
		</html>
	`)
}

// This function will handle the new url based off the user selection of Auto-Generate or User Input.
// If the key is already found, either a new key will be generated (auto-generate option), or the user will
// be notified that the key is already in use and needs to be re-assigned.
// This is where we want to store the new shorturl into the table with it's respective expiration date,
// longurl, and view count. View count is defaulted to 0.
func handleNewURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Check if the url structure is valid
	originalURL := r.FormValue("url")
	if !isUrl(originalURL) {
		http.Error(w, "Not a valid original url", http.StatusBadRequest)
		return
	}

	// Check if the url is a valid/public site
	if !isReachable(originalURL) {
		http.Error(w, "Not a valid original url", http.StatusBadRequest)
		return
	}

	// Generate a unique shortened key for the original URL
	var shortKey string
	if r.FormValue(("urlOption")) == "Auto-Generate" {
		shortKey = generateShortKey()
		// Make sure that key is not already in the map. If it's in the map, keep generating new keys.
		val := urls[shortKey]
		for val != "" {
			shortKey = generateShortKey()
			val = urls[shortKey]
		}
	} else {
		shortKey = r.FormValue("shortenedURL")
	}

	//Case in which a url requested from the user is already in use.
	//TODO: In the future, can try to make it so that the user is notified in real time
	//from the initial page so that they don't need to submit the form to know that the url is already in use.
	if urls[shortKey] != "" {
		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<head>
				<title>URL Shortener</title>
			</head>
			<body>
				<h2> <a href="`, homeURL, `">`, "URL Shortener", `</a> </h2>
			</body>
			</html>
		`)
		http.Error(w, "URL is already in use", http.StatusBadRequest)
		return
	}
	urls[shortKey] = originalURL

	// Construct the full shortened URL
	shortenedURL := fmt.Sprintf("http://localhost:%s/short/%s", serverLoc, shortKey)

	fmt.Fprint(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>URL Shortener</title>
		</head>
		<body>
			<h2> <a href="`, homeURL, `">`, "URL Shortener", `</a> </h2>
			<p> Original URL: `, originalURL, `</p> 
			<p> Shortened URL: <a href="`, shortenedURL, `">`, shortenedURL, `</a></p>
		</body>
		</html>
	`)
}

// At this redirect, we want to update the view count for the given shorturl.
// Down the line, possibly want to make it so this is no longer updated per redirect,
// but maybe done in "batches". This app probably won't ever get to a point that this is needed though...
func handleRedirect(w http.ResponseWriter, r *http.Request) {
	shortKey := strings.TrimPrefix(r.URL.Path, "/short/")
	if shortKey == "" {
		fmt.Fprint(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>URL Shortener</title>
		</head>
		<body>
			<h2> <a href="`, homeURL, `">`, "Return to homepage", `</a> </h2>
		</body>
		</html>
		`)
		http.Error(w, "Shortened key is missing", http.StatusBadRequest)
		return
	}

	// Retrieve the original URL from the `urls` map using the shortened key
	originalURL, found := urls[shortKey]
	if !found {
		fmt.Fprint(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>URL Shortener</title>
		</head>
		<body>
			<h2> <a href="`, homeURL, `">`, "Return to homepage", `</a> </h2>
		</body>
		</html>
		`)
		http.Error(w, "Shortened key not found", http.StatusNotFound)
		return
	}

	// Redirect the user to the original URL
	http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
}

// Can change this up. Maybe the length can be randomized from a certain length to another. Ex: 5-8 characters long.
func generateShortKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const keyLength = 4
	const backTruncAmount = 1000
	const frontTruncAmount = 1000000000
	curTime := time.Now().UnixMilli()
	frontTruncTime := curTime / frontTruncAmount
	backTruncTime := curTime % backTruncAmount
	newRand := rand.New(rand.NewSource(curTime))
	shortKey := make([]byte, keyLength)
	for i := range shortKey {
		shortKey[i] = charset[newRand.Intn(len(charset))]
	}
	return strconv.FormatInt(frontTruncTime, 10) + strconv.FormatInt(backTruncTime, 10) + string(shortKey)
}

func isUrl(testUrl string) bool {
	u, err := url.ParseRequestURI(testUrl)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func isReachable(testUrl string) bool {
	resp, err := http.Get(testUrl)
	if err != nil {
		fmt.Println(err)
		return false
	} else {
		fmt.Printf("\nStatus code is: %d \nStatus is: %s", resp.StatusCode, resp.Status)
		return true
	}
}
