package netease

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/core/agents"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/cache"
)

const (
	neteaseAgentName = "netease"
)

type neteaseAgent struct {
	ds         model.DataStore
	client     *client
	httpClient httpDoer
}

func neteaseConstructor(ds model.DataStore) *neteaseAgent {
	if !conf.Server.Netease.Enabled {
		return nil
	}

	// Get API URLs from config, fallback to defaults if empty
	apiURLs := conf.Server.Netease.APIUrls
	var urls []string
	if apiURLs != "" {
		urls = strings.Split(apiURLs, ",")
		for i := range urls {
			urls[i] = strings.TrimSpace(urls[i])
		}
	}

	// Determine load balance mode
	var mode LoadBalanceMode
	switch conf.Server.Netease.LoadBalanceMode {
	case "roundrobin":
		mode = LoadBalanceModeRoundRobin
	case "random":
		fallthrough
	default:
		mode = LoadBalanceModeRandom
	}

	hc := &http.Client{
		Timeout: consts.DefaultHttpClientTimeOut,
	}
	chc := cache.NewHTTPClient(hc, consts.DefaultHttpClientTimeOut)

	return &neteaseAgent{
		ds:         ds,
		httpClient: chc,
		client:     newClient(chc, urls, mode),
	}
}

func (n *neteaseAgent) AgentName() string {
	return neteaseAgentName
}

func (n *neteaseAgent) GetArtistMBID(ctx context.Context, id string, name string) (string, error) {
	// NetEase doesn't have MBID concept, return empty
	return "", agents.ErrNotFound
}

func (n *neteaseAgent) GetArtistURL(ctx context.Context, id, name, mbid string) (string, error) {
	// Search for artist to get ID
	artists, err := n.client.searchArtists(ctx, name, 1)
	if err != nil {
		if err == ErrNotFound {
			return "", agents.ErrNotFound
		}
		log.Error(ctx, "Error searching for artist", "artist", name, err)
		return "", err
	}

	if len(artists) == 0 {
		return "", agents.ErrNotFound
	}

	// NetEase artist page URL format
	return fmt.Sprintf("https://music.163.com/#/artist?id=%d", artists[0].ID), nil
}

func (n *neteaseAgent) GetArtistBiography(ctx context.Context, id, name, mbid string) (string, error) {
	// Search for artist to get ID
	artists, err := n.client.searchArtists(ctx, name, 1)
	if err != nil {
		if err == ErrNotFound {
			return "", agents.ErrNotFound
		}
		log.Error(ctx, "Error searching for artist", "artist", name, err)
		return "", err
	}

	if len(artists) == 0 {
		return "", agents.ErrNotFound
	}

	// Get artist description
	desc, err := n.client.getArtistDesc(ctx, artists[0].ID)
	if err != nil {
		if err == ErrNotFound {
			return "", agents.ErrNotFound
		}
		log.Error(ctx, "Error getting artist description", "artist", name, "id", artists[0].ID, err)
		return "", err
	}

	// Combine brief description and introduction sections
	var bio strings.Builder
	if desc.BriefDesc != "" {
		bio.WriteString(desc.BriefDesc)
	}

	for _, intro := range desc.Introduction {
		if intro.Ti != "" {
			bio.WriteString("\n\n")
			bio.WriteString(intro.Ti)
			bio.WriteString(":\n")
		}
		if intro.Txt != "" {
			bio.WriteString(intro.Txt)
		}
	}

	result := strings.TrimSpace(bio.String())
	if result == "" {
		return "", agents.ErrNotFound
	}

	return result, nil
}

func (n *neteaseAgent) GetSimilarArtists(ctx context.Context, id, name, mbid string, limit int) ([]agents.Artist, error) {
	// According to requirements, NetEase agent doesn't support similar artists
	return nil, agents.ErrNotFound
}

func (n *neteaseAgent) GetArtistTopSongs(ctx context.Context, id, artistName, mbid string, count int) ([]agents.Song, error) {
	// Search for artist to get ID
	artists, err := n.client.searchArtists(ctx, artistName, 1)
	if err != nil {
		if err == ErrNotFound {
			return nil, agents.ErrNotFound
		}
		log.Error(ctx, "Error searching for artist", "artist", artistName, err)
		return nil, err
	}

	if len(artists) == 0 {
		return nil, agents.ErrNotFound
	}

	// Get artist top songs
	topSongs, err := n.client.getArtistTopSongs(ctx, artists[0].ID)
	if err != nil {
		if err == ErrNotFound {
			return nil, agents.ErrNotFound
		}
		log.Error(ctx, "Error getting artist top songs", "artist", artistName, "id", artists[0].ID, err)
		return nil, err
	}

	if len(topSongs.Songs) == 0 {
		return nil, agents.ErrNotFound
	}

	// Convert to agents.Song format
	songs := make([]agents.Song, 0, min(count, len(topSongs.Songs)))
	for i, song := range topSongs.Songs {
		if i >= count {
			break
		}
		artistNames := make([]string, 0, len(song.Ar))
		for _, artist := range song.Ar {
			artistNames = append(artistNames, artist.Name)
		}
		songs = append(songs, agents.Song{
			Name:   song.Name,
			Artist: strings.Join(artistNames, ", "),
		})
	}

	return songs, nil
}

