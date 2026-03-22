package netease

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/conf/configtest"
	"github.com/navidrome/navidrome/core/agents"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("neteaseAgent", func() {
	var ds model.DataStore
	var ctx context.Context
	BeforeEach(func() {
		ds = &tests.MockDataStore{}
		ctx = context.Background()
		DeferCleanup(configtest.SetupConfig())
		conf.Server.Netease.Enabled = true
		conf.Server.Netease.ApiURLs = ""
		conf.Server.Netease.LoadBalanceMode = "random"
	})

	Describe("neteaseConstructor", func() {
		When("Agent is properly configured", func() {
			It("creates agent with default settings", func() {
				agent := neteaseConstructor(ds)
				Expect(agent).ToNot(BeNil())
				Expect(agent.AgentName()).To(Equal("netease"))
			})

			It("creates agent with custom API URLs", func() {
				conf.Server.Netease.ApiURLs = "https://custom1.com,https://custom2.com"
				agent := neteaseConstructor(ds)
				Expect(agent).ToNot(BeNil())
			})

			It("creates agent with round-robin mode", func() {
				conf.Server.Netease.LoadBalanceMode = "roundrobin"
				agent := neteaseConstructor(ds)
				Expect(agent).ToNot(BeNil())
			})
		})

		When("Agent is disabled", func() {
			It("returns nil", func() {
				conf.Server.Netease.Enabled = false
				Expect(neteaseConstructor(ds)).To(BeNil())
			})
		})
	})

	Describe("GetArtistURL", func() {
		var agent *neteaseAgent
		var httpClient *tests.FakeHttpClient
		BeforeEach(func() {
			httpClient = &tests.FakeHttpClient{}
			client := newClient(httpClient, []string{"https://api.test.com"}, LoadBalanceModeRandom)
			agent = neteaseConstructor(ds)
			agent.client = client
		})

		It("returns artist URL on successful search", func() {
			f, _ := os.Open("tests/fixtures/netease.search.artists.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			url, err := agent.GetArtistURL(ctx, "123", "åšæ°ï¿?, "")
			Expect(err).To(BeNil())
			Expect(url).To(Equal("https://music.163.com/#/artist?id=6452"))
			Expect(httpClient.RequestCount).To(Equal(1))
			Expect(httpClient.SavedRequest.URL.String()).To(ContainSubstring("keywords=åšæ°ï¿?))
		})

		It("returns ErrNotFound when artist not found", func() {
			f, _ := os.Open("tests/fixtures/netease.search.empty.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := agent.GetArtistURL(ctx, "123", "UnknownArtist", "")
			Expect(err).To(MatchError(agents.ErrNotFound))
		})

		It("returns error on API error", func() {
			httpClient.Err = errors.New("network error")
			_, err := agent.GetArtistURL(ctx, "123", "åšæ°ï¿?, "")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetArtistBiography", func() {
		var agent *neteaseAgent
		var httpClient *tests.FakeHttpClient
		BeforeEach(func() {
			httpClient = &tests.FakeHttpClient{}
			client := newClient(httpClient, []string{"https://api.test.com"}, LoadBalanceModeRandom)
			agent = neteaseConstructor(ds)
			agent.client = client
		})

		It("returns artist biography on successful search", func() {
			// Mock artist search
			fSearch, _ := os.Open("tests/fixtures/netease.search.artists.json")
			// Mock artist description
			fDesc, _ := os.Open("tests/fixtures/netease.artist.desc.json")
			
			httpClient.Res = http.Response{Body: fSearch, StatusCode: 200}
			// First call is search, second call is get description
			httpClient.Responses = []http.Response{
				{Body: fSearch, StatusCode: 200},
				{Body: fDesc, StatusCode: 200},
			}

			bio, err := agent.GetArtistBiography(ctx, "123", "åšæ°ï¿?, "")
			Expect(err).To(BeNil())
			Expect(bio).To(ContainSubstring("åšæ°ï¿?))
			Expect(httpClient.RequestCount).To(Equal(2))
		})

		It("returns ErrNotFound when artist not found", func() {
			f, _ := os.Open("tests/fixtures/netease.search.empty.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := agent.GetArtistBiography(ctx, "123", "UnknownArtist", "")
			Expect(err).To(MatchError(agents.ErrNotFound))
		})

		It("returns ErrNotFound when description is empty", func() {
			fSearch, _ := os.Open("tests/fixtures/netease.search.artists.json")
			fEmptyDesc, _ := os.Open("tests/fixtures/netease.artist.desc.empty.json")
			
			httpClient.Responses = []http.Response{
				{Body: fSearch, StatusCode: 200},
				{Body: fEmptyDesc, StatusCode: 200},
			}

			_, err := agent.GetArtistBiography(ctx, "123", "åšæ°ï¿?, "")
			Expect(err).To(MatchError(agents.ErrNotFound))
		})
	})

	Describe("GetSimilarArtists", func() {
		var agent *neteaseAgent
		BeforeEach(func() {
			agent = neteaseConstructor(ds)
		})

		It("returns ErrNotFound as NetEase doesn't support similar artists", func() {
			similar, err := agent.GetSimilarArtists(ctx, "123", "åšæ°ï¿?, "", 5)
			Expect(err).To(MatchError(agents.ErrNotFound))
			Expect(similar).To(BeNil())
		})
	})

	Describe("GetArtistTopSongs", func() {
		var agent *neteaseAgent
		var httpClient *tests.FakeHttpClient
		BeforeEach(func() {
			httpClient = &tests.FakeHttpClient{}
			client := newClient(httpClient, []string{"https://api.test.com"}, LoadBalanceModeRandom)
			agent = neteaseConstructor(ds)
			agent.client = client
		})

		It("returns artist top songs on successful search", func() {
			// Mock artist search
			fSearch, _ := os.Open("tests/fixtures/netease.search.artists.json")
			// Mock top songs
			fTopSongs, _ := os.Open("tests/fixtures/netease.artist.topsongs.json")
			
			httpClient.Responses = []http.Response{
				{Body: fSearch, StatusCode: 200},
				{Body: fTopSongs, StatusCode: 200},
			}

			songs, err := agent.GetArtistTopSongs(ctx, "123", "åšæ°ï¿?, "", 3)
			Expect(err).To(BeNil())
			Expect(len(songs)).To(BeNumerically(">", 0))
			Expect(songs[0].Name).To(Equal("äžéï¿?))
			Expect(songs[0].Artist).To(ContainSubstring("åšæ°ï¿?))
			Expect(httpClient.RequestCount).To(Equal(2))
		})

		It("returns ErrNotFound when artist not found", func() {
			f, _ := os.Open("tests/fixtures/netease.search.empty.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := agent.GetArtistTopSongs(ctx, "123", "UnknownArtist", "", 3)
			Expect(err).To(MatchError(agents.ErrNotFound))
		})

		It("returns ErrNotFound when no top songs found", func() {
			fSearch, _ := os.Open("tests/fixtures/netease.search.artists.json")
			fEmptyTopSongs, _ := os.Open("tests/fixtures/netease.artist.topsongs.empty.json")
			
			httpClient.Responses = []http.Response{
				{Body: fSearch, StatusCode: 200},
				{Body: fEmptyTopSongs, StatusCode: 200},
			}

			_, err := agent.GetArtistTopSongs(ctx, "123", "åšæ°ï¿?, "", 3)
			Expect(err).To(MatchError(agents.ErrNotFound))
		})
	})

	Describe("GetSimilarSongsByTrack", func() {
		var agent *neteaseAgent
		BeforeEach(func() {
			agent = neteaseConstructor(ds)
		})

		It("returns ErrNotFound as NetEase doesn't support similar songs", func() {
			similar, err := agent.GetSimilarSongsByTrack(ctx, "123", "äžéï¿?, "åšæ°ï¿?, "", 5)
			Expect(err).To(MatchError(agents.ErrNotFound))
			Expect(similar).To(BeNil())
		})
	})

	Describe("GetArtistImages", func() {
		var agent *neteaseAgent
		var httpClient *tests.FakeHttpClient
		BeforeEach(func() {
			httpClient = &tests.FakeHttpClient{}
			client := newClient(httpClient, []string{"https://api.test.com"}, LoadBalanceModeRandom)
			agent = neteaseConstructor(ds)
			agent.client = client
		})

		It("returns artist images on successful search", func() {
			f, _ := os.Open("tests/fixtures/netease.search.artists.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			images, err := agent.GetArtistImages(ctx, "123", "åšæ°ï¿?, "")
			Expect(err).To(BeNil())
			Expect(len(images)).To(Equal(2))
			Expect(images[0].URL).To(Equal("https://p1.music.126.net/BbR3TuhPULMLDV0MjczI4g==/109951165588539524.jpg"))
			Expect(images[0].Size).To(Equal(640))
			Expect(images[1].URL).To(Equal("https://p1.music.126.net/BbR3TuhPULMLDV0MjczI4g==/109951165588539524.jpg?param=300y300"))
			Expect(images[1].Size).To(Equal(300))
			Expect(httpClient.RequestCount).To(Equal(1))
		})

		It("returns ErrNotFound when artist not found", func() {
			f, _ := os.Open("tests/fixtures/netease.search.empty.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := agent.GetArtistImages(ctx, "123", "UnknownArtist", "")
			Expect(err).To(MatchError(agents.ErrNotFound))
		})

		It("returns ErrNotFound when no images available", func() {
			f, _ := os.Open("tests/fixtures/netease.search.artists.noimages.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := agent.GetArtistImages(ctx, "123", "åšæ°ï¿?, "")
			Expect(err).To(MatchError(agents.ErrNotFound))
		})
	})

	Describe("GetAlbumInfo", func() {
		var agent *neteaseAgent
		var httpClient *tests.FakeHttpClient
		BeforeEach(func() {
			httpClient = &tests.FakeHttpClient{}
			client := newClient(httpClient, []string{"https://api.test.com"}, LoadBalanceModeRandom)
			agent = neteaseConstructor(ds)
			agent.client = client
		})

		It("returns album info on successful search", func() {
			// Mock album search
			fSearch, _ := os.Open("tests/fixtures/netease.search.albums.json")
			// Mock album detail
			fDetail, _ := os.Open("tests/fixtures/netease.album.detail.json")
			
			httpClient.Responses = []http.Response{
				{Body: fSearch, StatusCode: 200},
				{Body: fDetail, StatusCode: 200},
			}

			albumInfo, err := agent.GetAlbumInfo(ctx, "äžéï¿?, "åšæ°ï¿?, "")
			Expect(err).To(BeNil())
			Expect(albumInfo.Name).To(Equal("äžéï¿?))
			Expect(albumInfo.Description).To(ContainSubstring("äžéï¿?))
			Expect(albumInfo.URL).To(Equal("https://music.163.com/#/album?id=16947"))
			Expect(httpClient.RequestCount).To(Equal(2))
		})

		It("returns ErrNotFound when album not found", func() {
			f, _ := os.Open("tests/fixtures/netease.search.empty.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := agent.GetAlbumInfo(ctx, "UnknownAlbum", "UnknownArtist", "")
			Expect(err).To(MatchError(agents.ErrNotFound))
		})
	})

	Describe("GetAlbumImages", func() {
		var agent *neteaseAgent
		var httpClient *tests.FakeHttpClient
		BeforeEach(func() {
			httpClient = &tests.FakeHttpClient{}
			client := newClient(httpClient, []string{"https://api.test.com"}, LoadBalanceModeRandom)
			agent = neteaseConstructor(ds)
			agent.client = client
		})

		It("returns album images on successful search", func() {
			f, _ := os.Open("tests/fixtures/netease.search.albums.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			images, err := agent.GetAlbumImages(ctx, "äžéï¿?, "åšæ°ï¿?, "")
			Expect(err).To(BeNil())
			Expect(len(images)).To(Equal(2))
			Expect(images[0].URL).To(Equal("https://p1.music.126.net/BbR3TuhPULMLDV0MjczI4g==/109951165588539524.jpg"))
			Expect(images[0].Size).To(Equal(640))
			Expect(images[1].URL).To(Equal("https://p1.music.126.net/BbR3TuhPULMLDV0MjczI4g==/109951165588539524.jpg?param=150y150"))
			Expect(images[1].Size).To(Equal(300))
			Expect(httpClient.RequestCount).To(Equal(1))
		})

		It("returns ErrNotFound when album not found", func() {
			f, _ := os.Open("tests/fixtures/netease.search.empty.json")
			httpClient.Res = http.Response{Body: f, StatusCode: 200}

			_, err := agent.GetAlbumImages(ctx, "UnknownAlbum", "UnknownArtist", "")
			Expect(err).To(MatchError(agents.ErrNotFound))
		})
	})

	Describe("GetArtistMBID", func() {
		var agent *neteaseAgent
		BeforeEach(func() {
			agent = neteaseConstructor(ds)
		})

		It("returns ErrNotFound as NetEase doesn't have MBID", func() {
			mbid, err := agent.GetArtistMBID(ctx, "123", "åšæ°ï¿?)
			Expect(err).To(MatchError(agents.ErrNotFound))
			Expect(mbid).To(Equal(""))
		})
	})
})