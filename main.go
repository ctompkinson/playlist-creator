package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

var (
	authConfig       spotify.Authenticator
	oauthStateString = ""
)

type statusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func main() {
	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/", handleMain)
	http.HandleFunc("/login", handleSpotifyLogin)
	http.HandleFunc("/callback", handleSpotifyCallback)
	http.HandleFunc("/generate_playlist", handleGenerateFollowersPlaylist)
	http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))

	fmt.Println("Listening on 0.0.0.0:80")
	http.ListenAndServe("0.0.0.0:80", nil)
}

func init() {
	oauthStateString = generateRandomString(10)
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	redirectUri := os.Getenv("REDIRECT_URI")
	scopes := "user-read-private user-read-email user-follow-read playlist-read-private playlist-modify-private playlist-modify-public"
	authConfig = spotify.NewAuthenticator(redirectUri, scopes)
	authConfig.SetAuthInfo(clientID, clientSecret)
}

// Http handlers

func handleMain(w http.ResponseWriter, r *http.Request) {
	_, err := r.Cookie("access_token")
	if err != nil {
		http.ServeFile(w, r, "./public/login.html")
		return
	}

	http.ServeFile(w, r, "./public/home.html")
}

func handleSpotifyLogin(w http.ResponseWriter, r *http.Request) {
	url := authConfig.AuthURL(oauthStateString)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleSpotifyCallback(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("state") != oauthStateString {
		logrus.Info("oauth state string does not match")
		sendStatusResponse(
			statusResponse{"error", "oauth state string does not match"},
			http.StatusUnauthorized,
			w,
		)
		return
	}

	token, err := authConfig.Exchange(r.FormValue("code"))
	if err != nil {
		logrus.Errorf("code exchange failed: %s", err)
		sendStatusResponse(
			statusResponse{"error", "code exchange failed"},
			http.StatusUnauthorized,
			w,
		)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "access_token",
		Value:   token.AccessToken,
		Expires: time.Now().Add(time.Hour * time.Duration(1)),
	})

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func handleGenerateFollowersPlaylist(w http.ResponseWriter, r *http.Request) {
	reqValues, err := parseJsonFormResponse(r)
	if err != nil {
		logrus.Errorf("failed to parse form: %s", err)
		sendStatusResponse(
			statusResponse{"error", "failed to parse form"},
			http.StatusInternalServerError,
			w,
		)
		return
	}

	maxTracks, err := strconv.ParseInt(reqValues["max_tracks"], 10, 64)
	if err != nil {
		logrus.Errorf("failed to parse max tracks: %s", err)
		sendStatusResponse(
			statusResponse{"error", "failed to parse max tracks"},
			http.StatusInternalServerError,
			w,
		)
		return
	}

	accessToken, err := r.Cookie("access_token")
	if err != nil || accessToken == nil || accessToken.Value == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	client := authConfig.NewClient(&oauth2.Token{AccessToken: accessToken.Value})
	client.AutoRetry = true

	user, err := client.CurrentUser()
	if err != nil {
		logrus.Errorf("failed to get current user: %s", err)
		sendStatusResponse(
			statusResponse{"error", "failed to get current user"},
			http.StatusInternalServerError,
			w,
		)
		return
	}

	followedArtists, err := getFollowedArtists(client)
	if err != nil {
		logrus.Errorf("failed to get followed artists: %s", err)
		sendStatusResponse(
			statusResponse{"error", "failed to get followed artists"},
			http.StatusInternalServerError,
			w,
		)
		return
	}

	playlistTracks, err := getTopTracksForArists(followedArtists, maxTracks, client)
	if err != nil {
		logrus.Errorf("failed to get artists top tracks: %s", err)
		sendStatusResponse(
			statusResponse{"error", "failed to get artists top tracks"},
			http.StatusInternalServerError,
			w,
		)
		return
	}

	playlistName := reqValues["playlist_name"]
	playlistDescription := reqValues["playlist_description"]
	playlist, err := client.CreatePlaylistForUser(user.ID, playlistName, playlistDescription, false)

	var trackIDs []spotify.ID
	for _, track := range playlistTracks {
		trackIDs = append(trackIDs, track.ID)
	}

	if reqValues[""] == "on" {
		ShuffleTrackIds(trackIDs)
	}

	err = addTracksToPlaylist(trackIDs, playlist, client)
	if err != nil {
		logrus.Errorf("failed to add tracks to playlist: %s", err)
		sendStatusResponse(
			statusResponse{"error", "failed to add tracks to playlist"},
			http.StatusInternalServerError,
			w,
		)
		return
	}

	sendStatusResponse(
		statusResponse{"success", "playlist was successfully created"},
		http.StatusOK,
		w,
	)
}

// Spotify Functions

func addTracksToPlaylist(trackIDs []spotify.ID, playlist *spotify.FullPlaylist, client spotify.Client, ) error {
	batchSize := 100
	for i := 0; i < len(trackIDs); i += batchSize {
		j := i + batchSize
		if j > len(trackIDs) {
			j = len(trackIDs)
		}

		_, err := client.AddTracksToPlaylist(playlist.ID, trackIDs[i:j]...)
		if err != nil {
			return err
		}
	}

	return nil
}

func getTopTracksForArists(artists []spotify.FullArtist, maxTracks int64, client spotify.Client) ([]spotify.FullTrack, error) {
	var playlistTracks []spotify.FullTrack
	for _, artist := range artists {
		tracks, err := client.GetArtistsTopTracks(artist.ID, "from_token")
		if err != nil {
			return nil, err
		}

		if len(tracks) > int(maxTracks) {
			tracks = tracks[:maxTracks]
		}
		playlistTracks = append(playlistTracks, tracks...)
	}
	return playlistTracks, nil
}

func getFollowedArtists(client spotify.Client) ([]spotify.FullArtist, error) {
	var followedArtists []spotify.FullArtist
	after := ""
	for len(followedArtists) < 10000 {
		f, err := client.CurrentUsersFollowedArtistsOpt(50, after)
		if err != nil {
			return nil, err
		}

		followedArtists = append(followedArtists, f.Artists...)
		after = f.Cursor.After
		if len(f.Artists) < 50 {
			break
		}
	}
	return followedArtists, nil
}

func parseJsonFormResponse(r *http.Request) (map[string]string, error) {
	decoder := json.NewDecoder(r.Body)
	var v []map[string]string
	err := decoder.Decode(&v)
	if err != nil {
		panic(err)
	}

	reqValues := make(map[string]string)
	for _, pair := range v {
		reqValues[pair["name"]] = pair["value"]
	}

	return reqValues, err
}

// Helpers

func sendStatusResponse(status statusResponse, responseCode int, w http.ResponseWriter) {
	b, _ := json.Marshal(status)
	http.Error(w, string(b), responseCode)
}

func generateRandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func ShuffleTrackIds(vals []spotify.ID) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for len(vals) > 0 {
		n := len(vals)
		randIndex := r.Intn(n)
		vals[n-1], vals[randIndex] = vals[randIndex], vals[n-1]
		vals = vals[:n-1]
	}
}
