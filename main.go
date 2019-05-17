package main

import (
	"fmt"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

var (
	authConfig spotify.Authenticator
	oauthStateString = ""
)

func main() {
	rand.Seed(time.Now().UnixNano())
	http.HandleFunc("/", handleMain)
	http.HandleFunc("/login", handleSpotifyLogin)
	http.HandleFunc("/callback", handleSpotifyCallback)
	http.HandleFunc("/generate_playlist", handleGenerateFollowersPlaylist)
	fmt.Println("Listening on 0.0.0.0:7777")
	http.ListenAndServe("0.0.0.0:7777", nil)
}

func init() {
	oauthStateString = generateRandomString(10)
	clientId := "f0f0b67da33f4f91905518aff025da5e"
	clientSecret := "b2a1683f60ca435ea536b0144100ff89"
	scopes := "user-read-private user-read-email user-follow-read playlist-read-private playlist-modify-private playlist-modify-public"
	authConfig = spotify.NewAuthenticator("http://localhost:7777/callback", scopes)
	authConfig.SetAuthInfo(clientId, clientSecret)
}

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
		fmt.Fprintf(w, "oauth state string does not match")
	}

	token, err := authConfig.Exchange(r.FormValue("code"))
	if err != nil {
		fmt.Fprintf(w, "code exchange failed: %s", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:       "access_token",
		Value:      token.AccessToken,
		Expires:    time.Now().Add(time.Hour * time.Duration(1)),
	})

	http.ServeFile(w, r, "./public/home.html")
}

func handleGenerateFollowersPlaylist(w http.ResponseWriter, r *http.Request) {
	accessToken, err := r.Cookie("access_token")
	if err != nil || accessToken == nil || accessToken.Value == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	client := authConfig.NewClient(&oauth2.Token{AccessToken: accessToken.Value})
	client.AutoRetry = true

	user, err := client.CurrentUser()
	if err != nil {
		fmt.Fprintf(w, "failed to get current user: %s", err)
		return
	}

	var followedArtists []spotify.FullArtist
	after := 0
	for len(followedArtists) < 10000 {

		f, err := client.CurrentUsersFollowedArtistsOpt(50, strconv.Itoa(after))
		if err != nil {
			fmt.Fprintf(w, "failed to get followed arists: %s", err)
			return
		}

		followedArtists = append(followedArtists, f.Artists...)
		after = after + 50
		if len(f.Artists) < 50 {
			break
		}
	}

	var playlistTracks []spotify.FullTrack
	for _, artist := range followedArtists {
		tracks, err := client.GetArtistsTopTracks(artist.ID, "from_token")
		if err != nil {
			fmt.Fprintf(w, "failed to get artists top tracks: %s", err)
			return
		}
		playlistTracks = append(playlistTracks, tracks...)
	}

	playlistName := "foo"
	playlistDescription := "bar"
	playlist, err := client.CreatePlaylistForUser(user.ID, playlistName, playlistDescription, false)

	var trackIDs []spotify.ID
	for _, track := range playlistTracks {
		trackIDs = append(trackIDs, track.ID)
	}

	batchSize := 100
	for i := 0; i < len(trackIDs); i += batchSize {
		j := i + batchSize
		if j > len(trackIDs) {
			j = len(trackIDs)
		}

		_, err = client.AddTracksToPlaylist(playlist.ID, trackIDs[i:j]...)
		if err != nil {
			fmt.Fprintf(w, "failed to add tracks to playlist: %s", err)
			return
		}
	}

	fmt.Fprintf(w, "success")
}

// Helpers
func generateRandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
