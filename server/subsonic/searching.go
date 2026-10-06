package subsonic

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/sanitize"
	"github.com/navidrome/navidrome/core/publicurl"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/subsonic/responses"
	"github.com/navidrome/navidrome/utils/opencc"
	"github.com/navidrome/navidrome/utils/req"
	"github.com/navidrome/navidrome/utils/slice"
	"golang.org/x/sync/errgroup"
)

type searchParams struct {
	query        string
	artistCount  int
	artistOffset int
	albumCount   int
	albumOffset  int
	songCount    int
	songOffset   int
}

func (api *Router) getSearchParams(r *http.Request) (*searchParams, error) {
	p := req.Params(r)
	sp := &searchParams{}
	sp.query = p.StringOr("query", `""`)
	sp.artistCount = p.IntOr("artistCount", 20)
	sp.artistOffset = p.IntOr("artistOffset", 0)
	sp.albumCount = p.IntOr("albumCount", 20)
	sp.albumOffset = p.IntOr("albumOffset", 0)
	sp.songCount = p.IntOr("songCount", 20)
	sp.songOffset = p.IntOr("songOffset", 0)
	return sp, nil
}

type searchFunc[T any] func(q string, options ...model.QueryOptions) (T, error)

func callSearch[T any](ctx context.Context, s searchFunc[T], q string, options model.QueryOptions, result *T) func() error {
	return func() error {
		if options.Max == 0 {
			return nil
		}
		typ := strings.TrimPrefix(reflect.TypeOf(*result).String(), "model.")
		var err error
		start := time.Now()
		*result, err = s(q, options)
		if err != nil {
			log.Error(ctx, "Error searching "+typ, "query", q, "elapsed", time.Since(start), err)
		} else {
			log.Trace(ctx, "Search for "+typ+" completed", "query", q, "elapsed", time.Since(start))
		}
		return nil
	}
}

// searchMediaFilesWithVariants 使用多个查询变体搜索MediaFiles，用于中英文繁简体统一搜索
func searchMediaFilesWithVariants(
	ctx context.Context,
	searchFn func(q string, options ...model.QueryOptions) (model.MediaFiles, error),
	queries []string,
	options model.QueryOptions,
) model.MediaFiles {
	if len(queries) == 1 {
		result, _ := searchFn(queries[0], options)
		return result
	}

	var results []model.MediaFiles
	var mu sync.Mutex

	g, _ := errgroup.WithContext(ctx)
	for _, q := range queries {
		g.Go(func(query string) func() error {
			return func() error {
				log.Debug(ctx, "Searching MediaFiles with variant", "query", query)
				result, err := searchFn(query, options)
				if err == nil {
					mu.Lock()
					results = append(results, result)
					mu.Unlock()
					log.Debug(ctx, "MediaFiles search variant completed", "query", query, "count", len(result))
				} else {
					log.Error(ctx, "MediaFiles search variant failed", "query", query, err)
				}
				return nil
			}
		}(q))
	}
	g.Wait()

	merged := mergeMediaFiles(results)
	log.Debug(ctx, "MediaFiles merged results", "totalVariants", len(results), "mergedCount", len(merged))
	return merged
}

// searchAlbumsWithVariants 使用多个查询变体搜索Albums，用于中英文繁简体统一搜索
func searchAlbumsWithVariants(
	ctx context.Context,
	searchFn func(q string, options ...model.QueryOptions) (model.Albums, error),
	queries []string,
	options model.QueryOptions,
) model.Albums {
	if len(queries) == 1 {
		result, _ := searchFn(queries[0], options)
		return result
	}

	var results []model.Albums
	var mu sync.Mutex

	g, _ := errgroup.WithContext(ctx)
	for _, q := range queries {
		g.Go(func(query string) func() error {
			return func() error {
				result, err := searchFn(query, options)
				if err == nil {
					mu.Lock()
					results = append(results, result)
					mu.Unlock()
				}
				return nil
			}
		}(q))
	}
	g.Wait()

	return mergeAlbums(results)
}

// searchArtistsWithVariants 使用多个查询变体搜索Artists，用于中英文繁简体统一搜索
func searchArtistsWithVariants(
	ctx context.Context,
	searchFn func(q string, options ...model.QueryOptions) (model.Artists, error),
	queries []string,
	options model.QueryOptions,
) model.Artists {
	if len(queries) == 1 {
		result, _ := searchFn(queries[0], options)
		return result
	}

	var results []model.Artists
	var mu sync.Mutex

	g, _ := errgroup.WithContext(ctx)
	for _, q := range queries {
		g.Go(func(query string) func() error {
			return func() error {
				result, err := searchFn(query, options)
				if err == nil {
					mu.Lock()
					results = append(results, result)
					mu.Unlock()
				}
				return nil
			}
		}(q))
	}
	g.Wait()

	return mergeArtists(results)
}

func mergeMediaFiles(results []model.MediaFiles) model.MediaFiles {
	if len(results) == 0 {
		return model.MediaFiles{}
	}
	if len(results) == 1 {
		return results[0]
	}

	seen := make(map[string]bool)
	var merged model.MediaFiles
	for _, r := range results {
		for _, item := range r {
			if !seen[item.ID] {
				seen[item.ID] = true
				merged = append(merged, item)
			}
		}
	}
	return merged
}