func (n *neteaseAgent) GetSimilarSongsByTrack(ctx context.Context, id, name, artist, mbid string, count int) ([]agents.Song, error) {
	// NetEase doesn't have similar songs API, return empty
	return nil, agents.ErrNotFound
}

func (n *neteaseAgent) GetArtistImages(ctx context.Context, id, name, mbid string) ([]agents.ExternalImage, error) {
	// Search for artist to get ID
	artists, err := n.client.searchArtists(ctx, name, 1)
	if err != nil {
		if err == ErrNotFound {
			return nil, agents.ErrNotFound
		}
		log.Error(ctx, "Error searching for artist", "artist", name, err)
		return nil, err
	}

	if len(artists) == 0 {
		return nil, agents.ErrNotFound
	}

	artist := artists[0]
	images := make([]agents.ExternalImage, 0)

	// Handle image URL mapping (some APIs return 'cover' and 'avatar' instead of 'picUrl' and 'img1v1Url')
	picURL := artist.PicURL
	img1v1URL := artist.Img1v1URL

	// If PicURL is empty but we have other image fields, try to map them
	if picURL == "" {
		// Check if there are other image fields in the artist struct
		// Note: We might need to get artist detail for full information
		artistDetail, err := n.client.getArtistDetail(ctx, artist.ID)
		if err == nil && artistDetail.Code == 200 {
			detailArtist := artistDetail.Data.Artist
			// Map cover to picUrl if available
			if detailArtist.PicURL == "" {
				// Try to check for other possible image fields
				// Some APIs might return different field names
			}
		}
	}

	// Add main picture if available
	if picURL != "" {
		images = append(images, agents.ExternalImage{
			URL:  picURL,
			Size: 640, // Approximate size for main artist image
		})
	}

	// Add 1v1 picture if available and different from main
	if img1v1URL != "" && img1v1URL != picURL {
		images = append(images, agents.ExternalImage{
			URL:  img1v1URL,
			Size: 300, // Approximate size for 1v1 image
		})
	}

	if len(images) == 0 {
		return nil, agents.ErrNotFound
	}

	return images, nil
}

func (n *neteaseAgent) GetAlbumInfo(ctx context.Context, name, artist, mbid string) (*agents.AlbumInfo, error) {
	// Search for album to get ID
	albums, err := n.client.searchAlbums(ctx, name, 1)
	if err != nil {
		if err == ErrNotFound {
			return nil, agents.ErrNotFound
		}
		log.Error(ctx, "Error searching for album", "album", name, err)
		return nil, err
	}

	if len(albums) == 0 {
		return nil, agents.ErrNotFound
	}

	// Get album detail
	albumDetail, err := n.client.getAlbumDetail(ctx, albums[0].ID)
	if err != nil {
		if err == ErrNotFound {
			return nil, agents.ErrNotFound
		}
		log.Error(ctx, "Error getting album detail", "album", name, "id", albums[0].ID, err)
		return nil, err
	}

	album := albumDetail.Album
	info := &agents.AlbumInfo{
		Name:        album.Name,
		Description: album.Description,
		URL:         fmt.Sprintf("https://music.163.com/#/album?id=%d", album.ID),
	}

	return info, nil
}

func (n *neteaseAgent) GetAlbumImages(ctx context.Context, name, artist, mbid string) ([]agents.ExternalImage, error) {
	// Search for album to get ID
	albums, err := n.client.searchAlbums(ctx, name, 1)
	if err != nil {
		if err == ErrNotFound {
			return nil, agents.ErrNotFound
		}
		log.Error(ctx, "Error searching for album", "album", name, err)
		return nil, err
	}

	if len(albums) == 0 {
		return nil, agents.ErrNotFound
	}

	album := albums[0]
	images := make([]agents.ExternalImage, 0)

	// Add album cover if available
	if album.PicURL != "" {
		images = append(images, agents.ExternalImage{
			URL:  album.PicURL,
			Size: 640, // Approximate size for album cover
		})
	}

	// Add blur cover if available and different from main
	if album.BlurPicURL != "" && album.BlurPicURL != album.PicURL {
		images = append(images, agents.ExternalImage{
			URL:  album.BlurPicURL,
			Size: 300, // Approximate size for blur cover
		})
	}

	if len(images) == 0 {
		return nil, agents.ErrNotFound
	}

	return images, nil
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	conf.AddHook(func() {
		agents.Register(neteaseAgentName, func(ds model.DataStore) agents.Interface {
			a := neteaseConstructor(ds)
			if a != nil {
				return a
			}
			return nil
		})
	})
}
