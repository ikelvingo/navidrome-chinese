package netease

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("client", func() {
	var httpClient *tests.FakeHttpClient
	var client *client

	BeforeEach(func() {
		httpClient = &tests.FakeHttpClient{}
		client = newClient(httpClient, []string{"https://api.test.com"}, LoadBalanceModeRandom)
	})

	Describe("getBaseURL", func() {
		Context("with multiple URLs", func() {
			BeforeEach(func() {
				client = newClient(httpClient, []string{
					"https://api1.test.com",
					"https://api2.test.com",
					"https://api3.test.com",
				}, LoadBalanceModeRandom)
			})

			It("returns one of the URLs in random mode", func() {
				url := client.getBaseURL()
				Expect(url).To(Or(
					Equal("https://api1.test.com"),
					Equal("https://api2.test.com"),
					Equal("https://api3.test.com"),
				))
			})

			It("cycles through URLs in round-robin mode", func() {
				client.loadBalanceMode = LoadBalanceModeRoundRobin
				url1 := client.getBaseURL()
				url2 := client.getBaseURL()
				url3 := client.getBaseURL()
				url4 := client.getBaseURL()

				// Should cycle through all URLs
				Expect(url1).To(Equal("https://api1.test.com"))
				Expect(url2).To(Equal("https://api2.test.com"))
				Expect(url3).To(Equal("https://api3.test.com"))
				Expect(url4).To(Equal("https://api1.test.com"))
			})
		})

		Context("with single URL", func() {
			BeforeEach(func() {
				client = newClient(httpClient, []string{"https://single.test.com"}, LoadBalanceModeRandom)
			})

			It("returns the single URL", func() {
				url := client.getBaseURL()
				Expect(url).To(Equal("https://single.test.com"))
			})
		})

		Context("with empty URLs", func() {
			BeforeEach(func() {
				client = newClient(httpClient, []string{}, LoadBalanceModeRandom)
			})

			It("returns default URL", func() {
				url := client.getBaseURL()
				Expect(url).To(Equal(defaultAPIBaseURLs[0]))
			})
		})
	})

	Describe("searchArtists", func() {
		It("returns artists on successful response", func() {
			f, _ := os.Open("tests/fixtures/netease.search.artists.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			artists, err := client.searchArtists(context.Background(), "å¨æ°ï¿?, 5)
			Expect(err).To(BeNil())
			Expect(len(artists)).To(Equal(2))
			Expect(artists[0].Name).To(Equal("å¨æ°ï¿?))
			Expect(artists[0].ID).To(Equal(6452))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("keywords=å¨æ°ï¿?))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("type=100"))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("limit=5"))
		})

		It("returns ErrNotFound when no artists found", func() {
			f, _ := os.Open("tests/fixtures/netease.search.empty.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := client.searchArtists(context.Background(), "UnknownArtist", 5)
			Expect(err).To(MatchError(ErrNotFound))
		})

		It("returns ErrInvalidCode when API returns error code", func() {
			httpClient.Res = http.Response{
				Body:       io.NopCloser(bytes.NewBufferString(`{"code":400,"message":"Bad Request"}`)),
				StatusCode: 200,
			}

			_, err := client.searchArtists(context.Background(), "å¨æ°ï¿?, 5)
			Expect(err).To(MatchError("netease: invalid response code: code 400"))
		})

		It("returns ErrAPIError on HTTP error", func() {
			httpClient.Res = http.Response{
				Body:       io.NopCloser(bytes.NewBufferString(`Internal Server Error`)),
				StatusCode: 500,
			}

			_, err := client.searchArtists(context.Background(), "å¨æ°ï¿?, 5)
			Expect(err).To(MatchError("netease: api error: http status 500"))
		})

		It("fails if HttpClient.Do() returns error", func() {
			httpClient.Err = errors.New("network error")

			_, err := client.searchArtists(context.Background(), "å¨æ°ï¿?, 5)
			Expect(err).To(MatchError("network error"))
		})
	})

	Describe("searchSongs", func() {
		It("returns songs on successful response", func() {
			f, _ := os.Open("tests/fixtures/netease.search.songs.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			songs, err := client.searchSongs(context.Background(), "ä¸éï¿?, 5)
			Expect(err).To(BeNil())
			Expect(len(songs)).To(Equal(2))
			Expect(songs[0].Name).To(Equal("ä¸éï¿?))
			Expect(songs[0].ID).To(Equal(1859245776))
			Expect(songs[0].Artists[0].Name).To(Equal("å¨æ°ï¿?))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("keywords=ä¸éï¿?))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("type=1"))
		})

		It("returns ErrNotFound when no songs found", func() {
			f, _ := os.Open("tests/fixtures/netease.search.empty.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := client.searchSongs(context.Background(), "UnknownSong", 5)
			Expect(err).To(MatchError(ErrNotFound))
		})
	})

	Describe("searchAlbums", func() {
		It("returns albums on successful response", func() {
			f, _ := os.Open("tests/fixtures/netease.search.albums.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			albums, err := client.searchAlbums(context.Background(), "ä¸éï¿?, 5)
			Expect(err).To(BeNil())
			Expect(len(albums)).To(Equal(2))
			Expect(albums[0].Name).To(Equal("ä¸éï¿?))
			Expect(albums[0].ID).To(Equal(16947))
			Expect(albums[0].Artist.Name).To(Equal("å¨æ°ï¿?))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("keywords=ä¸éï¿?))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("type=10"))
		})

		It("returns ErrNotFound when no albums found", func() {
			f, _ := os.Open("tests/fixtures/netease.search.empty.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := client.searchAlbums(context.Background(), "UnknownAlbum", 5)
			Expect(err).To(MatchError(ErrNotFound))
		})
	})

	Describe("getArtistDesc", func() {
		It("returns artist description on successful response", func() {
			f, _ := os.Open("tests/fixtures/netease.artist.desc.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			desc, err := client.getArtistDesc(context.Background(), 6452)
			Expect(err).To(BeNil())
			Expect(desc.Code).To(Equal(200))
			Expect(desc.BriefDesc).To(ContainSubstring("å¨æ°ï¿?))
			Expect(len(desc.Introduction)).To(BeNumerically(">", 0))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("id=6452"))
		})

		It("returns ErrInvalidCode when API returns error code", func() {
			httpClient.Res = http.Response{
				Body:       io.NopCloser(bytes.NewBufferString(`{"code":400,"message":"Bad Request"}`)),
				StatusCode: 200,
			}

			_, err := client.getArtistDesc(context.Background(), 6452)
			Expect(err).To(MatchError("netease: invalid response code: code 400"))
		})
	})

	Describe("getArtistTopSongs", func() {
		It("returns artist top songs on successful response", func() {
			f, _ := os.Open("tests/fixtures/netease.artist.topsongs.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			topSongs, err := client.getArtistTopSongs(context.Background(), 6452)
			Expect(err).To(BeNil())
			Expect(topSongs.Code).To(Equal(200))
			Expect(len(topSongs.Songs)).To(BeNumerically(">", 0))
			Expect(topSongs.Songs[0].Name).To(Equal("ä¸éï¿?))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("id=6452"))
		})
	})

	Describe("getAlbumDetail", func() {
		It("returns album detail on successful response", func() {
			f, _ := os.Open("tests/fixtures/netease.album.detail.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			albumDetail, err := client.getAlbumDetail(context.Background(), 16947)
			Expect(err).To(BeNil())
			Expect(albumDetail.Code).To(Equal(200))
			Expect(albumDetail.Album.Name).To(Equal("ä¸éï¿?))
			Expect(albumDetail.Album.ID).To(Equal(16947))
			Expect(albumDetail.Album.Artist.Name).To(Equal("å¨æ°ï¿?))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("id=16947"))
		})
	})

	Describe("makeRequest", func() {
		It("returns error on non-200 HTTP status", func() {
			httpClient.Res = http.Response{
				Body:       io.NopCloser(bytes.NewBufferString(`{"code":200}`)),
				StatusCode: 404,
			}

			req, _ := http.NewRequest("GET", "https://api.test.com/test", nil)
			var result map[string]interface{}
			err := client.makeRequest(req, &result)
			Expect(err).To(MatchError("netease: api error: http status 404"))
		})

		It("returns error on invalid JSON", func() {
			httpClient.Res = http.Response{
				Body:       io.NopCloser(bytes.NewBufferString(`<xml>NOT_JSON</xml>`)),
				StatusCode: 200,
			}

			req, _ := http.NewRequest("GET", "https://api.test.com/test", nil)
			var result map[string]interface{}
			err := client.makeRequest(req, &result)
			Expect(err).To(HaveOccurred())
		})
	})
})