func mergeAlbums(results []model.Albums) model.Albums {
	if len(results) == 0 {
		return model.Albums{}
	}
	if len(results) == 1 {
		return results[0]
	}

	seen := make(map[string]bool)
	var merged model.Albums
	for _, r := range results {
		for _, item := range r {
			if !seen[item.ID] {
				seen[item.ID] = true
				merged = append(merged, item)
			}
		}
	}
	return merged
}

func mergeArtists(results []model.Artists) model.Artists {
	if len(results) == 0 {
		return model.Artists{}
	}
	if len(results) == 1 {
		return results[0]
	}

	seen := make(map[string]bool)
	var merged model.Artists
	for _, r := range results {
		for _, item := range r {
			if !seen[item.ID] {
				seen[item.ID] = true
				merged = append(merged, item)
			}
		}
	}
	return merged
}

func (api *Router) searchAll(ctx context.Context, sp *searchParams, musicFolderIds []int) (mediaFiles model.MediaFiles, albums model.Albums, artists model.Artists) {
	start := time.Now()
	q := sanitize.Accents(strings.ToLower(strings.TrimSuffix(sp.query, "*")))

	queries := opencc.GetSearchQueries(q)
	if len(queries) > 1 {
		log.Debug(ctx, "Chinese query conversion", "original", q, "variants", queries)
	}

	// Build options with offset/size/filters packed in
	songOpts := model.QueryOptions{Max: sp.songCount, Offset: sp.songOffset}
	albumOpts := model.QueryOptions{Max: sp.albumCount, Offset: sp.albumOffset}
	artistOpts := model.QueryOptions{Max: sp.artistCount, Offset: sp.artistOffset}

	if len(musicFolderIds) > 0 {
		songOpts.Filters = Eq{"library_id": musicFolderIds}
		albumOpts.Filters = Eq{"library_id": musicFolderIds}
		artistOpts.Filters = Eq{"library_id": musicFolderIds}
	}

	// Run searches in parallel with variants support
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		mediaFiles = searchMediaFilesWithVariants(ctx, api.ds.MediaFile(ctx).Search, queries, songOpts)
		return nil
	})
	g.Go(func() error {
		albums = searchAlbumsWithVariants(ctx, api.ds.Album(ctx).Search, queries, albumOpts)
		return nil
	})
	g.Go(func() error {
		artists = searchArtistsWithVariants(ctx, api.ds.Artist(ctx).Search, queries, artistOpts)
		return nil
	})
	err := g.Wait()
	if err == nil {
		log.Debug(ctx, fmt.Sprintf("Search resulted in %d songs, %d albums and %d artists",
			len(mediaFiles), len(albums), len(artists)), "query", sp.query, "elapsedTime", time.Since(start))
	} else {
		log.Warn(ctx, "Search was interrupted", "query", sp.query, "elapsedTime", time.Since(start), err)
	}
	return mediaFiles, albums, artists
}

func (api *Router) Search2(r *http.Request) (*responses.Subsonic, error) {
	ctx := r.Context()
	sp, err := api.getSearchParams(r)
	if err != nil {
		return nil, err
	}

	// Get optional library IDs from musicFolderId parameter
	musicFolderIds, err := selectedMusicFolderIds(r, false)
	if err != nil {
		return nil, err
	}
	mfs, als, as := api.searchAll(ctx, sp, musicFolderIds)

	response := newResponse()
	searchResult2 := &responses.SearchResult2{}
	searchResult2.Artist = slice.Map(as, func(artist model.Artist) responses.Artist {
		a := responses.Artist{
			Id:         artist.ID,
			Name:       artist.Name,
			UserRating: int32(artist.Rating),
			CoverArt:   coverArtOrEmpty(artist.CoverArtID(), artist.ImageAbsent),
		}
		if !artist.ImageAbsent {
			a.ArtistImageUrl = publicurl.ImageURL(r.Context(), artist.CoverArtID(), 600)
		}
		if artist.Starred {
			a.Starred = artist.StarredAt
		}
		return a
	})
	searchResult2.Album = slice.MapWithArg(als, ctx, childFromAlbum)
	searchResult2.Song = slice.MapWithArg(mfs, ctx, childFromMediaFile)
	response.SearchResult2 = searchResult2
	return response, nil
}

func (api *Router) Search3(r *http.Request) (*responses.Subsonic, error) {
	ctx := r.Context()
	sp, err := api.getSearchParams(r)
	if err != nil {
		return nil, err
	}

	// Get optional library IDs from musicFolderId parameter
	musicFolderIds, err := selectedMusicFolderIds(r, false)
	if err != nil {
		return nil, err
	}
	mfs, als, as := api.searchAll(ctx, sp, musicFolderIds)

	response := newResponse()
	searchResult3 := &responses.SearchResult3{}
	searchResult3.Artist = slice.MapWithArg(as, r, toArtistID3)
	searchResult3.Album = slice.MapWithArg(als, ctx, buildAlbumID3)
	searchResult3.Song = slice.MapWithArg(mfs, ctx, childFromMediaFile)
	response.SearchResult3 = searchResult3
	return response, nil
}
